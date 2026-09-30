package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// rect is a screen-space rectangle in terminal cells, zero-based.
type rect struct {
	X, Y, W, H int
}

func (r rect) contains(x, y int) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// rowOffset reports the line index that follows s when the two are
// concatenated. Chrome blocks rendered above scrollable content always end
// with "\n", so the next content starts on the row after the last newline.
func rowOffset(s string) int {
	return strings.Count(s, "\n")
}

// contentAreaTop is the screen row where the page content area begins (the
// tab bar occupies the rows above it). The dashboard renders without a tab
// bar.
func (m Model) contentAreaTop(width int) int {
	if m.Page == PageDashboard {
		return 0
	}
	return lipgloss.Height(m.renderMainTabBar(width))
}

// tabRects returns one click target per main tab, or nil when the tab bar
// wrapped onto multiple rows (very narrow terminals keep key-only switching).
func (m Model) tabRects(width int) []rect {
	if m.Page == PageDashboard {
		return nil
	}
	if lipgloss.Height(m.renderMainTabBar(width)) != 1 {
		return nil
	}
	tabs := mainTabs()
	rects := make([]rect, 0, len(tabs))
	x := 0
	for i := range tabs {
		style := tabItemStyle
		if i == m.TabCursor {
			style = tabItemActiveStyle
		}
		w := lipgloss.Width(style.Render(tabs[i]))
		rects = append(rects, rect{X: x, Y: 0, W: w, H: 1})
		x += w
		if i < len(tabs)-1 {
			next := tabItemStyle
			if i+1 == m.TabCursor {
				next = tabItemActiveStyle
			}
			x += lipgloss.Width(powerlineSeparatorRight(style.GetBackground(), next.GetBackground()))
		}
	}
	return rects
}

// postsListView is the computed geometry of the post list container.
type postsListView struct {
	Active         bool
	TopY           int // screen row of the first viewport row
	ContentWidth   int // viewport width in cells (scrollbar column excluded)
	ViewportHeight int
	BarX           int // screen column of the scrollbar
	Bar            scrollbarState
}

func (m Model) postsListView() postsListView {
	width := m.Width
	height := m.contentAreaHeightForSize(width, m.Height)
	p := &m.Posts
	if !p.listViewportActive() {
		return postsListView{}
	}
	contentWidth := p.postListContentWidth(width, height)
	return postsListView{
		Active:         true,
		TopY:           m.contentAreaTop(width) + p.listChromeLines(width),
		ContentWidth:   contentWidth,
		ViewportHeight: p.calcPostViewportHeight(height),
		BarX:           contentWidth,
		Bar:            p.postListBar(width, height),
	}
}

// searchBoxRect is the clickable search input row. The search box is only
// rendered on the list page, not in the detail view.
func (m Model) searchBoxRect() rect {
	width := m.Width
	p := &m.Posts
	if p.ShowPostDetail {
		return rect{}
	}
	return rect{X: 0, Y: m.contentAreaTop(width) + 1, W: width, H: p.searchBoxHeight()}
}

// postsDetailView is the computed geometry of the post detail page.
type postsDetailView struct {
	Active        bool
	HasComments   bool
	Header        rect
	Body          rect
	CommentsTitle rect
	Comments      rect
	BodyBar       scrollbarState
	CommentsBar   scrollbarState
}

func (m Model) postsDetailView() postsDetailView {
	p := &m.Posts
	if !p.detailBodyActive() {
		return postsDetailView{}
	}
	width := m.Width
	height := m.contentAreaHeightForSize(width, m.Height)
	top := m.contentAreaTop(width)

	metrics := p.detailMetrics(width, height)
	headerHeight := lipgloss.Height(p.renderDetailHeader(width))
	titleHeight := lipgloss.Height(p.renderDetailCommentsTitle(width, p.detailCommentsTitleStyle(), p.detailCommentsTitle()))

	bodyTop := top + headerHeight
	commentsTitleTop := bodyTop + metrics.BodyHeight + 1
	commentsTop := commentsTitleTop + titleHeight + 1

	view := postsDetailView{
		Active:        true,
		Header:        rect{X: 0, Y: top, W: width, H: headerHeight},
		Body:          rect{X: 2, Y: bodyTop, W: metrics.BodyWidth, H: metrics.BodyHeight},
		CommentsTitle: rect{X: 0, Y: commentsTitleTop, W: width, H: titleHeight},
		BodyBar:       metrics.BodyBar,
	}
	if p.detailCommentsActive() {
		view.HasComments = true
		view.Comments = rect{X: 2, Y: commentsTop, W: metrics.CommentsWidth, H: metrics.CommentsHeight}
		view.CommentsBar = metrics.CommentsBar
	}
	return view
}

// bodyHit reports whether a cell belongs to the detail body pane, including
// its scrollbar column.
func (v postsDetailView) bodyHit(x, y int) bool {
	if v.Body.contains(x, y) {
		return true
	}
	return v.BodyBar.Visible && x == v.Body.X+v.Body.W && y >= v.Body.Y && y < v.Body.Y+v.Body.H
}

// commentsHit reports whether a cell belongs to the comments pane, including
// its scrollbar column.
func (v postsDetailView) commentsHit(x, y int) bool {
	if !v.HasComments {
		return false
	}
	if v.Comments.contains(x, y) {
		return true
	}
	return v.CommentsBar.Visible && x == v.Comments.X+v.Comments.W && y >= v.Comments.Y && y < v.Comments.Y+v.Comments.H
}

// scoresPageView is the computed geometry of the score table rows and its
// scrollbar.
type scoresPageView struct {
	Active bool
	Rows   rect
	Offset int
	BarX   int
	Bar    scrollbarState
}

func (m Model) scoresPageView() scoresPageView {
	p := &m.Scores
	width := m.Width
	height := m.contentAreaHeightForSize(width, m.Height)
	if p.Summary == nil || len(p.Summary.Scores) == 0 {
		return scoresPageView{}
	}
	prelude := p.scoresPrelude(width)
	top := m.contentAreaTop(width) + rowOffset(prelude)
	availableRows := maxInt(1, height-16)
	offset := clampInt(p.Offset, 0, maxInt(0, len(p.Summary.Scores)-availableRows))
	rows := minInt(len(p.Summary.Scores)-offset, availableRows)
	bodyWidth := maxInt(20, width-4)
	bar, _ := p.scoresBar(height)
	return scoresPageView{
		Active: true,
		Rows:   rect{X: 0, Y: top, W: bodyWidth, H: rows},
		Offset: offset,
		BarX:   bodyWidth,
		Bar:    bar,
	}
}

// toolsPanelRect mirrors renderPanelScreenWithStyle: the 80%-sized centered
// panel (min 40x22) that hosts the tools dialog.
func (m Model) toolsPanelRect(width, height int) rect {
	statusLineHeight := lipgloss.Height(m.renderStatusLine(width))
	availableHeight := maxInt(1, height-statusLineHeight)
	panelW := width * 4 / 5
	panelH := availableHeight * 4 / 5
	if panelW < 40 {
		panelW = 40
	}
	if panelH < 22 {
		panelH = 22
	}
	if panelW > width {
		panelW = width
	}
	if panelH > availableHeight {
		panelH = availableHeight
	}
	return rect{X: (width - panelW) / 2, Y: (availableHeight - panelH) / 2, W: panelW, H: panelH}
}

// toolsContentView is the rect of the tools dialog content inside the panel
// (the panel is padded by (1,3)).
func (m Model) toolsContentView(width, height int) rect {
	panel := m.toolsPanelRect(width, height)
	contentW := maxInt(20, panel.W-panelContentStyle.GetHorizontalFrameSize())
	contentH := maxInt(8, panel.H-panelContentStyle.GetVerticalFrameSize())
	return rect{X: panel.X + 3, Y: panel.Y + 1, W: contentW, H: contentH}
}

// toolsBodyView is the rect of the section body inside the tools dialog
// (below the section tabs row and its blank spacer).
func (m Model) toolsBodyView(width, height int) rect {
	content := m.toolsContentView(width, height)
	return rect{X: content.X, Y: content.Y + 2, W: content.W, H: maxInt(3, content.H-2)}
}

// postListBaseWidth is the list viewport width before scrollbar reservation.
func (p PostsPageModel) postListBaseWidth(width int) int {
	return maxInt(20, width-8)
}

// postListOverflows reports whether the list needs a scrollbar. The decision
// is made from the unreserved width so it cannot oscillate with the extra
// column.
func (p PostsPageModel) postListOverflows(width, height int) bool {
	if !p.listViewportActive() {
		return false
	}
	return p.totalPostLinesAtWidth(p.postListBaseWidth(width)) > p.calcPostViewportHeight(height)
}

func (p PostsPageModel) postListContentWidth(width, height int) int {
	base := p.postListBaseWidth(width)
	if p.postListOverflows(width, height) {
		return maxInt(1, base-1)
	}
	return base
}

func (p PostsPageModel) postListBar(width, height int) scrollbarState {
	if !p.postListOverflows(width, height) || p.PostViewport == nil {
		return scrollbarState{}
	}
	viewportHeight := p.calcPostViewportHeight(height)
	totalLines := p.totalPostLinesAtWidth(p.postListContentWidth(width, height))
	return scrollbarFor(viewportHeight, totalLines, viewportHeight, p.PostViewport.YOffset())
}

// detailBaseWidth is the detail pane width before scrollbar reservation.
func (p PostsPageModel) detailBaseWidth(width int) int {
	return maxInt(20, width-8)
}

func (p PostsPageModel) detailBodyActive() bool {
	return p.ShowPostDetail && p.CurrentPost != nil
}

func (p PostsPageModel) detailCommentsActive() bool {
	return p.detailBodyActive() && len(p.CommentList) > 0
}

// detailMetrics is the single source of truth for the detail page geometry:
// pane widths (including scrollbar reservation) and heights are derived only
// from (width, height, model state) so rendering, syncing and hit-testing
// cannot drift apart.
type detailMetrics struct {
	BodyHeight     int
	CommentsHeight int
	BodyWidth      int
	CommentsWidth  int
	BodyBar        scrollbarState
	CommentsBar    scrollbarState
}

func (p *PostsPageModel) detailMetrics(width, height int) detailMetrics {
	base := p.detailBaseWidth(width)
	// First pass: heights at the unreserved width decide which panes need a
	// scrollbar column.
	bodyHeight, commentsHeight := p.detailViewportHeightsAt(width, height, base, base)
	bodyWidth, commentsWidth := base, base
	if p.detailBodyActive() && base > 1 && p.detailBodyLineCountAtWidth(base) > bodyHeight {
		bodyWidth = base - 1
	}
	if p.detailCommentsActive() && base > 1 && p.commentLineCountAtWidth(base) > commentsHeight {
		commentsWidth = base - 1
	}
	// Second pass: heights for the final widths.
	bodyHeight, commentsHeight = p.detailViewportHeightsAt(width, height, bodyWidth, commentsWidth)

	metrics := detailMetrics{
		BodyHeight:     bodyHeight,
		CommentsHeight: commentsHeight,
		BodyWidth:      bodyWidth,
		CommentsWidth:  commentsWidth,
	}
	if bodyWidth != base && p.PostBodyViewport != nil {
		metrics.BodyBar = scrollbarFor(bodyHeight, p.detailBodyLineCountAtWidth(bodyWidth), bodyHeight, p.PostBodyViewport.YOffset())
	}
	if commentsWidth != base && p.CommentViewport != nil {
		metrics.CommentsBar = scrollbarFor(commentsHeight, p.commentLineCountAtWidth(commentsWidth), commentsHeight, p.CommentViewport.YOffset())
	}
	return metrics
}
