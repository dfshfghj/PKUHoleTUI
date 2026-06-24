package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type AuthChallengeDialogModel struct {
	kind        AuthChallengeType
	title       string
	message     string
	input       textinput.Model
	focusSend   bool
	errorText   string
	statusText  string
	submitting  bool
	smsSentOnce bool
}

func NewAuthChallengeDialog(state SessionState) AuthChallengeDialogModel {
	input := textinput.New()
	input.Prompt = ""
	input.SetWidth(24)
	m := AuthChallengeDialogModel{input: input}
	m.ApplyState(state)
	return m
}

func (m AuthChallengeDialogModel) initialized() bool {
	return m.input.Width() > 0
}

func (m *AuthChallengeDialogModel) ApplyState(state SessionState) {
	if state.Challenge != m.kind {
		m.input.Reset()
		m.errorText = ""
		m.statusText = ""
		if state.Challenge != AuthChallengeTypeSMS {
			m.smsSentOnce = false
		}
	}
	m.kind = state.Challenge
	m.message = state.ChallengeMessage
	if m.message == "" {
		m.message = state.Message
	}
	switch state.Challenge {
	case AuthChallengeTypeUsername:
		m.title = "账号登录"
		m.input.Placeholder = "输入用户名"
		m.input.EchoMode = textinput.EchoNormal
		m.focusSend = false
	case AuthChallengeTypePassword:
		m.title = "密码登录"
		m.input.Placeholder = "输入用户密码"
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '*'
		m.focusSend = false
	case AuthChallengeTypeSMS:
		m.title = "短信验证"
		m.input.Placeholder = "输入短信验证码"
		m.input.EchoMode = textinput.EchoNormal
		m.focusSend = true
	case AuthChallengeTypeOTP:
		m.title = "令牌验证"
		m.input.Placeholder = "输入手机令牌"
		m.input.EchoMode = textinput.EchoNormal
		m.focusSend = false
	default:
		m.title = "认证验证"
		m.input.Placeholder = "输入验证码"
		m.input.EchoMode = textinput.EchoNormal
		m.focusSend = false
	}
	_ = m.input.Focus()
}

func (m *AuthChallengeDialogModel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if m.kind == AuthChallengeTypeSMS {
		switch msg.Code {
		case tea.KeyTab, tea.KeyUp, tea.KeyDown:
			m.focusSend = !m.focusSend
			if m.focusSend {
				m.input.Blur()
				return nil
			}
			return m.input.Focus()
		}
		if msg.String() == "shift+tab" {
			m.focusSend = !m.focusSend
			if m.focusSend {
				m.input.Blur()
				return nil
			}
			return m.input.Focus()
		}
		if m.focusSend {
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *AuthChallengeDialogModel) SetError(err error) {
	if err == nil {
		m.errorText = ""
		return
	}
	m.errorText = err.Error()
}

func (m *AuthChallengeDialogModel) SetStatus(message string) {
	m.statusText = message
}

func (m *AuthChallengeDialogModel) SetSubmitting(submitting bool) {
	m.submitting = submitting
}

func (m *AuthChallengeDialogModel) MarkSMSSent() {
	m.smsSentOnce = true
}

func (m AuthChallengeDialogModel) Value() string {
	return strings.TrimSpace(m.input.Value())
}

func (m AuthChallengeDialogModel) Kind() AuthChallengeType {
	return m.kind
}

func (m AuthChallengeDialogModel) ShouldAutoSendSMS() bool {
	return m.kind == AuthChallengeTypeSMS && !m.smsSentOnce
}

func (m AuthChallengeDialogModel) IsSendFocused() bool {
	return m.kind == AuthChallengeTypeSMS && m.focusSend
}

func (m AuthChallengeDialogModel) View(width int) string {
	var b strings.Builder
	input := m.input
	input.SetWidth(maxInt(20, width-18))

	b.WriteString(vDialogTitleStyle.Render(m.title))
	b.WriteString("\n\n")
	if m.message != "" {
		b.WriteString(m.message)
	}
	if m.kind == AuthChallengeTypeSMS {
		btn := vButtonDefault.Render("发送验证码")
		if m.focusSend {
			btn = vButtonActive.Render("发送验证码")
		}
		if m.smsSentOnce {
			btn += " " + vHelpStyle.Render("已发送，可再次确认重发")
		}
		b.WriteString(btn)
		b.WriteString("\n\n")
	}
	b.WriteString(input.View())
	if m.statusText != "" {
		b.WriteString("\n\n")
		b.WriteString(vHelpStyle.Render(m.statusText))
	}
	if m.errorText != "" {
		b.WriteString("\n\n")
		b.WriteString(vErrorStyle.Render(m.errorText))
	}
	b.WriteString("\n\n")
	if m.kind == AuthChallengeTypeSMS {
		b.WriteString(vDialogHelpStyle.Render("Tab/↑↓: 切换 | Ctrl+R: 重发验证码 | Esc: 进入离线模式"))
	} else {
		b.WriteString(vDialogHelpStyle.Render("Enter: 提交 | Esc: 进入离线模式"))
	}
	if m.submitting {
		b.WriteString("\n")
		b.WriteString(vLoadingStyle.Render("处理中..."))
	}
	return b.String()
}
