package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// mouseWheelLines is the number of lines one wheel notch scrolls.
const mouseWheelLines = 3

// doubleClickWindow is the maximum delay between two clicks on the same spot
// that still counts as a double click.
const doubleClickWindow = 400 * time.Millisecond

type mouseTarget int

const (
	mouseTargetNone mouseTarget = iota
	mouseTargetTabBar
	mouseTargetSearchBox
	mouseTargetPostsList
	mouseTargetPostBody
	mouseTargetComments
	mouseTargetScores
	mouseTargetLogs
	mouseTargetConfig
	mouseTargetNotifications
)

// dragState tracks an in-progress scrollbar drag.
type dragState struct {
	Active    bool
	Target    mouseTarget
	TrackTop  int
	TrackH    int
	ThumbSize int
	MaxOffset int
	GrabDelta int
}

// clickTracker detects double clicks on the same cell region.
type clickTracker struct {
	LastAt time.Time
	LastX  int
	LastY  int
}

func (c *clickTracker) register(x, y int, now time.Time) (double bool) {
	double = !c.LastAt.IsZero() &&
		now.Sub(c.LastAt) <= doubleClickWindow &&
		absInt(x-c.LastX) <= 2 && absInt(y-c.LastY) <= 2
	c.LastAt = now
	c.LastX = x
	c.LastY = y
	return double
}

func (c *clickTracker) reset() {
	c.LastAt = time.Time{}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// toggleMouse turns terminal mouse reporting on or off. With reporting off the
// terminal regains its native text selection (and cannot send clicks/wheel).
func (m Model) toggleMouse() (Model, tea.Cmd) {
	m.MouseDisabled = !m.MouseDisabled
	m.Drag = dragState{}
	m.Clicks.reset()
	if m.MouseDisabled {
		return m, m.showToast("鼠标已关闭，可直接用终端选择文本（M 或 :mouse 恢复）")
	}
	return m, m.showToast("鼠标已开启")
}

// handleMouseClick routes a left click to the top-most mouse-aware widget.
func (m Model) handleMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || m.Drag.Active {
		return m, nil
	}
	if _, toastRect, ok := m.toastView(m.Width); ok && toastRect.contains(msg.X, msg.Y) {
		return m, nil
	}
	if m.Dialog != DialogNone {
		return m.handleDialogMouseClick(msg)
	}
	for i, tab := range m.tabRects(m.Width) {
		if tab.contains(msg.X, msg.Y) {
			if Page(i) == m.Page {
				return m, nil
			}
			return m.switchPage(Page(i))
		}
	}
	switch m.Page {
	case PagePosts:
		return m.handlePostsMouseClick(msg)
	case PageScores:
		return m.handleScoresMouseClick(msg)
	}
	return m, nil
}

func (m Model) handleScoresMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	view := m.scoresPageView()
	if !view.Active {
		return m, nil
	}
	if view.Bar.Visible && msg.X == view.BarX && msg.Y >= view.Rows.Y && msg.Y < view.Rows.Y+view.Rows.H {
		return m.beginScrollbarDrag(mouseTargetScores, view.Bar, view.Rows.Y, msg.Y-view.Rows.Y), nil
	}
	return m, nil
}

// handleMouseWheel scrolls the container under the cursor.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -mouseWheelLines
	case tea.MouseWheelDown:
		delta = mouseWheelLines
	default:
		return m, nil
	}
	if m.Dialog != DialogNone {
		m.scrollDialogAt(msg.X, msg.Y, delta)
		return m, nil
	}
	switch m.Page {
	case PagePosts:
		return m.scrollPostsAt(msg.X, msg.Y, delta)
	case PageScores:
		m.Scores.Move(delta, m.contentAreaHeightForSize(m.Width, m.Height))
	}
	return m, nil
}

// handleMouseMotion continues an active scrollbar drag.
func (m Model) handleMouseMotion(msg tea.MouseMotionMsg) (Model, tea.Cmd) {
	if !m.Drag.Active || msg.Button != tea.MouseLeft {
		return m, nil
	}
	bar := m.Drag.bar()
	offset := bar.offsetAtRow(msg.Y-m.Drag.TrackTop, m.Drag.GrabDelta)
	m.applyScrollOffset(m.Drag.Target, offset)
	return m, nil
}

func (m Model) handleMouseRelease(msg tea.MouseReleaseMsg) (Model, tea.Cmd) {
	target := m.Drag.Target
	m.Drag = dragState{}
	// A drag that ends near the bottom of the list should keep loading, just
	// like holding Down would.
	if target == mouseTargetPostsList && m.Posts.shouldPrefetchMore() {
		m.Posts.PostListLoading = true
		if m.Posts.SearchActive {
			return m, searchPostsCmd(m.Provider, m.Posts.SearchInput, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
		return m, loadPostsCmd(m.Provider, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
	}
	return m, nil
}

func (d dragState) bar() scrollbarState {
	return scrollbarState{
		Visible:   true,
		TrackH:    d.TrackH,
		ThumbSize: d.ThumbSize,
		MaxOffset: d.MaxOffset,
	}
}

// applyScrollOffset writes a scroll offset into the dragged container.
func (m *Model) applyScrollOffset(target mouseTarget, offset int) {
	offset = maxInt(0, offset)
	switch target {
	case mouseTargetPostsList:
		if m.Posts.PostViewport == nil {
			return
		}
		m.Posts.PostViewport.SetYOffset(offset)
		m.Posts.adjustSelectedToViewport()
		m.Posts.syncCursorToSelection()
	case mouseTargetPostBody:
		if m.Posts.PostBodyViewport == nil {
			return
		}
		m.Posts.PostBodyViewport.SetYOffset(offset)
	case mouseTargetComments:
		if m.Posts.CommentViewport == nil {
			return
		}
		m.Posts.CommentViewport.SetYOffset(offset)
	case mouseTargetScores:
		m.Scores.Offset = clampInt(offset, 0, m.scoresMaxOffset())
	case mouseTargetLogs:
		logs := &m.ToolsDialog.Logs
		logs.offset = clampInt(offset, 0, maxInt(0, len(logs.lines)-1))
	case mouseTargetConfig:
		config := &m.ToolsDialog.Config
		body := m.toolsBodyView(m.Width, m.Height)
		maxOffset := maxInt(0, len(config.lines)-config.editorHeight(body.H))
		config.offset = clampInt(offset, 0, maxOffset)
	case mouseTargetNotifications:
		notifications := &m.ToolsDialog.Notifications
		notifications.selected = clampInt(offset, 0, maxInt(0, len(notifications.items)-1))
	}
}

// beginScrollbarDrag starts a drag or performs a track jump.
func (m Model) beginScrollbarDrag(target mouseTarget, bar scrollbarState, trackTop, row int) Model {
	thumbTop, grabDelta := bar.grabAtRow(row)
	m.Drag = dragState{
		Active:    true,
		Target:    target,
		TrackTop:  trackTop,
		TrackH:    bar.TrackH,
		ThumbSize: bar.ThumbSize,
		MaxOffset: bar.MaxOffset,
		GrabDelta: grabDelta,
	}
	m.applyScrollOffset(target, thumbTop*bar.MaxOffset/maxInt(1, bar.TrackH-bar.ThumbSize))
	return m
}

func (m Model) handlePostsMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	// The search input owns the page while it is focused.
	if m.Posts.Searching {
		return m, nil
	}
	if m.searchBoxRect().contains(msg.X, msg.Y) {
		return m.beginSearchInput(), nil
	}
	if m.Posts.ShowPostDetail {
		return m.handleDetailMouseClick(msg)
	}

	list := m.postsListView()
	if !list.Active {
		return m, nil
	}
	inRows := msg.Y >= list.TopY && msg.Y < list.TopY+list.ViewportHeight
	if inRows && list.Bar.Visible && msg.X == list.BarX {
		return m.beginScrollbarDrag(mouseTargetPostsList, list.Bar, list.TopY, msg.Y-list.TopY), nil
	}
	if inRows && msg.X >= 0 && msg.X < list.ContentWidth {
		contentLine := m.Posts.PostViewport.YOffset() + (msg.Y - list.TopY)
		if pid, action, ok := m.Posts.postActionAtContentLine(contentLine, msg.X); ok {
			m.Posts.selectPostAtContentLine(contentLine)
			m.Clicks.reset()
			return m.runPostAction(pid, action)
		}
		if !m.Posts.selectPostAtContentLine(contentLine) {
			return m, nil
		}
		if m.Clicks.register(msg.X, msg.Y, time.Now()) {
			return m.openSelectedPostDetail()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleDetailMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	detail := m.postsDetailView()
	if !detail.Active {
		return m, nil
	}
	if detail.Header.contains(msg.X, msg.Y) && m.Posts.CurrentPost != nil {
		if action, ok := m.Posts.detailHeaderActionAt(m.Width, msg.Y-detail.Header.Y, msg.X); ok {
			return m.runPostAction(m.Posts.CurrentPost.Pid, action)
		}
	}
	if detail.BodyBar.Visible && msg.X == detail.Body.X+detail.Body.W &&
		msg.Y >= detail.Body.Y && msg.Y < detail.Body.Y+detail.Body.H {
		return m.beginScrollbarDrag(mouseTargetPostBody, detail.BodyBar, detail.Body.Y, msg.Y-detail.Body.Y), nil
	}
	if detail.CommentsBar.Visible && detail.HasComments && msg.X == detail.Comments.X+detail.Comments.W &&
		msg.Y >= detail.Comments.Y && msg.Y < detail.Comments.Y+detail.Comments.H {
		return m.beginScrollbarDrag(mouseTargetComments, detail.CommentsBar, detail.Comments.Y, msg.Y-detail.Comments.Y), nil
	}
	if detail.Body.contains(msg.X, msg.Y) {
		m.Posts.DetailFocus = DetailFocusPost
		return m, nil
	}
	if detail.CommentsTitle.contains(msg.X, msg.Y) {
		m.Posts.DetailFocus = DetailFocusComments
		return m, nil
	}
	if detail.HasComments && detail.Comments.contains(msg.X, msg.Y) {
		m.Posts.DetailFocus = DetailFocusComments
		contentLine := m.Posts.CommentViewport.YOffset() + (msg.Y - detail.Comments.Y)
		m.Posts.selectCommentAtContentLine(contentLine)
		return m, nil
	}
	// Clicking the section padding around the comments still focuses them.
	if detail.HasComments && msg.Y >= detail.Comments.Y && msg.Y < detail.Comments.Y+detail.Comments.H && msg.X >= 0 && msg.X < detail.Comments.X {
		m.Posts.DetailFocus = DetailFocusComments
	}
	return m, nil
}

// scrollPostsAt turns a wheel notch into the same cursor movement a repeated
// Up/Down key press would produce, so wheel and keyboard stay interchangeable
// (including the load-more trigger near the end of the list).
func (m Model) scrollPostsAt(x, y, delta int) (Model, tea.Cmd) {
	p := &m.Posts
	if p.ShowPostDetail {
		detail := m.postsDetailView()
		if !detail.Active {
			return m, nil
		}
		if detail.bodyHit(x, y) {
			p.PostBodyViewport.ScrollDown(delta)
			m.syncPostsPage()
			return m, m.imageRefreshCmd(nil)
		}
		if detail.commentsHit(x, y) {
			return m.moveCommentsCursor(delta)
		}
		return m, nil
	}
	list := m.postsListView()
	if !list.Active {
		return m, nil
	}
	inRows := y >= list.TopY && y < list.TopY+list.ViewportHeight
	if !inRows || x < 0 || x >= list.ContentWidth+1 {
		return m, nil
	}
	return m.movePostsCursor(delta)
}

// scoresMaxOffset is the largest valid scroll offset of the scores table.
func (m Model) scoresMaxOffset() int {
	height := m.contentAreaHeightForSize(m.Width, m.Height)
	availableRows := maxInt(1, height-16)
	if m.Scores.Summary == nil {
		return 0
	}
	return maxInt(0, len(m.Scores.Summary.Scores)-availableRows)
}
