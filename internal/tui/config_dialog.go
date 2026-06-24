package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"treehole/internal/config"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ConfigEditorMode int

const (
	ConfigEditorNormal ConfigEditorMode = iota
	ConfigEditorInsert
)

type ConfigDialogModel struct {
	lines      []string
	cursorRow  int
	cursorCol  int
	offset     int
	columnOff  int
	mode       ConfigEditorMode
	pendingG   bool
	saving     bool
	saveOK     bool
	lastErr    string
	viewHeight int
	viewWidth  int
}

func NewConfigDialog(cfg *config.Config) ConfigDialogModel {
	if cfg == nil {
		defaultConfig := config.DefaultConfig()
		cfg = &defaultConfig
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		data = []byte("{}")
	}
	return ConfigDialogModel{
		lines: strings.Split(string(data), "\n"),
		mode:  ConfigEditorNormal,
	}
}

func (m ConfigDialogModel) initialized() bool {
	return len(m.lines) > 0
}

func (m *ConfigDialogModel) SetConfig(cfg *config.Config) {
	next := NewConfigDialog(cfg)
	*m = next
}

func (m *ConfigDialogModel) SetSaving(saving bool) {
	m.saving = saving
	if saving {
		m.saveOK = false
		m.lastErr = ""
	}
}

func (m *ConfigDialogModel) SetSaveResult(err error) {
	m.saving = false
	if err != nil {
		m.saveOK = false
		m.lastErr = err.Error()
		return
	}
	m.saveOK = true
	m.lastErr = ""
}

func (m ConfigDialogModel) Mode() ConfigEditorMode {
	return m.mode
}

func (m ConfigDialogModel) Text() string {
	return strings.Join(m.lines, "\n")
}

func (m *ConfigDialogModel) ToConfig() (*config.Config, error) {
	var result config.Config
	if err := json.Unmarshal([]byte(m.Text()), &result); err != nil {
		return nil, fmt.Errorf("JSON 无效: %w", err)
	}
	return &result, nil
}

func (m *ConfigDialogModel) Update(msg tea.KeyPressMsg) {
	if m.mode == ConfigEditorInsert {
		m.updateInsert(msg)
	} else {
		m.updateNormal(msg)
	}
	m.clampCursor()
	m.ensureCursorVisible()
}

func (m *ConfigDialogModel) updateInsert(msg tea.KeyPressMsg) {
	switch msg.Code {
	case tea.KeyEscape:
		m.mode = ConfigEditorNormal
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case tea.KeyLeft:
		m.moveHorizontal(-1)
	case tea.KeyRight:
		m.moveHorizontal(1)
	case tea.KeyUp:
		m.moveVertical(-1)
	case tea.KeyDown:
		m.moveVertical(1)
	case tea.KeyEnter:
		line := []rune(m.lines[m.cursorRow])
		left := string(line[:m.cursorCol])
		right := string(line[m.cursorCol:])
		m.lines[m.cursorRow] = left
		m.lines = append(m.lines, "")
		copy(m.lines[m.cursorRow+2:], m.lines[m.cursorRow+1:])
		m.lines[m.cursorRow+1] = right
		m.cursorRow++
		m.cursorCol = 0
	case tea.KeyBackspace:
		m.backspace()
	default:
		if msg.Text != "" {
			m.insertRunes([]rune(msg.Text))
		}
	}
}

func (m *ConfigDialogModel) updateNormal(msg tea.KeyPressMsg) {
	if msg.String() != "g" {
		m.pendingG = false
	}
	switch msg.Code {
	case tea.KeyLeft:
		m.moveHorizontal(-1)
	case tea.KeyRight:
		m.moveHorizontal(1)
	case tea.KeyUp:
		m.moveVertical(-1)
	case tea.KeyDown:
		m.moveVertical(1)
	default:
		switch msg.String() {
		case "h":
			m.moveHorizontal(-1)
		case "j":
			m.moveVertical(1)
		case "k":
			m.moveVertical(-1)
		case "l":
			m.moveHorizontal(1)
		case "i":
			m.mode = ConfigEditorInsert
		case "a":
			if m.cursorCol < len([]rune(m.lines[m.cursorRow])) {
				m.cursorCol++
			}
			m.mode = ConfigEditorInsert
		case "o":
			m.insertLine(m.cursorRow + 1)
			m.cursorRow++
			m.cursorCol = 0
			m.mode = ConfigEditorInsert
		case "O":
			m.insertLine(m.cursorRow)
			m.cursorCol = 0
			m.mode = ConfigEditorInsert
		case "x":
			m.deleteRune()
		case "0":
			m.cursorCol = 0
		case "$":
			m.cursorCol = maxInt(0, len([]rune(m.lines[m.cursorRow]))-1)
		case "G":
			m.cursorRow = len(m.lines) - 1
			m.cursorCol = 0
		case "g":
			if m.pendingG {
				m.cursorRow = 0
				m.cursorCol = 0
				m.pendingG = false
			} else {
				m.pendingG = true
				return
			}
		default:
			m.pendingG = false
		}
	}
}

func (m *ConfigDialogModel) insertRunes(value []rune) {
	line := []rune(m.lines[m.cursorRow])
	next := make([]rune, 0, len(line)+len(value))
	next = append(next, line[:m.cursorCol]...)
	next = append(next, value...)
	next = append(next, line[m.cursorCol:]...)
	m.lines[m.cursorRow] = string(next)
	m.cursorCol += len(value)
}

func (m *ConfigDialogModel) backspace() {
	line := []rune(m.lines[m.cursorRow])
	if m.cursorCol > 0 {
		m.lines[m.cursorRow] = string(append(line[:m.cursorCol-1], line[m.cursorCol:]...))
		m.cursorCol--
		return
	}
	if m.cursorRow == 0 {
		return
	}
	previous := []rune(m.lines[m.cursorRow-1])
	m.cursorCol = len(previous)
	m.lines[m.cursorRow-1] += m.lines[m.cursorRow]
	m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)
	m.cursorRow--
}

func (m *ConfigDialogModel) deleteRune() {
	line := []rune(m.lines[m.cursorRow])
	if len(line) == 0 || m.cursorCol >= len(line) {
		return
	}
	m.lines[m.cursorRow] = string(append(line[:m.cursorCol], line[m.cursorCol+1:]...))
}

func (m *ConfigDialogModel) insertLine(index int) {
	m.lines = append(m.lines, "")
	copy(m.lines[index+1:], m.lines[index:])
	m.lines[index] = ""
}

func (m *ConfigDialogModel) moveHorizontal(delta int) {
	m.cursorCol += delta
}

func (m *ConfigDialogModel) moveVertical(delta int) {
	m.cursorRow += delta
}

func (m *ConfigDialogModel) clampCursor() {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	m.cursorRow = clampInt(m.cursorRow, 0, len(m.lines)-1)
	maxCol := len([]rune(m.lines[m.cursorRow]))
	if m.mode == ConfigEditorNormal && maxCol > 0 {
		maxCol--
	}
	m.cursorCol = clampInt(m.cursorCol, 0, maxCol)
}

func (m *ConfigDialogModel) ensureCursorVisible() {
	if m.viewHeight < 1 {
		return
	}
	if m.cursorRow < m.offset {
		m.offset = m.cursorRow
	}
	if m.cursorRow >= m.offset+m.viewHeight {
		m.offset = m.cursorRow - m.viewHeight + 1
	}
	if m.viewWidth < 1 {
		return
	}
	if m.cursorCol < m.columnOff {
		m.columnOff = m.cursorCol
	}
	if m.cursorCol >= m.columnOff+m.viewWidth {
		m.columnOff = m.cursorCol - m.viewWidth + 1
	}
}

func (m *ConfigDialogModel) View(width, height int) string {
	var b strings.Builder
	statusHeight := 0
	if m.saving || m.saveOK || m.lastErr != "" {
		statusHeight = 1
	}
	editorHeight := maxInt(1, height-1-statusHeight)
	m.viewHeight = editorHeight
	m.ensureCursorVisible()

	end := minInt(len(m.lines), m.offset+editorHeight)
	lineNumberWidth := len(fmt.Sprintf("%d", len(m.lines)))
	separatorWidth := lipgloss.Width(" │ ")
	contentWidth := maxInt(1, width-lineNumberWidth-separatorWidth)
	m.viewWidth = contentWidth
	m.ensureCursorVisible()
	fill := dialogBackgroundFillStyle()
	for i := m.offset; i < end; i++ {
		number := vStatLabelStyle.
			Background(colorBg).
			Width(lineNumberWidth).
			Render(fmt.Sprintf("%d", i+1))
		line := lipgloss.NewStyle().
			Background(colorBg).
			Render(m.renderLine(i, contentWidth))
		line = fillRenderedBackground(line, contentWidth, fill)
		row := number +
			fill.Render(" │ ") +
			line
		row = fillRenderedBackground(row, width, fill)
		b.WriteString(row)
		if i != end-1 {
			b.WriteString("\n")
		}
	}

	if m.saving {
		b.WriteString("\n")
		b.WriteString(vLoadingStyle.Render("保存中..."))
	} else if m.saveOK {
		b.WriteString("\n")
		b.WriteString(vStatusRunningStyle.Render("配置已保存"))
	} else if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(vErrorStyle.Render(m.lastErr))
	}

	help := "NORMAL | Ctrl+S: 保存 | i/a/o/O: 编辑 | hjkl: 移动 | x: 删除 | gg/G"
	if m.mode == ConfigEditorInsert {
		help = "INSERT | Ctrl+S: 保存 | Esc: NORMAL | Enter: 换行 | Backspace: 删除"
	}
	return renderToolsBodyWithFooter(b.String(), help, width, height)
}

func (m ConfigDialogModel) renderLine(row, width int) string {
	line := []rune(m.lines[row])
	start := minInt(m.columnOff, len(line))
	end := minInt(len(line), start+width)
	if row != m.cursorRow {
		return string(line[start:end])
	}
	col := clampInt(m.cursorCol, 0, len(line))
	if col < start {
		col = start
	}
	if col > end {
		col = end
	}
	before := string(line[start:col])
	cursor := " "
	after := ""
	if col < end {
		if line[col] != ' ' && line[col] != '\t' {
			cursor = string(line[col])
		}
		after = string(line[col+1 : end])
	}
	rendered := before + lipgloss.NewStyle().
		Background(colorAccent).
		Foreground(colorAccentText).
		Render(cursor) + after
	return rendered
}
