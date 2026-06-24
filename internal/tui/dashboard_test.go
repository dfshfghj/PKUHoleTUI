package tui

import (
	"strings"
	"testing"

	"treehole/internal/config"
	"treehole/internal/models"
)

func TestNewModelStartsOnDashboardWithoutRecoveryDialog(t *testing.T) {
	m := NewModel(nil, nil, &config.Config{}, SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonNetwork,
		Message:       "offline",
	})

	if m.Page != PageDashboard {
		t.Fatalf("page = %v, want dashboard", m.Page)
	}
	if m.Dialog != DialogNone {
		t.Fatalf("dialog = %v, dashboard should remain visible at startup", m.Dialog)
	}
}

func TestDashboardRendersLogoUnreadNotificationsAndActions(t *testing.T) {
	dashboard := NewDashboardModel()
	dashboard.SetNotifications([]models.Notification{
		{
			ID:        1,
			Type:      models.NotificationTypeInteractive,
			PID:       42,
			Content:   "new reply",
			CreatedAt: "2026-06-23 12:00:00",
		},
	}, nil)

	output := stripANSI(dashboard.View(100, 36))
	for _, want := range []string{"████████╗", "Notifications", "new reply", "#42", "Explore", "Config", "e", "n", "c"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dashboard missing %q:\n%s", want, output)
		}
	}
}

func TestDashboardShortcutsOpenExploreNotificationsAndConfig(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard

	explore, cmd := m.handleKey(keyPress('e'))
	if explore.Page != PagePosts || cmd == nil {
		t.Fatal("e should enter posts explore and load posts")
	}

	notifications, cmd := m.handleKey(keyPress('n'))
	if notifications.Dialog != DialogTools ||
		notifications.ToolsDialog.Section() != ToolsSectionInteractive ||
		cmd == nil {
		t.Fatal("n should open interactive notifications")
	}

	configModel, cmd := m.handleKey(keyPress('c'))
	if configModel.Dialog != DialogTools ||
		configModel.ToolsDialog.Section() != ToolsSectionConfig ||
		cmd == nil {
		t.Fatal("c should open config")
	}
}

func TestDashboardShowsOnlyUnreadItemsFromLoadMessage(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	result, _ := m.Update(LoadDashboardNotificationsMsg{Items: []models.Notification{
		{ID: 1, Content: "unread", Read: false},
		{ID: 2, Content: "read", Read: true},
	}})
	got := result.(Model)
	if len(got.Dashboard.Notifications) != 1 {
		t.Fatalf("dashboard notifications = %+v", got.Dashboard.Notifications)
	}
}
