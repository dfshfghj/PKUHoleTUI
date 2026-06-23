package tui

import (
	"strconv"
	"strings"
	"testing"

	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNotificationDialogSelectsMessagesAndAcceptsFlattenedType(t *testing.T) {
	dialog := NewNotificationDialog()
	dialog.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 1, Content: "first"},
		{ID: 2, Content: "second"},
	}, 2)

	dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := dialog.Selected(); got == nil || got.ID != 2 {
		t.Fatalf("selected = %+v, want id 2", got)
	}

	dialog.SetMessageType(models.NotificationTypeSystem)
	if dialog.MessageType() != models.NotificationTypeSystem {
		t.Fatalf("message type = %q", dialog.MessageType())
	}
}

func TestNotificationDialogMarksReadAndRendersTypeDifference(t *testing.T) {
	dialog := NewNotificationDialog()
	dialog.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 1, PID: 42, Content: "reply text", CreatedAt: "2026-04-08 15:27:19"},
	}, 1)

	output := stripANSI(dialog.View(80, 30))
	for _, want := range []string{"reply text", "●", "#42"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	for _, duplicate := range []string{"通知中心", "互动消息", "系统消息"} {
		if strings.Contains(output, duplicate) {
			t.Fatalf("notification body should not repeat flattened header %q:\n%s", duplicate, output)
		}
	}
	if strings.Contains(output, "│ 未读") || strings.Contains(output, "│ 已读") {
		t.Fatalf("read state should use only the unread marker:\n%s", output)
	}

	dialog.MarkRead(1)
	if selected := dialog.Selected(); selected == nil || !selected.Read {
		t.Fatalf("selected notification was not marked read: %+v", selected)
	}
}

func TestNotificationDialogFillsAvailableListHeight(t *testing.T) {
	dialog := NewNotificationDialog()
	items := make([]models.Notification, 13)
	for i := range items {
		items[i] = models.Notification{
			ID:      i + 1,
			Content: "notification " + strconv.Itoa(i+1),
		}
	}
	dialog.SetNotifications(models.NotificationTypeInteractive, items, len(items))

	output := stripANSI(dialog.View(180, 35))
	if !strings.Contains(output, "notification 12") {
		t.Fatalf("dialog should use available height for more notifications:\n%s", output)
	}
}

func TestNotificationDialogSystemMessagesDoNotAllowSingleRead(t *testing.T) {
	dialog := NewNotificationDialog()
	dialog.SetNotifications(models.NotificationTypeSystem, []models.Notification{
		{ID: 1, Content: "system", Read: false},
	}, 1)

	if dialog.CanMarkSelectedRead() {
		t.Fatal("system messages must not expose single-message read")
	}
	output := stripANSI(dialog.View(80, 30))
	if strings.Contains(output, "Enter: 当前已读") {
		t.Fatalf("system help should not advertise single-message read:\n%s", output)
	}
}
