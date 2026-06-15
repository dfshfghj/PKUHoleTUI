package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type LogsDialogModel struct {
	lines   []string
	offset  int
	loading bool
	lastErr string
}

func NewLogsDialog() LogsDialogModel {
	return LogsDialogModel{}
}

func (m LogsDialogModel) initialized() bool {
	return true
}

func (m *LogsDialogModel) SetLoading(loading bool) {
	m.loading = loading
}

func (m *LogsDialogModel) SetLines(lines []string) {
	m.loading = false
	m.lastErr = ""
	m.lines = lines
	if m.offset >= len(m.lines) {
		m.offset = maxInt(0, len(m.lines)-1)
	}
}

func (m *LogsDialogModel) SetError(err error) {
	m.loading = false
	if err == nil {
		m.lastErr = ""
		return
	}
	m.lastErr = err.Error()
}

func (m *LogsDialogModel) Offset() int {
	return m.offset
}

func (m *LogsDialogModel) Lines() []string {
	return m.lines
}

func (m *LogsDialogModel) Loading() bool {
	return m.loading
}

func (m *LogsDialogModel) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		if m.offset > 0 {
			m.offset--
		}
	case "down":
		if m.offset < len(m.lines)-1 {
			m.offset++
		}
	case "pgup":
		m.offset -= 20
		if m.offset < 0 {
			m.offset = 0
		}
	case "pgdown":
		m.offset += 20
		if m.offset >= len(m.lines) {
			m.offset = len(m.lines) - 1
		}
	case "r":
		m.loading = true
		return loadLogsCmd()
	}
	return nil
}

func (m LogsDialogModel) View(width int) string {
	var b strings.Builder

	b.WriteString(vDialogTitleStyle.Render("运行日志"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(vLoadingStyle.Render("加载日志中..."))
	} else if len(m.lines) == 0 {
		b.WriteString(vEmptyStyle.Render("暂无日志"))
	} else {
		end := m.offset + 15
		if end > len(m.lines) {
			end = len(m.lines)
		}

		lineWidth := width - 20
		if lineWidth < 20 {
			lineWidth = 20
		}
		for i := m.offset; i < end; i++ {
			line := m.lines[i]
			if len(line) > lineWidth {
				line = line[:lineWidth]
			}
			b.WriteString(vLogLineStyle.Render(line))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		totalLines := len(m.lines)
		b.WriteString(vPaginationStyle.Render(
			fmt.Sprintf("日志: %d 行 | 当前: %d-%d | ↑↓/PgUp/PgDn滚动 | r: 刷新",
				totalLines, m.offset+1, minInt(m.offset+15, totalLines)),
		))
	}

	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(vErrorStyle.Render("错误: " + m.lastErr))
	}

	b.WriteString("\n")
	b.WriteString(vDialogHelpStyle.Render("Esc: 关闭"))
	return b.String()
}
