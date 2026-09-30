package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// dialogScrollView returns the scroll container of the open dialog under the
// cursor, if any.
func (m Model) dialogScrollView(x, y int) (mouseTarget, rect, bool) {
	if m.Dialog != DialogTools {
		return mouseTargetNone, rect{}, false
	}
	body := m.toolsBodyView(m.Width, m.Height)
	if !body.contains(x, y) {
		return mouseTargetNone, rect{}, false
	}
	switch m.ToolsDialog.Section() {
	case ToolsSectionLogs:
		return mouseTargetLogs, body, true
	case ToolsSectionConfig:
		return mouseTargetConfig, body, true
	case ToolsSectionInteractive, ToolsSectionSystem:
		return mouseTargetNotifications, body, true
	}
	return mouseTargetNone, rect{}, false
}

// dialogScrollbarAt resolves a click on a dialog scrollbar: the bar state
// and the screen row of its first track cell.
func (m Model) dialogScrollbarAt(x, y int) (mouseTarget, scrollbarState, int, bool) {
	target, body, ok := m.dialogScrollView(x, y)
	if !ok {
		return mouseTargetNone, scrollbarState{}, 0, false
	}
	switch target {
	case mouseTargetLogs:
		contentWidth, visibleLines, bar := m.ToolsDialog.Logs.logsLayout(body.W, body.H)
		if bar.Visible && x == body.X+contentWidth && y < body.Y+visibleLines {
			return target, bar, body.Y, true
		}
	case mouseTargetConfig:
		lineNumberWidth, separatorWidth, contentWidth, bar := m.ToolsDialog.Config.editorLayout(body.W, body.H)
		editorHeight := m.ToolsDialog.Config.editorHeight(body.H)
		if bar.Visible && x == body.X+lineNumberWidth+separatorWidth+contentWidth && y < body.Y+editorHeight {
			return target, bar, body.Y, true
		}
	case mouseTargetNotifications:
		rendered, start, end, itemWidth, bar := m.ToolsDialog.Notifications.windowLayout(body.W, body.H)
		if bar.Visible && x == body.X+itemWidth && y < body.Y+renderedLines(rendered[start:end]) {
			return target, bar, body.Y, true
		}
	}
	return mouseTargetNone, scrollbarState{}, 0, false
}

// scrollDialogAt applies wheel scrolling inside the open dialog.
func (m *Model) scrollDialogAt(x, y, delta int) {
	target, _, ok := m.dialogScrollView(x, y)
	if !ok {
		return
	}
	switch target {
	case mouseTargetLogs:
		logs := &m.ToolsDialog.Logs
		logs.offset = clampInt(logs.offset+delta, 0, maxInt(0, len(logs.lines)-1))
	case mouseTargetConfig:
		m.scrollConfigEditor(delta)
	case mouseTargetNotifications:
		notifications := &m.ToolsDialog.Notifications
		notifications.selected = clampInt(notifications.selected+delta, 0, maxInt(0, len(notifications.items)-1))
	}
}

func (m *Model) scrollConfigEditor(delta int) {
	config := &m.ToolsDialog.Config
	body := m.toolsBodyView(m.Width, m.Height)
	maxOffset := maxInt(0, len(config.lines)-config.editorHeight(body.H))
	config.offset = clampInt(config.offset+delta, 0, maxOffset)
}

// handleDialogMouseClick routes clicks inside dialogs.
func (m Model) handleDialogMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	if target, bar, trackTop, ok := m.dialogScrollbarAt(msg.X, msg.Y); ok {
		return m.beginScrollbarDrag(target, bar, trackTop, msg.Y-trackTop), nil
	}
	target, body, ok := m.dialogScrollView(msg.X, msg.Y)
	if !ok {
		return m, nil
	}
	switch target {
	case mouseTargetConfig:
		m.clickConfigEditor(body, msg.X, msg.Y)
	case mouseTargetNotifications:
		m.clickNotificationItem(body, msg.X, msg.Y)
	}
	return m, nil
}

func (m *Model) clickConfigEditor(body rect, x, y int) {
	config := &m.ToolsDialog.Config
	if len(config.lines) == 0 {
		return
	}
	editorHeight := config.editorHeight(body.H)
	row := config.offset + (y - body.Y)
	if row < config.offset || row >= config.offset+editorHeight || row >= len(config.lines) {
		return
	}
	lineNumberWidth, separatorWidth, _, _ := config.editorLayout(body.W, body.H)
	textX := x - body.X - lineNumberWidth - separatorWidth
	config.cursorRow = row
	if textX <= 0 {
		config.cursorCol = 0
	} else {
		config.cursorCol = runeColumnAtWidth(config.lines[row], config.columnOff, textX)
	}
	config.clampCursor()
}

// runeColumnAtWidth maps a cell offset inside a rendered line back to a rune
// index, accounting for wide glyphs.
func runeColumnAtWidth(line string, startColumn, cellOffset int) int {
	runes := []rune(line)
	col := clampInt(startColumn, 0, len(runes))
	cells := 0
	for col < len(runes) {
		width := lipgloss.Width(string(runes[col]))
		if cells+width > cellOffset {
			return col
		}
		cells += width
		col++
	}
	return len(runes)
}

func (m *Model) clickNotificationItem(body rect, x, y int) {
	notifications := &m.ToolsDialog.Notifications
	if len(notifications.items) == 0 {
		return
	}
	rendered, start, end, _, _ := notifications.windowLayout(body.W, body.H)
	rowOffset := y - body.Y
	used := 0
	for i := start; i < end; i++ {
		height := lipgloss.Height(rendered[i])
		if rowOffset < used+height {
			notifications.selected = i
			return
		}
		used += height
	}
}
