package tui

import (
	"fmt"
	"strings"
)

type ImageDialogModel struct {
	title string
	items []resolvedMedia
	index int
}

func NewImageDialog() ImageDialogModel {
	return ImageDialogModel{}
}

func (m ImageDialogModel) initialized() bool {
	return true
}

func (m *ImageDialogModel) Open(title string, items []resolvedMedia) {
	m.title = strings.TrimSpace(title)
	m.items = append([]resolvedMedia(nil), items...)
	m.index = 0
}

func (m *ImageDialogModel) Clear() {
	m.title = ""
	m.items = nil
	m.index = 0
}

func (m ImageDialogModel) Count() int {
	return len(m.items)
}

func (m ImageDialogModel) HasImages() bool {
	return len(m.items) > 0
}

func (m ImageDialogModel) Current() *resolvedMedia {
	if m.index < 0 || m.index >= len(m.items) {
		return nil
	}
	current := m.items[m.index]
	return &current
}

func (m *ImageDialogModel) Prev() {
	if len(m.items) == 0 {
		return
	}
	m.index = (m.index - 1 + len(m.items)) % len(m.items)
}

func (m *ImageDialogModel) Next() {
	if len(m.items) == 0 {
		return
	}
	m.index = (m.index + 1) % len(m.items)
}

func (m ImageDialogModel) View(width, height int, kittyEnabled bool) (string, []imagePlacement) {
	var b strings.Builder

	innerWidth := maxInt(20, width-panelContentStyle.GetHorizontalFrameSize())
	innerHeight := maxInt(8, height-panelContentStyle.GetVerticalFrameSize())

	title := "图片预览"
	if m.title != "" {
		title = m.title
	}
	b.WriteString(vDialogTitleStyle.Render(title))
	b.WriteString("\n\n")

	if !m.HasImages() {
		b.WriteString(vEmptyStyle.Render("当前没有可显示的图片"))
		b.WriteString("\n")
		b.WriteString(vDialogHelpStyle.Render("Esc: 关闭"))
		return b.String(), nil
	}

	current := m.Current()
	progress := fmt.Sprintf("%d/%d", m.index+1, len(m.items))
	b.WriteString(vPaginationStyle.Render(progress))
	b.WriteString("\n")

	imageTop := 3
	imageHeight := innerHeight - imageTop - 3
	if imageHeight < 5 {
		imageHeight = 5
	}

	if kittyEnabled && current != nil && current.path != "" {
		for i := 0; i < imageHeight; i++ {
			b.WriteString(strings.Repeat(" ", innerWidth))
			b.WriteString("\n")
		}
		placements := []imagePlacement{{
			path:        current.path,
			cols:        innerWidth,
			rows:        imageHeight,
			left:        0,
			top:         imageTop,
			z:           99,
			placeholder: true,
		}}
		b.WriteString(vStatLabelStyle.Render(current.id))
		b.WriteString("\n")
		b.WriteString(vDialogHelpStyle.Render("Left/Right: 切换图片 | Esc: 关闭"))
		return b.String(), placements
	}

	b.WriteString(vEmptyStyle.Render("当前终端不支持 kitty 图像协议"))
	b.WriteString("\n")
	if current != nil {
		b.WriteString(vStatLabelStyle.Render(current.path))
		b.WriteString("\n")
	}
	b.WriteString(vDialogHelpStyle.Render("Esc: 关闭"))
	return b.String(), nil
}
