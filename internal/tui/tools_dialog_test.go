package tui

import (
	"strings"
	"testing"

	"treehole/internal/config"
	"treehole/internal/models"
)

func TestToolsDialogFlattensNotificationTypesIntoPrimaryTabs(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	dialog.Switch(ToolsSectionInteractive)
	dialog.Notifications.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 1, Content: "reply"},
	}, 1)

	output := stripANSI(dialog.View(80, 30))
	for _, want := range []string{"配置", "日志", "互动", "系统", "reply"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools dialog missing %q:\n%s", want, output)
		}
	}
	for _, duplicate := range []string{"通知中心", "互动消息", "系统消息"} {
		if strings.Contains(output, duplicate) {
			t.Fatalf("tools dialog should not render nested title %q:\n%s", duplicate, output)
		}
	}
}

func TestToolsDialogPaintsTabSeparators(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	dialog.Switch(ToolsSectionInteractive)

	output := dialog.renderTabs()
	if strings.Contains(output, "\x1b[m  ") {
		t.Fatalf("tab separators should not be plain spaces after a style reset:\n%q", output)
	}
	if !strings.Contains(output, paintedDialogSpaces(2)) {
		t.Fatalf("tab separators should carry dialog background:\n%q", output)
	}
}

func TestToolsDialogViewPreservesBackgroundAfterInlineResets(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	dialog.Switch(ToolsSectionInteractive)
	dialog.Notifications.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 1, PID: 42, Content: "reply", CreatedAt: "2026-04-08 15:27:19"},
	}, 1)

	output := dialog.View(80, 20)
	for _, resetLeak := range []string{
		"\x1b[m  ",
		"●\x1b[m  #",
	} {
		if strings.Contains(output, resetLeak) {
			t.Fatalf("tools dialog view should preserve dialog background after inline reset %q:\n%q", resetLeak, output)
		}
	}
}

func TestToolsDialogLogsDoNotRepeatTitle(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	dialog.Switch(ToolsSectionLogs)
	dialog.Logs.SetLines([]string{"log entry"})

	output := stripANSI(dialog.View(80, 30))
	if strings.Contains(output, "运行日志") {
		t.Fatalf("logs body should not repeat the primary tab title:\n%s", output)
	}
	if !strings.Contains(output, "log entry") {
		t.Fatalf("logs body missing content:\n%s", output)
	}
}

func TestToolsDialogPinsFooterToLastLine(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	dialog.Switch(ToolsSectionLogs)
	dialog.Logs.SetLines([]string{"one line"})

	output := stripANSI(dialog.View(60, 20))
	lines := strings.Split(output, "\n")
	if len(lines) != 20 {
		t.Fatalf("height = %d, want 20:\n%s", len(lines), output)
	}
	if !strings.Contains(lines[len(lines)-1], "r: 刷新") {
		t.Fatalf("footer is not on the last line:\n%s", output)
	}
	if strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Fatalf("short body should expand before the footer:\n%s", output)
	}
}

func TestToolsDialogOmitsRedundantTitle(t *testing.T) {
	dialog := NewToolsDialog(&config.Config{})
	output := stripANSI(dialog.View(60, 20))
	if strings.Contains(output, "工具") {
		t.Fatalf("tools title should be omitted:\n%s", output)
	}
}
