package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SessionPromptDialogModel struct {
	Title    string
	Message  string
	Options  []string
	Selected int
}

func NewSessionPromptDialog(state SessionState) SessionPromptDialogModel {
	m := SessionPromptDialogModel{}
	m.ApplyState(state)
	return m
}

func (m SessionPromptDialogModel) initialized() bool {
	return m.Options != nil
}

func (m *SessionPromptDialogModel) ApplyState(state SessionState) {
	switch state.FailureReason {
	case SessionFailureReasonLogin:
		m.Title = "登录不可用"
		m.Message = state.Message
		if m.Message == "" {
			m.Message = "当前登录态不可用。"
		}
		if state.NeedsConfig {
			m.Options = []string{"打开配置", "进入离线模式"}
		} else {
			m.Options = []string{"重新登录", "进入离线模式"}
		}
	case SessionFailureReasonNetwork:
		m.Title = "网络错误"
		m.Message = state.Message
		if m.Message == "" {
			m.Message = "当前无法连接树洞服务。"
		}
		m.Options = []string{"进入离线模式"}
	default:
		m.Title = "在线模式"
		m.Message = "在线能力可用"
		m.Options = []string{"确定"}
	}
	if m.Selected >= len(m.Options) {
		m.Selected = 0
	}
}

func (m *SessionPromptDialogModel) Update(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if m.Selected > 0 {
			m.Selected--
		}
	case "down", "j":
		if m.Selected < len(m.Options)-1 {
			m.Selected++
		}
	}
}

func (m SessionPromptDialogModel) SelectedOption() string {
	if m.Selected < 0 || m.Selected >= len(m.Options) {
		return ""
	}
	return m.Options[m.Selected]
}

func (m SessionPromptDialogModel) View(width int) string {
	var b strings.Builder
	b.WriteString(vDialogTitleStyle.Render(m.Title))
	b.WriteString("\n\n")
	b.WriteString(m.Message)
	b.WriteString("\n\n")
	for i, option := range m.Options {
		prefix := "  "
		if i == m.Selected {
			prefix = "→ "
		}
		b.WriteString(prefix + option + "\n")
	}
	b.WriteString("\n")
	b.WriteString(vDialogHelpStyle.Render("Enter: 确认 | Esc: 关闭"))
	return b.String()
}
