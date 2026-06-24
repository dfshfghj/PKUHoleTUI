package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ComposerMode int

const (
	ComposerModePost ComposerMode = iota
	ComposerModeComment
)

type ComposerDialogModel struct {
	input       textarea.Model
	imageInput  textarea.Model
	focusImages bool
	mode        ComposerMode
	title       string
	description string
	errorText   string
	quoteTarget *models.Comment
}

const composerPlaceholder = "输入内容"
const composerImagePlaceholder = "图片路径，可用逗号/分号/换行分隔"

func NewComposerDialog() ComposerDialogModel {
	input := textarea.New()
	input.Placeholder = composerPlaceholder
	input.CharLimit = 2000
	input.SetWidth(50)
	input.SetHeight(6)
	input.ShowLineNumbers = false
	input.Prompt = ""
	styleTextarea(&input, colorBg, colorText, colorMuted)
	_ = input.Focus()

	imageInput := textarea.New()
	imageInput.Placeholder = composerImagePlaceholder
	imageInput.CharLimit = 4000
	imageInput.SetWidth(50)
	imageInput.SetHeight(2)
	imageInput.ShowLineNumbers = false
	imageInput.Prompt = ""
	styleTextarea(&imageInput, colorBg, colorText, colorMuted)
	imageInput.Blur()

	return ComposerDialogModel{input: input, imageInput: imageInput, title: "发布内容"}
}

func (m ComposerDialogModel) initialized() bool {
	return m.input.Width() > 0 && m.imageInput.Width() > 0
}

func (m *ComposerDialogModel) Configure(mode ComposerMode) {
	m.mode = mode
	m.errorText = ""
	m.quoteTarget = nil
	m.focusImages = false
	m.input.Reset()
	m.input.Placeholder = composerPlaceholder
	styleTextarea(&m.input, colorBg, colorText, colorMuted)
	_ = m.input.Focus()
	m.imageInput.Reset()
	m.imageInput.Placeholder = composerImagePlaceholder
	styleTextarea(&m.imageInput, colorBg, colorText, colorMuted)
	m.imageInput.Blur()
	m.description = "正文支持多行；图片路径可选；Tab 切换输入框"
	if mode == ComposerModeComment {
		m.title = "发布评论"
	} else {
		m.title = "发布帖子"
	}
}

func (m *ComposerDialogModel) SetQuoteTarget(comment *models.Comment) {
	m.quoteTarget = comment
}

func (m *ComposerDialogModel) QuoteTarget() *models.Comment {
	return m.quoteTarget
}

func (m *ComposerDialogModel) SetError(err error) {
	if err == nil {
		m.errorText = ""
		return
	}
	m.errorText = err.Error()
}

func (m *ComposerDialogModel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "tab" {
		m.focusImages = !m.focusImages
		if m.focusImages {
			m.input.Blur()
			_ = m.imageInput.Focus()
		} else {
			m.imageInput.Blur()
			_ = m.input.Focus()
		}
		return nil
	}
	var cmd tea.Cmd
	if m.focusImages {
		m.imageInput, cmd = m.imageInput.Update(msg)
	} else {
		m.input, cmd = m.input.Update(msg)
	}
	return cmd
}

func (m ComposerDialogModel) Value() string {
	return strings.TrimSpace(m.input.Value())
}

func (m ComposerDialogModel) ImagePaths() []string {
	return parseComposerImagePaths(m.imageInput.Value())
}

func (m ComposerDialogModel) Mode() ComposerMode {
	return m.mode
}

func (m ComposerDialogModel) quotePreview(width int) string {
	if m.quoteTarget == nil {
		return ""
	}
	name := m.quoteTarget.NameTag
	if name == "" {
		name = "匿名"
	}
	preview := fmt.Sprintf("引用 #%d %s: %s", m.quoteTarget.Cid, name, strings.ReplaceAll(m.quoteTarget.Text, "\n", " "))
	return truncateVisibleLine(preview, width, "...")
}

func (m ComposerDialogModel) View(width, height int) string {
	var b strings.Builder

	innerWidth := maxInt(24, width-panelContentStyle.GetHorizontalFrameSize())
	inputWidth := maxInt(20, innerWidth-4)
	preview := m.quotePreview(innerWidth - 2)
	errorHeight := 0
	if m.errorText != "" {
		errorHeight = lipgloss.Height(vErrorStyle.Render(m.errorText)) + 2
	}
	previewHeight := 0
	if preview != "" {
		previewHeight = lipgloss.Height(vCommentQuoteStyle.Width(innerWidth).Render(preview)) + 1
	}
	imageInputHeight := 2
	inputHeight := height - panelContentStyle.GetVerticalFrameSize() - 14 - previewHeight - errorHeight - imageInputHeight
	inputHeight = clampInt(inputHeight, 6, 16)

	input := m.input
	input.SetWidth(inputWidth)
	input.SetHeight(inputHeight)
	imageInput := m.imageInput
	imageInput.SetWidth(inputWidth)
	imageInput.SetHeight(imageInputHeight)

	b.WriteString(vDialogTitleStyle.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(m.description)
	if preview != "" {
		b.WriteString("\n")
		b.WriteString(vCommentQuoteStyle.Width(innerWidth).Render(preview))
	}
	b.WriteString("\n\n")
	inputView := m.renderInputBlock(input, inputWidth, inputHeight, m.composerInputFocused(false))
	b.WriteString(inputView)
	b.WriteString("\n\n")
	b.WriteString(composerImageLabel(inputWidth))
	b.WriteString("\n\n")
	b.WriteString(m.renderInputBlock(imageInput, inputWidth, imageInputHeight, m.composerInputFocused(true), composerImagePlaceholder))
	if m.errorText != "" {
		b.WriteString("\n\n")
		b.WriteString(vErrorStyle.Render(m.errorText))
	}
	b.WriteString("\n\n")
	b.WriteString(vDialogHelpStyle.Render("Tab: 切换 | Enter: 换行 | Ctrl+S: 提交 | Esc: 取消"))
	return b.String()
}

func composerImageLabel(width int) string {
	return lipgloss.NewStyle().
		Foreground(colorMuted).
		Width(width).
		Render("图片")
}

func (m ComposerDialogModel) composerInputFocused(images bool) bool {
	return m.focusImages == images
}

func (m ComposerDialogModel) renderInputBlock(input textarea.Model, width, height int, focused bool, placeholder ...string) string {
	fillBg := colorBg
	if focused {
		fillBg = colorSurface
	}
	fill := lipgloss.NewStyle().Background(fillBg).Foreground(colorText)
	if input.Value() == "" {
		text := composerPlaceholder
		if len(placeholder) > 0 && placeholder[0] != "" {
			text = placeholder[0]
		}
		lines := make([]string, 0, height)
		lines = append(lines, lipgloss.NewStyle().Background(fillBg).Foreground(colorMuted).Width(width).Render(text))
		for len(lines) < height {
			lines = append(lines, fill.Width(width).Render(""))
		}
		return strings.Join(lines, "\n")
	}
	return fill.Width(width).Height(height).Render(input.View())
}

func parseComposerImagePaths(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	paths := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}
