package tui

import (
	"strings"
	"testing"

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
		ConfigDialog: NewConfigDialog(&config.Config{
			Username:  "testuser",
			Password:  "testpass",
			SecretKey: "testkey",
		}),
		LogsDialog: NewLogsDialog(),
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

	if m.Dialog != DialogConfig {
		t.Errorf("Dialog = %v, want DialogConfig", m.Dialog)
	}
	if m.ConfigDialog.FocusIndex() != 0 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 0", m.ConfigDialog.FocusIndex())
	}
	if cmd == nil {
		t.Error("Opening config should trigger loadConfigCmd")
	}
}

func TestHandleKeyOpenLogs(t *testing.T) {
	m := newTestModel()

	result, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result

	if m.Dialog != DialogLogs {
		t.Errorf("Dialog = %v, want DialogLogs", m.Dialog)
	}
	if !m.LogsDialog.Loading() {
		t.Error("LogsDialog.Loading should be true")
	}
	if cmd == nil {
		t.Error("Opening logs should trigger loadLogsCmd")
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

	if m.Page != PageHome {
		t.Errorf("Page = %v, want PageHome", m.Page)
	}
	if m.TabCursor != 0 {
		t.Errorf("TabCursor = %d, want 0", m.TabCursor)
	}
}

func TestHandleKeyTabSwitchBack(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.TabCursor = 0

	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = result

	if m.Page != PagePosts {
		t.Errorf("Page = %v, want PagePosts", m.Page)
	}
	if m.TabCursor != 1 {
		t.Errorf("TabCursor = %d, want 1", m.TabCursor)
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

func TestHandleConfigKeyInput(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{})

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.ConfigDialog.Username() != "a" {
		t.Errorf("ConfigDialog.Username = %s, want 'a'", m.ConfigDialog.Username())
	}

	m.ConfigDialog.setFocus(1)
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.ConfigDialog.Password() != "b" {
		t.Errorf("ConfigDialog.Password = %s, want 'b'", m.ConfigDialog.Password())
	}

	m.ConfigDialog.setFocus(2)
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.ConfigDialog.SecretKey() != "c" {
		t.Errorf("ConfigDialog.SecretKey = %s, want 'c'", m.ConfigDialog.SecretKey())
	}
}

func TestHandleConfigKeyBackspace(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{Username: "test"})

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.ConfigDialog.Username() != "tes" {
		t.Errorf("ConfigDialog.Username = %s, want 'tes'", m.ConfigDialog.Username())
	}
}

func TestHandleConfigKeyAllowsHorizontalCursorMovement(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{Username: "abc"})

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})

	if got := m.ConfigDialog.Username(); got != "abXc" {
		t.Fatalf("ConfigDialog.Username = %q, want %q", got, "abXc")
	}
}

func TestHandleConfigKeyNavigation(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(m.Config)

	// Down should move from field 0 -> 1 -> 2
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.ConfigDialog.FocusIndex() != 1 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 1", m.ConfigDialog.FocusIndex())
	}

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.ConfigDialog.FocusIndex() != 2 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 2", m.ConfigDialog.FocusIndex())
	}

	// Down from field 2 should stay at 2 (capped)
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.ConfigDialog.FocusIndex() != 3 {
		t.Errorf("ConfigDialog.FocusIndex should move to save button, got %d", m.ConfigDialog.FocusIndex())
	}

	// Up should move from save -> field 2 -> 1 -> 0
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ConfigDialog.FocusIndex() != 2 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 2", m.ConfigDialog.FocusIndex())
	}

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ConfigDialog.FocusIndex() != 1 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 1", m.ConfigDialog.FocusIndex())
	}

	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ConfigDialog.FocusIndex() != 0 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 0", m.ConfigDialog.FocusIndex())
	}

	// Up from field 0 should stay at 0
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ConfigDialog.FocusIndex() != 0 {
		t.Errorf("ConfigDialog.FocusIndex should stay at 0, got %d", m.ConfigDialog.FocusIndex())
	}

	// Enter on an input field moves focus to the save button.
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.ConfigDialog.FocusIndex() != 4 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 4", m.ConfigDialog.FocusIndex())
	}

	// From save, up should go back to field 3
	m, _ = m.handleConfigKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.ConfigDialog.FocusIndex() != 3 {
		t.Errorf("ConfigDialog.FocusIndex = %d, want 3", m.ConfigDialog.FocusIndex())
	}
}

func TestHandleConfigKeySave(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog.setFocus(m.ConfigDialog.saveIndex())

	result, cmd := m.handleConfigKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result

	if !m.ConfigDialog.saving {
		t.Error("ConfigDialog.saving should be true")
	}
	if cmd == nil {
		t.Error("Should trigger saveConfigCmd")
	}
}

func TestHandleLogsKeyNavigation(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogLogs
	m.LogsDialog.SetLines([]string{"line1", "line2", "line3", "line4", "line5"})
	m.LogsDialog.offset = 2

	m, _ = m.handleLogsKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.LogsDialog.Offset() != 3 {
		t.Errorf("LogsDialog.Offset = %d, want 3", m.LogsDialog.Offset())
	}

	m, _ = m.handleLogsKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.LogsDialog.Offset() != 2 {
		t.Errorf("LogsDialog.Offset = %d, want 2", m.LogsDialog.Offset())
	}

	m, _ = m.handleLogsKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.LogsDialog.Offset() != 4 {
		t.Errorf("LogsDialog.Offset = %d, want 4", m.LogsDialog.Offset())
	}

	m, _ = m.handleLogsKey(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.LogsDialog.Offset() != 0 {
		t.Errorf("LogsDialog.Offset = %d, want 0", m.LogsDialog.Offset())
	}
}

func TestHandleLogsKeyRefresh(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogLogs

	result, cmd := m.handleLogsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result

	if !m.LogsDialog.Loading() {
		t.Error("LogsDialog.Loading should be true")
	}
	if cmd == nil {
		t.Error("Should trigger loadLogsCmd")
	}
}

func TestHandleDialogEscClose(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig

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

	if !m.ConfigDialog.saveOK {
		t.Error("ConfigDialog.saveOK should be true")
	}
	if m.ConfigDialog.saving {
		t.Error("ConfigDialog.saving should be false")
	}
}

func TestSaveConfigMsgError(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(SaveConfigMsg{Error: errTest})
	m = result.(Model)

	if m.ConfigDialog.saveOK {
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
	if !containsStr(output, "回复:5") {
		t.Error("View() should contain reply count")
	}
	if !containsStr(output, "赞:10") {
		t.Error("View() should contain like count")
	}
	if !containsStr(output, "关:3") {
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

	if !containsStr(output, "赞:0") {
		t.Fatalf("View() should keep praise count separate from follow count, got %q", output)
	}
	if !containsStr(output, "关:9") {
		t.Fatalf("View() should show follow count separately, got %q", output)
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
	if !containsStr(output, "正序") {
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
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{
		Username:  "testuser",
		Password:  "secret",
		SecretKey: "KEY123",
	})
	m.Width = 80
	m.Height = 40

	output := m.View()

	if !containsStr(output, "配置管理") {
		t.Error("View() should show '配置管理'")
	}
	if !containsStr(output, "testuser") {
		t.Error("View() should show username")
	}
	if !containsStr(output, "保存配置") {
		t.Error("View() should show save button")
	}
}

func TestViewConfigDialogMaskedPassword(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{Password: "mypassword"})
	m.Width = 80
	m.Height = 24

	output := m.View()

	if containsStr(output, "mypassword") {
		t.Error("View() should NOT show plaintext password")
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
	m.Dialog = DialogLogs
	m.LogsDialog.SetLines([]string{"2024-01-01 INFO: started", "2024-01-01 INFO: done"})
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "运行日志") {
		t.Error("View() should show '运行日志'")
	}
	if !containsStr(output, "started") {
		t.Error("View() should show log content")
	}
}

func TestViewLogsDialogEmpty(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogLogs
	m.LogsDialog.SetLines(nil)
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "暂无日志") {
		t.Error("View() should show '暂无日志'")
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

func TestMaskField(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		mask     bool
		expected string
	}{
		{"empty no mask", "", false, "(空)"},
		{"empty mask", "", true, "(空)"},
		{"visible", "hello", false, "hello"},
		{"masked", "secret", true, "******"},
		{"single masked", "a", true, "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskField(tt.value, tt.mask)
			if result != tt.expected {
				t.Errorf("maskField(%q, %v) = %q, want %q", tt.value, tt.mask, result, tt.expected)
			}
		})
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
	if result.Dialog != DialogConfig {
		t.Fatalf("dialog = %v, want config", result.Dialog)
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
	if result.Posts.StatusText == "" {
		t.Fatal("expected offline status text after auth challenge escape")
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
