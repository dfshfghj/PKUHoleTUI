package tui

import (
	"reflect"
	"strings"
	"testing"

	"treehole/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfigDialogUpdateSwitchesSections(t *testing.T) {
	dialog := NewConfigDialog(nil)
	if dialog.ActiveSection() != ConfigSectionAuth {
		t.Fatalf("initial section = %v, want auth", dialog.ActiveSection())
	}

	dialog.setFocus(dialog.saveIndex())
	dialog.Update(tea.KeyMsg{Type: tea.KeyRight})
	if dialog.ActiveSection() != ConfigSectionDatabase {
		t.Fatalf("section after right = %v, want database", dialog.ActiveSection())
	}
	if dialog.FocusIndex() != 0 {
		t.Fatalf("focus after section switch = %d, want 0", dialog.FocusIndex())
	}

	dialog.setFocus(dialog.saveIndex())
	dialog.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if dialog.ActiveSection() != ConfigSectionAuth {
		t.Fatalf("section after left = %v, want auth", dialog.ActiveSection())
	}
}

func TestConfigDialogToConfigPreservesExistingAndAppliesDatabaseEdits(t *testing.T) {
	existing := &config.Config{
		Username:   "old-user",
		Password:   "old-pass",
		SecretKey:  "old-secret",
		DeviceUUID: "old-device",
		Database: config.DatabaseConfig{
			Type:    "sqlite3",
			DBFile:  "./treehole.db",
			SSLMode: "disable",
		},
		Cors: config.CorsConfig{
			AllowOrigins: []string{"http://localhost:3000"},
			AllowMethods: []string{"GET", "POST"},
			AllowHeaders: []string{"Authorization"},
		},
	}

	dialog := NewConfigDialog(existing)
	dialog.authInputs[0].SetValue("new-user")
	dialog.authInputs[3].SetValue("device-2")
	dialog.databaseInputs[0].SetValue("postgres")
	dialog.databaseInputs[1].SetValue("db.internal")
	dialog.databaseInputs[2].SetValue("15432")
	dialog.databaseInputs[3].SetValue("dbuser")
	dialog.databaseInputs[4].SetValue("dbpass")
	dialog.databaseInputs[5].SetValue("treehole")
	dialog.databaseInputs[6].SetValue("./ignored.db")
	dialog.databaseInputs[7].SetValue("require")
	dialog.databaseInputs[8].SetValue("postgres://example")

	got := dialog.ToConfig(existing)

	if got.Username != "new-user" {
		t.Fatalf("username = %q, want %q", got.Username, "new-user")
	}
	if got.DeviceUUID != "device-2" {
		t.Fatalf("device uuid = %q, want %q", got.DeviceUUID, "device-2")
	}
	if got.Database.Type != "postgres" || got.Database.Host != "db.internal" || got.Database.Port != 15432 {
		t.Fatalf("database not updated correctly: %+v", got.Database)
	}
	if !reflect.DeepEqual(got.Cors, existing.Cors) {
		t.Fatalf("cors changed unexpectedly: got %+v want %+v", got.Cors, existing.Cors)
	}
}

func TestConfigDialogViewWrapsLongValuesInsideFieldBoxes(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{
		Username:   "2400011506",
		Password:   "abcdefghijklmnopqrstuvwxyz",
		SecretKey:  "1234567890abcdef",
		DeviceUUID: "UID_1903fec8-eb6a-41a3-8a0a-75cf31d48b8e",
	})

	output := stripANSI(dialog.View(80, 40))
	expected := []string{
		"│用户名: 2400011506",
		"│密码: **************************",
		"│SecretKey: ****************",
		"│DeviceUUID: UID_1903fec8-eb6a-41a3-8a0a-",
		"│            75cf31d48b8e",
	}
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Fatalf("config dialog output missing %q\n%s", want, output)
		}
	}
	if strings.Contains(output, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("config dialog should not render plaintext secret, got:\n%s", output)
	}
}

func TestConfigDialogViewWrapsLongDatabaseValues(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Type = "postgres"
	cfg.Database.DSN = "postgres://treehole:password@10.129.246.201:5432/treehole_db?sslmode=disable&application_name=treehole-tui"

	dialog := NewConfigDialog(cfg)
	dialog.switchSection(ConfigSectionDatabase)
	dialog.setFocus(8)

	output := stripANSI(dialog.View(80, 40))
	if !strings.Contains(output, "│DSN: postgres://treehole:password@10.129") {
		t.Fatalf("database DSN first wrapped line missing, got:\n%s", output)
	}
	if !strings.Contains(output, "│     .246.201:5432/treehole_db?sslmode=d") {
		t.Fatalf("database DSN continuation line missing, got:\n%s", output)
	}
	if !strings.Contains(output, "│     isable&application_name=treehole-tu") || !strings.Contains(output, "│     i") {
		t.Fatalf("database DSN final continuation line missing, got:\n%s", output)
	}
}

func TestConfigDialogUpdateAllowsHorizontalCursorMovementWithinField(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{Username: "abc"})

	dialog.Update(tea.KeyMsg{Type: tea.KeyLeft})
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})

	if got := dialog.Username(); got != "abXc" {
		t.Fatalf("username after cursor move edit = %q, want %q", got, "abXc")
	}
	if dialog.ActiveSection() != ConfigSectionAuth {
		t.Fatalf("section changed unexpectedly: %v", dialog.ActiveSection())
	}
}

func TestConfigDialogViewScrollsFocusedFieldIntoViewport(t *testing.T) {
	dialog := NewConfigDialog(&config.Config{})
	dialog.switchSection(ConfigSectionDatabase)
	dialog.setFocus(dialog.saveIndex())

	output := stripANSI(dialog.View(80, 18))
	if dialog.formViewport.YOffset == 0 {
		t.Fatalf("expected form viewport to scroll for save button focus")
	}
	if !strings.Contains(output, "保存配置") {
		t.Fatalf("expected save button to remain visible after scrolling, got:\n%s", output)
	}
}
