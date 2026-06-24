package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (m *LogsDialogModel) Update(msg tea.KeyPressMsg) tea.Cmd {
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

func (m LogsDialogModel) View(width, height int) string {
	var b strings.Builder

	innerWidth := maxInt(20, width-panelContentStyle.GetHorizontalFrameSize())
	fill := dialogBackgroundFillStyle()

	if m.loading {
		b.WriteString(fillRenderedBackground(vLoadingStyle.Render("加载日志中..."), innerWidth, fill))
	} else if len(m.lines) == 0 {
		b.WriteString(fillRenderedBackground(vEmptyStyle.Render("暂无日志"), innerWidth, fill))
	} else {
		visibleLines := maxInt(1, height-3)
		end := m.offset + visibleLines
		if end > len(m.lines) {
			end = len(m.lines)
		}

		for i := m.offset; i < end; i++ {
			line := m.lines[i]
			if lipgloss.Width(line) > innerWidth {
				line = clipToVisibleWidth(line, innerWidth)
			}
			renderedLine := vLogLineStyle.Background(colorBg).Render(line)
			b.WriteString(fillRenderedBackground(renderedLine, innerWidth, fill))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		totalLines := len(m.lines)
		pagination := vPaginationStyle.Render(
			fmt.Sprintf("日志: %d 行 | 当前: %d-%d | ↑↓/PgUp/PgDn滚动 | r: 刷新",
				totalLines, m.offset+1, minInt(end, totalLines)),
		)
		b.WriteString(fillRenderedBackground(pagination, innerWidth, fill))
	}

	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(fillRenderedBackground(vErrorStyle.Render("错误: "+m.lastErr), innerWidth, fill))
	}

	return renderToolsBodyWithFooter(b.String(), "↑↓/PgUp/PgDn: 滚动 | r: 刷新 | Esc: 关闭", width, height)
}
