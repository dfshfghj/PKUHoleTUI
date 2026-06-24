package tui

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"treehole/internal/config"
	"treehole/internal/models"

	"charm.land/lipgloss/v2"
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
	dashboard.SetHotPosts([]DashboardHotPost{
		{ID: 8347014, Text: "hot post", FollowNum: 4},
	}, nil)

	output := stripANSI(dashboard.View(100, 36))
	for _, want := range []string{"████████╗", "Notifications", "new reply", "#42", "热榜", "#8347014 hot post", "★ 4", "Explore", "Config", "e", "n", "c"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dashboard missing %q:\n%s", want, output)
		}
	}
}

func TestDashboardHotPostLineNormalizesAndTruncatesText(t *testing.T) {
	line := dashboardHotPostLine(DashboardHotPost{
		ID:        8347014,
		Text:      "第一行\r\n第二行\r第三行\n第四行 很长很长很长很长很长很长",
		FollowNum: 114,
	}, 34)

	if strings.ContainsAny(line, "\r\n") {
		t.Fatalf("hot post line should be single-line, got %q", line)
	}
	if !strings.Contains(line, "...") {
		t.Fatalf("hot post line should truncate with ellipsis, got %q", line)
	}
	if !strings.HasSuffix(line, "★ 114") {
		t.Fatalf("hot post likes should be right-aligned at line end, got %q", line)
	}
	if got := lipgloss.Width(line); got != 34 {
		t.Fatalf("hot post line width = %d, want 34: %q", got, line)
	}
}

func TestDashboardWriteHotPostsFrame(t *testing.T) {
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
	dashboard.SetHotPosts([]DashboardHotPost{
		{
			ID:        8347014,
			Text:      "妈的怎么会有人平时作业用ai做还抄袭\r\n期末答得一坨还空着题不写想捞都捞不动",
			FollowNum: 114,
		},
		{ID: 8347015, Text: "短热榜", FollowNum: 4},
	}, nil)

	output := stripANSI(dashboard.View(80, 40))
	_, filename, _, _ := runtime.Caller(0)
	outDir := filepath.Join(filepath.Dir(filename), "../..", ".out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "current-frame-dashboard.txt"), []byte(output), 0644); err != nil {
		t.Fatalf("write dashboard frame: %v", err)
	}

	if !strings.Contains(output, "热榜") || !strings.Contains(output, "★ 114") {
		t.Fatalf("dashboard frame missing hot posts:\n%s", output)
	}
	if strings.Contains(output, "\r") {
		t.Fatalf("dashboard frame should normalize carriage returns:\n%s", output)
	}
	for i, line := range strings.Split(output, "\n") {
		if got := lipgloss.Width(line); got > 80 {
			t.Fatalf("line %d width = %d, want <= 80: %q", i, got, line)
		}
	}
}

func TestBuildDashboardHotPostsURLUsesRecentTwelveHourWindow(t *testing.T) {
	previousEndpoint := dashboardHotPostsEndpoint
	dashboardHotPostsEndpoint = "https://example.invalid/posts"
	defer func() { dashboardHotPostsEndpoint = previousEndpoint }()

	now := time.Now().Unix()
	parsed, err := url.Parse(buildDashboardHotPostsURL(now))
	if err != nil {
		t.Fatalf("parse hot posts url: %v", err)
	}
	query := parsed.Query()
	if query.Get("limit") != "5" {
		t.Fatalf("limit = %q, want 5", query.Get("limit"))
	}
	if query.Get("order_by") != "likenum" {
		t.Fatalf("order_by = %q, want likenum", query.Get("order_by"))
	}
	gotEnd, err := strconv.ParseInt(query.Get("end_time"), 10, 64)
	if err != nil {
		t.Fatalf("parse end_time: %v", err)
	}
	gotStart, err := strconv.ParseInt(query.Get("start_time"), 10, 64)
	if err != nil {
		t.Fatalf("parse start_time: %v", err)
	}
	if gotEnd != now {
		t.Fatalf("end_time = %d, want %d", gotEnd, now)
	}
	if delta := gotEnd - gotStart; delta != int64(12*time.Hour/time.Second) {
		t.Fatalf("time window = %d seconds, want 12h", delta)
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

func TestDashboardExploreOpensSessionPromptWhenLoginUnavailable(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)

	got, cmd := m.handleKey(keyPress('e'))
	if cmd != nil {
		t.Fatal("login recovery prompt should not load posts immediately")
	}
	if got.Page != PageDashboard {
		t.Fatalf("page = %v, want dashboard while login prompt is open", got.Page)
	}
	if got.Dialog != DialogSessionPrompt {
		t.Fatalf("dialog = %v, want session prompt", got.Dialog)
	}
	if !got.SessionDialog.NeedsCredentials() {
		t.Fatal("session prompt should show username/password fields")
	}
}

func TestDashboardExploreOpensAuthChallengeWhenLoginNeedsInput(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	m.Session = SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypePassword,
		ChallengeMessage: "请输入密码",
		Message:          "请输入密码",
	}
	m.AuthDialog = NewAuthChallengeDialog(m.Session)

	got, cmd := m.handleKey(keyPress('e'))
	if cmd != nil {
		t.Fatal("auth challenge should not load posts immediately")
	}
	if got.Page != PageDashboard {
		t.Fatalf("page = %v, want dashboard while auth dialog is open", got.Page)
	}
	if got.Dialog != DialogAuthChallenge {
		t.Fatalf("dialog = %v, want auth challenge", got.Dialog)
	}
	if got.AuthDialog.Kind() != AuthChallengeTypePassword {
		t.Fatalf("auth kind = %v, want password", got.AuthDialog.Kind())
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
