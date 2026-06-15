package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type ComposerMode int

const (
	ComposerModePost ComposerMode = iota
	ComposerModeComment
)

type ComposerDialogModel struct {
	input       textarea.Model
	mode        ComposerMode
	title       string
	description string
	errorText   string
	quoteTarget *models.Comment
}

func NewComposerDialog() ComposerDialogModel {
	input := textarea.New()
	input.Placeholder = "输入内容"
	input.CharLimit = 2000
	input.SetWidth(50)
	input.SetHeight(6)
	input.ShowLineNumbers = false
	input.Prompt = ""
	_ = input.Focus()
	return ComposerDialogModel{input: input, title: "发布内容"}
}

func (m ComposerDialogModel) initialized() bool {
	return m.input.Width() > 0
}

func (m *ComposerDialogModel) Configure(mode ComposerMode) {
	m.mode = mode
	m.errorText = ""
	m.quoteTarget = nil
	m.input.Reset()
	m.input.Placeholder = "输入内容"
	_ = m.input.Focus()
	m.description = "支持多行输入；Enter 换行，Ctrl+S 提交"
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

func (m *ComposerDialogModel) Update(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m ComposerDialogModel) Value() string {
	return strings.TrimSpace(m.input.Value())
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

func (m ComposerDialogModel) View(width int) string {
	var b strings.Builder
	input := m.input
	input.SetWidth(maxInt(24, width-18))
	input.SetHeight(6)

	b.WriteString(vDialogTitleStyle.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(m.description)
	if preview := m.quotePreview(width - 10); preview != "" {
		b.WriteString("\n")
		b.WriteString(vCommentQuoteStyle.Render(preview))
	}
	b.WriteString("\n\n")
	b.WriteString(input.View())
	if m.errorText != "" {
		b.WriteString("\n\n")
		b.WriteString(vErrorStyle.Render(m.errorText))
	}
	b.WriteString("\n\n")
	b.WriteString(vDialogHelpStyle.Render("Enter: 换行 | Ctrl+S: 提交 | Esc: 取消"))
	return b.String()
}
