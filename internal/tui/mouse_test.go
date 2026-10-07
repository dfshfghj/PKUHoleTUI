package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"treehole/internal/models"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func lineIndexContaining(lines []string, needle string) int {
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

func testListModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24
	m.Posts.PostList = []models.Post{
		{Pid: 111, Text: "ALPHA_MARKER", Timestamp: 1000, Anonymous: true},
		{Pid: 222, Text: "BETA_MARKER", Timestamp: 2000, Anonymous: true},
		{Pid: 333, Text: "GAMMA_MARKER", Timestamp: 3000, Anonymous: true},
	}
	m.Posts.SelectedPostIdx = 0
	m.Posts.CursorLine = 0
	m.syncPostsPage()
	return m
}

// TestPostsListRowBaselineMatchesFrame locks the geometry helpers to the
// actually rendered frame: every click/wheel mapping derives rows from the
// same helpers, so a drift here silently breaks mouse targeting.
func TestPostsListRowBaselineMatchesFrame(t *testing.T) {
	for _, width := range []int{80, 48, 30} {
		t.Run("width", func(t *testing.T) {
			m := testListModel(t)
			m.Width = width
			m.syncPostsPage()

			view := m.postsListView()
			if !view.Active {
				t.Fatal("list view should be active")
			}
			if view.Bar.Visible {
				t.Fatal("three short posts must not need a scrollbar")
			}
			if view.ContentWidth != maxInt(20, width-8) {
				t.Fatalf("content width = %d, want %d", view.ContentWidth, maxInt(20, width-8))
			}

			lines := frameLines(viewString(m))
			first := lineIndexContaining(lines, "#111")
			if first != view.TopY {
				t.Fatalf("first post header at row %d, want list top %d", first, view.TopY)
			}
			second := lineIndexContaining(lines, "#222")
			wantSecond := view.TopY + m.Posts.postRenderedLinesAt(0) + 1
			if second != wantSecond {
				t.Fatalf("second post header at row %d, want %d", second, wantSecond)
			}
			tabLine := lineIndexContaining(lines, "同步")
			if tabLine != 0 {
				t.Fatalf("tab bar row = %d, want 0", tabLine)
			}
			searchLine := lineIndexContaining(lines, "按 / 搜索")
			if searchLine != m.searchBoxRect().Y+1 {
				t.Fatalf("search box text at row %d, want %d", searchLine, m.searchBoxRect().Y+1)
			}
		})
	}
}

func overflowListModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24
	for i := 1; i <= 8; i++ {
		m.Posts.PostList = append(m.Posts.PostList, models.Post{
			Pid:       int32(i),
			Text:      strings.Repeat("x", 200),
			Timestamp: int32(1000) + int32(i),
			Anonymous: true,
		})
	}
	m.Posts.SelectedPostIdx = 0
	m.Posts.CursorLine = 0
	m.syncPostsPage()
	return m
}

func clickAt(m Model, x, y int) Model {
	next, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return next.(Model)
}

func wheelAt(m Model, x, y int, button tea.MouseButton) Model {
	next, _ := m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
	return next.(Model)
}

func TestScrollbarReservedOnlyOnOverflow(t *testing.T) {
	short := testListModel(t)
	if bar := short.Posts.postListBar(short.Width, short.contentAreaHeightForSize(short.Width, short.Height)); bar.Visible {
		t.Fatal("short list must not show a scrollbar")
	}

	long := overflowListModel(t)
	bar := long.Posts.postListBar(long.Width, long.contentAreaHeightForSize(long.Width, long.Height))
	if !bar.Visible {
		t.Fatal("overflowing list must show a scrollbar")
	}
	if want := long.Posts.postListBaseWidth(long.Width) - 1; long.Posts.currentListContentWidth() != want {
		t.Fatalf("content width = %d, want %d", long.Posts.currentListContentWidth(), want)
	}
	thumb := lipglossThumb()
	lines := frameLines(viewString(long))
	found := false
	for _, line := range lines {
		if strings.Contains(line, thumb) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("rendered frame should contain the scrollbar thumb %q", thumb)
	}
}

func lipglossThumb() string { return "█" }

// A wheel notch must be indistinguishable from repeated Down/Up presses: the
// cursor moves first, the page follows once the cursor nears an edge, and the
// load-more trigger stays reachable.
func TestMouseWheelMatchesRepeatedKeyPresses(t *testing.T) {
	wheeled := overflowListModel(t)
	keyed := overflowListModel(t)
	view := wheeled.postsListView()
	if !view.Bar.Visible {
		t.Fatal("expected overflow")
	}
	x, y := view.ContentWidth/2, view.TopY+2

	press := func(m Model, code rune, times int) Model {
		for i := 0; i < times; i++ {
			next, _ := m.Update(keyCode(code))
			m = next.(Model)
		}
		return m
	}
	compare := func(dir string, notch int) {
		t.Helper()
		if wheeled.Posts.CursorLine != keyed.Posts.CursorLine {
			t.Fatalf("%s notch %d: cursor = %d, keyed = %d", dir, notch, wheeled.Posts.CursorLine, keyed.Posts.CursorLine)
		}
		if wheeled.Posts.PostViewport.YOffset() != keyed.Posts.PostViewport.YOffset() {
			t.Fatalf("%s notch %d: offset = %d, keyed = %d", dir, notch, wheeled.Posts.PostViewport.YOffset(), keyed.Posts.PostViewport.YOffset())
		}
	}

	for i := 1; i <= 10; i++ {
		wheeled = wheelAt(wheeled, x, y, tea.MouseWheelDown)
		keyed = press(keyed, tea.KeyDown, mouseWheelLines)
		compare("down", i)
	}
	for i := 1; i <= 12; i++ {
		wheeled = wheelAt(wheeled, x, y, tea.MouseWheelUp)
		keyed = press(keyed, tea.KeyUp, mouseWheelLines)
		compare("up", i)
	}

	// Wheel above the list (tab bar) must not move anything.
	before := wheeled.Posts.CursorLine
	wheeled = wheelAt(wheeled, x, 0, tea.MouseWheelDown)
	if wheeled.Posts.CursorLine != before {
		t.Fatal("wheel on the tab bar must not move the list cursor")
	}
}

func TestMouseWheelClampsAtBottomAndTop(t *testing.T) {
	m := overflowListModel(t)
	view := m.postsListView()
	x, y := view.ContentWidth/2, view.TopY+1
	total := m.Posts.totalPostLines()

	for i := 0; i < total; i++ {
		m = wheelAt(m, x, y, tea.MouseWheelDown)
	}
	if got := m.Posts.CursorLine; got != total-1 {
		t.Fatalf("cursor at bottom = %d, want %d", got, total-1)
	}
	offset := m.Posts.PostViewport.YOffset()
	visible := m.Posts.PostViewport.VisibleLineCount()
	if m.Posts.CursorLine < offset || m.Posts.CursorLine >= offset+visible {
		t.Fatalf("cursor line %d outside viewport [%d,%d)", m.Posts.CursorLine, offset, offset+visible)
	}

	for i := 0; i < total; i++ {
		m = wheelAt(m, x, y, tea.MouseWheelUp)
	}
	if got := m.Posts.PostViewport.YOffset(); got != 0 {
		t.Fatalf("offset at top = %d, want 0", got)
	}
	if got := m.Posts.CursorLine; got != 0 {
		t.Fatalf("cursor at top = %d, want 0", got)
	}
}

// Wheeling towards the end of the loaded list must request the next page, the
// same way pressing Down would.
func TestMouseWheelNearEndRequestsMorePosts(t *testing.T) {
	m := overflowListModel(t)
	m.Posts.PostListHasMore = true
	m.Posts.PostListLoading = false
	total := m.Posts.totalPostLines()
	nearEnd := total - 3
	m.Posts.moveCursor(nearEnd - m.Posts.CursorLine)
	m.syncPostsPage()

	view := m.postsListView()
	next, cmd := m.Update(tea.MouseWheelMsg{X: view.ContentWidth / 2, Y: view.TopY + 1, Button: tea.MouseWheelDown})
	m = next.(Model)
	if !m.Posts.PostListLoading {
		t.Fatal("wheeling near the end should start loading more posts")
	}
	if cmd == nil {
		t.Fatal("wheeling near the end should return a load command")
	}
}

func TestMouseClickTabSwitchesPage(t *testing.T) {
	m := testListModel(t)
	rects := m.tabRects(m.Width)
	if len(rects) != 4 {
		t.Fatalf("tab rects = %d, want 4", len(rects))
	}
	m = clickAt(m, rects[2].X+1, 0)
	if m.Page != PageSchedule || m.TabCursor != 2 {
		t.Fatalf("page/tab = %v/%d, want schedule/2", m.Page, m.TabCursor)
	}
}

func TestClickSearchBoxStartsSearch(t *testing.T) {
	m := testListModel(t)
	box := m.searchBoxRect()
	m = clickAt(m, 4, box.Y+1)
	if !m.Posts.Searching {
		t.Fatal("clicking the search box should start search input")
	}
}

// glyphPoint returns the click point of the nth occurrence of glyph in the
// rendered frame, so the test targets exactly what the user sees.
func glyphPoint(t *testing.T, m Model, glyph string, occurrence int) (int, int) {
	t.Helper()
	seen := 0
	for row, line := range frameLines(viewString(m)) {
		idx := 0
		for {
			rel := strings.Index(line[idx:], glyph)
			if rel < 0 {
				break
			}
			seen++
			if seen == occurrence {
				return lipgloss.Width(line[:idx+rel]), row
			}
			idx += rel + len(glyph)
		}
	}
	t.Fatalf("glyph %q occurrence %d not found in frame", glyph, occurrence)
	return 0, 0
}

func glyphActionModel(t *testing.T) (Model, *stubPostsProvider) {
	t.Helper()
	m := testListModel(t)
	stub := &stubPostsProvider{canWrite: true, mode: SessionModeOnline}
	m.Provider = stub
	m.Session.Mode = SessionModeOnline
	m.Session.CanWriteOnline = true
	m.syncPostsPage()
	return m, stub
}

func TestClickPraiseAndFollowGlyphsRunActions(t *testing.T) {
	m, stub := glyphActionModel(t)

	x, y := glyphPoint(t, m, "♡", 2) // the second post's praise glyph
	next, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("clicking ♡ should run a command")
	}
	if _, ok := cmd().(ActionResultMsg); !ok {
		t.Fatal("clicking ♡ should toggle praise")
	}
	if stub.togglePraisePID != 222 {
		t.Fatalf("praise pid = %d, want 222", stub.togglePraisePID)
	}

	x, y = glyphPoint(t, m, "☆", 2) // the second post's follow glyph
	next, cmd = m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("clicking ☆ should run a command")
	}
	if _, ok := cmd().(ActionResultMsg); !ok {
		t.Fatal("clicking ☆ should toggle follow")
	}
	if stub.toggleAttentionPID != 222 {
		t.Fatalf("follow pid = %d, want 222", stub.toggleAttentionPID)
	}
}

// Clicking a glyph twice must toggle twice, not open the detail page.
func TestDoubleClickOnGlyphDoesNotOpenDetail(t *testing.T) {
	m, stub := glyphActionModel(t)
	x, y := glyphPoint(t, m, "♡", 1)
	for i := 0; i < 2; i++ {
		next, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		m = next.(Model)
		if cmd != nil {
			cmd()
		}
	}
	if m.Posts.ShowPostDetail {
		t.Fatal("clicking a glyph twice must not open the detail page")
	}
	if stub.togglePraisePID != 111 {
		t.Fatalf("praise pid = %d, want 111", stub.togglePraisePID)
	}
}

func TestClickDetailHeaderGlyphRunsAction(t *testing.T) {
	m := detailTestModel(t)
	stub := &stubPostsProvider{canWrite: true, mode: SessionModeOnline}
	m.Provider = stub
	m.Session.Mode = SessionModeOnline
	m.Session.CanWriteOnline = true
	m.syncPostsPage()

	detail := m.postsDetailView()
	if !detail.Active || detail.Header.H <= 0 {
		t.Fatal("detail header rect should be present")
	}
	header := detail.Header
	lines := frameLines(viewString(m))
	if header.Y >= len(lines) {
		t.Fatalf("header row %d outside frame", header.Y)
	}
	idx := strings.Index(lines[header.Y], "♡")
	if idx < 0 {
		t.Fatalf("praise glyph missing from detail header %q", lines[header.Y])
	}
	x := lipgloss.Width(lines[header.Y][:idx])

	next, cmd := m.Update(tea.MouseClickMsg{X: x, Y: header.Y, Button: tea.MouseLeft})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("clicking the detail header ♡ should run a command")
	}
	if _, ok := cmd().(ActionResultMsg); !ok {
		t.Fatal("clicking the detail header ♡ should toggle praise")
	}
	if stub.togglePraisePID != 42 {
		t.Fatalf("praise pid = %d, want 42", stub.togglePraisePID)
	}
}

// M (or :mouse) must stop mouse reporting so the terminal regains its own
// text selection, and stop the app from consuming any stray events.
func TestMouseToggleDisablesReporting(t *testing.T) {
	m := testListModel(t)
	if got := viewString(m); got == "" {
		t.Fatal("frame should render")
	}
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("mouse reporting should start enabled")
	}

	next, _ := m.Update(keyPress('M'))
	m = next.(Model)
	if !m.MouseDisabled {
		t.Fatal("M should disable the mouse")
	}
	if m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("disabled mouse must stop reporting (MouseModeNone)")
	}

	box := m.searchBoxRect()
	if clicked := clickAt(m, 4, box.Y+1); clicked.Posts.Searching {
		t.Fatal("clicks must be ignored while the mouse is disabled")
	}

	next, _ = m.Update(keyPress('M'))
	m = next.(Model)
	if m.MouseDisabled {
		t.Fatal("M should re-enable the mouse")
	}
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("re-enabled mouse must report again")
	}
	if clicked := clickAt(m, 4, box.Y+1); !clicked.Posts.Searching {
		t.Fatal("clicks should work again once the mouse is re-enabled")
	}

	// The :mouse command toggles as well.
	next, _ = m.executeCommand("mouse")
	m = next.(Model)
	if !m.MouseDisabled {
		t.Fatal(":mouse should disable the mouse")
	}
}

func TestToastSwallowsClicks(t *testing.T) {
	m := testListModel(t)
	m.ToastMsg = "测试提示"
	_, toastRect, ok := m.toastView(m.Width)
	if !ok {
		t.Fatal("toast rect should be visible")
	}
	box := m.searchBoxRect()
	x, y := toastRect.X+2, toastRect.Y+toastRect.H-1
	if !box.contains(x, y) {
		t.Fatalf("toast rect %+v must overlap the search box %+v for this test", toastRect, box)
	}

	if clicked := clickAt(m, x, y); clicked.Posts.Searching {
		t.Fatal("clicks on the toast overlay must not reach the search box")
	}

	m.ToastMsg = ""
	if clicked := clickAt(m, x, y); !clicked.Posts.Searching {
		t.Fatal("the same click must reach the search box once the toast is gone")
	}
}

func TestMouseClickSelectsPostAndDoubleClickOpensDetail(t *testing.T) {
	m := testListModel(t)
	lines := frameLines(viewString(m))
	second := lineIndexContaining(lines, "#222")
	if second < 0 {
		t.Fatal("second post not rendered")
	}
	y := second

	m = clickAt(m, 3, y)
	if m.Posts.SelectedPostIdx != 1 {
		t.Fatalf("selected post = %d, want 1", m.Posts.SelectedPostIdx)
	}
	if m.Posts.ShowPostDetail {
		t.Fatal("single click must not open the detail page")
	}

	m = clickAt(m, 3, y)
	if !m.Posts.ShowPostDetail {
		t.Fatal("double click should open the post detail")
	}
	if m.Posts.CurrentPost == nil || m.Posts.CurrentPost.Pid != 222 {
		t.Fatalf("current post = %#v, want pid 222", m.Posts.CurrentPost)
	}
}

func TestStaleDoubleClickDoesNotOpenDetail(t *testing.T) {
	m := testListModel(t)
	view := m.postsListView()
	y := view.TopY
	m = clickAt(m, 3, y)
	m.Clicks.LastAt = m.Clicks.LastAt.Add(-time.Second)
	m = clickAt(m, 3, y)
	if m.Posts.ShowPostDetail {
		t.Fatal("clicks far apart must not count as a double click")
	}
}

func TestDragScrollbarFollowsMouse(t *testing.T) {
	m := overflowListModel(t)
	view := m.postsListView()
	bar := view.Bar
	if !bar.Visible {
		t.Fatal("expected overflow")
	}

	// Press on the thumb and drag it to the bottom of the track.
	grabY := view.TopY + bar.ThumbPos
	m = clickAt(m, view.BarX, grabY)
	if !m.Drag.Active || m.Drag.Target != mouseTargetPostsList {
		t.Fatalf("drag state = %+v, want active list drag", m.Drag)
	}
	if got := m.Posts.PostViewport.YOffset(); got != 0 {
		t.Fatalf("pressing the thumb must not jump, offset = %d", got)
	}

	bottomY := view.TopY + bar.TrackH - 1
	moved, _ := m.Update(tea.MouseMotionMsg{X: view.BarX, Y: bottomY, Button: tea.MouseLeft})
	m = moved.(Model)
	if got, want := m.Posts.PostViewport.YOffset(), bar.MaxOffset; got != want {
		t.Fatalf("drag to bottom offset = %d, want %d", got, want)
	}

	released, _ := m.Update(tea.MouseReleaseMsg{X: view.BarX, Y: bottomY, Button: tea.MouseLeft})
	m = released.(Model)
	if m.Drag.Active {
		t.Fatal("release should end the drag")
	}
}

func TestTrackClickJumps(t *testing.T) {
	m := overflowListModel(t)
	view := m.postsListView()
	bar := view.Bar

	// Click one row below the thumb: the thumb centers there and the offset
	// moves proportionally.
	target := view.TopY + bar.ThumbPos + bar.ThumbSize + 1
	if target >= view.TopY+bar.TrackH {
		t.Skip("track too short for a below-thumb click")
	}
	m = clickAt(m, view.BarX, target)
	offset := m.Posts.PostViewport.YOffset()
	if offset <= 0 {
		t.Fatalf("track click should jump, offset = %d", offset)
	}
	if !m.Drag.Active {
		t.Fatal("track click should start a drag")
	}
	want := bar.offsetAtRow(target-view.TopY, m.Drag.GrabDelta)
	if offset != want {
		t.Fatalf("offset = %d, want %d", offset, want)
	}
}

func TestResizeCancelsDrag(t *testing.T) {
	m := overflowListModel(t)
	view := m.postsListView()
	m = clickAt(m, view.BarX, view.TopY+view.Bar.ThumbPos)
	if !m.Drag.Active {
		t.Fatal("expected an active drag")
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m = next.(Model)
	if m.Drag.Active {
		t.Fatal("resize should cancel the drag")
	}
}

func detailTestModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 80
	m.Height = 24
	m.Posts.ShowPostDetail = true
	m.Posts.PostsMode = PostsModeDetail
	m.Posts.CurrentPost = &models.Post{
		Pid: 42, Text: "BODYMARKER " + strings.Repeat("B", 3000), Timestamp: 1000, Reply: 6,
	}
	for i := 1; i <= 6; i++ {
		m.Posts.CommentList = append(m.Posts.CommentList, models.Comment{
			Cid: int32(i), Pid: 42, NameTag: "user",
			Text:      "CMTMARKER" + string(rune('0'+i)) + " " + strings.Repeat("c", 150),
			Timestamp: int32(1000 + i),
		})
	}
	m.Posts.DetailFocus = DetailFocusComments
	m.syncPostsPage()
	return m
}

// TestDetailLayoutMatchesFrame locks the detail geometry (body rect,
// comments rect and both scrollbars) to the rendered frame.
func TestDetailLayoutMatchesFrame(t *testing.T) {
	m := detailTestModel(t)
	detail := m.postsDetailView()
	if !detail.Active || !detail.HasComments {
		t.Fatal("detail view with comments should be active")
	}
	if !detail.BodyBar.Visible {
		t.Fatal("long body should show a scrollbar")
	}
	if !detail.CommentsBar.Visible {
		t.Fatal("long comments should show a scrollbar")
	}
	if detail.Body.W != m.Posts.PostBodyViewport.Width() {
		t.Fatalf("body rect width %d != viewport width %d", detail.Body.W, m.Posts.PostBodyViewport.Width())
	}
	if detail.Comments.W != m.Posts.CommentViewport.Width() {
		t.Fatalf("comments rect width %d != viewport width %d", detail.Comments.W, m.Posts.CommentViewport.Width())
	}

	lines := frameLines(viewString(m))
	if got := lineIndexContaining(lines, "BODYMARKER"); got != detail.Body.Y {
		t.Fatalf("body first row = %d, want %d", got, detail.Body.Y)
	}
	if got := lineIndexContaining(lines, "CMTMARKER1"); got != detail.Comments.Y {
		t.Fatalf("first comment row = %d, want %d", got, detail.Comments.Y)
	}
	if got := lineIndexContaining(lines, "评论"); got < detail.CommentsTitle.Y ||
		got >= detail.CommentsTitle.Y+detail.CommentsTitle.H {
		t.Fatalf("comments title row = %d, want within %+v", got, detail.CommentsTitle)
	}
}

func TestMouseWheelScrollsDetailPanes(t *testing.T) {
	m := detailTestModel(t)
	detail := m.postsDetailView()

	m = wheelAt(m, detail.Body.X+2, detail.Body.Y+1, tea.MouseWheelDown)
	if got := m.Posts.PostBodyViewport.YOffset(); got != mouseWheelLines {
		t.Fatalf("body offset = %d, want %d", got, mouseWheelLines)
	}
	if got := m.Posts.CommentViewport.YOffset(); got != 0 {
		t.Fatalf("comments should not scroll when the wheel is over the body, offset = %d", got)
	}

	// Over the comments the wheel moves the comment cursor first and the pane
	// only follows once the cursor nears the bottom margin.
	startLine := m.Posts.CommentCursorLine
	m = wheelAt(m, detail.Comments.X+2, detail.Comments.Y+1, tea.MouseWheelDown)
	if got := m.Posts.CommentCursorLine; got != startLine+mouseWheelLines {
		t.Fatalf("comment cursor line = %d, want %d", got, startLine+mouseWheelLines)
	}
	if got := m.Posts.CommentViewport.YOffset(); got != 0 {
		t.Fatalf("comments pane must stay put until the cursor nears the edge, offset = %d", got)
	}

	total := m.Posts.commentLineCount()
	for i := 0; i < total; i++ {
		m = wheelAt(m, detail.Comments.X+2, detail.Comments.Y+1, tea.MouseWheelDown)
	}
	offset := m.Posts.CommentViewport.YOffset()
	if offset == 0 {
		t.Fatal("comments pane should follow the cursor after it passes the bottom margin")
	}
	visible := m.Posts.CommentViewport.VisibleLineCount()
	if m.Posts.CommentCursorLine < offset || m.Posts.CommentCursorLine >= offset+visible {
		t.Fatalf("comment cursor line %d outside viewport [%d,%d)", m.Posts.CommentCursorLine, offset, offset+visible)
	}
}

func TestMouseClickDetailFocusAndCommentSelect(t *testing.T) {
	m := detailTestModel(t)
	detail := m.postsDetailView()

	m = clickAt(m, detail.Body.X+3, detail.Body.Y+1)
	if m.Posts.DetailFocus != DetailFocusPost {
		t.Fatal("clicking the body should focus the post body")
	}

	lines := frameLines(viewString(m))
	markerRow := lineIndexContaining(lines, "CMTMARKER3")
	if markerRow < 0 {
		t.Fatal("comment 3 not rendered")
	}
	m = clickAt(m, detail.Comments.X+3, markerRow)
	if m.Posts.DetailFocus != DetailFocusComments {
		t.Fatal("clicking a comment should focus the comments pane")
	}
	if m.Posts.SelectedCommentIdx != 2 {
		t.Fatalf("selected comment = %d, want 2", m.Posts.SelectedCommentIdx)
	}
}

func TestDetailScrollbarDragTargetsBody(t *testing.T) {
	m := detailTestModel(t)
	detail := m.postsDetailView()
	barX := detail.Body.X + detail.Body.W
	y := detail.Body.Y + detail.BodyBar.ThumbPos
	m = clickAt(m, barX, y)
	if !m.Drag.Active || m.Drag.Target != mouseTargetPostBody {
		t.Fatalf("drag = %+v, want body drag", m.Drag)
	}
	bottom := detail.Body.Y + detail.BodyBar.TrackH - 1
	moved, _ := m.Update(tea.MouseMotionMsg{X: barX, Y: bottom, Button: tea.MouseLeft})
	m = moved.(Model)
	if got, want := m.Posts.PostBodyViewport.YOffset(), detail.BodyBar.MaxOffset; got != want {
		t.Fatalf("body drag offset = %d, want %d", got, want)
	}
}

func TestWheelOverCommentsBarStillMovesComments(t *testing.T) {
	m := detailTestModel(t)
	detail := m.postsDetailView()
	barX := detail.Comments.X + detail.Comments.W
	start := m.Posts.CommentCursorLine
	m = wheelAt(m, barX, detail.Comments.Y+1, tea.MouseWheelDown)
	if got := m.Posts.CommentCursorLine; got != start+mouseWheelLines {
		t.Fatalf("comment cursor line = %d, want %d", got, start+mouseWheelLines)
	}
}

// TestClickRowSelectsRenderedPost verifies the row→post mapping used by
// mouse clicks against the rendered frame.
func TestClickRowSelectsRenderedPost(t *testing.T) {
	m := testListModel(t)
	lines := frameLines(viewString(m))

	second := lineIndexContaining(lines, "#222")
	view := m.postsListView()

	// content line = viewport y offset + row inside the viewport
	got := m.Posts
	target := second - view.TopY + m.Posts.PostViewport.YOffset()
	if !got.selectPostAtContentLine(target) {
		t.Fatal("selectPostAtContentLine should succeed")
	}
	if got.SelectedPostIdx != 1 {
		t.Fatalf("selected post = %d, want 1", got.SelectedPostIdx)
	}
	if got.CursorLine != target {
		t.Fatalf("cursor line = %d, want %d", got.CursorLine, target)
	}
}

func TestLogsDialogWheelScrollsAndShowsBar(t *testing.T) {
	m := newTestModel()
	m.Width = 80
	m.Height = 40
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "log line"
	}
	m.ToolsDialog.Logs.SetLines(lines)

	body := m.panelBodyView(m.Width, m.Height)
	m = wheelAt(m, body.X+2, body.Y+1, tea.MouseWheelDown)
	if got := m.ToolsDialog.Logs.Offset(); got != mouseWheelLines {
		t.Fatalf("logs offset = %d, want %d", got, mouseWheelLines)
	}

	frame := stripANSI(viewString(m))
	if !strings.Contains(frame, "█") {
		t.Fatal("overflowing logs dialog should render a scrollbar thumb")
	}

	// Drag the bar to the bottom.
	contentWidth, visibleLines, bar := m.ToolsDialog.Logs.logsLayout(body.W, body.H)
	if !bar.Visible {
		t.Fatal("expected a visible logs scrollbar")
	}
	barX := body.X + contentWidth
	m = clickAt(m, barX, body.Y+bar.ThumbPos)
	if !m.Drag.Active || m.Drag.Target != mouseTargetLogs {
		t.Fatalf("drag = %+v, want logs drag", m.Drag)
	}
	moved, _ := m.Update(tea.MouseMotionMsg{X: barX, Y: body.Y + visibleLines - 1, Button: tea.MouseLeft})
	m = moved.(Model)
	if got, want := m.ToolsDialog.Logs.Offset(), bar.MaxOffset; got != want {
		t.Fatalf("logs drag offset = %d, want %d", got, want)
	}
}

func TestConfigEditorClickPlacesCursor(t *testing.T) {
	m := newTestModel()
	m.Width = 80
	m.Height = 40
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config.lines = []string{"aaaa", "bbbb", "cccc", "dddd"}
	m.ToolsDialog.Config.offset = 0

	body := m.panelBodyView(m.Width, m.Height)
	lines := frameLines(viewString(m))
	row := lineIndexContaining(lines, "cccc")
	if row < 0 {
		t.Fatal("config row not rendered")
	}
	lineNumberWidth, separatorWidth, _, _ := m.ToolsDialog.Config.editorLayout(body.W, body.H)
	m = clickAt(m, body.X+lineNumberWidth+separatorWidth+2, row)
	if m.ToolsDialog.Config.cursorRow != 2 {
		t.Fatalf("cursor row = %d, want 2", m.ToolsDialog.Config.cursorRow)
	}
	if m.ToolsDialog.Config.cursorCol != 2 {
		t.Fatalf("cursor col = %d, want 2", m.ToolsDialog.Config.cursorCol)
	}
}

func TestNotificationClickSelectsItem(t *testing.T) {
	m := newTestModel()
	m.Width = 80
	m.Height = 40
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionInteractive)
	items := []models.Notification{
		{ID: 1, Content: "first"},
		{ID: 2, Content: "second"},
		{ID: 3, Content: "third"},
	}
	m.ToolsDialog.Notifications.SetNotifications(models.NotificationTypeInteractive, items, len(items))

	body := m.panelBodyView(m.Width, m.Height)
	rendered, start, _, _, _ := m.ToolsDialog.Notifications.windowLayout(body.W, body.H)
	if start != 0 {
		t.Fatalf("window start = %d, want 0", start)
	}
	// Click on the second item's title row.
	y := body.Y + lipgloss.Height(rendered[0]) + 1
	m = clickAt(m, body.X+4, y)
	if m.ToolsDialog.Notifications.selected != 1 {
		t.Fatalf("selected notification = %d, want 1", m.ToolsDialog.Notifications.selected)
	}
}

func TestScoresPageScrollbarAndWheel(t *testing.T) {
	m := newTestModel()
	m.Page = PageScores
	m.Width = 80
	m.Height = 24
	m.TabCursor = 3
	scores := make([]models.CourseScore, 0, 40)
	for i := 0; i < 40; i++ {
		scores = append(scores, models.CourseScore{YearTerm: "2024-1", Name: "course", Credit: "3.0", Score: "90"})
	}
	m.Scores.Summary = &models.ScoreSummary{Scores: scores}

	view := m.scoresPageView()
	if !view.Active || !view.Bar.Visible {
		t.Fatalf("scores view = %+v, want active with a scrollbar", view)
	}
	m = wheelAt(m, 4, view.Rows.Y+1, tea.MouseWheelDown)
	if m.Scores.Offset != mouseWheelLines {
		t.Fatalf("scores offset = %d, want %d", m.Scores.Offset, mouseWheelLines)
	}
	frame := stripANSI(viewString(m))
	if !strings.Contains(frame, "█") {
		t.Fatal("overflowing scores table should render a scrollbar thumb")
	}
}

func TestDialogIsModalForMouse(t *testing.T) {
	m := testListModel(t)
	before := m.Posts.SelectedPostIdx
	m.Dialog = DialogHelp
	m = clickAt(m, 3, m.postsListView().TopY+1)
	if m.Posts.SelectedPostIdx != before {
		t.Fatal("clicks under an open dialog must not reach the list")
	}
	m.Dialog = DialogTags
	m = clickAt(m, 3, m.postsListView().TopY+1)
	if m.Posts.SelectedPostIdx != before {
		t.Fatal("clicks under the tags dialog must not reach the list")
	}
}

func testHomeModel(t *testing.T) Model {
	t.Helper()
	useTempWorkingDir(t, t.TempDir())
	m := newTestModel()
	m.Page = PageHome
	m.Width = 80
	m.Height = 30
	m.Home.Jobs = []CrawlerJob{
		newCrawlerJob("任务一", defaultCrawlerParams()),
		newCrawlerJob("任务二", defaultCrawlerParams()),
	}
	m.Home.SelectedJob = 0
	m.Home.Focus = HomeFocusJobs
	return m
}

// TestHomeRowGeometryMatchesFrame locks the sync page row rectangles to the
// rendered frame so clicks land on what the user sees.
func TestHomeRowGeometryMatchesFrame(t *testing.T) {
	m := testHomeModel(t)
	view := m.homePageView()
	if !view.Active {
		t.Fatal("home view should be active on PageHome")
	}
	lines := frameLines(viewString(m))
	for idx, row := range view.Jobs {
		if row.Y < 0 || row.Y >= len(lines) {
			t.Fatalf("job %d row %d outside the frame", idx, row.Y)
		}
		if !strings.Contains(lines[row.Y], m.Home.Jobs[idx].Name) {
			t.Fatalf("row %d = %q, want job %q", row.Y, lines[row.Y], m.Home.Jobs[idx].Name)
		}
	}
	for idx, row := range view.Params {
		if row.Y < 0 || row.Y >= len(lines) {
			t.Fatalf("param row %d outside the frame", idx)
		}
		if !strings.Contains(lines[row.Y], m.Home.paramRows()[idx].Label) {
			t.Fatalf("row %d = %q, want param %q", row.Y, lines[row.Y], m.Home.paramRows()[idx].Label)
		}
	}
}

func TestHomeMouseClickSelectsJobAndTogglesParams(t *testing.T) {
	m := testHomeModel(t)
	view := m.homePageView()

	m = clickAt(m, 5, view.Jobs[1].Y)
	if m.Home.SelectedJob != 1 {
		t.Fatalf("selected job = %d, want 1 after clicking the second row", m.Home.SelectedJob)
	}
	if m.Home.Focus != HomeFocusJobs {
		t.Fatalf("focus = %v, want the job list", m.Home.Focus)
	}

	rows := m.Home.paramRows()
	boolIdx := -1
	startIdx := -1
	for i, row := range rows {
		switch row.Label {
		case "抓取图片":
			boolIdx = i
		case "起始页":
			startIdx = i
		}
	}
	if boolIdx < 0 || startIdx < 0 {
		t.Fatalf("param rows missing expected entries: %+v", rows)
	}

	before := m.Home.Jobs[1].Params.FetchImages
	m = clickAt(m, 30, view.Params[boolIdx].Y)
	if m.Home.Jobs[1].Params.FetchImages == before {
		t.Fatal("clicking the 抓取图片 checkbox should toggle it")
	}
	if m.Home.Focus != HomeFocusParams || m.Home.ParamRow != boolIdx {
		t.Fatalf("focus/row = %v/%d, want params/%d", m.Home.Focus, m.Home.ParamRow, boolIdx)
	}

	m = clickAt(m, 20, view.Params[startIdx].Y)
	if !m.Home.ParamEditing() {
		t.Fatal("clicking a numeric parameter should focus the inline input")
	}
}

func TestHomeMouseClickSelectsCrawlMode(t *testing.T) {
	m := testHomeModel(t)
	view := m.homePageView()
	modeRow := view.Params[0]
	spans := homeModeOptionSpans()

	for i, mode := range homeModeOptions {
		x := modeRow.X + (spans[i][0]+spans[i][1])/2
		m = clickAt(m, x, modeRow.Y)
		if got := m.Home.Jobs[m.Home.SelectedJob].Params.Mode; got != mode {
			t.Fatalf("clicking option %d (%s) -> mode %v, want %v", i, crawlModeShort(mode), got, mode)
		}
		if m.Home.Focus != HomeFocusParams || m.Home.ParamRow != 0 {
			t.Fatalf("focus/row = %v/%d, want params/0", m.Home.Focus, m.Home.ParamRow)
		}
	}
}

// TestHomeModeRowGeometryMatchesFrame locks the option spans to the rendered
// row so a click lands on the option the user sees.
func TestHomeModeRowGeometryMatchesFrame(t *testing.T) {
	m := testHomeModel(t)
	m.Home.Focus = HomeFocusParams
	m.Home.ParamRow = 0

	lines := frameLines(viewString(m))
	modeLine := -1
	for i, line := range lines {
		if strings.Contains(line, "类型") {
			modeLine = i
			break
		}
	}
	if modeLine < 0 {
		t.Fatal("mode row not found in the rendered frame")
	}
	plain := lines[modeLine]
	spans := homeModeOptionSpans()
	for i, mode := range homeModeOptions {
		name := crawlModeShort(mode)
		idx := strings.Index(plain, name)
		if idx < 0 {
			t.Fatalf("option %q missing from row %q", name, plain)
		}
		if col := lipgloss.Width(plain[:idx]); col != spans[i][0]+2 {
			t.Fatalf("option %q starts at column %d, want %d (spans=%v)",
				name, col, spans[i][0]+2, spans)
		}
	}
}

func TestHomeMouseWheelMovesSelection(t *testing.T) {
	m := testHomeModel(t)
	view := m.homePageView()

	m = wheelAt(m, 5, view.Jobs[0].Y, tea.MouseWheelDown)
	if m.Home.SelectedJob != 1 {
		t.Fatalf("selected job = %d, want 1 after one wheel notch", m.Home.SelectedJob)
	}
	m = wheelAt(m, 5, view.Jobs[1].Y, tea.MouseWheelDown)
	if m.Home.SelectedJob != 1 {
		t.Fatalf("selection must clamp at the last job, got %d", m.Home.SelectedJob)
	}
	m = wheelAt(m, 5, view.Jobs[1].Y, tea.MouseWheelUp)
	if m.Home.SelectedJob != 0 {
		t.Fatalf("selected job = %d, want 0 after wheel up", m.Home.SelectedJob)
	}

	m.Home.Focus = HomeFocusParams
	m.Home.ParamRow = 0
	paramView := m.homePageView()
	m = wheelAt(m, 20, paramView.Params[1].Y, tea.MouseWheelDown)
	if m.Home.ParamRow != 1 {
		t.Fatalf("param row = %d, want 1 after wheel over the panel", m.Home.ParamRow)
	}
}

func TestJobLogDialogWheelScrolls(t *testing.T) {
	m := testHomeModel(t)
	m.Dialog = DialogJobLog
	m.JobLog.Open(m.Home.Jobs[0])
	lines := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("[Crawler] 第 %d 页完成", i+1))
	}
	m.JobLog.SetLines(lines)

	body := m.panelBodyView(m.Width, m.Height)
	m = wheelAt(m, body.X+2, body.Y+1, tea.MouseWheelDown)
	if m.JobLog.Logs.Offset() == 0 {
		t.Fatal("wheel over the job log body should scroll it")
	}
}
