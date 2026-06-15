package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

type tagsDialogPhase int

const (
	tagsPhaseGroups tagsDialogPhase = iota
	tagsPhaseChildren
)

type TagGroup struct {
	Parent   models.Tag
	Children []models.Tag
}

type TagsDialogModel struct {
	groups        []TagGroup
	phase         tagsDialogPhase
	selectedGroup int
	selectedChild int
	errorText     string
}

func NewTagsDialog() TagsDialogModel {
	return TagsDialogModel{groups: []TagGroup{}}
}

func (m TagsDialogModel) initialized() bool {
	return m.groups != nil
}

func (m *TagsDialogModel) SetTags(tags []models.Tag) {
	childrenByParent := map[int][]models.Tag{}
	var parents []models.Tag
	for _, tag := range tags {
		if tag.ParentID == 0 {
			parents = append(parents, tag)
			continue
		}
		childrenByParent[tag.ParentID] = append(childrenByParent[tag.ParentID], tag)
	}
	groups := make([]TagGroup, 0, len(parents))
	for _, parent := range parents {
		groups = append(groups, TagGroup{Parent: parent, Children: childrenByParent[parent.ID]})
	}
	m.groups = groups
	m.phase = tagsPhaseGroups
	m.selectedGroup = 0
	m.selectedChild = 0
	m.errorText = ""
}

func (m *TagsDialogModel) SetError(err error) {
	if err == nil {
		m.errorText = ""
		return
	}
	m.errorText = err.Error()
}

func (m TagsDialogModel) currentChildren() []models.Tag {
	if m.selectedGroup < 0 || m.selectedGroup >= len(m.groups) {
		return nil
	}
	return m.groups[m.selectedGroup].Children
}

func (m TagsDialogModel) phaseName() string {
	if m.phase == tagsPhaseChildren {
		return "二级标签"
	}
	return "标签分类"
}

func (m *TagsDialogModel) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "left", "h", "backspace":
		if m.phase == tagsPhaseChildren {
			m.phase = tagsPhaseGroups
		}
	case "up", "k":
		if m.phase == tagsPhaseChildren {
			if m.selectedChild > 0 {
				m.selectedChild--
			}
		} else if m.selectedGroup > 0 {
			m.selectedGroup--
		}
	case "down", "j":
		if m.phase == tagsPhaseChildren {
			if m.selectedChild < len(m.currentChildren())-1 {
				m.selectedChild++
			}
		} else if m.selectedGroup < len(m.groups)-1 {
			m.selectedGroup++
		}
	}
}

func (m *TagsDialogModel) Enter() bool {
	if m.phase == tagsPhaseGroups {
		if len(m.currentChildren()) == 0 {
			return true
		}
		m.phase = tagsPhaseChildren
		m.selectedChild = 0
		return false
	}
	return true
}

func (m *TagsDialogModel) Back() bool {
	if m.phase == tagsPhaseChildren {
		m.phase = tagsPhaseGroups
		return true
	}
	return false
}

func (m TagsDialogModel) SelectedTag() *models.Tag {
	if m.phase == tagsPhaseGroups {
		if m.selectedGroup < 0 || m.selectedGroup >= len(m.groups) {
			return nil
		}
		if len(m.groups[m.selectedGroup].Children) == 0 {
			tag := m.groups[m.selectedGroup].Parent
			return &tag
		}
		return nil
	}
	children := m.currentChildren()
	if len(children) == 0 || m.selectedChild < 0 || m.selectedChild >= len(children) {
		return nil
	}
	tag := children[m.selectedChild]
	return &tag
}

func (m TagsDialogModel) View(width int) string {
	var b strings.Builder
	b.WriteString(vDialogTitleStyle.Render("标签筛选"))
	b.WriteString("\n\n")
	b.WriteString(vSubtitleStyle.Render(m.phaseName()))
	b.WriteString("\n\n")
	if m.errorText != "" {
		b.WriteString(vErrorStyle.Render(m.errorText))
		b.WriteString("\n\n")
	}
	if len(m.groups) == 0 {
		b.WriteString(vEmptyStyle.Render("暂无标签"))
	} else if m.phase == tagsPhaseGroups {
		for i, group := range m.groups {
			prefix := "  "
			if i == m.selectedGroup {
				prefix = "→ "
			}
			b.WriteString(fmt.Sprintf("%s%s (%d)\n", prefix, group.Parent.Name, len(group.Children)))
		}
	} else {
		group := m.groups[m.selectedGroup]
		b.WriteString(vStatLabelStyle.Render("分类: " + group.Parent.Name))
		b.WriteString("\n")
		for i, tag := range group.Children {
			prefix := "  "
			if i == m.selectedChild {
				prefix = "→ "
			}
			name := tag.Label
			if name == "" {
				name = tag.Name
			}
			b.WriteString(fmt.Sprintf("%s%s (#%d)\n", prefix, name, tag.ID))
		}
	}
	b.WriteString("\n")
	help := "Enter: 进入/应用 | c: 清除 | Esc: 关闭"
	if m.phase == tagsPhaseChildren {
		help = "Enter: 应用 | ←/Backspace: 返回 | c: 清除 | Esc: 关闭"
	}
	b.WriteString(vDialogHelpStyle.Render(help))
	return b.String()
}
