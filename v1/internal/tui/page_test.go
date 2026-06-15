package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "../..")
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	return stripANSISequences(s)
}

// frameLines returns the full stripped frame lines, preserving blank rows.
func frameLines(output string) []string {
	stripped := stripANSI(output)
	stripped = strings.TrimSuffix(stripped, "\n")
	return strings.Split(stripped, "\n")
}

// visibleLines returns the non-empty lines from a stripped output string.
func visibleLines(output string) []string {
	var lines []string
	for _, line := range frameLines(output) {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// loadRealPosts loads posts from the real treehole.db for testing.
func loadRealPosts(t *testing.T) []models.Post {
	t.Helper()

	dbPath := filepath.Join(projectRoot(), "treehole.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("treehole.db not found, skipping real data test")
	}

	cfg := &config.Config{
		Username:  "test",
		Password:  "test",
		SecretKey: "test",
		Database: config.DatabaseConfig{
			Type:   "sqlite3",
			DBFile: dbPath,
		},
	}

	database, err := db.NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer database.Close()

	posts, err := database.GetPostsCursor(0, 50, false)
	if err != nil {
		t.Fatalf("GetPostsCursor: %v", err)
	}

	if len(posts) == 0 {
		t.Skip("no posts returned from treehole.db")
	}

	return posts
}

func TestViewPostsRealDataOverflow(t *testing.T) {
	posts := loadRealPosts(t)

	// Find the longest post
	var longest models.Post
	for _, p := range posts {
		if len(p.Text) > len(longest.Text) {
			longest = p
		}
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{longest}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	// Should not panic or produce empty output
	if output == "" {
		t.Fatal("View() returned empty string")
	}

	// Should contain the post text (at least the first line)
	firstLine := strings.Split(longest.Text, "\n")[0]
	if firstLine != "" && !containsStr(output, strings.TrimSpace(firstLine)) {
		t.Errorf("View() missing first line of long post: %q", firstLine)
	}

	t.Logf("Rendered post pid=%d, text_len=%d, output_len=%d",
		longest.Pid, len(longest.Text), len(output))
}

func TestViewPostsRealDataMultiLine(t *testing.T) {
	posts := loadRealPosts(t)

	// Find the post with most newlines
	var mostLines models.Post
	maxNewlines := 0
	for _, p := range posts {
		nl := strings.Count(p.Text, "\n")
		if nl > maxNewlines {
			maxNewlines = nl
			mostLines = p
		}
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{mostLines}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string")
	}

	// Verify multiline content is rendered
	lines := strings.Split(mostLines.Text, "\n")
	renderedCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && containsStr(output, trimmed) {
			renderedCount++
		}
	}

	t.Logf("Post pid=%d has %d lines, %d lines found in output",
		mostLines.Pid, len(lines), renderedCount)

	if renderedCount == 0 {
		t.Error("No multiline content found in output")
	}
}

func TestViewPostsExtremeLongText(t *testing.T) {
	// Create a post with extremely long single line (2000 chars)
	longLine := strings.Repeat("A", 2000)

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: longLine, Timestamp: 1000, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Should not panic
	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for extreme long text")
	}

	// The viewport should handle overflow gracefully
	t.Logf("Output length for 2000-char line: %d", len(output))
}

func TestBuildPostListContentWrapsToViewportWidth(t *testing.T) {
	longLine := strings.Repeat("A", 50)

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: longLine, Timestamp: 1000, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 28
	m.Height = 12
	m.syncPostsPage()

	contentWidth := m.Posts.currentListContentWidth()
	content, _ := m.Posts.buildPostListContent(contentWidth)
	stripped := stripANSI(content)

	if !strings.Contains(stripped, strings.Repeat("A", 16)) {
		t.Fatal("wrapped content missing expected first chunk")
	}
	if strings.Contains(stripped, longLine) {
		t.Fatal("long line should be split before rendering")
	}

	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > contentWidth {
			t.Fatalf("rendered line width = %d, want <= %d: %q", w, contentWidth, line)
		}
	}

	if got := m.Posts.postRenderedLinesAt(0); got < 5 {
		t.Fatalf("postRenderedLinesAt(0) = %d, want at least 5 after wrapping", got)
	}
}

func TestBuildPostListContentNormalizesKeycapSequences(t *testing.T) {
	if got := normalizeRenderedText("q22⑦76️⃣8545"); got != "q22⑦768545" {
		t.Fatalf("normalizeRenderedText() = %q, want %q", got, "q22⑦768545")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "求一份lgh中国现代文学史笔记，价格您定+q22⑦76️⃣8545", Timestamp: 1000, Anonymous: true},
		{Pid: 2, Text: "next post", Timestamp: 1001, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 58
	m.Height = 12
	m.syncPostsPage()

	contentWidth := m.Posts.currentListContentWidth()
	contentRaw, _ := m.Posts.buildPostListContent(contentWidth)
	content := stripANSI(contentRaw)

	if strings.Contains(content, "\uFE0F") || strings.Contains(content, "\u20E3") {
		t.Fatalf("rendered content should strip keycap combining runes, got %q", content)
	}
	if !strings.Contains(content, "q22⑦7685") || !strings.Contains(content, "45") {
		t.Fatalf("rendered content should keep readable base digits after wrapping, got %q", content)
	}
}

func TestBuildDetailBodyContentWrapsToInnerWidth(t *testing.T) {
	longLine := strings.Repeat("B", 50)

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: longLine, Timestamp: 1000}
	m.Posts.DetailFocus = DetailFocusPost
	m.Width = 28
	m.Height = 12
	m.syncPostsPage()

	contentWidth := m.Posts.PostBodyViewport.Width
	content, _ := m.Posts.buildDetailBodyContent(contentWidth)
	lines := strings.Split(strings.TrimRight(stripANSI(content), "\n"), "\n")
	maxWidth := m.Posts.detailBodyTextWidth(contentWidth) + vPostTextStyle.GetHorizontalFrameSize()

	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			t.Fatalf("detail line width = %d, want <= %d: %q", w, maxWidth, line)
		}
	}
}

func TestViewPostsManyNewlines(t *testing.T) {
	// Create a post with many newlines (100 empty lines)
	manyNewlines := strings.Repeat("\n", 100) + "bottom line"

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: manyNewlines, Timestamp: 1000, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for many newlines")
	}

	// Viewport should handle the overflow
	t.Logf("Output length for 100-newline post: %d", len(output))
}

func TestViewPostsWideTerminal(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 3 {
		t.Skip("need at least 3 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:3]
	m.Posts.SelectedPostIdx = 0
	m.Width = 200
	m.Height = 100 // Very tall to fit all posts even with wrapping

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for wide terminal")
	}

	// At minimum, the first post should always be visible
	firstPost := posts[0]
	firstLine := strings.Split(firstPost.Text, "\n")[0]
	if firstLine != "" && !containsStr(output, strings.TrimSpace(firstLine)) {
		t.Errorf("Wide terminal: missing first post pid=%d", firstPost.Pid)
	}

	// Count how many posts are visible
	visibleCount := 0
	for _, p := range posts[:3] {
		fl := strings.Split(p.Text, "\n")[0]
		if fl != "" && containsStr(output, strings.TrimSpace(fl)) {
			visibleCount++
		}
	}

	t.Logf("Wide terminal (200x100): %d/%d posts visible", visibleCount, 3)
}

func TestViewPostsNarrowTerminal(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 1 {
		t.Skip("need at least 1 post")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:1]
	m.Posts.SelectedPostIdx = 0
	m.Width = 40
	m.Height = 12

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for narrow terminal")
	}

	t.Logf("Narrow terminal (40x12) output length: %d", len(output))
}

func TestViewPostsTinyTerminal(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "test", Timestamp: 1000},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 10
	m.Height = 5

	// Should not panic
	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for tiny terminal")
	}

	t.Logf("Tiny terminal (10x5) output length: %d", len(output))
}

func TestViewPostDetailLongText(t *testing.T) {
	posts := loadRealPosts(t)

	var longest models.Post
	for _, p := range posts {
		if len(p.Text) > len(longest.Text) {
			longest = p
		}
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &longest
	m.Posts.CommentList = nil
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for detail view")
	}

	// Should contain post content
	if !containsStr(output, longest.Text[:20]) {
		t.Error("Detail view missing post content")
	}

	t.Logf("Detail view output length for long post: %d", len(output))
}

func TestViewPostDetailManyComments(t *testing.T) {
	// Create 50 comments
	var comments []models.Comment
	for i := 1; i <= 50; i++ {
		comments = append(comments, models.Comment{
			Cid:       int32(i),
			Pid:       1,
			Text:      strings.Repeat("C", 100),
			Timestamp: int32(1000 + i*100),
			NameTag:   "user",
		})
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: "Post with many comments", Timestamp: 1000}
	m.Posts.CommentList = comments
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for many comments")
	}

	// Should show comment count
	if !containsStr(output, "50") {
		t.Error("Detail view should show comment count")
	}

	t.Logf("Detail view with 50 comments output length: %d", len(output))
}

func TestScrollToSelectedPostBoundary(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 10 {
		t.Skip("need at least 10 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:10]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Scroll to last post
	m.Posts.SelectedPostIdx = 9
	m.Posts.scrollToSelectedPost()

	// Should not panic
	m.View()

	t.Logf("Scrolled to last post, viewport YOffset=%d", m.Posts.PostViewport.YOffset)
}

func TestAdjustSelectedToViewportBoundary(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 10 {
		t.Skip("need at least 10 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:10]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Simulate viewport scrolled to bottom
	m.Posts.PostViewport.GotoBottom()
	m.Posts.adjustSelectedToViewport()

	// SelectedPostIdx should be updated to match viewport
	t.Logf("After GotoBottom + adjustSelectedToViewport: SelectedPostIdx=%d", m.Posts.SelectedPostIdx)
}

func TestFastScrollPgDownBoundary(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 20 {
		t.Skip("need at least 20 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:20]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Rapid PgDn
	for i := 0; i < 20; i++ {
		m, _ = m.handlePostsKey(keyPgDown())
	}

	// Should not panic
	output := m.View()
	if output == "" {
		t.Fatal("View() returned empty after rapid PgDn")
	}

	t.Logf("After 20x PgDn: SelectedPostIdx=%d, YOffset=%d", m.Posts.SelectedPostIdx, m.Posts.PostViewport.YOffset)
}

func TestFastScrollPgUpBoundary(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 20 {
		t.Skip("need at least 20 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:20]
	m.Posts.SelectedPostIdx = 19
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Rapid PgUp
	for i := 0; i < 20; i++ {
		m, _ = m.handlePostsKey(keyPgUp())
	}

	// Should not panic
	output := m.View()
	if output == "" {
		t.Fatal("View() returned empty after rapid PgUp")
	}

	// Should be at or near top
	if m.Posts.SelectedPostIdx > 2 {
		t.Errorf("SelectedPostIdx = %d, expected near 0 after 20x PgUp", m.Posts.SelectedPostIdx)
	}

	t.Logf("After 20x PgUp: SelectedPostIdx=%d, YOffset=%d", m.Posts.SelectedPostIdx, m.Posts.PostViewport.YOffset)
}

func TestFastScrollMixedBoundary(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 20 {
		t.Skip("need at least 20 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:20]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Mixed: 10x PgDn, 5x PgUp, 15x PgDn, 30x PgUp
	for i := 0; i < 10; i++ {
		m, _ = m.handlePostsKey(keyPgDown())
	}
	for i := 0; i < 5; i++ {
		m, _ = m.handlePostsKey(keyPgUp())
	}
	for i := 0; i < 15; i++ {
		m, _ = m.handlePostsKey(keyPgDown())
	}
	for i := 0; i < 30; i++ {
		m, _ = m.handlePostsKey(keyPgUp())
	}

	// Should not panic
	output := m.View()
	if output == "" {
		t.Fatal("View() returned empty after mixed fast scroll")
	}

	// After 30x PgUp from bottom, should be at top
	if m.Posts.SelectedPostIdx > 3 {
		t.Errorf("SelectedPostIdx = %d, expected near 0 after mixed scroll ending with PgUp", m.Posts.SelectedPostIdx)
	}

	t.Logf("After mixed scroll: SelectedPostIdx=%d, YOffset=%d", m.Posts.SelectedPostIdx, m.Posts.PostViewport.YOffset)
}

func TestFastScrollKeepsCursorVisible(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 20 {
		t.Skip("need at least 20 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:20]
	m.Width = 80
	m.Height = 24

	m.View()

	for i := 0; i < 8; i++ {
		m, _ = m.handlePostsKey(keyPgDown())
		top := m.Posts.PostViewport.YOffset
		bottom := top + m.Posts.PostViewport.VisibleLineCount() - 1
		if m.Posts.CursorLine < top || m.Posts.CursorLine > bottom {
			t.Fatalf("cursor outside viewport after PgDn: cursor=%d viewport=[%d,%d]", m.Posts.CursorLine, top, bottom)
		}
	}

	for i := 0; i < 8; i++ {
		m, _ = m.handlePostsKey(keyPgUp())
		top := m.Posts.PostViewport.YOffset
		bottom := top + m.Posts.PostViewport.VisibleLineCount() - 1
		if m.Posts.CursorLine < top || m.Posts.CursorLine > bottom {
			t.Fatalf("cursor outside viewport after PgUp: cursor=%d viewport=[%d,%d]", m.Posts.CursorLine, top, bottom)
		}
	}
}

func TestRefreshClearsState(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 5 {
		t.Skip("need at least 5 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:5]
	m.Posts.SelectedPostIdx = 3
	m.Posts.SearchActive = false
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Press 'r' to refresh
	m, _ = m.handlePostsKey(keyR())

	if !m.Posts.PostListLoading {
		t.Error("PostListLoading should be true after refresh")
	}
	if len(m.Posts.PostList) != 0 {
		t.Errorf("PostList should be empty after refresh, got %d", len(m.Posts.PostList))
	}
	if m.Posts.SelectedPostIdx != 0 {
		t.Errorf("SelectedPostIdx should be 0 after refresh, got %d", m.Posts.SelectedPostIdx)
	}

	// View during loading should show "加载中..."
	output := m.View()
	if !containsStr(output, "加载中") {
		t.Error("View() should show '加载中...' during refresh")
	}
}

func TestRefreshDuringSearch(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.SearchActive = true
	m.Posts.SearchInput = "test"
	m.Posts.PostList = []models.Post{{Pid: 1, Text: "test result", Timestamp: 1000}}

	// Press 'r' during search - should NOT trigger refresh
	m, cmd := m.handlePostsKey(keyR())

	if m.Posts.PostListLoading {
		t.Error("PostListLoading should NOT change during search")
	}
	if cmd != nil {
		t.Error("r during search should NOT trigger reload")
	}
	if len(m.Posts.PostList) != 1 {
		t.Error("PostList should NOT be cleared during search")
	}
}

func TestViewPostsErrorState(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostListError = "connection refused"
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "错误") {
		t.Error("View() should show error indicator")
	}
	if !containsStr(output, "connection refused") {
		t.Error("View() should show error message")
	}
}

func TestViewPostsLoadingMoreKeepsContentVisible(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "hello world", Timestamp: 1000},
	}
	m.Posts.PostListLoading = true
	m.Width = 80
	m.Height = 24

	output := m.View()

	if !containsStr(output, "hello world") {
		t.Error("View() should keep existing post content visible while loading more")
	}
	if !containsStr(output, "正在加载更多") {
		t.Error("View() should show incremental loading hint while loading more")
	}
}

func TestViewPostsErrorWithPartialData(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Partial data", Timestamp: 1000},
	}
	m.Posts.PostListError = "timeout on page 2"
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	// Should show both data and error
	if !containsStr(output, "Partial data") {
		t.Error("View() should still show partial data")
	}
	if !containsStr(output, "timeout on page 2") {
		t.Error("View() should show error message")
	}
}

func TestViewHomeExtremeStats(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.LoggedIn = true
	m.Home.LoginUser = "testuser"
	m.Home.CrawlerState = CrawlerRunning
	m.Home.LastCrawlPage = 99999
	m.Width = 80
	m.Height = 24

	// Should not panic with large numbers
	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string with extreme stats")
	}

	t.Logf("Home view with extreme stats output length: %d", len(output))
}

func TestViewPostDetailEmptyPost(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: "", Timestamp: 1000}
	m.Posts.CommentList = nil
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for empty post detail")
	}

	if !containsStr(output, "#1") {
		t.Error("View() should show post pid")
	}
}

func TestViewPostDetailUnicodeContent(t *testing.T) {
	unicodeText := "🎉 测试中文和emoji混合 🚀\n日本語テスト\n한국어 테스트\nSpecial: αβγδε ∑∏∫"

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 1, Text: unicodeText, Timestamp: 1000}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "评论测试 🎊", Timestamp: 1100, NameTag: "用户"},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for unicode content")
	}

	// Should contain at least some unicode content
	if !containsStr(output, "测试") {
		t.Error("View() should show unicode content")
	}
}

func TestViewPostsManyPosts(t *testing.T) {
	posts := loadRealPosts(t)

	// Load all available posts
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	if output == "" {
		t.Fatal("View() returned empty string for many posts")
	}

	t.Logf("Rendered %d posts, output length: %d", len(posts), len(output))
}

func TestViewPostsScrollThroughAll(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 10 {
		t.Skip("need at least 10 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render to initialize viewport
	m.View()

	// Scroll through all posts
	for i := 0; i < len(posts)*20 && m.Posts.SelectedPostIdx < len(posts)-1; i++ {
		m, _ = m.handlePostsKey(keyDown())
	}

	if m.Posts.SelectedPostIdx != len(posts)-1 {
		t.Errorf("SelectedPostIdx = %d, want %d", m.Posts.SelectedPostIdx, len(posts)-1)
	}
	if m.Posts.PostViewport.YOffset < 0 {
		t.Errorf("YOffset = %d, should not be negative", m.Posts.PostViewport.YOffset)
	}

	// Should not panic
	output := m.View()
	if output == "" {
		t.Fatal("View() returned empty after scrolling through all posts")
	}

	t.Logf("Scrolled through %d posts successfully", len(posts))
}

func TestViewPostsViewportContentUpdate(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 5 {
		t.Skip("need at least 5 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:3]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	m.syncPostsPage()
	content1 := m.Posts.postContent

	m.Posts.PostList = posts[:5]
	m.syncPostsPage()
	content2 := m.Posts.postContent

	if len(content2) <= len(content1) {
		t.Errorf("content should grow with more posts: before=%d, after=%d", len(content1), len(content2))
	}

	for _, p := range posts[3:5] {
		firstLine := strings.Split(p.Text, "\n")[0]
		if firstLine != "" && !containsStr(content2, strings.TrimSpace(firstLine)) {
			t.Errorf("New post pid=%d not visible after adding", p.Pid)
		}
	}

	t.Logf("Content lengths: 3 posts=%d, 5 posts=%d", len(content1), len(content2))
}

func TestViewPostsResizeDuringRender(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 3 {
		t.Skip("need at least 3 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:3]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	// Render at 80x24
	output1 := m.View()

	// Resize to 120x40
	m.Width = 120
	m.Height = 40
	m.Posts.postContent = "" // Force content update

	output2 := m.View()

	if output1 == output2 {
		t.Log("Output changed after resize (expected due to different dimensions)")
	}

	t.Logf("Output lengths: 80x24=%d, 120x40=%d", len(output1), len(output2))
}

func TestStripANSIRemovesAllCodes(t *testing.T) {
	input := "\x1b[38;5;205mHello\x1b[0m World\x1b[1mBold\x1b[22m"
	result := stripANSI(input)
	expected := "Hello WorldBold"
	if result != expected {
		t.Errorf("stripANSI(%q) = %q, want %q", input, result, expected)
	}
}

func TestViewPostsStrippedLines(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 3 {
		t.Skip("need at least 3 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:3]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	if len(lines) == 0 {
		t.Fatal("No visible lines after stripping ANSI codes")
	}

	foundTitle := false
	for _, line := range lines {
		if strings.Contains(line, "帖子列表") {
			foundTitle = true
			break
		}
	}
	if !foundTitle {
		t.Error("No posts title found in visible lines")
	}

	// Should contain search hint
	foundSearch := false
	for _, line := range lines {
		if strings.Contains(line, "搜索") {
			foundSearch = true
			break
		}
	}
	if !foundSearch {
		t.Error("No search hint found in visible lines")
	}

	t.Logf("Total visible lines: %d", len(lines))
	for i, line := range lines {
		if i < 5 {
			t.Logf("  line[%d]: %q", i, line)
		}
	}
}

func TestViewShowsTabBarAtTop(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24

	lines := visibleLines(m.View())
	if len(lines) == 0 {
		t.Fatal("No visible lines after stripping ANSI codes")
	}

	if !strings.Contains(lines[0], "同步") || !strings.Contains(lines[0], "帖子") {
		t.Fatalf("Line[0] = %q, want tab bar with 同步 and 帖子", lines[0])
	}
}

func TestViewLinesDoNotOverflowWidth(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24

	for i, line := range frameLines(m.View()) {
		if lipgloss.Width(line) > m.Width {
			t.Fatalf("line[%d] width = %d, want <= %d: %q", i, lipgloss.Width(line), m.Width, line)
		}
	}
}

func TestViewFrameMatchesConfiguredDimensionsPosts(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24

	lines := frameLines(m.View())
	if len(lines) != m.Height {
		t.Fatalf("frame line count = %d, want %d", len(lines), m.Height)
	}

	for i, line := range lines {
		if lipgloss.Width(line) > m.Width {
			t.Fatalf("line[%d] width = %d, want <= %d: %q", i, lipgloss.Width(line), m.Width, line)
		}
	}
}

func TestViewFrameMatchesConfiguredDimensionsHome(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.LoggedIn = true
	m.Home.LoginUser = "testuser"
	m.Home.CrawlerState = CrawlerStopped
	m.Width = 80
	m.Height = 24

	lines := frameLines(m.View())
	if len(lines) != m.Height {
		t.Fatalf("frame line count = %d, want %d", len(lines), m.Height)
	}

	for i, line := range lines {
		if lipgloss.Width(line) > m.Width {
			t.Fatalf("line[%d] width = %d, want <= %d: %q", i, lipgloss.Width(line), m.Width, line)
		}
	}
}

func TestViewFrameMatchesConfiguredDimensionsDetail(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: strings.Repeat("Detail post text\n", 8), Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	for i := 0; i < 20; i++ {
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      strings.Repeat("comment body ", 6),
			Timestamp: int32(1100 + i*10),
			NameTag:   "user",
		})
	}
	m.Width = 80
	m.Height = 24

	lines := frameLines(m.View())
	if len(lines) != m.Height {
		t.Fatalf("detail frame line count = %d, want %d", len(lines), m.Height)
	}

	for i, line := range lines {
		if lipgloss.Width(line) > m.Width {
			t.Fatalf("detail line[%d] width = %d, want <= %d: %q", i, lipgloss.Width(line), m.Width, line)
		}
	}
}

func TestViewDetailUsesAvailableHeight(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: strings.Repeat("Detail post text\n", 6), Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	for i := 0; i < 20; i++ {
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      strings.Repeat("comment body ", 5),
			Timestamp: int32(1100 + i*10),
			NameTag:   "user",
		})
	}
	m.Width = 80
	m.Height = 24

	lines := frameLines(m.View())
	footerLine := -1
	for i, line := range lines {
		if strings.Contains(line, "OFFLINE") || strings.Contains(line, "ONLINE") {
			footerLine = i
		}
	}
	if footerLine == -1 {
		t.Fatalf("missing footer line in detail view")
	}
	lastContentLine := -1
	for i, line := range lines[:footerLine] {
		if strings.TrimSpace(line) != "" {
			lastContentLine = i
		}
	}
	if lastContentLine == -1 {
		t.Fatalf("detail view should render content before footer")
	}
	if footerLine-lastContentLine > 1 {
		t.Fatalf("detail view leaves too much blank space before footer: content=%d footer=%d", lastContentLine, footerLine)
	}
}

func TestViewStatusLineShowsNormalPostsMode(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{{Pid: 1, Text: "hello", Timestamp: 1000}}
	m.Posts.SelectedPostIdx = 0

	output := stripANSI(m.View())
	expected := []string{"NORMAL", "帖子 列表", "1 条", "OFFLINE"}
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Fatalf("status line missing %q in output:\n%s", want, output)
		}
	}
}

func TestViewStatusLineShowsLoadingProgress(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostListLoading = true

	output := stripANSI(m.View())
	expected := []string{"NORMAL", "帖子 列表", "加载帖子中", "OFFLINE"}
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Fatalf("status line missing %q in loading output:\n%s", want, output)
		}
	}
}

func TestViewStatusLineShowsDetailMode(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 42, Text: "detail", Timestamp: 1000}
	m.Posts.CommentList = []models.Comment{{Cid: 1, Text: "comment", Timestamp: 1001}}

	output := stripANSI(m.View())
	expected := []string{"DETAIL-CMT", "帖子 #42", "评论 1", "焦点: 评论"}
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Fatalf("detail status line missing %q in output:\n%s", want, output)
		}
	}
}

func TestViewStatusLineKeepsClockOnSameLine(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{{Pid: 1, Text: "hello", Timestamp: 1000}}

	lines := frameLines(m.View())
	var footer string
	for _, line := range lines {
		if strings.Contains(line, "OFFLINE") || strings.Contains(line, "ONLINE") {
			footer = line
			break
		}
	}
	if footer == "" {
		t.Fatalf("missing footer line in output:\n%s", stripANSI(m.View()))
	}
	if !regexp.MustCompile(`\d{2}:\d{2}:\d{2}`).MatchString(footer) {
		t.Fatalf("expected clock on footer line, got: %q", footer)
	}
}

func TestDetailViewportBodyHeightCappedToHalf(t *testing.T) {
	p := NewPostsPageModel()
	p.ShowPostDetail = true
	p.CurrentPost = &models.Post{
		Pid:  42,
		Text: strings.Repeat("这是一段很长的正文，用来测试正文高度上限。", 40),
	}
	p.CommentList = []models.Comment{
		{Cid: 1, Text: "评论1", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "评论2", Timestamp: 1200, NameTag: "user2"},
	}

	bodyHeight, commentHeight := p.calcDetailViewportHeights(80, 24)
	available := 24 - 4

	if bodyHeight > available/2 {
		t.Fatalf("bodyHeight = %d, want <= %d", bodyHeight, available/2)
	}
	if commentHeight < available-bodyHeight {
		t.Fatalf("commentHeight = %d, want >= %d", commentHeight, available-bodyHeight)
	}
}

func TestDetailViewportBodyHeightStaysCompactForShortPost(t *testing.T) {
	p := NewPostsPageModel()
	p.ShowPostDetail = true
	p.CurrentPost = &models.Post{
		Pid:  7,
		Text: "短正文",
	}
	p.CommentList = []models.Comment{
		{Cid: 1, Text: "评论1", Timestamp: 1100, NameTag: "user1"},
	}

	bodyHeight, _ := p.calcDetailViewportHeights(80, 24)
	if bodyHeight > 2 {
		t.Fatalf("bodyHeight = %d, want <= 2 for short post", bodyHeight)
	}
}

func TestDetailViewportAccountsForWrappedShortcutLines(t *testing.T) {
	p := NewPostsPageModel()
	p.ShowPostDetail = true
	p.CurrentPost = &models.Post{
		Pid: 42, Text: strings.Repeat("正文 ", 8), Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	for i := 0; i < 12; i++ {
		p.CommentList = append(p.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      strings.Repeat("评论内容 ", 5),
			Timestamp: int32(1100 + i*10),
			NameTag:   "user",
		})
	}

	width := 36
	height := 24
	bodyHeight, commentHeight := p.calcDetailViewportHeights(width, height)
	fixedLines := p.detailFixedLineCount(width)
	if got := bodyHeight + commentHeight + fixedLines; got != height {
		t.Fatalf("detail layout uses %d lines, want %d", got, height)
	}
}

func TestViewPostDetailBottomShowsLastCommentWithWrappedShortcut(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	for i := 0; i < 12; i++ {
		text := fmt.Sprintf("comment %02d", i+1)
		if i == 11 {
			text = "LAST COMMENT"
		}
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      text,
			Timestamp: int32(1100 + i*10),
			NameTag:   "user",
		})
	}
	m.Width = 36
	m.Height = 24
	m.syncPostsPage()
	m.Posts.CommentViewport.GotoBottom()

	output := m.View()
	if !strings.Contains(strings.Join(visibleLines(output), "\n"), "LAST COMMENT") {
		t.Fatalf("bottom of detail view should show last comment, got:\n%s", output)
	}
}

func TestViewHomeStrippedLines(t *testing.T) {
	m := newTestModel()
	m.Page = PageHome
	m.Home.LoggedIn = true
	m.Home.LoginUser = "testuser"
	m.Home.CrawlerState = CrawlerStopped
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	if len(lines) == 0 {
		t.Fatal("No visible lines")
	}

	// Title should be in the content (tab bar is first, then separator, then content)
	titleFound := false
	for _, line := range lines {
		if strings.Contains(line, "TreeHole TUI") {
			titleFound = true
			break
		}
	}
	if !titleFound {
		t.Errorf("Title 'TreeHole TUI' not found in visible lines")
	}

	// Check key content lines
	allText := strings.Join(lines, " ")
	expectedContent := []string{"TreeHole", "已登录", "testuser", "已停止", "上次爬取", "第0页"}
	for _, want := range expectedContent {
		if !strings.Contains(allText, want) {
			t.Errorf("Missing expected content: %q", want)
		}
	}

	t.Logf("Home view: %d visible lines", len(lines))
}

func TestViewPostDetailStrippedLines(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "First comment", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "Second comment", Timestamp: 1200, NameTag: "user2", Quote: &models.Comment{NameTag: "user1", Text: "quoted text"}},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	allText := strings.Join(lines, " ")
	expectedContent := []string{"#42", "Detail post text", "First comment", "Second comment", "user1: quoted text", "正序", "评论 2", "焦点: 评论"}
	for _, want := range expectedContent {
		if !strings.Contains(allText, want) {
			t.Errorf("Missing expected content: %q", want)
		}
	}

	t.Logf("Post detail: %d visible lines", len(lines))
}

func TestViewPostDetailOmitsBodyHeading(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "First comment", Timestamp: 1100, NameTag: "user1"},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	for _, line := range lines {
		if strings.TrimSpace(line) == "正文" {
			t.Fatalf("detail view should not show standalone body heading: %q", line)
		}
	}
	allText := strings.Join(lines, " ")
	if !strings.Contains(allText, "评论 1  正序") {
		t.Fatalf("detail view missing comments heading: %q", allText)
	}
}

func TestViewPostDetailCommentFormatMatchesTarget(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "什么专业", Timestamp: 1200, NameTag: "Bob"},
		{Cid: 2, Text: "外院……", Timestamp: 1260, NameTag: "洞主", Quote: &models.Comment{NameTag: "Bob", Text: "什么专业"}},
	}
	m.Width = 80
	m.Height = 24

	lines := visibleLines(m.View())
	allText := strings.Join(lines, "\n")

	for _, want := range []string{
		"评论 2  正序",
		"1970-01-01 08:20",
		"Bob: 什么专业",
		"1970-01-01 08:21",
		"Bob: 什么专业",
		"洞主: 外院……",
	} {
		if !strings.Contains(allText, want) {
			t.Fatalf("detail view missing %q in:\n%s", want, allText)
		}
	}
}

func TestViewPostListShowsImagePlaceholder(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "带图帖子", Timestamp: 1000, MediaIds: "1"},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()
	if !strings.Contains(strings.Join(visibleLines(output), "\n"), "[图片]") {
		t.Fatalf("post list should show image placeholder, got:\n%s", output)
	}
}

func TestViewPostDetailShowsImagePlaceholder(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10, MediaIds: "1",
	}
	m.Width = 80
	m.Height = 24

	output := m.View()
	if !strings.Contains(strings.Join(visibleLines(output), "\n"), "[图片]") {
		t.Fatalf("post detail should show image placeholder, got:\n%s", output)
	}
}

func TestViewPostDetailCommentShowsImagePlaceholder(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "Detail post text", Timestamp: 1000,
		Reply: 5, Likenum: 10,
	}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "评论文本", Timestamp: 1100, NameTag: "user1", MediaIds: "10"},
	}
	m.Width = 80
	m.Height = 24

	output := m.View()
	if !strings.Contains(strings.Join(visibleLines(output), "\n"), "[图片]") {
		t.Fatalf("comment should show image placeholder, got:\n%s", output)
	}
}

func TestBuildPostListContentUsesThumbnailBlocksWhenPreviewEnabled(t *testing.T) {
	m := newTestModel()
	m.Posts.ImagePreview = true
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "带图帖子", Timestamp: 1000, MediaIds: "30518"},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24
	m.syncPostsPage()

	contentWidth := m.Posts.currentListContentWidth()
	content, placements := m.Posts.buildPostListContent(contentWidth)
	if strings.Contains(stripANSI(content), "[图片]") {
		t.Fatalf("thumbnail preview should replace placeholder, got:\n%s", stripANSI(content))
	}
	if len(placements) != 1 {
		t.Fatalf("placements = %d, want 1", len(placements))
	}
	if placements[0].cols != listImageCellSize || placements[0].rows != listImageCellSize {
		t.Fatalf("thumbnail placement size = %dx%d, want %dx%d", placements[0].cols, placements[0].rows, listImageCellSize, listImageCellSize)
	}
	if got := m.Posts.postRenderedLinesAt(0); got < 6 {
		t.Fatalf("postRenderedLinesAt(0) = %d, want image block to contribute rows", got)
	}
}

func TestBuildDetailBodyContentPlacesMultipleImagesHorizontally(t *testing.T) {
	m := newTestModel()
	m.Posts.ImagePreview = true
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "详情图", Timestamp: 1000, MediaIds: "30518,30669",
	}
	m.Width = 100
	m.Height = 30
	m.syncPostsPage()

	content, placements := m.Posts.buildDetailBodyContent(m.Posts.PostBodyViewport.Width)
	if strings.Contains(stripANSI(content), "[图片]") {
		t.Fatalf("detail preview should replace placeholder, got:\n%s", stripANSI(content))
	}
	if len(placements) != 2 {
		t.Fatalf("placements = %d, want 2", len(placements))
	}
	if placements[0].top != placements[1].top {
		t.Fatalf("placements should share the same row for horizontal layout: %+v", placements)
	}
	if placements[1].left <= placements[0].left {
		t.Fatalf("second placement should be to the right of the first: %+v", placements)
	}
}

func TestBuildCommentContentReverseOrder(t *testing.T) {
	m := newTestModel()
	m.Posts.CommentList = []models.Comment{
		{Cid: 2, Text: "Second comment", Timestamp: 1200, NameTag: "user2"},
		{Cid: 1, Text: "First comment", Timestamp: 1100, NameTag: "user1"},
	}
	m.Posts.CommentSortAsc = false

	content := stripANSI(m.Posts.buildCommentContent(72))
	firstIdx := strings.Index(content, "Second comment")
	secondIdx := strings.Index(content, "First comment")
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatal("comment content missing expected comments")
	}
	if firstIdx > secondIdx {
		t.Fatal("reverse order should render second comment before first comment")
	}
}

func TestViewConfigDialogStrippedLines(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogConfig
	m.ConfigDialog = NewConfigDialog(&config.Config{
		Username:  "testuser",
		Password:  "secret",
		SecretKey: "KEY123",
	})
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	allText := strings.Join(lines, " ")
	expectedContent := []string{"配置管理", "data/config.json", "用户名", "密码", "SecretKey"}
	for _, want := range expectedContent {
		if !strings.Contains(allText, want) {
			t.Errorf("Missing expected content: %q", want)
		}
	}

	// Password should be masked
	if strings.Contains(allText, "secret") {
		t.Error("Plaintext password found in output")
	}

	t.Logf("Config dialog: %d visible lines", len(lines))
}

func TestViewHelpDialogStrippedLines(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogHelp
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	allText := strings.Join(lines, " ")
	expectedContent := []string{"快捷键", "帖子列表", "Esc", "关闭帮助", "Enter", "打开详情", "/", "搜索帖子", "r", "刷新列表", "c", "打开配置"}
	for _, want := range expectedContent {
		if !strings.Contains(allText, want) {
			t.Errorf("Missing expected content: %q", want)
		}
	}

	t.Logf("Help dialog: %d visible lines", len(lines))
}

func TestViewHelpDialogUsesDetailContext(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.ShowPostDetail = true
	m.Posts.CanWrite = true
	m.Posts.CurrentPost = &models.Post{Pid: 42, Text: "detail", Timestamp: 1000}
	m.Dialog = DialogHelp
	m.Width = 140
	m.Height = 24

	output := strings.Join(visibleLines(m.View()), " ")
	expected := []string{"帖子详情", "Tab", "切换正文/评论", "s", "切换排序", "q", "引用评论"}
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Fatalf("detail help missing %q in output:\n%s", want, output)
		}
	}
	unexpected := []string{"打开详情", "搜索帖子", "标签筛选"}
	for _, bad := range unexpected {
		if strings.Contains(output, bad) {
			t.Fatalf("detail help should not show %q in output:\n%s", bad, output)
		}
	}
}

func TestViewPostsStrippedLinesWithRealData(t *testing.T) {
	posts := loadRealPosts(t)
	if len(posts) < 3 {
		t.Skip("need at least 3 posts")
	}

	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = posts[:3]
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()
	lines := visibleLines(output)

	// Verify structure
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 visible lines, got %d", len(lines))
	}

	// First line is the tab bar, content starts after
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 visible lines, got %d", len(lines))
	}

	foundTitle := false
	for _, line := range lines {
		if strings.Contains(line, "帖子列表") {
			foundTitle = true
			break
		}
	}
	if !foundTitle {
		t.Error("No posts title found in visible lines")
	}

	// Should contain post text
	postText := posts[0].Text
	firstLine := strings.Split(postText, "\n")[0]
	trimmedFirst := strings.TrimSpace(firstLine)
	if trimmedFirst != "" {
		found := false
		for _, line := range lines {
			// The text may be split across lines, check if any line contains part of it
			if strings.Contains(line, trimmedFirst[:min(len(trimmedFirst), 20)]) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Post text first line %q not found in visible lines", trimmedFirst[:min(len(trimmedFirst), 40)])
		}
	}

	// Should contain statusline count hint
	foundCountHint := false
	for _, line := range lines {
		if strings.Contains(line, " 条") {
			foundCountHint = true
			break
		}
	}
	if !foundCountHint {
		t.Error("Statusline count hint not found")
	}

	t.Logf("Real data posts view: %d visible lines", len(lines))
}

func TestViewNoANSILeakage(t *testing.T) {
	// Verify that ANSI codes are properly closed (no leakage)
	m := newTestModel()
	m.Page = PagePosts
	m.Posts.PostList = []models.Post{
		{Pid: 1, Text: "Test post", Timestamp: 1000, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Width = 80
	m.Height = 24

	output := m.View()

	// Count opening and closing ANSI sequences
	openCount := strings.Count(output, "\x1b[")
	// Each ANSI sequence should end with a letter
	closeCount := len(regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`).FindAllString(output, -1))

	if openCount != closeCount {
		t.Errorf("ANSI sequence mismatch: %d opens, %d closes", openCount, closeCount)
	}

	t.Logf("ANSI sequences: %d open, %d close", openCount, closeCount)
}

func TestViewStrippedOutputNotEmpty(t *testing.T) {
	// All view states should produce non-empty stripped output
	tests := []struct {
		name  string
		model Model
	}{
		{"home_stopped", func() Model {
			m := newTestModel()
			m.Page = PageHome
			m.Home.CrawlerState = CrawlerStopped
			return m
		}()},
		{"home_running", func() Model {
			m := newTestModel()
			m.Page = PageHome
			m.Home.CrawlerState = CrawlerRunning
			return m
		}()},
		{"posts_empty", func() Model {
			m := newTestModel()
			m.Page = PagePosts
			m.Posts.PostList = nil
			return m
		}()},
		{"posts_with_data", func() Model {
			m := newTestModel()
			m.Page = PagePosts
			m.Posts.PostList = []models.Post{{Pid: 1, Text: "Hello", Timestamp: 1000}}
			m.Posts.SelectedPostIdx = 0
			return m
		}()},
		{"detail_view", func() Model {
			m := newTestModel()
			m.Page = PagePosts
			m.Posts.ShowPostDetail = true
			m.Posts.CurrentPost = &models.Post{Pid: 1, Text: "Post", Timestamp: 1000}
			return m
		}()},
		{"config_dialog", func() Model {
			m := newTestModel()
			m.Dialog = DialogConfig
			return m
		}()},
		{"help_dialog", func() Model {
			m := newTestModel()
			m.Dialog = DialogHelp
			return m
		}()},
		{"logs_dialog", func() Model {
			m := newTestModel()
			m.Dialog = DialogLogs
			m.LogsDialog.SetLines([]string{"log line"})
			return m
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.model.View()
			stripped := stripANSI(output)
			lines := visibleLines(output)

			if stripped == "" {
				t.Errorf("Stripped output is empty for %s", tt.name)
			}
			if len(lines) == 0 {
				t.Errorf("No visible lines for %s", tt.name)
			}

			t.Logf("%s: %d stripped chars, %d visible lines", tt.name, len(stripped), len(lines))
		})
	}
}

func TestNormalizeRenderedText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "keycap numbers",
			input:    "1️⃣ 2️⃣ 3️⃣",
			expected: "1 2 3",
		},
		{
			name:     "emoji with skin tone",
			input:    "✍🏻 ✍🏼 ✍🏽 ✍🏾 ✍🏿",
			expected: "✍ ✍ ✍ ✍ ✍",
		},
		{
			name:     "variation selectors",
			input:    "A\uFE0F B\uFE00 C\uFE0E",
			expected: "A B C",
		},
		{
			name:     "combining diacritical marks",
			input:    "a\u0300a\u0301a\u0302a\u0303a\u0308a\u030A",
			expected: "aaaaaa",
		},
		{
			name:     "mixed complex unicode",
			input:    "Hello ✍🏻 World 1️⃣ 2️⃣! A\uFE0F",
			expected: "Hello ✍ World 1 2! A",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "chinese text with emoji",
			input:    "测试 ✍🏻 中文 1️⃣",
			expected: "测试 ✍ 中文 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRenderedText(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeRenderedText(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Key helpers

func keyDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}

func keyPgDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyPgDown}
}

func keyPgUp() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyPgUp}
}

func keyR() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
}
