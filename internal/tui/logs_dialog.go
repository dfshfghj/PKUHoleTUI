package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
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
	switch {
	case key.Matches(msg, shortcut.Up, shortcut.VimUp):
		if m.offset > 0 {
			m.offset--
		}
	case key.Matches(msg, shortcut.Down, shortcut.VimDown):
		if m.offset < len(m.lines)-1 {
			m.offset++
		}
	case key.Matches(msg, shortcut.PgUp):
		m.offset -= 20
		if m.offset < 0 {
			m.offset = 0
		}
	case key.Matches(msg, shortcut.PgDown):
		m.offset += 20
		if m.offset >= len(m.lines) {
			m.offset = len(m.lines) - 1
		}
	case key.Matches(msg, keymap.Direct.Refresh):
		m.loading = true
		return loadLogsCmd()
	}
	return nil
}

// logsLayout computes the visible log window for a given size, including the
// scrollbar reservation. Shared by View and mouse hit-testing.
func (m LogsDialogModel) logsLayout(width, height int) (contentWidth, visibleLines int, bar scrollbarState) {
	visibleLines = maxInt(1, height-3)
	contentWidth = maxInt(20, width)
	bar = scrollbarFor(visibleLines, len(m.lines), visibleLines, m.offset)
	if bar.Visible {
		contentWidth = maxInt(1, contentWidth-1)
	}
	return contentWidth, visibleLines, bar
}

func (m LogsDialogModel) View(width, height int) string {
	var b strings.Builder

	innerWidth := maxInt(20, width)
	fill := dialogBackgroundFillStyle()

	if m.loading {
		b.WriteString(fillRenderedBackground(vLoadingStyle.Render("加载日志中..."), innerWidth, fill))
	} else if len(m.lines) == 0 {
		b.WriteString(fillRenderedBackground(vEmptyStyle.Render("暂无日志"), innerWidth, fill))
	} else {
		contentWidth, visibleLines, bar := m.logsLayout(width, height)
		end := m.offset + visibleLines
		if end > len(m.lines) {
			end = len(m.lines)
		}

		var rows strings.Builder
		for i := m.offset; i < end; i++ {
			line := m.lines[i]
			if lipgloss.Width(line) > contentWidth {
				line = clipToVisibleWidth(line, contentWidth)
			}
			renderedLine := vLogLineStyle.Background(colorBg).Render(line)
			rows.WriteString(fillRenderedBackground(renderedLine, contentWidth, fill))
			if i != end-1 {
				rows.WriteString("\n")
			}
		}
		window := rows.String()
		if bar.Visible {
			window = appendScrollbar(window, bar, contentWidth)
		}
		b.WriteString(window)

		b.WriteString("\n")
		totalLines := len(m.lines)
		pagination := vPaginationStyle.Render(
			fmt.Sprintf("日志: %d 行 | 当前: %d-%d",
				totalLines, m.offset+1, minInt(end, totalLines)),
		)
		b.WriteString(fillRenderedBackground(pagination, innerWidth, fill))
	}

	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(fillRenderedBackground(vErrorStyle.Render("错误: "+m.lastErr), innerWidth, fill))
	}

	return renderToolsBodyWithFooter(b.String(), "r: 刷新", width, height)
}
