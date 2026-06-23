package tui

import (
	"strings"
	"testing"

	"treehole/internal/config"

	tea "github.com/charmbracelet/bubbletea"
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

	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyEscape})

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

	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyEscape})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

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

	dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	if dialog.cursorRow != 1 || dialog.cursorCol != 0 {
		t.Fatalf("cursor = %d:%d, want 1:0", dialog.cursorRow, dialog.cursorCol)
	}

	output := renderOpaquePanelBlanks(dialog.View(60, 12))
	if !strings.Contains(stripANSI(output), "█") {
		t.Fatalf("blank-cell cursor was swallowed by background fill:\n%s", stripANSI(output))
	}
}

func TestConfigDialogCursorRemainsVisibleAtEmptyLineEnd(t *testing.T) {
	dialog := NewConfigDialog(nil)
	dialog.lines = []string{""}

	output := renderOpaquePanelBlanks(dialog.View(60, 12))
	if !strings.Contains(stripANSI(output), "█") {
		t.Fatalf("empty-line cursor is not visible:\n%s", stripANSI(output))
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
