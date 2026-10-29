package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/models"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func newTestModel() Model {
	pv := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	cv := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	posts := NewPostsPageModel()
	posts.PostViewport = &pv
	posts.CommentViewport = &cv
	posts.PostPerPage = 20
	home := NewHomePageModel()
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

	result, cmd := m.handlePostsKey(keyPress('s'))
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

func TestUpdateCrawlerJobsLoadedMsg(t *testing.T) {
	m := newTestModel()

	job := newCrawlerJob("全量循环", CrawlerParams{Mode: CrawlSequential, StartPage: 3})
	result, _ := m.Update(crawlerJobsLoadedMsg{Jobs: []CrawlerJob{job}})
	m = result.(Model)

	if len(m.Home.Jobs) != 1 || m.Home.Jobs[0].Name != "全量循环" {
		t.Fatalf("jobs = %+v, want the loaded job", m.Home.Jobs)
	}
	if m.Home.SelectedJob != 0 {
		t.Errorf("SelectedJob = %d, want 0", m.Home.SelectedJob)
	}
}

func TestUpdateCrawlerJobsLoadedMsgError(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(crawlerJobsLoadedMsg{Error: errTest})
	m = result.(Model)

	if len(m.Home.Jobs) != 0 {
		t.Errorf("jobs = %+v, want none on load error", m.Home.Jobs)
	}
	if m.Home.LastError != "test error" {
		t.Errorf("LastError = %q, want %q", m.Home.LastError, "test error")
	}
}

func TestUpdateJobLogLoadedMsgOnlyAppliesToOpenDialog(t *testing.T) {
	m := newTestModel()
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("任务", defaultCrawlerParams())}
	m.Dialog = DialogJobLog
	m.JobLog.Open(m.Home.Jobs[0])

	result, _ := m.Update(jobLogLoadedMsg{JobID: m.Home.Jobs[0].ID, Lines: []string{"日志行"}})
	m = result.(Model)
	if len(m.JobLog.Logs.Lines()) != 1 || m.JobLog.Logs.Lines()[0] != "日志行" {
		t.Fatalf("lines = %v, want the loaded job log", m.JobLog.Logs.Lines())
	}

	other := newCrawlerJob("其他", defaultCrawlerParams())
	result, _ = m.Update(jobLogLoadedMsg{JobID: other.ID, Lines: []string{"别的日志"}})
	m = result.(Model)
	for _, line := range m.JobLog.Logs.Lines() {
		if strings.Contains(line, "别的日志") {
			t.Fatal("job log for another job must not replace the open dialog's content")
		}
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

	result, cmd := m.handleKey(keyPress('q'))
	m = result

	if cmd == nil {
		t.Error("q should trigger tea.Quit")
	}
}

func TestHandleKeyQuitInDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogHelp

	result, cmd := m.handleKey(keyPress('q'))
	m = result

	if cmd != nil {
		t.Error("q in dialog should NOT trigger any cmd")
	}
	if m.Dialog != DialogHelp {
		t.Error("Dialog should remain open")
	}
}

func TestHandleKeyOpenConfigWithLeader(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(keyPress('c'))
	if result.Dialog == DialogTools || cmd != nil {
		t.Fatal("bare c should not open config after config moved behind leader")
	}
	result, cmd = result.handleKey(keyPress(' '))
	if !result.LeaderPending || cmd != nil {
		t.Fatal("space should start leader before opening config")
	}
	result, cmd = result.handleKey(keyPress('c'))
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

	result, cmd := m.handleKey(keyPress(' '))
	if cmd != nil {
		t.Fatal("leader prefix should not emit command")
	}
	if !result.LeaderPending {
		t.Fatal("space should enter leader pending state")
	}
	result, cmd = result.handleKey(keyPress('l'))
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

	result, cmd := m.handleKey(keyPress(' '))
	if cmd != nil {
		t.Fatal("leader prefix should not emit command")
	}
	if !result.LeaderPending {
		t.Fatal("space should enter leader pending state")
	}
	result, cmd = result.handleKey(keyPress('n'))
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

func TestLeaderOpensGlobalToolsAndHelp(t *testing.T) {
	tests := []struct {
		name        string
		key         rune
		wantDialog  DialogType
		wantSection ToolsSection
		wantCmd     bool
	}{
		{name: "config", key: 'c', wantDialog: DialogTools, wantSection: ToolsSectionConfig, wantCmd: true},
		{name: "logs", key: 'l', wantDialog: DialogTools, wantSection: ToolsSectionLogs, wantCmd: true},
		{name: "notifications", key: 'n', wantDialog: DialogTools, wantSection: ToolsSectionInteractive, wantCmd: true},
		{name: "help", key: '?', wantDialog: DialogTools, wantSection: ToolsSectionHelp, wantCmd: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.Client = &client.Client{}

			result, cmd := m.handleKey(keyPress(' '))
			if cmd != nil {
				t.Fatal("leader prefix should not emit command")
			}
			if !result.LeaderPending {
				t.Fatal("space should enter leader pending state")
			}
			result, cmd = result.handleKey(keyPress(tt.key))

			if result.Dialog != tt.wantDialog {
				t.Fatalf("dialog = %v, want %v", result.Dialog, tt.wantDialog)
			}
			if result.Dialog == DialogTools && result.ToolsDialog.Section() != tt.wantSection {
				t.Fatalf("section = %v, want %v", result.ToolsDialog.Section(), tt.wantSection)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Fatalf("cmd present = %v, want %v", cmd != nil, tt.wantCmd)
			}
			if result.LeaderPending {
				t.Fatal("leader state should be cleared after second key")
			}
		})
	}
}

func TestLeaderDoesNotStealSearchInput(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true
	m.Posts.SearchField = newSearchInput()
	_ = m.Posts.SearchField.Focus()

	result, _ := m.handleKey(keyPress(' '))
	if result.LeaderPending {
		t.Fatal("space should not start leader while search input is active")
	}
	if result.Posts.SearchInput != " " {
		t.Fatalf("search input = %q, want a space", result.Posts.SearchInput)
	}

	result, _ = result.handleKey(keyPress(':'))
	if result.CommandActive {
		t.Fatal("colon should not start command while search input is active")
	}
	if result.Posts.SearchInput != " :" {
		t.Fatalf("search input = %q, want space+colon", result.Posts.SearchInput)
	}
}

func TestCommandOpensToolsAndSwitchesPage(t *testing.T) {
	m := newTestModel()
	m.Client = &client.Client{}

	m, _ = m.handleKey(keyPress(':'))
	for _, r := range "logs" {
		m, _ = m.handleKey(keyPress(r))
	}
	result, cmd := m.handleKey(keyCode(tea.KeyEnter))

	if result.CommandActive || result.CommandInput != "" {
		t.Fatal("command input should reset after Enter")
	}
	if result.Dialog != DialogTools || result.ToolsDialog.Section() != ToolsSectionLogs {
		t.Fatalf("dialog/section = %v/%v, want tools/logs", result.Dialog, result.ToolsDialog.Section())
	}
	if cmd == nil {
		t.Fatal(":logs should load logs")
	}

	result, _ = result.handleKey(keyPress(':'))
	for _, r := range "home" {
		result, _ = result.handleKey(keyPress(r))
	}
	result, cmd = result.handleKey(keyCode(tea.KeyEnter))
	if cmd != nil {
		t.Fatal(":home should not load data")
	}
	if result.Page != PageHome || result.Dialog != DialogNone {
		t.Fatalf("page/dialog = %v/%v, want home/no dialog", result.Page, result.Dialog)
	}
}

func TestCommandSearchAndClearPostsFilters(t *testing.T) {
	m := newTestModel()
	m.Provider = &stubPostsProvider{}

	m, _ = m.handleKey(keyPress(':'))
	for _, r := range "search abc" {
		m, _ = m.handleKey(keyPress(r))
	}
	result, cmd := m.handleKey(keyCode(tea.KeyEnter))
	if cmd == nil {
		t.Fatal(":search should trigger search command")
	}
	if !result.Posts.PostListLoading || result.Posts.SearchInput != "abc" {
		t.Fatalf("loading/input = %v/%q, want true/abc", result.Posts.PostListLoading, result.Posts.SearchInput)
	}

	result.Posts.SearchActive = true
	result.Posts.ActiveTagID = 3
	result.Posts.ActiveTag = "tag"
	result, _ = result.handleKey(keyPress(':'))
	for _, r := range "clear" {
		result, _ = result.handleKey(keyPress(r))
	}
	result, cmd = result.handleKey(keyCode(tea.KeyEnter))
	if cmd == nil {
		t.Fatal(":clear should reload posts after clearing filters")
	}
	if result.Posts.SearchActive || result.Posts.ActiveTagID != 0 || result.Posts.ActiveTag != "" {
		t.Fatalf("filters not cleared: search=%v tag=%d/%q", result.Posts.SearchActive, result.Posts.ActiveTagID, result.Posts.ActiveTag)
	}
}

func TestExecuteCrawlerCommandCreatesAndStartsJob(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	installFakeJobLauncher(t)
	m := newTestModel()

	result, cmd := m.executeCommand(`crawler --start-page 3 --pages 5 --loop --loop-delay 7 --page-delay 2 --fetch-images --save-raw-json --post-batch-size 80 --comment-limit 90`)
	if cmd == nil {
		t.Fatal(":crawler should start the new background job")
	}
	if result.Page != PageHome {
		t.Fatalf("page = %v, want PageHome", result.Page)
	}
	if len(result.Home.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(result.Home.Jobs))
	}
	job := result.Home.Jobs[0]
	p := job.Params
	if p.Mode != CrawlSequential {
		t.Fatalf("mode = %v, want sequential", p.Mode)
	}
	if p.StartPage != 3 || p.Pages != 5 {
		t.Fatalf("start/pages = %d/%d, want 3/5", p.StartPage, p.Pages)
	}
	if !p.Loop || p.LoopDelaySeconds != 7 || p.PageDelaySeconds != 2 {
		t.Fatalf("loop=%v delay=%d/%d, want true 7/2", p.Loop, p.LoopDelaySeconds, p.PageDelaySeconds)
	}
	if !p.FetchImages || !p.SaveRawJSON {
		t.Fatal("fetch-images and save-raw-json should be enabled")
	}
	if p.PostBatchSize != 80 || p.CommentLimit != 90 {
		t.Fatalf("batch/comment = %d/%d, want 80/90", p.PostBatchSize, p.CommentLimit)
	}
	if job.PID <= 0 || job.State() != JobRunning {
		t.Fatalf("job pid/state = %d/%v, want a running background process", job.PID, job.State())
	}
	loaded, err := loadCrawlerJobs()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("jobs should be persisted after start, got %d jobs err=%v", len(loaded), err)
	}
}

func TestExecuteCrawlerCommandSupportsFetchThumbnails(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	installFakeJobLauncher(t)
	m := newTestModel()

	result, cmd := m.executeCommand(`crawler fetch-thumbnails --start-id 10 --end-id 20 --convert-webp`)
	if cmd == nil {
		t.Fatal(":crawler fetch-thumbnails should start the background job")
	}
	if len(result.Home.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(result.Home.Jobs))
	}
	job := result.Home.Jobs[0]
	if job.Params.Mode != CrawlFetchThumbnails {
		t.Fatalf("mode = %v, want fetch-thumbnails", job.Params.Mode)
	}
	if job.Params.ThumbnailStartID != 10 || job.Params.ThumbnailEndID != 20 {
		t.Fatalf("thumbnail range = %d-%d, want 10-20", job.Params.ThumbnailStartID, job.Params.ThumbnailEndID)
	}
	if !job.Params.ConvertWebp {
		t.Fatal("convert-webp should be enabled")
	}
	want := []string{"crawler", "fetch-thumbnails", "--start-id", "10", "--end-id", "20", "--convert-webp=true"}
	if got := jobArgs(job); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("jobArgs = %v, want %v", got, want)
	}
}

func TestToolsDialogSwitchesSectionsWithoutNestedTabs(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)

	result, cmd := m.handleToolsDialogKey(keyPress('L'))
	if result.ToolsDialog.Section() != ToolsSectionLogs || cmd == nil {
		t.Fatal("L should switch to logs and load them")
	}
	result, cmd = result.handleToolsDialogKey(keyPress('I'))
	if result.ToolsDialog.Section() != ToolsSectionInteractive || cmd == nil {
		t.Fatal("I should switch to notifications and load them")
	}
	result, cmd = result.handleToolsDialogKey(keyPress('C'))
	if result.ToolsDialog.Section() != ToolsSectionConfig || cmd != nil {
		t.Fatal("C should return to the existing config buffer without reloading it")
	}
	result, cmd = result.handleToolsDialogKey(keyPress('S'))
	if result.ToolsDialog.Section() != ToolsSectionSystem ||
		result.ToolsDialog.Notifications.MessageType() != models.NotificationTypeSystem ||
		cmd == nil {
		t.Fatal("S should switch to system notifications and load them")
	}
	result, cmd = result.handleToolsDialogKey(keyPress('?'))
	if result.ToolsDialog.Section() != ToolsSectionHelp || cmd != nil {
		t.Fatal("? should switch to help without loading")
	}
}

func TestToolsDialogDoesNotSwitchSectionsWhileInsertingJSON(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.lines = []string{""}
	m.ToolsDialog.Config.Update(keyPress('i'))

	result, _ := m.handleToolsDialogKey(keyPress('2'))
	if result.ToolsDialog.Section() != ToolsSectionConfig {
		t.Fatal("section shortcut must be inserted as text in insert mode")
	}
	if result.ToolsDialog.Config.Text() != "2" {
		t.Fatalf("config text = %q, want inserted shortcut rune", result.ToolsDialog.Config.Text())
	}

	result, _ = result.handleToolsDialogKey(keyPress('?'))
	if result.ToolsDialog.Section() != ToolsSectionConfig {
		t.Fatal("help shortcut must be inserted as text in insert mode")
	}
	if result.ToolsDialog.Config.Text() != "2?" {
		t.Fatalf("config text = %q, want inserted shortcut rune", result.ToolsDialog.Config.Text())
	}
}

func TestToolsDialogEscapeLeavesInsertBeforeClosing(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.Update(keyPress('i'))

	result, _ := m.handleKey(keyCode(tea.KeyEscape))
	if result.Dialog != DialogTools || result.ToolsDialog.Config.Mode() != ConfigEditorNormal {
		t.Fatal("first Esc should leave insert mode and keep the tools dialog open")
	}
	result, _ = result.handleKey(keyCode(tea.KeyEscape))
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

	result, cmd := m.handleToolsDialogKey(keyCode(tea.KeyEnter))
	if cmd == nil || !result.ToolsDialog.Notifications.action {
		t.Fatal("interactive Enter should start a single-read action")
	}

	result.ToolsDialog.Notifications.SetNotifications(models.NotificationTypeSystem, []models.Notification{
		{ID: 11, Content: "system"},
	}, 1)
	result, cmd = result.handleToolsDialogKey(keyCode(tea.KeyEnter))
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
	m.ToolsDialog.Notifications.Update(keyCode(tea.KeyDown))

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

	result, cmd := m.handleKey(keyPress('?'))
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

	result, cmd := m.handleKey(keyPress('?'))
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

	result, _ := m.handleKey(keyCode(tea.KeyTab))
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

	result, _ := m.handleKey(keyCode(tea.KeyTab))
	m = result

	if m.Page != PageHome {
		t.Errorf("Page = %v, want PageHome", m.Page)
	}
	if m.TabCursor != 0 {
		t.Errorf("TabCursor = %d, want 0", m.TabCursor)
	}
}

func TestHandleHomeKeyStartsAndStopsJob(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	installFakeJobLauncher(t)
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("测试任务", defaultCrawlerParams())}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusJobs

	m, _ = m.handleHomeKey(keyPress('s'))
	pid := m.Home.Jobs[0].PID
	if pid <= 0 {
		t.Fatal("s should launch the background job and record its pid")
	}
	if m.Home.Jobs[0].State() != JobRunning {
		t.Fatalf("state = %v, want running", m.Home.Jobs[0].State())
	}
	loaded, err := loadCrawlerJobs()
	if err != nil || len(loaded) != 1 || loaded[0].PID != pid {
		t.Fatalf("started job should be persisted, got %+v err=%v", loaded, err)
	}

	m, _ = m.handleHomeKey(keyPress('s'))
	if !m.Home.Jobs[0].Stopping {
		t.Fatal("pressing s again should send SIGTERM and mark the job stopping")
	}
	deadline := time.Now().Add(3 * time.Second)
	for jobAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if jobAlive(pid) {
		t.Fatal("job process should exit after SIGTERM")
	}
	m.Home.RefreshJobs()
	if m.Home.Jobs[0].State() != JobFinished {
		t.Fatalf("state = %v, want finished after the process exits", m.Home.Jobs[0].State())
	}
}

func TestHandleHomeKeyNewJobFocusesParams(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.Focus = HomeFocusJobs

	m, _ = m.handleHomeKey(keyPress('n'))
	if len(m.Home.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1 after n", len(m.Home.Jobs))
	}
	if m.Home.SelectedJob != 0 || m.Home.Focus != HomeFocusParams {
		t.Fatalf("selection/focus = %d/%v, want 0/params", m.Home.SelectedJob, m.Home.Focus)
	}
	if m.Home.Jobs[0].Name != "新任务 1" {
		t.Fatalf("job name = %q, want 新任务 1", m.Home.Jobs[0].Name)
	}
}

func TestHandleHomeKeyDeleteRequiresConfirmation(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{
		newCrawlerJob("一", defaultCrawlerParams()),
		newCrawlerJob("二", defaultCrawlerParams()),
	}
	m.Home.SelectedJob = 1
	m.Home.Focus = HomeFocusJobs

	m, _ = m.handleHomeKey(keyPress('x'))
	if len(m.Home.Jobs) != 2 {
		t.Fatal("first x must not delete the job")
	}
	if m.Home.PendingDeleteID != m.Home.Jobs[1].ID {
		t.Fatal("first x should arm the delete confirmation")
	}
	// Expired confirmation must re-arm instead of deleting.
	m.Home.PendingDeleteAt = time.Now().Add(-deleteConfirmWindow - time.Second)
	m, _ = m.handleHomeKey(keyPress('x'))
	if len(m.Home.Jobs) != 2 {
		t.Fatal("expired confirmation must not delete the job")
	}
	m, _ = m.handleHomeKey(keyPress('x'))
	if len(m.Home.Jobs) != 1 || m.Home.Jobs[0].Name != "一" {
		t.Fatalf("second x within the window should delete the selected job, jobs = %+v", m.Home.Jobs)
	}
	if m.Home.PendingDeleteID != "" {
		t.Fatal("confirmation state should reset after deletion")
	}
}

func TestHomeActionToasts(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	installFakeJobLauncher(t)
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("一", defaultCrawlerParams())}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusParams

	m, _ = m.handleHomeKey(keyCtrl('s'))
	if m.ToastMsg != "任务已保存" {
		t.Fatalf("Ctrl+S toast = %q, want 任务已保存", m.ToastMsg)
	}

	m, _ = m.handleHomeKey(keyCode(tea.KeyEscape))
	m, _ = m.handleHomeKey(keyPress('s'))
	if !strings.Contains(m.ToastMsg, "任务已启动 (pid ") {
		t.Fatalf("start toast = %q, want 任务已启动 (pid ...)", m.ToastMsg)
	}
	m, _ = m.handleHomeKey(keyPress('s'))
	if !strings.Contains(m.ToastMsg, "已发送停止信号") {
		t.Fatalf("stop toast = %q, want 已发送停止信号", m.ToastMsg)
	}

	m.Home.Jobs[0].PID = 0
	m, _ = m.handleHomeKey(keyPress('x'))
	if m.ToastMsg != "再按一次 x 确认删除该任务" {
		t.Fatalf("delete confirm toast = %q", m.ToastMsg)
	}
	m, _ = m.handleHomeKey(keyPress('x'))
	if len(m.Home.Jobs) != 0 || m.ToastMsg != "任务已保存" {
		t.Fatalf("delete left %d jobs with toast %q", len(m.Home.Jobs), m.ToastMsg)
	}
}

func TestHandleHomeKeyOpensJobLog(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("任务", defaultCrawlerParams())}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusJobs

	result, cmd := m.handleHomeKey(keyPress('L'))
	if result.Dialog != DialogJobLog {
		t.Fatalf("dialog = %v, want DialogJobLog", result.Dialog)
	}
	if result.JobLog.JobID != m.Home.Jobs[0].ID {
		t.Fatalf("job log opened for %q, want %q", result.JobLog.JobID, m.Home.Jobs[0].ID)
	}
	if cmd == nil {
		t.Fatal("opening the job log should load the log file")
	}
}

func TestJobLogDialogShowsTail(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	job := newCrawlerJob("任务", defaultCrawlerParams())
	path, err := jobLogPath(job)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 1; i <= 600; i++ {
		fmt.Fprintf(&b, "[Crawler] 第 %d 页完成\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readJobLogLines(job.ID, 500)
	if err != nil {
		t.Fatalf("readJobLogLines: %v", err)
	}
	if len(lines) != 500 {
		t.Fatalf("lines = %d, want the last 500", len(lines))
	}
	if !strings.Contains(lines[0], "第 101 页完成") || !strings.Contains(lines[len(lines)-1], "第 600 页完成") {
		t.Fatalf("tail window = %q .. %q, want 101..600", lines[0], lines[len(lines)-1])
	}

	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{job}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusJobs
	result, _ := m.handleHomeKey(keyPress('L'))
	loaded, _ := result.Update(loadJobLogCmd(job.ID)())
	result = loaded.(Model)
	if result.Dialog != DialogJobLog || len(result.JobLog.Logs.Lines()) != 500 {
		t.Fatalf("dialog lines = %d, want 500", len(result.JobLog.Logs.Lines()))
	}
	if out := viewString(result); !containsStr(out, "任务日志") || !containsStr(out, "第 101 页完成") {
		t.Errorf("job log dialog should render the tail window, got:\n%s", out)
	}
}

func TestHomeParamEditingAndInlineInput(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("任务", defaultCrawlerParams())}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusParams
	m.Home.ParamRow = 0

	rows := m.Home.paramRows()
	modeRow, boolRowIdx, startRow, batchRow := -1, -1, -1, -1
	for i, row := range rows {
		switch {
		case row.Kind == paramRowMode:
			modeRow = i
		case row.Kind == paramRowBool && boolRowIdx == -1:
			boolRowIdx = i
		case row.Kind == paramRowInt && row.Label == "起始页":
			startRow = i
		case row.Kind == paramRowInt && row.Label == "帖子批次":
			batchRow = i
		}
	}
	if modeRow < 0 || boolRowIdx < 0 || startRow < 0 || batchRow < 0 {
		t.Fatalf("param rows missing expected entries: %+v", rows)
	}

	m.Home.ParamRow = boolRowIdx
	boolBefore := *rows[boolRowIdx].Bool(&m.Home.Jobs[0].Params)
	m, _ = m.handleHomeKey(keyPress(' '))
	if *rows[boolRowIdx].Bool(&m.Home.Jobs[0].Params) == boolBefore {
		t.Fatalf("Space should toggle %q", rows[boolRowIdx].Label)
	}

	m.Home.ParamRow = modeRow
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	if m.Home.Jobs[0].Params.Mode != CrawlMonitor {
		t.Fatalf("mode = %v, want monitor after Enter on the mode row", m.Home.Jobs[0].Params.Mode)
	}
	m.Home.Jobs[0].Params.Mode = CrawlSequential
	m.Home.ParamRow = startRow
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	if !m.Home.ParamEditing() {
		t.Fatal("Enter on an int row should focus the inline input")
	}
	m.Home.Input.SetValue("500")
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	if m.Home.ParamEditing() {
		t.Fatal("Enter should commit the inline edit")
	}
	if got := m.Home.Jobs[0].Params.StartPage; got != 500 {
		t.Fatalf("StartPage = %d, want 500", got)
	}

	// Out-of-range input is clamped on commit (帖子批次 is bounded to 1..100).
	m.Home.ParamRow = batchRow
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	m.Home.Input.SetValue("99999")
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	if got := m.Home.Jobs[0].Params.PostBatchSize; got != 100 {
		t.Fatalf("PostBatchSize = %d, want clamped to 100", got)
	}

	// Esc cancels without applying.
	m, _ = m.handleHomeKey(keyCode(tea.KeyEnter))
	kept := m.Home.Jobs[0].Params.StartPage
	m.Home.Input.SetValue("7")
	m, _ = m.handleHomeKey(keyCode(tea.KeyEscape))
	if m.Home.ParamEditing() {
		t.Fatal("Esc should leave the inline edit")
	}
	if m.Home.Jobs[0].Params.StartPage != kept {
		t.Fatalf("Esc must not apply the edit, got %d want %d", m.Home.Jobs[0].Params.StartPage, kept)
	}
}

func TestHandlePostsKeySearch(t *testing.T) {
	m := newTestModel()

	result, _ := m.handlePostsKey(keyPress('/'))
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

	result, _ := m.handlePostsKey(keyPress('t'))
	m = result

	if m.Posts.SearchInput != "t" {
		t.Errorf("SearchInput = %s, want 't'", m.Posts.SearchInput)
	}

	result, _ = m.handlePostsKey(keyPress('e'))
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

	m, _ = m.handlePostsKey(keyCode(tea.KeyLeft))
	m, _ = m.handlePostsKey(keyPress('X'))

	if got := m.Posts.SearchInput; got != "abXc" {
		t.Fatalf("SearchInput after cursor edit = %q, want %q", got, "abXc")
	}
}

func TestHandlePostsKeySearchBackspace(t *testing.T) {
	m := newTestModel()
	m.Posts.Searching = true
	m.Posts.SearchInput = "test"

	result, _ := m.handlePostsKey(keyCode(tea.KeyBackspace))
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

	result, _ := m.handlePostsKey(keyCode(tea.KeyEscape))
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

	m, _ = m.handlePostsKey(keyCode(tea.KeyUp))
	if m.Posts.SelectedPostIdx != 0 {
		t.Errorf("SelectedPostIdx = %d, want 0", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(keyCode(tea.KeyDown))
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(keyCode(tea.KeyDown))
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1 while still inside the same post", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(keyCode(tea.KeyDown))
	if m.Posts.SelectedPostIdx != 1 {
		t.Errorf("SelectedPostIdx = %d, want 1 on separator line after the current post", m.Posts.SelectedPostIdx)
	}

	m, _ = m.handlePostsKey(keyCode(tea.KeyDown))
	if m.Posts.SelectedPostIdx != 2 {
		t.Errorf("SelectedPostIdx = %d, want 2 after moving into the next post", m.Posts.SelectedPostIdx)
	}

	if m.Posts.PostViewport.YOffset() == 0 {
		t.Error("viewport should start scrolling before the selected post leaves the visible area")
	}
}

func TestHandlePostsKeyEnterDetail(t *testing.T) {
	m := newTestModel()
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Post 1", Timestamp: 1000},
	}
	m.Posts.SelectedPostIdx = 0

	result, cmd := m.handlePostsKey(keyCode(tea.KeyEnter))
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

	result, _ := m.handlePostsKey(keyCode(tea.KeyEscape))
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

	result, cmd := m.handlePostsKey(keyPress('r'))
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

	result, cmd := m.handlePostsKey(keyPress('r'))
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
	m.Posts.SearchInput = "test"

	result, cmd := m.handlePostsKey(keyPress('r'))
	m = result

	if !m.Posts.PostListLoading {
		t.Error("PostListLoading should be set during search refresh")
	}
	if cmd == nil {
		t.Error("r during search should trigger search reload")
	}
}

func TestHandlePostsKeyEscClearsTagFilter(t *testing.T) {
	m := newTestModel()
	m.Posts.ActiveTagID = 12
	m.Posts.ActiveTag = "课程吐槽"
	m.Posts.PostList = []models.Post{{Pid: 1}}
	m.Posts.PostsMode = PostsModeList

	result, cmd := m.handlePostsKey(keyCode(tea.KeyEscape))
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

	result, cmd := m.handlePostsKey(keyCode(tea.KeyEscape))
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

	result, cmd := m.handlePostsKey(keyCode(tea.KeyDown))
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

	m, _ = m.handleToolsDialogKey(keyPress('i'))
	if m.ToolsDialog.Config.Mode() != ConfigEditorInsert {
		t.Fatal("i should enter insert mode")
	}
	m, _ = m.handleToolsDialogKey(keyCtrl('s'))
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

	m, _ = m.handleToolsDialogKey(keyCode(tea.KeyDown))
	if m.ToolsDialog.Logs.Offset() != 3 {
		t.Errorf("offset = %d, want 3", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(keyCode(tea.KeyUp))
	if m.ToolsDialog.Logs.Offset() != 2 {
		t.Errorf("offset = %d, want 2", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(keyCode(tea.KeyPgDown))
	if m.ToolsDialog.Logs.Offset() != 4 {
		t.Errorf("offset = %d, want 4", m.ToolsDialog.Logs.Offset())
	}

	m, _ = m.handleToolsDialogKey(keyCode(tea.KeyPgUp))
	if m.ToolsDialog.Logs.Offset() != 0 {
		t.Errorf("offset = %d, want 0", m.ToolsDialog.Logs.Offset())
	}
}

func TestHandleLogsKeyRefresh(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)

	result, cmd := m.handleToolsDialogKey(keyPress('r'))
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

	result, _ := m.handlePostsKey(keyPress('o'))

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

	result, _ := m.handlePostsKey(keyPress('o'))

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

	result, _ := m.handlePostsKey(keyPress('o'))
	if result.Dialog != DialogImage {
		t.Fatalf("dialog = %v, want image dialog", result.Dialog)
	}
	if got := result.ImageDialog.Current(); got == nil || got.id != firstID {
		t.Fatalf("initial image = %#v, want first image", got)
	}

	result, _ = result.handleImageDialogKey(keyCode(tea.KeyRight))
	if got := result.ImageDialog.Current(); got == nil || got.id != secondID {
		t.Fatalf("after right image = %#v, want second image", got)
	}

	result, _ = result.handleImageDialogKey(keyCode(tea.KeyLeft))
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

	result, _ := m.handlePostsKey(keyPress('o'))
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

	result, _ := m.handleKey(keyCode(tea.KeyEscape))
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
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("全量循环", CrawlerParams{
		Mode: CrawlSequential, StartPage: 5, Pages: 10, PostBatchSize: 200, CommentLimit: 200, ConvertWebp: true,
	})}
	m.Home.SelectedJob = 0
	m.Width = 80
	m.Height = 24

	output := viewString(m)

	expectedStrings := []string{
		"TreeHole TUI",
		"已登录",
		"testuser",
		"后台任务",
		"0 运行中 / 共 1",
		"全量循环",
		"未启动",
		"参数 — 全量循环",
		"起始页",
		"n 新建",
		"Enter 进入参数编辑",
	}

	for _, s := range expectedStrings {
		if !containsStr(output, s) {
			t.Errorf("View() output missing expected string: %q", s)
		}
	}
}

func TestViewHomeShowsRunningJob(t *testing.T) {
	dir := t.TempDir()
	useTempWorkingDir(t, dir)
	m := newTestModel()
	m.Page = PageHome
	job := newCrawlerJob("全量", defaultCrawlerParams())
	job.PID = spawnFakeCrawler(t)
	job.StartedAt = time.Now()
	m.Home.Jobs = []CrawlerJob{job}
	m.Home.SelectedJob = 0
	m.Width = 80
	m.Height = 24

	output := viewString(m)

	if !containsStr(output, "运行中") {
		t.Error("View() should show '运行中' when a job process is alive")
	}
	if !containsStr(output, "1 运行中 / 共 1") {
		t.Error("View() should count the running job")
	}
	if !containsStr(output, fmt.Sprintf("pid %d", job.PID)) {
		t.Errorf("View() should show the pid, output:\n%s", output)
	}
}

func TestViewHomeWarnsAboutConcurrentWriters(t *testing.T) {
	useTempWorkingDir(t, t.TempDir())
	m := newTestModel()
	m.Page = PageHome
	for i := 0; i < 2; i++ {
		job := newCrawlerJob(fmt.Sprintf("任务%d", i+1), defaultCrawlerParams())
		job.PID = spawnFakeCrawler(t)
		job.StartedAt = time.Now()
		m.Home.Jobs = append(m.Home.Jobs, job)
	}
	m.Home.SelectedJob = 0
	m.Width = 80
	m.Height = 30

	output := viewString(m)
	if !containsStr(output, "2 运行中 / 共 2") {
		t.Errorf("View() should count both running jobs, got:\n%s", output)
	}
	if !containsStr(output, "写锁") {
		t.Error("View() should warn about concurrent SQLite writers")
	}
}

func TestViewHomeShowsLastError(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.LastError = "启动失败: 权限不足"
	m.Width = 80
	m.Height = 24

	output := viewString(m)

	if !containsStr(output, "错误") {
		t.Error("View() should show '错误' when the page has an error")
	}
	if !containsStr(output, "启动失败: 权限不足") {
		t.Error("View() should show the last error message")
	}
}

func TestViewHomeMonitorMode(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.Jobs = []CrawlerJob{newCrawlerJob("监控", CrawlerParams{
		Mode: CrawlMonitor, MonitorPages: 3, PostBatchSize: 200, CommentLimit: 200, ConvertWebp: true,
	})}
	m.Home.SelectedJob = 0
	m.Width = 80
	m.Height = 24

	output := viewString(m)

	if !containsStr(output, "监控前 3 页") {
		t.Errorf("View() should summarize the monitor job, output:\n%s", output)
	}
	if !containsStr(output, "● 监控") {
		t.Error("View() should mark the monitor mode as selected")
	}
}

func TestViewPostsEmpty(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = nil
	m.Width = 80
	m.Height = 24

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)

	if !containsStr(output, "搜索") {
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

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)
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

	output := viewString(m)

	if !containsStr(output, "mypassword") {
		t.Error("JSON editor should show the editable config value")
	}
}

func TestViewHelpDialog(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogHelp
	m.Width = 80
	m.Height = 24

	output := viewString(m)

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

	output := viewString(m)

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

	output := viewString(m)
	plain := stripANSI(output)

	if !strings.Contains(plain, "暂无日志") {
		t.Fatalf("View() should show '暂无日志':\n%s", plain)
	}
}

func TestViewDefaultDimensions(t *testing.T) {
	m := newTestModel()
	m.Width = 0
	m.Height = 0

	output := viewString(m)

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

func TestSessionPromptNeedsConfigShowsCredentialForm(t *testing.T) {
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
	if !got.SessionDialog.NeedsCredentials() {
		t.Fatal("session prompt should show credential form")
	}
}

func TestHandleSessionDialogSubmitsCredentialForm(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)
	m.SessionDialog.Update(keyPress('a'))
	m.SessionDialog.Update(keyPress('l'))
	m.SessionDialog.Update(keyPress('i'))
	m.SessionDialog.Update(keyPress('c'))
	m.SessionDialog.Update(keyPress('e'))
	m.SessionDialog.Update(keyCode(tea.KeyTab))
	m.SessionDialog.Update(keyPress('s'))
	m.SessionDialog.Update(keyPress('e'))
	m.SessionDialog.Update(keyPress('c'))
	m.SessionDialog.Update(keyPress('r'))
	m.SessionDialog.Update(keyPress('e'))
	m.SessionDialog.Update(keyPress('t'))

	result, cmd := m.handleSessionDialogKey(keyCode(tea.KeyEnter))
	if result.Config.Username != "alice" || result.Config.Password != "secret" {
		t.Fatalf("config credentials = %q/%q, want alice/secret", result.Config.Username, result.Config.Password)
	}
	if cmd == nil {
		t.Fatal("expected save/login command")
	}
}

func TestAuthChallengeResultKeepsSMSChallengeInCredentialForm(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)

	result, _ := m.Update(AuthChallengeResultMsg{State: SessionState{
		Mode:             SessionModeOffline,
		Challenge:        AuthChallengeTypeSMS,
		ChallengeMessage: "需要短信验证码",
		Message:          "需要短信验证码",
	}})
	got := result.(Model)

	if got.Dialog != DialogSessionPrompt {
		t.Fatalf("dialog = %v, want session prompt", got.Dialog)
	}
	if got.SessionDialog.Challenge() != AuthChallengeTypeSMS {
		t.Fatalf("session challenge = %v, want sms", got.SessionDialog.Challenge())
	}
	if output := stripANSI(got.SessionDialog.View(80)); !strings.Contains(output, "输入短信验证码") || !strings.Contains(output, "发送验证码") {
		t.Fatalf("session prompt should append sms challenge controls:\n%s", output)
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

	result, _ := m.handleAuthChallengeKey(keyCode(tea.KeyEscape))

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

	result, cmd := m.handleAuthChallengeKey(keyCode(tea.KeyEnter))

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

	result, cmd := m.handleAuthChallengeKey(keyCode(tea.KeyEnter))

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

	_, cmd := m.handleAuthChallengeKey(keyCode(tea.KeyEnter))
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
	m.Session.FailureReason = SessionFailureReasonLogin
	m.Session.NeedsConfig = true
	m.Session.Challenge = AuthChallengeTypePassword
	m.forceOfflineMode("network")
	if m.Home.LoggedIn {
		t.Fatal("forceOfflineMode should clear logged-in state")
	}
	if m.Session.Mode != SessionModeOffline {
		t.Fatalf("session mode = %v, want offline", m.Session.Mode)
	}
	if m.Session.FailureReason != SessionFailureReasonNone || m.Session.NeedsConfig || m.Session.Challenge != AuthChallengeTypeNone {
		t.Fatalf("offline mode should clear login recovery state: %+v", m.Session)
	}
}

func TestSessionPromptOfflineModeDoesNotAskForCredentialsAgain(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)
	m.SessionDialog.Update(keyCode(tea.KeyTab))
	m.SessionDialog.Update(keyCode(tea.KeyTab))

	result, cmd := m.handleSessionDialogKey(keyCode(tea.KeyEnter))
	if result.Dialog != DialogNone || result.Session.FailureReason != SessionFailureReasonNone || result.Session.NeedsConfig {
		t.Fatalf("offline selection should close dialog and clear login state: dialog=%v session=%+v", result.Dialog, result.Session)
	}
	if cmd == nil {
		t.Fatal("offline selection should load local posts")
	}

	result.Page = PageDashboard
	result.Posts.PostListLoading = false
	next, cmd := result.handleKey(keyPress('e'))
	if next.Dialog != DialogNone || next.Page != PagePosts || cmd == nil {
		t.Fatalf("offline explore should enter posts without login prompt: page=%v dialog=%v cmd=%v", next.Page, next.Dialog, cmd)
	}
}

func TestTagSelectionClearsSearchState(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTags
	m.Posts.SearchActive = true
	m.Posts.Searching = true
	m.Posts.SearchInput = "keyword"
	m.TagsDialog.SetTags([]models.Tag{{ID: 1, Label: "课程心得"}})
	result, _ := m.handleTagsDialogKey(keyCode(tea.KeyEnter))
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
