package tui

import (
	"strings"
	"testing"

	"treehole/internal/config"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestConfigDialogRendersSingleJSONDocument(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{
		Username: "alice",
		Database: config.DatabaseConfig{Type: "sqlite3", DBFile: "./treehole.db"},
	})

	output := stripANSI(dialog.View(80, 30))
	for _, want := range []string{`"username": "alice"`, `"database": {`, `"db_file": "./treehole.db"`, "NORMAL | Ctrl+S"} {
		if !strings.Contains(output, want) {
			t.Fatalf("config JSON editor missing %q:\n%s", want, output)
		}
	}
}

func TestConfigDialogVimInsertAndNormalMovement(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{})
	dialog.lines = []string{"abc"}
	dialog.cursorRow = 0
	dialog.cursorCol = 0

	dialog.Update(keyPress('l'))
	dialog.Update(keyPress('i'))
	dialog.Update(keyPress('X'))
	dialog.Update(keyCode(tea.KeyEscape))

	if got := dialog.Text(); got != "aXbc" {
		t.Fatalf("text = %q, want %q", got, "aXbc")
	}
	if dialog.Mode() != ConfigEditorNormal {
		t.Fatal("Esc should return to normal mode")
	}
}

func TestConfigDialogSupportsOpenDeleteAndDocumentJumps(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{})
	dialog.lines = []string{"one", "two"}

	dialog.Update(keyPress('G'))
	dialog.Update(keyPress('o'))
	dialog.Update(keyPress('x'))
	dialog.Update(keyCode(tea.KeyEscape))
	dialog.Update(keyPress('0'))
	dialog.Update(keyPress('x'))
	dialog.Update(keyPress('g'))
	dialog.Update(keyPress('g'))

	if dialog.cursorRow != 0 {
		t.Fatalf("gg cursor row = %d, want 0", dialog.cursorRow)
	}
	if len(dialog.lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(dialog.lines))
	}
	if dialog.lines[2] != "" {
		t.Fatalf("x should delete the inserted rune, got %q", dialog.lines[2])
	}
}

func TestConfigDialogHorizontallyScrollsWithCursor(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{strings.Repeat("x", 80)}
	dialog.cursorCol = 79

	output := stripANSI(dialog.View(30, 12))
	if dialog.columnOff == 0 {
		t.Fatal("long line cursor should move the horizontal viewport")
	}
	if strings.Count(output, "x") < 10 {
		t.Fatalf("expected visible tail of long line:\n%s", output)
	}
}

func TestConfigDialogCursorRemainsVisibleOnBlankCell(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{"{", `  "username": ""`}
	dialog.cursorRow = 0
	dialog.cursorCol = 0

	dialog.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if dialog.cursorRow != 1 || dialog.cursorCol != 0 {
		t.Fatalf("cursor = %d:%d, want 1:0", dialog.cursorRow, dialog.cursorCol)
	}

	output := dialog.View(60, 12)
	if strings.Contains(stripANSI(output), "█") {
		t.Fatalf("blank-cell cursor should render as highlighted space, not a full block:\n%s", stripANSI(output))
	}
	if !strings.Contains(output, paintedCursorSpace()) {
		t.Fatalf("blank-cell cursor should carry accent background:\n%q", output)
	}
}

func TestConfigDialogCursorRemainsVisibleAtEmptyLineEnd(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{""}

	output := dialog.View(60, 12)
	if strings.Contains(stripANSI(output), "█") {
		t.Fatalf("empty-line cursor should render as highlighted space, not a full block:\n%s", stripANSI(output))
	}
	if !strings.Contains(output, paintedCursorSpace()) {
		t.Fatalf("empty-line cursor should carry accent background:\n%q", output)
	}
}

func paintedCursorSpace() string {
	cursor := lipgloss.NewStyle().
		Background(colorAccent).
		Foreground(colorAccentText).
		Render(" ")
	return strings.TrimSuffix(preserveBackgroundAfterReset(cursor, colorBg), resetWithBackground(colorBg))
}

func TestConfigDialogPaintsEditorSeparatorsAndTrailingBlanks(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{"{"}

	output := dialog.View(40, 8)
	fill := dialogBackgroundFillStyle()
	if !strings.Contains(output, preserveBackgroundAfterReset(fill.Render(" │ "), colorBg)) {
		t.Fatalf("config separator should carry dialog background:\n%q", output)
	}
	for i, line := range strings.Split(output, "\n") {
		if i > 0 {
			break
		}
		if got := lipgloss.Width(line); got != 40 {
			t.Fatalf("config editor line width = %d, want 40:\n%q", got, line)
		}
		if !strings.Contains(line, paintedDialogSpaces(4)) {
			t.Fatalf("config editor trailing blanks should carry dialog background:\n%q", line)
		}
	}
}

func TestConfigDialogParsesEditedJSON(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = strings.Split(`{
  "username": "new-user",
  "password": "",
  "secret_key": "",
  "device_uuid": "",
  "database": {"type": "sqlite3", "db_file": "./custom.db"},
  "cors": {"allow_origins": ["*"], "allow_methods": ["GET"], "allow_headers": []}
}`, "\n")

	cfg, err := dialog.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.Username != "new-user" || cfg.Database.DBFile != "./custom.db" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestConfigDialogRejectsInvalidJSON(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{"{"}

	if _, err := dialog.ToConfig(); err == nil || !strings.Contains(err.Error(), "JSON 无效") {
		t.Fatalf("error = %v, want JSON validation error", err)
	}
}
