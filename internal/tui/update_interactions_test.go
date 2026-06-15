package tui

import (
	"strings"
	"testing"

	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

type stubPostsProvider struct {
	refreshPost          *models.Post
	listTags             []models.Tag
	togglePraisePID      int32
	toggleAttentionPID   int32
	createCommentPID     int32
	createCommentText    string
	createCommentQuoteID *int32
	refreshCalls         int
	canWrite             bool
	mode                 SessionMode
}

func (s *stubPostsProvider) ListPosts(cursor, limit, label int, keyword string) ([]models.Post, int, bool, error) {
	return nil, 0, false, nil
}

func (s *stubPostsProvider) GetPostDetail(pid int32, sortAsc bool) (*models.Post, []models.Comment, int32, bool, error) {
	return s.refreshPost, nil, 0, false, nil
}

func (s *stubPostsProvider) ListComments(pid int32, sortAsc bool, cursor int32) ([]models.Comment, int32, bool, error) {
	return nil, 0, false, nil
}

func (s *stubPostsProvider) SearchPosts(keyword string, cursor, limit, label int) ([]models.Post, int, bool, error) {
	return nil, 0, false, nil
}

func (s *stubPostsProvider) ListTags() ([]models.Tag, error) { return s.listTags, nil }

func (s *stubPostsProvider) RefreshPost(pid int32) (*models.Post, error) {
	s.refreshCalls++
	if s.refreshPost == nil {
		s.refreshPost = &models.Post{Pid: pid}
	}
	return s.refreshPost, nil
}

func (s *stubPostsProvider) TogglePraise(pid int32) error {
	s.togglePraisePID = pid
	return nil
}

func (s *stubPostsProvider) ToggleAttention(pid int32) error {
	s.toggleAttentionPID = pid
	return nil
}

func (s *stubPostsProvider) CreateComment(pid int32, text string, quoteID *int32) error {
	s.createCommentPID = pid
	s.createCommentText = text
	s.createCommentQuoteID = quoteID
	return nil
}

func (s *stubPostsProvider) CreatePost(text string) error { return nil }
func (s *stubPostsProvider) CanWrite() bool               { return s.canWrite }
func (s *stubPostsProvider) Mode() SessionMode            { return s.mode }

func TestBuildCommentContentMarksSelectedComment(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{Cid: 1, Text: "First", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "Second", Timestamp: 1200, NameTag: "user2"},
	}
	page.SelectedCommentIdx = 1
	page.CommentCursorLine = 3

	output := page.buildCommentContent(60)
	if !strings.Contains(stripANSI(output), "▸ ") {
		t.Fatalf("expected selected comment marker in output, got %q", output)
	}
}

func TestRenderCommentHeaderUsesDistinctStyling(t *testing.T) {
	page := NewPostsPageModel()
	rendered := page.renderCommentHeader("1970-01-01 08:20")
	if got := stripANSI(rendered); got != "1970-01-01 08:20" {
		t.Fatalf("stripped header = %q, want timestamp only", got)
	}
}

func TestMoveCommentSelectionUsesRenderedRows(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{Cid: 1, Text: strings.Repeat("long comment ", 8), Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "Second", Timestamp: 1200, NameTag: "user2"},
	}
	page.CommentViewport.Width = 24
	page.CommentViewport.Height = 5
	page.CommentViewport.SetContent(page.buildCommentContent(24))

	page.moveCommentSelection(1)
	if page.SelectedCommentIdx != 0 {
		t.Fatalf("selection moved by comment instead of row: idx=%d, want 0", page.SelectedCommentIdx)
	}
	if page.CommentCursorLine != 1 {
		t.Fatalf("comment cursor line = %d, want 1", page.CommentCursorLine)
	}

	for page.SelectedCommentIdx == 0 {
		page.moveCommentSelection(1)
		if page.CommentCursorLine > 20 {
			t.Fatal("selection never advanced to next comment")
		}
	}
	if page.SelectedCommentIdx != 1 {
		t.Fatalf("selection idx = %d, want 1 after leaving first comment block", page.SelectedCommentIdx)
	}

	for page.SelectedCommentIdx == 1 {
		page.moveCommentSelection(-1)
		if page.CommentCursorLine < 0 {
			t.Fatal("selection never moved back to previous comment")
		}
	}
	if page.SelectedCommentIdx != 0 {
		t.Fatalf("selection idx = %d, want 0 after moving back upward", page.SelectedCommentIdx)
	}
}

func TestBuildCommentContentMovesCursorWithinSelectedComment(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{Cid: 1, Text: strings.Repeat("line ", 8), Timestamp: 1100, NameTag: "user1"},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.Width = 20
	page.CommentViewport.Height = 5

	page.CommentCursorLine = 0
	first := stripANSI(page.buildCommentContent(20))
	page.CommentCursorLine = 1
	second := stripANSI(page.buildCommentContent(20))

	firstLines := strings.Split(first, "\n")
	secondLines := strings.Split(second, "\n")
	if !strings.Contains(firstLines[0], "▸ ") {
		t.Fatalf("expected cursor on first rendered row, got %q", firstLines[0])
	}
	if strings.Contains(secondLines[0], "▸ ") {
		t.Fatalf("cursor should leave first row after moving, got %q", secondLines[0])
	}
	foundLaterCursor := false
	for _, line := range secondLines[1:] {
		if strings.Contains(line, "▸ ") {
			foundLaterCursor = true
			break
		}
	}
	if !foundLaterCursor {
		t.Fatalf("expected cursor on a later rendered row after moving one line, got %q", second)
	}
}

func TestCommentLineRangeMatchesRenderedContentWithLongAuthor(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{
			Cid:       1,
			Text:      "short",
			Timestamp: 1100,
			NameTag:   "averyverylongauthorname",
		},
	}
	page.SelectedCommentIdx = -1
	page.CommentViewport.Width = 18
	page.CommentViewport.Height = 6

	content := stripANSI(page.buildCommentContent(18))
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	start, end := page.commentLineRangeAt(0)
	if start != 0 {
		t.Fatalf("start = %d, want 0", start)
	}
	if got := end - start + 1; got != len(lines) {
		t.Fatalf("line range count = %d, want %d for rendered lines %q", got, len(lines), lines)
	}
}

func TestMoveCommentSelectionDoesNotAdvanceEarlyWithLongAuthorWrap(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{
			Cid:       1,
			Text:      "short",
			Timestamp: 1100,
			NameTag:   "averyverylongauthorname",
		},
		{
			Cid:       2,
			Text:      "second",
			Timestamp: 1200,
			NameTag:   "user2",
		},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.Width = 18
	page.CommentViewport.Height = 6
	page.CommentViewport.SetContent(page.buildCommentContent(18))

	firstStart, firstEnd := page.commentLineRangeAt(0)
	for step := 0; step < firstEnd-firstStart; step++ {
		page.moveCommentSelection(1)
		if page.SelectedCommentIdx != 0 {
			t.Fatalf("selection advanced early at step %d: idx=%d want 0", step, page.SelectedCommentIdx)
		}
	}

	page.moveCommentSelection(1)
	if page.SelectedCommentIdx != 1 {
		t.Fatalf("selection idx = %d, want 1 after leaving wrapped first comment", page.SelectedCommentIdx)
	}
}

func TestBuildCommentContentSelectedCommentDoesNotInsertBlankLines(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{
			Cid:       1,
			Text:      strings.Repeat("wrapped text ", 8),
			Timestamp: 1100,
			NameTag:   "洞主",
		},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.Width = 32
	page.CommentViewport.Height = 8

	lines := strings.Split(strings.TrimRight(stripANSI(page.buildCommentContent(32)), "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("unexpected blank rendered line at index %d: %q", i, lines)
		}
	}
}

func TestBuildCommentContentSelectedQuoteStaysOnOneLineWhenItFits(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{
			Cid:       1,
			Text:      "reply",
			Timestamp: 1100,
			NameTag:   "Alice",
			Quote:     &models.Comment{NameTag: "Bob", Text: "quoted text"},
		},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.Width = 28
	page.CommentViewport.Height = 6

	lines := strings.Split(strings.TrimRight(stripANSI(page.buildCommentContent(28)), "\n"), "\n")
	foundQuote := false
	for _, line := range lines {
		if strings.Contains(line, "Bob: quoted text") {
			foundQuote = true
		}
	}
	if !foundQuote {
		t.Fatalf("expected quote preview on one rendered line, got %q", lines)
	}
}

func TestMoveCommentSelectionDoesNotLoopBackToHeaderAtCommentEnd(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentList = []models.Comment{
		{
			Cid:       1,
			Text:      strings.Repeat("comment body ", 10),
			Timestamp: 1100,
			NameTag:   "洞主",
		},
		{
			Cid:       2,
			Text:      "next comment",
			Timestamp: 1200,
			NameTag:   "Alice",
		},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.Width = 32
	page.CommentViewport.Height = 6
	page.CommentViewport.SetContent(page.buildCommentContent(32))

	start, end := page.commentLineRangeAt(0)
	if start != 0 {
		t.Fatalf("start = %d, want 0", start)
	}
	for expectedLine := start + 1; expectedLine <= end; expectedLine++ {
		page.moveCommentSelection(1)
		if page.SelectedCommentIdx != 0 {
			t.Fatalf("selection advanced early at line %d: idx=%d", expectedLine, page.SelectedCommentIdx)
		}
		if page.CommentCursorLine != expectedLine {
			t.Fatalf("cursor line = %d, want %d", page.CommentCursorLine, expectedLine)
		}
	}

	page.moveCommentSelection(1)
	if page.SelectedCommentIdx != 1 {
		t.Fatalf("selection idx = %d, want 1 after leaving first comment", page.SelectedCommentIdx)
	}
	if page.CommentCursorLine <= end {
		t.Fatalf("cursor line = %d, want > %d after leaving first comment", page.CommentCursorLine, end)
	}
}

func TestSelectedCommentLineCountMatchesRenderedLinesAtExactWidth(t *testing.T) {
	page := NewPostsPageModel()
	page.CommentViewport.Width = 32
	page.CommentViewport.Height = 6

	bodyWidth := page.commentBodyTextWidth(32)
	text := strings.Repeat("x", maxInt(1, bodyWidth-len("Alice: ")))
	page.CommentList = []models.Comment{
		{Cid: 1, Text: text, Timestamp: 1100, NameTag: "Alice"},
		{Cid: 2, Text: "next comment", Timestamp: 1200, NameTag: "Bob"},
	}
	page.SelectedCommentIdx = 0
	page.CommentViewport.SetContent(page.buildCommentContent(32))

	lines := strings.Split(strings.TrimRight(stripANSI(page.buildCommentContent(32)), "\n"), "\n")
	if got, want := len(lines), page.commentLineCount(); got != want {
		t.Fatalf("rendered line count = %d, want %d\ncontent:\n%s", got, want, strings.Join(lines, "\n"))
	}

	_, end := page.commentLineRangeAt(0)
	for expected := 1; expected <= end; expected++ {
		page.moveCommentSelection(1)
		if page.SelectedCommentIdx != 0 {
			t.Fatalf("selection advanced early at line %d: idx=%d", expected, page.SelectedCommentIdx)
		}
	}

	page.moveCommentSelection(1)
	if page.SelectedCommentIdx != 1 {
		t.Fatalf("selection idx = %d, want 1 after leaving exact-fit selected comment", page.SelectedCommentIdx)
	}
}

func TestHandlePostsKeyCommentScrollDoesNotLoopBackAfterViewportSync(t *testing.T) {
	m := newTestModel()
	m.Width = 24
	m.Height = 10
	m.Posts.ShowPostDetail = true
	m.Posts.DetailFocus = DetailFocusComments
	m.Posts.CurrentPost = &models.Post{Pid: 42}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "comment body", Timestamp: 1100, NameTag: "A"},
		{Cid: 2, Text: strings.Repeat("next comment ", 8), Timestamp: 1200, NameTag: "B"},
		{Cid: 3, Text: "third", Timestamp: 1300, NameTag: "C"},
	}
	m.syncPostsPage()

	prevCursor := m.Posts.CommentCursorLine
	for step := 0; step < 20; step++ {
		next, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			t.Fatalf("down should not emit async command at step %d", step)
		}
		if next.Posts.CommentCursorLine < prevCursor {
			t.Fatalf("cursor regressed after viewport sync at step %d: prev=%d new=%d sel=%d y=%d", step, prevCursor, next.Posts.CommentCursorLine, next.Posts.SelectedCommentIdx, next.Posts.CommentViewport.YOffset)
		}
		m = next
		prevCursor = m.Posts.CommentCursorLine
		if prevCursor >= m.Posts.commentLineCount()-1 {
			break
		}
	}
}

func TestHandlePostsKeyQuoteOpensComposerWithSelectedComment(t *testing.T) {
	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CanWrite = true
	m.Posts.CurrentPost = &models.Post{Pid: 42}
	m.Posts.CommentList = []models.Comment{
		{Cid: 1, Text: "First", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Text: "Second line", Timestamp: 1200, NameTag: "user2"},
	}
	m.Posts.SelectedCommentIdx = 1

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("quote shortcut should not emit async command")
	}
	if result.Dialog != DialogComposer {
		t.Fatalf("dialog = %v, want composer", result.Dialog)
	}
	if result.Composer.Mode() != ComposerModeComment {
		t.Fatalf("composer mode = %v, want comment", result.Composer.Mode())
	}
	quote := result.Composer.QuoteTarget()
	if quote == nil || quote.Cid != 2 {
		t.Fatalf("quote target = %+v, want selected comment #2", quote)
	}
	if !strings.Contains(result.Composer.View(80), "引用 #2 user2: Second line") {
		t.Fatalf("composer view missing quote preview: %q", result.Composer.View(80))
	}
}

func TestHandlePostsKeyCommentPageMoveKeepsSelectionVisible(t *testing.T) {
	m := newTestModel()
	m.Posts.ShowPostDetail = true
	m.Posts.CurrentPost = &models.Post{Pid: 42}
	m.Posts.DetailFocus = DetailFocusComments
	for i := 0; i < 10; i++ {
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid:       int32(i + 1),
			Text:      strings.Repeat("comment body ", 3),
			Timestamp: int32(1100 + i),
			NameTag:   "user",
		})
	}
	m.Posts.CommentViewport.Width = 28
	m.Posts.CommentViewport.Height = 5
	m.Posts.CommentViewport.SetContent(m.Posts.buildCommentContent(28))

	result, cmd := m.handlePostsKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if cmd != nil {
		t.Fatal("pgdown should not emit async command without more comments")
	}
	start, end := result.Posts.commentLineRangeAt(result.Posts.SelectedCommentIdx)
	top := result.Posts.CommentViewport.YOffset
	bottom := top + result.Posts.CommentViewport.VisibleLineCount() - 1
	if end < top || start > bottom {
		t.Fatalf("selected comment fell outside viewport after pgdown: range=[%d,%d] viewport=[%d,%d]", start, end, top, bottom)
	}
}

func TestHandleTagsDialogKeyTwoLevelSelectionAppliesChild(t *testing.T) {
	m := newTestModel()
	m.Dialog = DialogTags
	m.TagsDialog.SetTags([]models.Tag{
		{ID: 1, Name: "课程", ParentID: 0},
		{ID: 11, Label: "课程心得", ParentID: 1},
		{ID: 12, Label: "课程吐槽", ParentID: 1},
	})

	result, cmd := m.handleTagsDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("entering child tag phase should not trigger load command")
	}
	if result.Dialog != DialogTags {
		t.Fatalf("dialog after entering child phase = %v, want tags", result.Dialog)
	}
	if result.Posts.ActiveTagID != 0 {
		t.Fatalf("active tag changed too early: %d", result.Posts.ActiveTagID)
	}

	result, _ = result.handleTagsDialogKey(tea.KeyMsg{Type: tea.KeyDown})
	result, cmd = result.handleTagsDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if result.Dialog != DialogNone {
		t.Fatalf("dialog after applying child tag = %v, want none", result.Dialog)
	}
	if result.Posts.ActiveTagID != 12 {
		t.Fatalf("active tag = %d, want 12", result.Posts.ActiveTagID)
	}
	if result.Posts.ActiveTag != "课程吐槽" {
		t.Fatalf("active tag label = %q, want %q", result.Posts.ActiveTag, "课程吐槽")
	}
	if cmd == nil {
		t.Fatal("applying child tag should trigger reload command")
	}
}

func TestTogglePraiseCmdRefreshesPost(t *testing.T) {
	provider := &stubPostsProvider{refreshPost: &models.Post{Pid: 7}}

	msg := togglePraiseCmd(provider, 7)()
	result, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("message type = %T, want ActionResultMsg", msg)
	}
	if provider.togglePraisePID != 7 {
		t.Fatalf("toggle praise pid = %d, want 7", provider.togglePraisePID)
	}
	if provider.refreshCalls != 1 {
		t.Fatalf("refresh calls = %d, want 1", provider.refreshCalls)
	}
	if result.Post == nil || result.Post.Pid != 7 {
		t.Fatalf("result post = %+v, want refreshed post #7", result.Post)
	}
}

func TestCreateCommentCmdPassesQuoteID(t *testing.T) {
	provider := &stubPostsProvider{}
	quote := &models.Comment{Cid: 456}

	msg := createCommentCmd(provider, 99, "hello", quote)()
	result, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("message type = %T, want ActionResultMsg", msg)
	}
	if provider.createCommentPID != 99 || provider.createCommentText != "hello" {
		t.Fatalf("create comment payload = pid:%d text:%q", provider.createCommentPID, provider.createCommentText)
	}
	if provider.createCommentQuoteID == nil || *provider.createCommentQuoteID != 456 {
		t.Fatalf("quote id = %+v, want 456", provider.createCommentQuoteID)
	}
	if result.Error != nil || result.Kind != "comment" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
