package tui

import (
	"strings"
	"testing"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/models"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	pv := viewport.New(80, 20)
	cv := viewport.New(80, 20)
	posts := NewPostsPageModel()
	posts.PostViewport = &pv
	posts.CommentViewport = &cv
	posts.PostPerPage = 20
	home := NewHomePageModel()
	home.CrawlerState = CrawlerStopped
	home.CrawlMode = CrawlSequential
	home.MonitorPages = 3
	return Model{
		Page:      PagePosts,
		TabCursor: 1,
		Dialog:    DialogNone,
		Home:      home,
		Posts:     posts,
		Width:     80,
		Height:    24,
		Config: &config.Config{
			Username:  "testuser",
			Password:  "testpass",
			SecretKey: "testkey",
		},
		ToolsDialog: NewToolsDialog(&config.Config{
			Username:  "testuser",
			Password:  "testpass",
			SecretKey: "testkey",
		}),
		AuthDialog: NewAuthChallengeDialog(SessionState{}),
		Composer:   NewComposerDialog(),
		TagsDialog: NewTagsDialog(),
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := newTestModel()
	m.Width = 0
	m.Height = 0

	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = result.(Model)

	if m.Width != 100 {
		t.Errorf("Width = %d, want 100", m.Width)
	}
	if m.Height != 30 {
		t.Errorf("Height = %d, want 30", m.Height)
	}
}

func TestUpdateLoginMsgSuccess(t *testing.T) {
	m := newTestModel()
	m.Home.LoggedIn = false

	result, _ := m.Update(LoginMsg{Username: "testuser"})
	m = result.(Model)

	if !m.Home.LoggedIn {
		t.Error("LoggedIn should be true")
	}
	if m.Home.LoginUser != "testuser" {
		t.Errorf("LoginUser = %s, want testuser", m.Home.LoginUser)
	}
}

func TestUpdateLoginMsgError(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(LoginMsg{Error: errTest})
	m = result.(Model)

	if m.Home.LoggedIn {
		t.Error("LoggedIn should be false on error")
	}
	if m.LastError == "" {
		t.Error("LastError should be set")
	}
}

var errTest = &testError{s: "test error"}

type testError struct{ s string }

func (e *testError) Error() string { return e.s }

func TestUpdateLoadPostsMsg(t *testing.T) {
	m := newTestModel()
	m.Posts.PostListLoading = true

	posts := []models.Post{
		{Pid: 1, Text: "Post 1", Timestamp: 1000},
		{Pid: 2, Text: "Post 2", Timestamp: 2000},
	}

	result, _ := m.Update(LoadPostsMsg{Posts: posts, RequestCursor: 0, NextCursor: 0, HasMore: true})
	m = result.(Model)

	if m.Posts.PostListLoading {
		t.Error("PostListLoading should be false after load")
	}
	if len(m.Posts.PostList) != 2 {
		t.Errorf("PostList len = %d, want 2", len(m.Posts.PostList))
	}
	if m.Posts.PostListTotal != 2 {
		t.Errorf("PostListTotal = %d, want loaded post count 2", m.Posts.PostListTotal)
	}
	if m.Posts.PostListError != "" {
		t.Errorf("PostListError should be empty, got: %s", m.Posts.PostListError)
	}
}

func TestUpdateLoadPostsMsgError(t *testing.T) {
	m := newTestModel()
	m.Posts.PostListLoading = true

	result, _ := m.Update(LoadPostsMsg{Error: errTest})
	m = result.(Model)

	if m.Posts.PostListLoading {
		t.Error("PostListLoading should be false")
	}
	if m.Posts.PostListError != "test error" {
		t.Errorf("PostListError = %s, want 'test error'", m.Posts.PostListError)
	}
}

func TestUpdateLoadCommentsMsg(t *testing.T) {
	m := newTestModel()

	comments := []models.Comment{
		{Cid: 1, Text: "Comment 1", Timestamp: 1000},
		{Cid: 2, Text: "Comment 2", Timestamp: 2000},
	}

	result, _ := m.Update(LoadCommentsMsg{Comments: comments, RequestCursor: 0, NextCursor: 0})
	m = result.(Model)

	if len(m.Posts.CommentList) != 2 {
		t.Errorf("CommentList len = %d, want 2", len(m.Posts.CommentList))
	}
}

func TestHandlePostsKeyToggleCommentSortInDetail(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CommentSortAsc = true
	m.Posts.CurrentPost = &models.Post{Pid: 1}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "Comment 1", Timestamp: 1000},
	}

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result

	if cmd == nil {
		t.Error("Should trigger reload of comments with new sort order")
	}
	if !m.Posts.CommentSortAsc {
		t.Error("CommentSortAsc should only change after LoadCommentsMsg")
	}
}

func TestShouldPrefetchCommentsMoreAtBottomWithWrappedShortcut(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 36
	m.Height = 24
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentListHasMore = true
	for i := 0; i < 20; i++ {
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      strings.Repeat("comment body ", 3),
			Timestamp: int32(1100 + i*10),
			NameTag:   "user",
		})
	}

	m.syncPostsPage()
	m.Posts.CommentViewport.GotoBottom()

	if !m.Posts.shouldPrefetchCommentsMore() {
		t.Fatal("should prefetch more comments when viewport is at bottom")
	}
}

func TestUpdateSearchPostsMsg(t *testing.T) {
	m := newTestModel()

	posts := []models.Post{
		{Pid: 1, Text: "Search result", Timestamp: 1000},
	}

	result, _ := m.Update(SearchPostsMsg{Posts: posts, RequestCursor: 0, NextCursor: 0, HasMore: false})
	m = result.(Model)

	if !m.Posts.SearchActive {
		t.Error("SearchActive should be true after search")
	}
	if m.Posts.Searching {
		t.Error("Searching should be false after results")
	}
	if m.Posts.PostListTotal != 1 {
		t.Errorf("PostListTotal = %d, want loaded search result count 1", m.Posts.PostListTotal)
	}
}

func TestUpdateCrawlMsgError(t *testing.T) {
	m := newTestModel()
	m.Home.CrawlerState = CrawlerRunning

	result, _ := m.Update(CrawlMsg{Error: errTest, Page: 1})
	m = result.(Model)

	if m.Home.CrawlerState != CrawlerError {
		t.Errorf("CrawlerState = %v, want CrawlerError", m.Home.CrawlerState)
	}
	if m.Home.HomeLastError != "test error" {
		t.Errorf("HomeLastError = %s, want 'test error'", m.Home.HomeLastError)
	}
}

func TestUpdateCrawlMsgSuccessReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.Home.CrawlerState = CrawlerRunning

	result, cmd := m.Update(CrawlMsg{Page: 3})
	m = result.(Model)

	if cmd == nil {
		t.Error("Expected a cmd to continue crawling")
	}
	if m.Home.LastCrawlPage != 3 {
		t.Errorf("LastCrawlPage = %d, want 3", m.Home.LastCrawlPage)
	}
}

func TestUpdateTickMsg(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(TickMsg{})
	m = result.(Model)

	if cmd == nil {
		t.Error("TickMsg should return a new tick cmd")
	}
}

func TestHandleKeyQuit(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = result

	if cmd == nil {
		t.Error("Ctrl+Q should trigger tea.Quit")
	}
}

func TestHandleKeyQDoesNotQuitOutsideDetail(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = result

	if cmd != nil {
		t.Error("q outside detail view should not trigger any cmd")
	}
}

func TestHandleKeyQuitInDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogHelp

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = result

	if cmd != nil {
		t.Error("q in dialog should NOT trigger any cmd")
	}
	if m.Dialog != DialogHelp {
		t.Error("Dialog should remain open")
	}
}

func TestHandleKeyOpenConfig(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = result

	if m.Dialog != DialogTools || m.ToolsDialog.Section() != ToolsSectionConfig {
		t.Errorf("dialog/section = %v/%v, want tools/config", m.Dialog, m.ToolsDialog.Section())
	}
	if cmd == nil {
		t.Error("Opening config should trigger loadConfigCmd")
	}
}

func TestHandleKeyOpenLogs(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result

	if m.Dialog != DialogTools || m.ToolsDialog.Section() != ToolsSectionLogs {
		t.Errorf("dialog/section = %v/%v, want tools/logs", m.Dialog, m.ToolsDialog.Section())
	}
	if !m.ToolsDialog.Logs.Loading() {
		t.Error("LogsDialog.Loading should be true")
	}
	if cmd == nil {
		t.Error("Opening logs should trigger loadLogsCmd")
	}
}

func TestHandleKeyOpenNotifications(t *testing.T) {
	m := newTestModel()
	m.Client = &client.Client{}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = result

	if m.Dialog != DialogTools || m.ToolsDialog.Section() != ToolsSectionInteractive {
		t.Fatalf("dialog/section = %v/%v, want tools/notifications", m.Dialog, m.ToolsDialog.Section())
	}
	if !m.ToolsDialog.Notifications.Loading() {
		t.Fatal("NotificationDialog.Loading should be true")
	}
	if cmd == nil {
		t.Fatal("opening notifications should trigger loadNotificationsCmd")
	}
}

func TestToolsDialogSwitchesSectionsWithoutNestedTabs(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)

	result, cmd := m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if result.ToolsDialog.Section() != ToolsSectionLogs || cmd == nil {
		t.Fatal("2 should switch to logs and load them")
	}
	result, cmd = result.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if result.ToolsDialog.Section() != ToolsSectionInteractive || cmd == nil {
		t.Fatal("3 should switch to notifications and load them")
	}
	result, cmd = result.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if result.ToolsDialog.Section() != ToolsSectionConfig || cmd != nil {
		t.Fatal("1 should return to the existing config buffer without reloading it")
	}
	result, cmd = result.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if result.ToolsDialog.Section() != ToolsSectionSystem ||
		result.ToolsDialog.Notifications.MessageType() != models.NotificationTypeSystem ||
		cmd == nil {
		t.Fatal("4 should switch to system notifications and load them")
	}
}

func TestToolsDialogDoesNotSwitchSectionsWhileInsertingJSON(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.lines = []string{""}
	m.ToolsDialog.Config.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	result, _ := m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if result.ToolsDialog.Section() != ToolsSectionConfig {
		t.Fatal("section shortcut must be inserted as text in insert mode")
	}
	if result.ToolsDialog.Config.Text() != "2" {
		t.Fatalf("config text = %q, want inserted shortcut rune", result.ToolsDialog.Config.Text())
	}
}

func TestToolsDialogEscapeLeavesInsertBeforeClosing(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if result.Dialog != DialogTools || result.ToolsDialog.Config.Mode() != ConfigEditorNormal {
		t.Fatal("first Esc should leave insert mode and keep the tools dialog open")
	}
	result, _ = result.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if result.Dialog != DialogNone {
		t.Fatal("second Esc should close the tools dialog")
	}
}

func TestHandleNotificationDialogSingleReadOnlyForInteractiveMessages(t *testing.T) {
	m := newTestModel()
	m.Client = &client.Client{}
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionInteractive)
	m.ToolsDialog.Notifications.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 10, Content: "reply"},
	}, 1)

	result, cmd := m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !result.ToolsDialog.Notifications.action {
		t.Fatal("interactive Enter should start a single-read action")
	}

	result.ToolsDialog.Notifications.SetNotifications(models.NotificationTypeSystem, []models.Notification{
		{ID: 11, Content: "system"},
	}, 1)
	result, cmd = result.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("system Enter must not call the single-read endpoint")
	}
}

func TestUpdateNotificationActionMarksRequestedNotificationRead(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionInteractive)
	m.ToolsDialog.Notifications.SetNotifications(models.NotificationTypeInteractive, []models.Notification{
		{ID: 10, Content: "first"},
		{ID: 11, Content: "second"},
	}, 2)
	m.ToolsDialog.Notifications.Update(tea.KeyMsg{Type: tea.KeyDown})

	result, _ := m.Update(NotificationActionMsg{
		MessageType: models.NotificationTypeInteractive,
		ID:          10,
	})
	got := result.(Model)
	if !got.ToolsDialog.Notifications.items[0].Read {
		t.Fatal("requested notification should be marked read")
	}
	if got.ToolsDialog.Notifications.items[1].Read {
		t.Fatal("currently selected notification must not be marked when another ID completed")
	}
}

func TestHandleKeyOpenHelp(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = result

	if m.Dialog != DialogHelp {
		t.Errorf("Dialog = %v, want DialogHelp", m.Dialog)
	}
	if cmd != nil {
		t.Error("Opening help should NOT trigger a cmd")
	}
}

func TestHandleKeyOpenHelpInDetail(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 42, Text: "detail", Timestamp: 1000}

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = result

	if m.Dialog != DialogHelp {
		t.Errorf("Dialog = %v, want DialogHelp", m.Dialog)
	}
	if cmd != nil {
		t.Error("Opening help in detail should NOT trigger a cmd")
	}
}

func TestHandleKeyTabSwitch(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.TabCursor = 1

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = result

	if m.Page != PageSchedule {
		t.Errorf("Page = %v, want PageSchedule", m.Page)
	}
	if m.TabCursor != 2 {
		t.Errorf("TabCursor = %d, want 2", m.TabCursor)
	}
}

func TestHandleKeyTabSwitchBack(t *testing.T) {
	m := newTestModel()
	m.Page = PageScores
	m.TabCursor = 3

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = result

	if m.Page != PageHome {
		t.Errorf("Page = %v, want PageHome", m.Page)
	}
	if m.TabCursor != 0 {
		t.Errorf("TabCursor = %d, want 0", m.TabCursor)
	}
}

func TestHandleHomeKeyStartCrawler(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.CrawlerState = CrawlerStopped
	m.Home.HomeButtonIdx = 0

	result, cmd := m.handleHomeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result

	if m.Home.CrawlerState != CrawlerRunning {
		t.Errorf("CrawlerState = %v, want CrawlerRunning", m.Home.CrawlerState)
	}
	if cmd == nil {
		t.Error("Starting crawler should trigger crawl command")
	}
}

func TestHandleHomeKeyStopCrawler(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.CrawlerState = CrawlerRunning
	m.Home.HomeButtonIdx = 1

	result, _ := m.handleHomeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result

	if m.Home.CrawlerState != CrawlerStopped {
		t.Errorf("CrawlerState = %v, want CrawlerStopped", m.Home.CrawlerState)
	}
}

func TestHandleHomeKeyToggleMode(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.HomeButtonIdx = 2
	m.Home.CrawlMode = CrawlSequential

	result, _ := m.handleHomeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result

	if m.Home.CrawlMode != CrawlMonitor {
		t.Errorf("CrawlMode = %v, want CrawlMonitor", m.Home.CrawlMode)
	}
}

func TestHandleHomeKeyButtonNavigation(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.HomeButtonIdx = 0

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.Home.HomeButtonIdx != 1 {
		t.Errorf("HomeButtonIdx = %d, want 1", m.Home.HomeButtonIdx)
	}

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.Home.HomeButtonIdx != 2 {
		t.Errorf("HomeButtonIdx = %d, want 2", m.Home.HomeButtonIdx)
	}

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.Home.HomeButtonIdx != 2 {
		t.Errorf("HomeButtonIdx should stay at 2, got %d", m.Home.HomeButtonIdx)
	}

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Home.HomeButtonIdx != 1 {
		t.Errorf("HomeButtonIdx = %d, want 1", m.Home.HomeButtonIdx)
	}

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Home.HomeButtonIdx != 0 {
		t.Errorf("HomeButtonIdx = %d, want 0", m.Home.HomeButtonIdx)
	}

	m, _ = m.handleHomeKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Home.HomeButtonIdx != 0 {
		t.Errorf("HomeButtonIdx should stay at 0, got %d", m.Home.HomeButtonIdx)
	}
}

func TestHandlePostsKeySearch(t *testing.T) {
	m := newTestModel()

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = result

	if !m.Posts.Searching {
		t.Error("Searching should be true after /")
	}
	if m.Posts.SearchInput != "" {
		t.Errorf("SearchInput = %s, want empty", m.Posts.SearchInput)
	}
}

func TestHandlePostsKeySearchInput(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = result

	if m.Posts.SearchInput != "t" {
		t.Errorf("SearchInput = %s, want 't'", m.Posts.SearchInput)
	}

	result, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = result

	if m.Posts.SearchInput != "te" {
		t.Errorf("SearchInput = %s, want 'te'", m.Posts.SearchInput)
	}
}

func TestHandlePostsKeySearchInputAllowsCursorMovement(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true
	m.Posts.SearchField = newSearchInput()
	m.Posts.SearchField.SetValue("abc")
	m.Posts.SearchInput = "abc"

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})

	if got := m.Posts.SearchInput; got != "abXc" {
		t.Fatalf("SearchInput after cursor edit = %q, want %q", got, "abXc")
	}
}

func TestHandlePostsKeySearchBackspace(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true
	m.Posts.SearchInput = "test"

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result

	if m.Posts.SearchInput != "tes" {
		t.Errorf("SearchInput = %s, want 'tes'", m.Posts.SearchInput)
	}
}

func TestHandlePostsKeySearchCancel(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true
	m.Posts.SearchInput = "test"
	m.Posts.SearchActive = true
	m.Posts.PostsMode = PostsModeSearchInput

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = result

	if m.Posts.Searching {
		t.Error("Searching should be false after Escape")
	}
	if m.Posts.SearchInput != "" {
		t.Errorf("SearchInput = %s, want empty", m.Posts.SearchInput)
	}
	if !m.Posts.SearchActive {
		t.Error("SearchActive should stay true when canceling input over existing search results")
	}
	if m.Posts.PostsMode != PostsModeSearchResults {
		t.Errorf("PostsMode = %v, want PostsModeSearchResults", m.Posts.PostsMode)
	}
}

func TestHandlePostsKeyNavigation(t *testing.T) {
	m := newTestModel()
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Post 1", Timestamp: 1000},
		{Pid: 2, Text: "Post 2", Timestamp: 2000},
		{Pid: 3, Text: "Post 3", Timestamp: 3000},
		{Pid: 4, Text: "Post 4", Timestamp: 4000},
		{Pid: 5, Text: "Post 5", Timestamp: 5000},
	}
	m.Height = 8
	m.Posts.SelectedPostIdx = 1
	m.syncPostsPage()

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.Posts.SelectedPostIdx != 0 {
		t.Errorf("SelectedPostIdx = %d, want 0", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1 while still inside the same post", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1 on separator line after the current post", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.Posts.SelectedPostIdx != 2 {
		t.Errorf("SelectedPostIdx = %d, want 2 after moving into the next post", m.Posts.SelectedPostIdx)
	}

	if m.Posts.PostViewport.YOffset == 0 {
		t.Error("viewport should start scrolling before the selected post leaves the visible area")
	}
}

func TestHandlePostsKeyEnterDetail(t *testing.T) {
	m := newTestModel()
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Post 1", Timestamp: 1000},
	}
	m.Posts.SelectedPostIdx = 0

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result

	if !m.Posts.ShowPostDetail {
		t.Error("ShowPostDetail should be true")
	}
	if m.Posts.CurrentPost == nil || m.Posts.CurrentPost.Pid != 1 {
		t.Error("CurrentPost should be set to selected post")
	}
	if cmd == nil {
		t.Error("Should trigger loadCommentsCmd")
	}
}

func TestHandlePostsKeyEscFromDetail(t *testing.T) {
	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1}
	m.Posts.CommentList = []models.Comment{{Cid: 1}}

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = result

	if m.Posts.ShowPostDetail {
		t.Error("ShowPostDetail should be false")
	}
	if m.Posts.CurrentPost != nil {
		t.Error("CurrentPost should be nil")
	}
	if len(m.Posts.CommentList) != 0 {
		t.Error("CommentList should be cleared")
	}
}

func TestHandlePostsKeyRefresh(t *testing.T) {
	m := newTestModel()
	m.Posts.PostList = []models.Post{{Pid: 1}}
	m.Posts.PostListTotal = 10
	m.Posts.SearchActive = false

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result

	if !m.Posts.PostListLoading {
		t.Error("PostListLoading should be true")
	}
	if len(m.Posts.PostList) != 0 {
		t.Error("PostList should be cleared on refresh")
	}
	if cmd == nil {
		t.Error("Should trigger loadPostsCmd")
	}
}

func TestHandlePostsKeyRefreshInDetail(t *testing.T) {
	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: "Post", Timestamp: 1000}
	m.Posts.CommentSortAsc = false

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result

	if !m.Posts.CommentListLoading {
		t.Error("CommentListLoading should be true during detail refresh")
	}
	if cmd == nil {
		t.Error("detail refresh should trigger loadPostDetailCmd")
	}
	if m.Posts.CurrentPost == nil || m.Posts.CurrentPost.Pid != 1 {
		t.Error("CurrentPost should remain set before refresh completes")
	}
}

func TestHandlePostsKeyRefreshDisabledDuringSearch(t *testing.T) {
	m := newTestModel()
	m.Posts.SearchActive = true

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result

	if m.Posts.PostListLoading {
		t.Error("PostListLoading should NOT change during search")
	}
	if cmd != nil {
		t.Error("r during search should NOT trigger reload")
	}
}

func TestHandlePostsKeyEscClearsTagFilter(t *testing.T) {
	m := newTestModel()
	m.Posts.ActiveTagID = 12
	m.Posts.ActiveTag = "课程吐槽"
	m.Posts.PostList = []models.Post{{Pid: 1}}
	m.Posts.PostsMode = PostsModeList

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = result

	if m.Posts.ActiveTagID != 0 {
		t.Fatalf("ActiveTagID = %d, want 0", m.Posts.ActiveTagID)
	}
	if m.Posts.ActiveTag != "" {
		t.Fatalf("ActiveTag = %q, want empty", m.Posts.ActiveTag)
	}
	if m.Posts.SearchActive {
		t.Fatal("SearchActive should remain false")
	}
	if !m.Posts.PostListLoading {
		t.Fatal("PostListLoading should be true after clearing filters")
	}
	if cmd == nil {
		t.Fatal("clearing tag filter should trigger reload")
	}
}

func TestHandlePostsKeyEscClearsSearchAndTagFilters(t *testing.T) {
	m := newTestModel()
	m.Posts.SearchActive = true
	m.Posts.SearchInput = "#123 keyword"
	m.Posts.ActiveTagID = 7
	m.Posts.ActiveTag = "课程学业"
	m.Posts.PostsMode = PostsModeSearchResults

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = result

	if m.Posts.SearchActive {
		t.Fatal("SearchActive should be false after Escape")
	}
	if m.Posts.SearchInput != "" {
		t.Fatalf("SearchInput = %q, want empty", m.Posts.SearchInput)
	}
	if m.Posts.ActiveTagID != 0 {
		t.Fatalf("ActiveTagID = %d, want 0", m.Posts.ActiveTagID)
	}
	if m.Posts.ActiveTag != "" {
		t.Fatalf("ActiveTag = %q, want empty", m.Posts.ActiveTag)
	}
	if m.Posts.PostsMode != PostsModeList {
		t.Fatalf("PostsMode = %v, want PostsModeList", m.Posts.PostsMode)
	}
	if !m.Posts.PostListLoading {
		t.Fatal("PostListLoading should be true after clearing filters")
	}
	if cmd == nil {
		t.Fatal("clearing filters should trigger reload")
	}
}

func TestHandlePostsKeyPrefetchMoreBeforeLastLine(t *testing.T) {
	m := newTestModel()
	m.Height = 12
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: strings.Repeat("a\n", 7) + "a", Timestamp: 1000},
		{Pid: 2, Text: strings.Repeat("b\n", 7) + "b", Timestamp: 2000},
	}
	m.Posts.PostListTotal = 10
	m.Posts.PostListHasMore = true
	m.Posts.PostListCursor = 2
	m.syncPostsPage()
	m.Posts.CursorLine = m.Posts.totalPostLines() - 8
	m.Posts.SelectedPostIdx = m.Posts.postIndexAtLine(m.Posts.CursorLine)

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
	m = result

	if !m.Posts.PostListLoading {
		t.Error("PostListLoading should become true when entering the prefetch buffer")
	}
	if cmd == nil {
		t.Error("Should trigger loading more posts before reaching the final line")
	}
}

func TestHandleToolsConfigInsertAndSave(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.lines = strings.Split(`{"username":"a","database":{},"cors":{}}`, "\n")

	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.ToolsDialog.Config.Mode() != ConfigEditorInsert {
		t.Fatal("i should enter insert mode")
	}
	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !m.ToolsDialog.Config.saving {
		t.Fatal("Ctrl+S should start saving valid JSON")
	}
}

func TestHandleLogsKeyNavigation(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)
	m.ToolsDialog.Logs.SetLines([]string{"line1", "line2", "line3", "line4", "line5"})
	m.ToolsDialog.Logs.offset = 2

	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.ToolsDialog.Logs.Offset() != 3 {
		t.Errorf("offset = %d, want 3", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ToolsDialog.Logs.Offset() != 2 {
		t.Errorf("offset = %d, want 2", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.ToolsDialog.Logs.Offset() != 4 {
		t.Errorf("offset = %d, want 4", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.ToolsDialog.Logs.Offset() != 0 {
		t.Errorf("offset = %d, want 0", m.ToolsDialog.Logs.Offset())
	}
}

func TestHandleLogsKeyRefresh(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)

	result, cmd := m.handleToolsDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result

	if !m.ToolsDialog.Logs.Loading() {
		t.Error("LogsDialog.Loading should be true")
	}
	if cmd == nil {
		t.Error("Should trigger loadLogsCmd")
	}
}

func TestHandlePostsKeyOpenImagePanelFromList(t *testing.T) {
	const mediaID = "test-list-image-open"
	writeTestMediaFile(t, mediaID)

	m := newTestModel()
	m.Posts.PostList = []models.Post{
		{Pid: 101, Text: "带图帖子", MediaIds: mediaID},
	}

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if result.Dialog != DialogImage {
		t.Fatalf("dialog = %v, want image dialog", result.Dialog)
	}
	if !result.ImageDialog.HasImages() {
		t.Fatal("image dialog should have images")
	}
	current := result.ImageDialog.Current()
	if current == nil || current.id != mediaID {
		t.Fatalf("current image = %#v, want id %q", current, mediaID)
	}
}

func TestHandlePostsKeyOpenImagePanelFromImageTypePostWithoutMediaIDs(t *testing.T) {
	const pid = "8319759"
	writeTestMediaFile(t, pid)

	m := newTestModel()
	m.Posts.PostList = []models.Post{
		{Pid: 8319759, Type: "image", Text: "走 pid 的图片帖"},
	}

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if result.Dialog != DialogImage {
		t.Fatalf("dialog = %v, want image dialog", result.Dialog)
	}
	current := result.ImageDialog.Current()
	if current == nil || current.id != pid {
		t.Fatalf("current image = %#v, want pid %q", current, pid)
	}
}

func TestHandleImageDialogKeyCyclesImages(t *testing.T) {
	const firstID = "test-detail-image-1"
	const secondID = "test-detail-image-2"
	writeTestMediaFile(t, firstID)
	writeTestMediaFile(t, secondID)

	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 42, Text: "正文"}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "带图评论", MediaIds: firstID + "," + secondID},
	}

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if result.Dialog != DialogImage {
		t.Fatalf("dialog = %v, want image dialog", result.Dialog)
	}
	if got := result.ImageDialog.Current(); got == nil || got.id != firstID {
		t.Fatalf("initial image = %#v, want first image", got)
	}

	result, _ = result.handleImageDialogKey(tea.KeyMsg{Type: tea.KeyRight})
	if got := result.ImageDialog.Current(); got == nil || got.id != secondID {
		t.Fatalf("after right image = %#v, want second image", got)
	}

	result, _ = result.handleImageDialogKey(tea.KeyMsg{Type: tea.KeyLeft})
	if got := result.ImageDialog.Current(); got == nil || got.id != firstID {
		t.Fatalf("after left image = %#v, want first image", got)
	}
}

func TestHandlePostsKeyOpenImagePanelFallsBackToPostWhenCommentImagesUnresolved(t *testing.T) {
	const postPID = "8319760"
	writeTestMediaFile(t, postPID)

	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 8319760, Type: "image", Text: "帖子图"}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Pid: 8319760, Text: "评论里也写了图", MediaIds: "missing-comment-image"},
	}

	result, _ := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if result.Dialog != DialogImage {
		t.Fatalf("dialog = %v, want image dialog", result.Dialog)
	}
	if got := result.ImageDialog.Current(); got == nil || got.id != postPID {
		t.Fatalf("current image = %#v, want post pid image", got)
	}
}

func TestImageDialogViewUsesForegroundZIndex(t *testing.T) {
	dialog := NewImageDialog()
	dialog.Open("图片预览", []resolvedMedia{{id: "1", path: "/tmp/test.jpg"}})

	_, placements := dialog.View(80, 30, true)
	if len(placements) != 1 {
		t.Fatalf("placements = %d, want 1", len(placements))
	}
	if placements[0].z <= 0 {
		t.Fatalf("placement z = %d, want positive foreground layer", placements[0].z)
	}
}

func TestHandleDialogEscClose(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = result

	if m.Dialog != DialogNone {
		t.Errorf("Dialog = %v, want DialogNone", m.Dialog)
	}
}

func TestSaveConfigMsgSuccess(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SaveConfigMsg{})
	m = result.(Model)

	if !m.ToolsDialog.Config.saveOK {
		t.Error("ConfigDialog.saveOK should be true")
	}
	if m.ToolsDialog.Config.saving {
		t.Error("ConfigDialog.saving should be false")
	}
}

func TestSaveConfigMsgError(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SaveConfigMsg{Error: errTest})
	m = result.(Model)

	if m.ToolsDialog.Config.saveOK {
		t.Error("ConfigDialog.saveOK should be false on error")
	}
	if m.LastError != "test error" {
		t.Errorf("LastError = %s, want 'test error'", m.LastError)
	}
}

func TestLoadConfigMsgNilConfig(t *testing.T) {
	m := newTestModel()
	original := m.Config

	result, _ := m.Update(LoadConfigMsg{Config: nil})
	m = result.(Model)

	// nil config should not crash or change fields unexpectedly
	if m.Config != original {
		t.Error("Config should remain unchanged when payload config is nil")
	}
}

func TestViewHomeContainsExpectedText(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.LoggedIn = true
	m.Home.LoginUser = "testuser"
	m.Home.CrawlerState = CrawlerStopped
	m.Home.LastCrawlPage = 5
	m.Width = 80
	m.Height = 24

	output := m.View()

	expectedStrings := []string{
		"TreeHole TUI",
		"已登录",
		"testuser",
		"已停止",
		"上次爬取",
		"第5页",
		"启动爬虫",
		"停止爬虫",
		"顺序爬取",
	}

	for _, s := range expectedStrings {
		if !containsStr(output, s) {
			t.Errorf("View() output missing expected string: %q", s)
		}
	}
}

func TestViewHomeCrawlerRunning(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.CrawlerState = CrawlerRunning
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "运行中") {
		t.Error("View() should show '运行中' when crawler is running")
	}
}

func TestViewHomeCrawlerError(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.CrawlerState = CrawlerError
	m.Home.HomeLastError = "connection timeout"
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "错误") {
		t.Error("View() should show '错误' when crawler has error")
	}
	if !containsStr(output, "connection timeout") {
		t.Error("View() should show the last error message")
	}
}

func TestViewHomeMonitorMode(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.CrawlMode = CrawlMonitor
	m.Home.MonitorPages = 3
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "监控模式") {
		t.Error("View() should show '监控模式' in monitor mode")
	}
	if !containsStr(output, "3") {
		t.Error("View() should show monitor page count")
	}
}

func TestViewPostsEmpty(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = nil
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "帖子列表") {
		t.Error("View() should show '帖子列表' header")
	}
	if !containsStr(output, "暂无数据") {
		t.Error("View() should show '暂无数据' when empty")
	}
}

func TestViewPostsContainsPostText(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Hello World", Timestamp: 1000, Reply: 5, PraiseNum: 10, Likenum: 3, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "Hello World") {
		t.Error("View() should contain post text 'Hello World'")
	}
	if !containsStr(output, "#1") {
		t.Error("View() should contain post pid '#1'")
	}
	if !containsStr(output, "❝ 5") {
		t.Error("View() should contain reply count")
	}
	if !containsStr(output, "♡ 10") {
		t.Error("View() should contain like count")
	}
	if !containsStr(output, "☆ 3") {
		t.Error("View() should contain follow count")
	}
}

func TestViewPostsSeparatesPraiseAndFollowCounts(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Hello World", Timestamp: 1000, Reply: 5, PraiseNum: 0, Likenum: 9, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "♡ 0") {
		t.Fatalf("View() should keep praise count separate from follow count, got %q", output)
	}
	if !containsStr(output, "☆ 9") {
		t.Fatalf("View() should show follow count separately, got %q", output)
	}
}

func TestViewPostsShowsPraiseAndFollowStates(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Hello World", Timestamp: 1000, PraiseNum: 10, Likenum: 3, IsPraise: true, IsFollow: true, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "♥ 10") {
		t.Fatalf("View() should show liked state, got %q", output)
	}
	if !containsStr(output, "★ 3") {
		t.Fatalf("View() should show followed state, got %q", output)
	}
}

func TestViewPostsNonAnonymous(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Real name post", Timestamp: 1000, Anonymous: false},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "实名") {
		t.Error("View() should show '实名' for non-anonymous post")
	}
}

func TestViewPostsSearchActive(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.SearchActive = true
	m.Posts.SearchInput = "test"
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "搜索结果") {
		t.Error("View() should show '搜索结果' when search is active")
	}
	if !containsStr(output, "test") {
		t.Error("View() should show search keyword")
	}
}

func TestViewPostsSearching(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.Searching = true
	m.Posts.SearchInput = "hello"
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "hello") {
		t.Error("View() should show search input when searching")
	}
}

func TestViewPostDetail(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "First comment", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "Second comment", Timestamp: 1200, NameTag: "user2"},
		{Cid: 3, Text: "Reply comment", Timestamp: 1300, NameTag: "user3", Quote: &models.Comment{NameTag: "quoted_user", Text: "quoted text"}},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "#42") {
		t.Error("View() should show post pid")
	}
	if !containsStr(output, "Detail post text") {
		t.Error("View() should show post text")
	}
	if !containsStr(output, "First comment") {
		t.Error("View() should show first comment")
	}
	if !containsStr(output, "Second comment") {
		t.Error("View() should show second comment")
	}
	if !containsStr(output, "quoted_user: quoted text") {
		t.Error("View() should show quoted comment preview")
	}
	if !containsStr(output, "▲") {
		t.Error("View() should show comment sort status")
	}
	if !containsStr(output, "评论") {
		t.Error("View() should show detail focus label in statusline")
	}
}

func TestViewPostDetailEmptyComments(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: "Post", Timestamp: 1000}
	m.Posts.CommentList = nil
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "暂无评论") {
		t.Error("View() should show '暂无评论'")
	}
}

func TestViewConfigDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config = NewConfigDialog(&config.Config{
		Username:  "testuser",
		Password:  "secret",
		SecretKey: "KEY123",
	})
	m.Width = 80
	m.Height = 40

	output := m.View()
	plain := stripANSI(output)

	if strings.Contains(plain, "工具") || !strings.Contains(plain, "配置") {
		t.Fatalf("View() should show tabs without a redundant tools title:\n%s", plain)
	}
	if !strings.Contains(plain, "testuser") {
		t.Error("View() should show username")
	}
	if !strings.Contains(plain, "Ctrl+S") {
		t.Error("View() should show JSON save shortcut")
	}
}

func TestViewConfigDialogShowsEditableJSONPassword(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config = NewConfigDialog(&config.Config{Password: "mypassword"})
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "mypassword") {
		t.Error("JSON editor should show the editable config value")
	}
}

func TestViewHelpDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogHelp
	m.Width = 80
	m.Height = 24

	output := m.View()

	expectedStrings := []string{
		"快捷键",
		"帖子列表",
		"打开配置",
		"搜索帖子",
		"刷新列表",
		"打开详情",
		"Esc",
	}

	for _, s := range expectedStrings {
		if !containsStr(output, s) {
			t.Errorf("View() missing expected string: %q", s)
		}
	}
}

func TestViewLogsDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)
	m.ToolsDialog.Logs.SetLines([]string{"2024-01-01 INFO: started", "2024-01-01 INFO: done"})
	m.Width = 80
	m.Height = 24

	output := m.View()

	if containsStr(output, "运行日志") {
		t.Error("logs page should not repeat its flattened title")
	}
	if !containsStr(output, "started") {
		t.Error("View() should show log content")
	}
}

func TestViewLogsDialogEmpty(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)
	m.ToolsDialog.Logs.SetLines(nil)
	m.Width = 80
	m.Height = 24

	output := m.View()
	plain := stripANSI(output)

	if !strings.Contains(plain, "暂无日志") {
		t.Fatalf("View() should show '暂无日志':\n%s", plain)
	}
}

func TestViewDefaultDimensions(t *testing.T) {
	m := newTestModel()
	m.Width = 0
	m.Height = 0

	output := m.View()

	// Should not panic and should produce output
	if output == "" {
		t.Error("View() should produce output even with zero dimensions")
	}
}

func TestCalcPostViewportHeight(t *testing.T) {
	m := newTestModel()
	m.Height = 24

	h := m.calcPostViewportHeight()
	if h < 3 {
		t.Errorf("calcPostViewportHeight = %d, should be >= 3", h)
	}
}

func TestCalcPostViewportHeightSmall(t *testing.T) {
	m := newTestModel()
	m.Height = 5

	h := m.calcPostViewportHeight()
	if h != 3 {
		t.Errorf("calcPostViewportHeight = %d, want 3 (minimum)", h)
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{100, 100, 100},
	}

	for _, tt := range tests {
		result := minInt(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSessionRefreshSuccessClosesPrompt(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogSessionPrompt
	m.SessionDialog = NewSessionPromptDialog(SessionState{FailureReason: SessionFailureReasonLogin, Message: "expired"})
	result, _ := m.Update(SessionRefreshMsg{State: SessionState{Mode: SessionModeOnline, CanReadOnline: true, CanWriteOnline: true}})
	got := result.(Model)
	if got.Dialog == DialogSessionPrompt {
		t.Fatal("session prompt should close after successful refresh")
	}
	if got.Session.Mode != SessionModeOnline {
		t.Fatalf("session mode = %v, want online", got.Session.Mode)
	}
}

func TestSessionPromptNeedsConfigShowsOpenConfig(t *testing.T) {
	m := newTestModel()
	state := SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}

	result, _ := m.Update(SessionRefreshMsg{State: state})
	got := result.(Model)

	if got.Dialog != DialogSessionPrompt {
		t.Fatalf("dialog = %v, want session prompt", got.Dialog)
	}
	if option := got.SessionDialog.SelectedOption(); option != "打开配置" {
		t.Fatalf("selected option = %q, want 打开配置", option)
	}
}

func TestHandleSessionDialogOpenConfig(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)

	result, cmd := m.handleSessionDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if result.Dialog != DialogTools || result.ToolsDialog.Section() != ToolsSectionConfig {
		t.Fatalf("dialog/section = %v/%v, want tools/config", result.Dialog, result.ToolsDialog.Section())
	}
	if cmd == nil {
		t.Fatal("expected load config command")
	}
}

func TestSessionRefreshSMSChallengeOpensAuthDialog(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SessionRefreshMsg{State: SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypeSMS,
		ChallengeMessage: "需要短信验证",
		Message:          "需要短信验证",
	}})
	got := result.(Model)

	if got.Dialog != DialogAuthChallenge {
		t.Fatalf("dialog = %v, want auth challenge", got.Dialog)
	}
	if got.AuthDialog.Kind() != AuthChallengeTypeSMS {
		t.Fatalf("auth dialog kind = %v, want sms", got.AuthDialog.Kind())
	}
	if !got.AuthDialog.IsSendFocused() {
		t.Fatal("sms auth dialog should focus send button first")
	}
}

func TestSessionRefreshPasswordChallengeOpensAuthDialog(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SessionRefreshMsg{State: SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypePassword,
		ChallengeMessage: "OAuth 登录未返回 token，请输入密码后重试",
		Message:          "OAuth 登录未返回 token，请输入密码后重试",
	}})
	got := result.(Model)

	if got.Dialog != DialogAuthChallenge {
		t.Fatalf("dialog = %v, want auth challenge", got.Dialog)
	}
	if got.AuthDialog.Kind() != AuthChallengeTypePassword {
		t.Fatalf("auth dialog kind = %v, want password", got.AuthDialog.Kind())
	}
}

func TestSessionRefreshUsernameChallengeOpensAuthDialog(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SessionRefreshMsg{State: SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypeUsername,
		ChallengeMessage: "未配置用户名，请输入账号后重试",
		Message:          "未配置用户名，请输入账号后重试",
	}})
	got := result.(Model)

	if got.Dialog != DialogAuthChallenge {
		t.Fatalf("dialog = %v, want auth challenge", got.Dialog)
	}
	if got.AuthDialog.Kind() != AuthChallengeTypeUsername {
		t.Fatalf("auth dialog kind = %v, want username", got.AuthDialog.Kind())
	}
}

func TestHandleAuthChallengeEscFallsBackOffline(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogAuthChallenge
	m.Session = SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypeOTP,
		ChallengeMessage: "需要令牌验证",
	}
	m.AuthDialog = NewAuthChallengeDialog(m.Session)

	result, _ := m.handleAuthChallengeKey(tea.KeyMsg{Type: tea.KeyEscape})

	if result.Dialog != DialogNone {
		t.Fatalf("dialog = %v, want none", result.Dialog)
	}
	if result.Session.Mode != SessionModeOffline {
		t.Fatalf("session mode = %v, want offline", result.Session.Mode)
	}
	if result.ToastMsg == "" {
		t.Fatal("expected offline toast after auth challenge escape")
	}
}

func TestHandleAuthChallengePasswordRequiresNonEmptyValue(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogAuthChallenge
	m.Session = SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypePassword,
		ChallengeMessage: "OAuth 登录未返回 token，请输入密码后重试",
	}
	m.AuthDialog = NewAuthChallengeDialog(m.Session)

	result, cmd := m.handleAuthChallengeKey(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("expected no command when password is empty")
	}
	if result.AuthDialog.errorText != "密码不能为空" {
		t.Fatalf("error text = %q, want 密码不能为空", result.AuthDialog.errorText)
	}
}

func TestHandleAuthChallengeUsernameRequiresNonEmptyValue(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogAuthChallenge
	m.Session = SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypeUsername,
		ChallengeMessage: "未配置用户名，请输入账号后重试",
	}
	m.AuthDialog = NewAuthChallengeDialog(m.Session)

	result, cmd := m.handleAuthChallengeKey(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("expected no command when username is empty")
	}
	if result.AuthDialog.errorText != "用户名不能为空" {
		t.Fatalf("error text = %q, want 用户名不能为空", result.AuthDialog.errorText)
	}
}

func TestSubmitUsernameChallengeCmdRequestsPasswordWhenPasswordMissing(t *testing.T) {
	cfg := &config.Config{}

	msg := submitUsernameChallengeCmd(nil, cfg, "testuser")().(AuthChallengeResultMsg)

	if msg.State.Challenge != AuthChallengeTypePassword {
		t.Fatalf("challenge = %v, want password", msg.State.Challenge)
	}
	if cfg.Username != "testuser" {
		t.Fatalf("cfg username = %q, want testuser", cfg.Username)
	}
}

func TestAuthSMSSentMsgUpdatesDialogStatus(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogAuthChallenge
	m.AuthDialog = NewAuthChallengeDialog(SessionState{Challenge: AuthChallengeTypeSMS})
	m.AuthDialog.SetSubmitting(true)

	result, _ := m.Update(AuthSMSSentMsg{Message: "验证码已发送"})
	got := result.(Model)

	if got.AuthDialog.statusText != "验证码已发送" {
		t.Fatalf("status text = %q, want sent message", got.AuthDialog.statusText)
	}
	if got.AuthDialog.submitting {
		t.Fatal("auth dialog should stop submitting after SMS send result")
	}
}

func TestHandleAuthChallengeEnterOnSMSButtonSendsCode(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogAuthChallenge
	m.Session = SessionState{Challenge: AuthChallengeTypeSMS}
	m.AuthDialog = NewAuthChallengeDialog(m.Session)

	_, cmd := m.handleAuthChallengeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected send-sms command when pressing enter on send button")
	}
	if !m.AuthDialog.IsSendFocused() {
		t.Fatal("send button should remain focused by default")
	}
}

func TestForceOfflineModeClearsLoginState(t *testing.T) {
	m := newTestModel()
	m.Home.LoggedIn = true
	m.forceOfflineMode("network")
	if m.Home.LoggedIn {
		t.Fatal("forceOfflineMode should clear logged-in state")
	}
	if m.Session.Mode != SessionModeOffline {
		t.Fatalf("session mode = %v, want offline", m.Session.Mode)
	}
}

func TestTagSelectionClearsSearchState(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTags
	m.Posts.SearchActive = true
	m.Posts.Searching = true
	m.Posts.SearchInput = "keyword"
	m.TagsDialog.SetTags([]models.Tag{{ID: 1, Label: "课程心得"}})
	result, _ := m.handleTagsDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if result.Posts.SearchActive {
		t.Fatal("tag selection should clear SearchActive")
	}
	if result.Posts.Searching {
		t.Fatal("tag selection should clear Searching")
	}
	if result.Posts.SearchInput != "" {
		t.Fatalf("search input = %q, want empty", result.Posts.SearchInput)
	}
	if result.Posts.ActiveTagID != 1 {
		t.Fatalf("active tag = %d, want 1", result.Posts.ActiveTagID)
	}
}
