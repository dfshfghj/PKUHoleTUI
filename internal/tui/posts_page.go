package tui

import (
	"fmt"
	"strings"
	"time"

	"treehole/internal/client"
	"treehole/internal/models"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type PostsPageModel struct {
	PostList        []models.Post
	PostListTotal   int
	PostListHasMore bool
	PostListCursor  int
	PostListLoading bool
	PostListError   string
	PostPerPage     int
	PostViewport    *viewport.Model
	postContent     string
	CursorLine      int
	SelectedPostIdx int

	ShowPostDetail     bool
	CurrentPost        *models.Post
	PostBodyViewport   *viewport.Model
	postBodyContent    string
	CommentList        []models.Comment
	CommentListHasMore bool
	CommentListLoading bool
	CommentListCursor  int32
	CommentListError   string
	CommentSortAsc     bool
	CommentViewport    *viewport.Model
	commentContent     string
	DetailFocus        DetailFocus
	CommentCursorLine  int
	SelectedCommentIdx int

	PostsMode    PostsMode
	Searching    bool
	SearchInput  string
	SearchField  textinput.Model
	SearchActive bool
	ActiveTagID  int
	ActiveTag    string
	SessionMode  SessionMode
	CanWrite     bool
	StatusText   string
	ImageClient  *client.Client
}

func NewPostsPageModel() PostsPageModel {
	pv := viewport.New(0, 0)
	bv := viewport.New(0, 0)
	cv := viewport.New(0, 0)
	search := newSearchInput()
	return PostsPageModel{
		PostPerPage:        20,
		PostViewport:       &pv,
		PostBodyViewport:   &bv,
		CommentViewport:    &cv,
		SearchField:        search,
		CommentSortAsc:     true,
		PostsMode:          PostsModeList,
		DetailFocus:        DetailFocusComments,
		CommentCursorLine:  0,
		SelectedCommentIdx: 0,
	}
}

func newSearchInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "按 / 搜索 内容 或 #pid 或 :follow"
	input.Width = 32
	styleTextInput(&input, colorSurface, colorText, colorMuted)
	return input
}

func (p *PostsPageModel) ensureInitialized() {
	if p.PostViewport == nil || p.PostBodyViewport == nil || p.CommentViewport == nil {
		*p = NewPostsPageModel()
	}
	if p.SearchField.Width == 0 {
		p.SearchField = newSearchInput()
		p.SearchField.SetValue(p.SearchInput)
	}
}

func (p *PostsPageModel) syncViewports(width, height int) {
	p.ensureInitialized()
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}

	contentWidth := width - 8
	if contentWidth < 20 {
		contentWidth = 20
	}
	postHeight := p.calcPostViewportHeight(height)

	if !p.ShowPostDetail && len(p.PostList) > 0 {
		p.syncCursorToSelection()
		newContent, _ := p.buildPostListContent(contentWidth)
		if p.postContent != newContent || p.PostViewport.Width != contentWidth || p.PostViewport.Height != postHeight {
			p.PostViewport.Width = contentWidth
			p.PostViewport.Height = postHeight
			p.PostViewport.SetContent(newContent)
			p.postContent = newContent
		}
	}

	if p.ShowPostDetail && p.CurrentPost != nil {
		bodyHeight, commentHeight := p.calcDetailViewportHeights(width, height)
		bodyContent, _ := p.buildDetailBodyContent(contentWidth)
		if p.postBodyContent != bodyContent || p.PostBodyViewport.Width != contentWidth || p.PostBodyViewport.Height != bodyHeight {
			p.PostBodyViewport.Width = contentWidth
			p.PostBodyViewport.Height = bodyHeight
			p.PostBodyViewport.SetContent(bodyContent)
			p.postBodyContent = bodyContent
		}

		commentContent := p.buildCommentContent(contentWidth)
		if p.commentContent != commentContent || p.CommentViewport.Width != contentWidth || p.CommentViewport.Height != commentHeight {
			p.CommentViewport.Width = contentWidth
			p.CommentViewport.Height = commentHeight
			p.CommentViewport.SetContent(commentContent)
			p.commentContent = commentContent
		}
		p.reconcileCommentSelectionWithCursor()
	}
}

func (p PostsPageModel) View(width, height int) (string, []imagePlacement) {
	p.ensureInitialized()
	if p.ShowPostDetail {
		return p.renderPostDetail(width, height)
	}
	return p.renderPosts(width, height)
}

func (p PostsPageModel) renderPosts(width, height int) (string, []imagePlacement) {
	var b strings.Builder
	var placements []imagePlacement

	if p.SearchActive {
		b.WriteString(vTitleStyle.Render(fmt.Sprintf("搜索结果: %s", p.SearchInput)))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}

	searchStyle := vSearchInput.Width(width)
	searchFocusedStyle := vSearchInputFocused.Width(width)
	if p.Searching {
		searchInput := p.SearchField
		if searchInput.Value() != p.SearchInput {
			searchInput.SetValue(p.SearchInput)
		}
		searchInput.Width = maxInt(1, width-searchFocusedStyle.GetHorizontalFrameSize()-1)
		inputView := searchInput.View()
		if searchInput.Value() == "" {
			inputView = fillRenderedBackground(
				inputView,
				searchInput.Width,
				lipgloss.NewStyle().Background(colorSurface).Foreground(colorText),
			)
		}
		b.WriteString(searchFocusedStyle.Render(inputView))
	} else {
		b.WriteString(searchStyle.Render("按 / 搜索"))
	}
	b.WriteString("\n")

	if p.PostListLoading && len(p.PostList) == 0 {
		b.WriteString(vLoadingStyle.Render("加载中..."))
		return b.String(), nil
	}

	if p.PostListError != "" {
		b.WriteString(vErrorStyle.Render("错误: " + p.PostListError))
		b.WriteString("\n")
	}

	if len(p.PostList) == 0 {
		b.WriteString(vEmptyStyle.Render("暂无数据"))
		return b.String(), nil
	}

	pageWidth := maxInt(20, width-8)
	contentWidth := pageWidth
	vp := viewport.New(contentWidth, p.calcPostViewportHeight(height))
	content, postPlacements := p.buildPostListContent(contentWidth)
	vp.SetContent(content)
	if p.PostViewport != nil {
		vp.SetYOffset(p.PostViewport.YOffset)
	}
	prefixHeight := lipgloss.Height(b.String())
	placements = append(placements, visiblePlacements(postPlacements, vp.YOffset, vp.Height, prefixHeight)...)
	b.WriteString(vp.View())
	return b.String(), placements
}

func (p PostsPageModel) renderPostDetail(width, height int) (string, []imagePlacement) {
	var b strings.Builder
	var placements []imagePlacement

	if p.CurrentPost == nil {
		return "无帖子数据", nil
	}
	b.WriteString(p.renderDetailHeader(width))
	b.WriteString("\n")

	dividerWidth := width - 8
	if dividerWidth < 20 {
		dividerWidth = 20
	}
	commentsTitle := p.detailCommentsTitle()
	commentsTitleStyle := vSectionTitleStyle
	bodySectionStyle := vDetailSection
	commentsSectionStyle := vDetailSection
	if p.DetailFocus == DetailFocusPost {
		bodySectionStyle = vDetailSectionFocused
	} else {
		commentsTitleStyle = vSectionTitleFocused
		commentsSectionStyle = vDetailSectionFocused
	}

	contentWidth := width - 8
	if contentWidth < 20 {
		contentWidth = 20
	}
	bodyHeight, commentHeight := p.calcDetailViewportHeights(width, height)
	bodyViewport := viewport.New(contentWidth, bodyHeight)
	bodyContent, bodyPlacements := p.buildDetailBodyContent(contentWidth)
	bodyViewport.SetContent(bodyContent)
	if p.PostBodyViewport != nil {
		bodyViewport.SetYOffset(p.PostBodyViewport.YOffset)
	}
	prefixHeight := lipgloss.Height(b.String())
	placements = append(placements, visiblePlacements(bodyPlacements, bodyViewport.YOffset, bodyViewport.Height, prefixHeight)...)
	b.WriteString(bodySectionStyle.Render(bodyViewport.View()))
	b.WriteString("\n")

	b.WriteString(vDividerStyle.Render(strings.Repeat("─", dividerWidth)))
	b.WriteString("\n")

	b.WriteString(p.renderDetailCommentsTitle(width, commentsTitleStyle, commentsTitle))
	b.WriteString("\n")

	if len(p.CommentList) == 0 {
		b.WriteString(commentsSectionStyle.Render(vEmptyStyle.Render("暂无评论")))
	} else {
		vp := viewport.New(contentWidth, commentHeight)
		vp.SetContent(p.buildCommentContent(contentWidth))
		if p.CommentViewport != nil {
			vp.SetYOffset(p.CommentViewport.YOffset)
		}
		b.WriteString(commentsSectionStyle.Render(vp.View()))
	}

	return b.String(), placements
}

func (p PostsPageModel) buildDetailBodyContent(contentWidth int) (string, []imagePlacement) {
	if p.CurrentPost == nil {
		return "", nil
	}
	textWidth := p.detailBodyTextWidth(contentWidth)
	lines := p.postDetailLines(*p.CurrentPost, textWidth)
	return vPostTextStyle.Render(strings.Join(lines, "\n")), nil
}

func (p PostsPageModel) buildPostListContent(contentWidth int) (string, []imagePlacement) {
	selStyle := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		Padding(0, 0, 0, 1).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorAccent).
		Render

	var content strings.Builder
	var placements []imagePlacement
	lineNo := 0
	for i, post := range p.PostList {
		if i > 0 {
			content.WriteString(p.linePrefix(lineNo) + "\n")
			lineNo++
		}

		selected := i == p.SelectedPostIdx
		lineWidth := p.listLineTextWidth(contentWidth, selected)
		headerLines := p.postHeaderLines(post, lineWidth)
		textLines := p.postListTextLines(post, lineWidth)
		mentionLine := p.postMentionLine(post, lineWidth)

		if selected {
			for _, line := range headerLines {
				content.WriteString(selStyle(p.linePrefix(lineNo)+line) + "\n")
				lineNo++
			}
			for _, line := range textLines {
				content.WriteString(p.linePrefix(lineNo) + line + "\n")
				lineNo++
			}
			if mentionLine != "" {
				content.WriteString(p.linePrefix(lineNo) + mentionLine + "\n")
				lineNo++
			}
		} else {
			for _, line := range headerLines {
				content.WriteString(p.linePrefix(lineNo) + line + "\n")
				lineNo++
			}
			for _, line := range textLines {
				content.WriteString(p.linePrefix(lineNo) + line + "\n")
				lineNo++
			}
			if mentionLine != "" {
				content.WriteString(p.linePrefix(lineNo) + mentionLine + "\n")
				lineNo++
			}
		}
	}
	return content.String(), placements
}

func (p PostsPageModel) buildCommentContent(contentWidth int) string {
	if len(p.CommentList) == 0 {
		return p.renderEmptyCommentState()
	}
	var content strings.Builder
	comments := p.orderedComments()
	lineNo := 0
	for i, c := range comments {
		if i > 0 {
			content.WriteString("\n")
		}
		renderedLines := p.renderCommentLines(c, contentWidth, lineNo, i == p.SelectedCommentIdx)
		block := strings.Join(renderedLines, "\n")
		lineNo += len(renderedLines)
		content.WriteString(block)
	}
	if p.CommentListError != "" {
		content.WriteString("\n\n")
		content.WriteString(vErrorStyle.Render("错误: " + p.CommentListError))
	} else if p.CommentListLoading {
		content.WriteString("\n\n")
		content.WriteString(vLoadingStyle.Render("加载更多评论中..."))
	}
	return content.String()
}

func (p PostsPageModel) renderEmptyCommentState() string {
	if p.CommentListLoading {
		return vLoadingStyle.Render("加载评论中...")
	}
	if p.CommentListError != "" {
		return vErrorStyle.Render("错误: " + p.CommentListError)
	}
	return vEmptyStyle.Render("暂无评论")
}

func (p PostsPageModel) renderCommentHeader(timestamp string) string {
	return vCommentMetaTimeStyle.Render(timestamp)
}

func (p PostsPageModel) renderCommentBodyLine(author, line string) string {
	prefix := author + ": "
	if strings.HasPrefix(line, prefix) {
		return vCommentAuthorStyle.Render(author) + ": " + strings.TrimPrefix(line, prefix)
	}
	return line
}

func (p PostsPageModel) commentLinePrefix(lineNo int) string {
	if lineNo == p.CommentCursorLine {
		return "▸ "
	}
	return "  "
}

func (p PostsPageModel) commentIndentedLine(lineNo int, line string) string {
	return p.commentLinePrefix(lineNo) + line
}

func (p PostsPageModel) commentLogicalLines(c models.Comment, contentWidth int) []string {
	lines := []string{p.renderCommentHeader(time.Unix(int64(c.Timestamp), 0).In(shanghaiLocation).Format("2006-01-02 15:04"))}
	if quotePreview := p.commentQuotePreview(c, p.commentQuoteTextWidth(contentWidth)); quotePreview != "" {
		lines = append(lines, vCommentQuoteStyle.Render(quotePreview))
	}
	cName := c.NameTag
	if cName == "" {
		cName = "匿名"
	}
	for _, line := range p.wrapPlainTextLines(p.commentDisplayText(c, cName), p.commentBodyTextWidth(contentWidth)) {
		lines = append(lines, p.renderCommentBodyLine(cName, line))
	}
	return lines
}

func (p PostsPageModel) renderCommentLines(c models.Comment, contentWidth, startLine int, selected bool) []string {
	logicalLines := p.commentLogicalLines(c, contentWidth)
	rendered := make([]string, 0, len(logicalLines))
	for offset, line := range logicalLines {
		renderedLine := p.commentIndentedLine(startLine+offset, line)
		if selected {
			renderedLine = vCommentSelectedStyle.Render(renderedLine)
		}
		rendered = append(rendered, renderedLine)
	}
	return rendered
}

func (p PostsPageModel) orderedComments() []models.Comment {
	return p.CommentList
}

func (p PostsPageModel) commentQuotePreview(c models.Comment, width int) string {
	if c.Quote == nil {
		return ""
	}
	quoteName := c.Quote.NameTag
	if quoteName == "" {
		quoteName = "匿名"
	}
	preview := fmt.Sprintf("%s: %s", quoteName, oneLinePreviewText(c.Quote.Text))
	return truncateVisibleLine(preview, width, "...")
}

func oneLinePreviewText(text string) string {
	normalized := normalizeRenderedText(text)
	normalized = strings.ReplaceAll(normalized, "\r\n", " ")
	normalized = strings.ReplaceAll(normalized, "\r", " ")
	return strings.ReplaceAll(normalized, "\n", " ")
}

func (p *PostsPageModel) scrollToSelectedPost() {
	if len(p.PostList) == 0 {
		return
	}
	startLine, _ := p.selectedPostLineRange()
	p.CursorLine = startLine
	p.scrollCursorIntoView()
}

func (p *PostsPageModel) selectedPostLineRange() (startLine, endLine int) {
	line := 0
	for i := 0; i < len(p.PostList); i++ {
		if i > 0 {
			line++
		}
		postLines := p.postRenderedLinesAt(i)
		if i == p.SelectedPostIdx {
			return line, line + postLines - 1
		}
		line += postLines
	}
	return 0, 0
}

func (p *PostsPageModel) adjustSelectedToViewport() {
	if len(p.PostList) == 0 {
		return
	}
	yOffset := p.PostViewport.YOffset
	visibleLines := p.PostViewport.VisibleLineCount()
	lineIdx := 0
	for i := 0; i < len(p.PostList); i++ {
		if i > 0 {
			if lineIdx == yOffset {
				p.SelectedPostIdx = i - 1
				return
			}
			lineIdx++
		}
		postLines := p.postRenderedLinesAt(i)
		if lineIdx+postLines > yOffset && lineIdx < yOffset+visibleLines {
			p.SelectedPostIdx = i
			return
		}
		lineIdx += postLines
	}
}

func (p *PostsPageModel) moveCursor(delta int) {
	if len(p.PostList) == 0 {
		return
	}
	totalLines := p.totalPostLines()
	if totalLines <= 0 {
		return
	}
	p.CursorLine = clampInt(p.CursorLine+delta, 0, totalLines-1)
	p.SelectedPostIdx = p.postIndexAtLine(p.CursorLine)
	p.scrollCursorIntoView()
}

func (p *PostsPageModel) pageMove(direction int) {
	if len(p.PostList) == 0 || direction == 0 {
		return
	}
	totalLines := p.totalPostLines()
	if totalLines <= 0 {
		return
	}

	step := p.pageStep()
	delta := step
	if direction < 0 {
		delta = -step
	}

	p.CursorLine = clampInt(p.CursorLine+delta, 0, totalLines-1)
	p.SelectedPostIdx = p.postIndexAtLine(p.CursorLine)

	maxOffset := maxInt(0, totalLines-p.PostViewport.VisibleLineCount())
	p.PostViewport.SetYOffset(clampInt(p.PostViewport.YOffset+delta, 0, maxOffset))
	p.scrollCursorIntoView()
}

func (p *PostsPageModel) pageStep() int {
	visibleLines := p.PostViewport.VisibleLineCount()
	if visibleLines <= 1 {
		return 1
	}
	step := visibleLines - 2
	if step < 1 {
		step = 1
	}
	return step
}

func (p *PostsPageModel) scrollCursorIntoView() {
	visibleLines := p.PostViewport.VisibleLineCount()
	if visibleLines <= 0 {
		return
	}

	topMargin := 2
	bottomMargin := 15
	if maxTop := visibleLines / 4; maxTop < topMargin {
		topMargin = maxTop
	}
	if maxBottom := visibleLines - 2; maxBottom < bottomMargin {
		bottomMargin = maxBottom
	}
	if topMargin < 1 {
		topMargin = 1
	}
	if bottomMargin < 1 {
		bottomMargin = 1
	}

	topThreshold := p.PostViewport.YOffset + topMargin
	bottomThreshold := p.PostViewport.YOffset + visibleLines - bottomMargin - 1

	if p.CursorLine < topThreshold {
		newOffset := p.CursorLine - topMargin
		if newOffset < 0 {
			newOffset = 0
		}
		p.PostViewport.SetYOffset(newOffset)
		return
	}
	if p.CursorLine > bottomThreshold {
		newOffset := p.CursorLine - visibleLines + bottomMargin + 1
		if newOffset < 0 {
			newOffset = 0
		}
		p.PostViewport.SetYOffset(newOffset)
	}
}

func (p *PostsPageModel) syncCursorToSelection() {
	if len(p.PostList) == 0 {
		p.CursorLine = 0
		p.SelectedPostIdx = 0
		return
	}
	if p.SelectedPostIdx < 0 {
		p.SelectedPostIdx = 0
	}
	if p.SelectedPostIdx >= len(p.PostList) {
		p.SelectedPostIdx = len(p.PostList) - 1
	}
	startLine, endLine := p.selectedPostLineRange()
	separatorAfter := endLine
	if p.SelectedPostIdx < len(p.PostList)-1 {
		separatorAfter = endLine + 1
	}
	if p.CursorLine < startLine || p.CursorLine > separatorAfter {
		p.CursorLine = startLine
	}
}

func (p *PostsPageModel) postIndexAtLine(target int) int {
	line := 0
	for i := 0; i < len(p.PostList); i++ {
		if i > 0 {
			if target == line {
				return i - 1
			}
			line++
		}
		postLines := p.postRenderedLinesAt(i)
		if target < line+postLines {
			return i
		}
		line += postLines
	}
	return maxInt(0, len(p.PostList)-1)
}

func (p *PostsPageModel) totalPostLines() int {
	total := 0
	for i := 0; i < len(p.PostList); i++ {
		if i > 0 {
			total++
		}
		total += p.postRenderedLinesAt(i)
	}
	return total
}

func (p *PostsPageModel) postRenderedLinesAt(index int) int {
	if index < 0 || index >= len(p.PostList) {
		return 0
	}
	post := p.PostList[index]
	selected := index == p.SelectedPostIdx
	lineWidth := p.listLineTextWidth(p.currentListContentWidth(), selected)
	headerLines := len(p.postHeaderLines(post, lineWidth))
	textLines := p.postListTextLines(post, lineWidth)
	textLineCount := len(textLines)
	mentionLineCount := 0
	if p.postMentionLine(post, lineWidth) != "" {
		mentionLineCount = 1
	}
	return headerLines + textLineCount + mentionLineCount
}

func (p *PostsPageModel) atLastContentLine() bool {
	total := p.totalPostLines()
	return total > 0 && p.CursorLine >= total-1
}

func (p *PostsPageModel) shouldPrefetchMore() bool {
	if p.PostListLoading || !p.PostListHasMore || len(p.PostList) == 0 {
		return false
	}
	totalLines := p.totalPostLines()
	remainingLines := totalLines - p.CursorLine - 1
	return remainingLines <= 10
}

func (p PostsPageModel) linePrefix(lineNo int) string {
	if lineNo == p.CursorLine {
		if p.isSeparatorLine(lineNo) {
			return lipgloss.NewStyle().Foreground(colorMuted).Render("· ")
		}
		return "▸ "
	}
	return "  "
}

func (p PostsPageModel) isSeparatorLine(target int) bool {
	line := 0
	for i := 0; i < len(p.PostList); i++ {
		if i > 0 {
			if line == target {
				return true
			}
			line++
		}
		line += p.postRenderedLinesAt(i)
	}
	return false
}

func (p *PostsPageModel) resetList() {
	p.PostList = nil
	p.PostListTotal = 0
	p.PostListHasMore = false
	p.PostListCursor = 0
	p.CursorLine = 0
	p.SelectedPostIdx = 0
	p.postContent = ""
	p.PostViewport.GotoTop()
	p.PostsMode = PostsModeList
}

func (p *PostsPageModel) resetComments() {
	p.CommentList = nil
	p.CommentListHasMore = false
	p.CommentListLoading = false
	p.CommentListCursor = 0
	p.CommentListError = ""
	p.CommentSortAsc = true
	p.commentContent = ""
	p.CommentCursorLine = 0
	p.SelectedCommentIdx = 0
	p.CommentViewport.GotoTop()
}

func (p *PostsPageModel) calcPostViewportHeight(height int) int {
	titleLines := 2
	searchLines := lipgloss.Height(vSearchInput.Render("x")) + 1
	avail := height - titleLines - searchLines
	if avail < 3 {
		avail = 3
	}
	return avail
}

func (p *PostsPageModel) calcDetailViewportHeights(width, height int) (int, int) {
	detailFixedLines := p.detailFixedLineCount(width)
	available := height - detailFixedLines
	if available < 8 {
		return 4, 3
	}

	minBodyHeight := 2
	minCommentHeight := 3
	maxBodyHeight := available / 2
	if maxBodyHeight < minBodyHeight {
		maxBodyHeight = minBodyHeight
	}
	if maxAllowed := available - minCommentHeight; maxBodyHeight > maxAllowed {
		maxBodyHeight = maxAllowed
	}

	bodyLines := p.detailBodyLineCount()
	commentLines := p.commentLineCount()

	bodyHeight := minInt(maxInt(bodyLines, minBodyHeight), maxBodyHeight)
	commentHeight := available - bodyHeight

	if commentHeight < minCommentHeight {
		commentHeight = minCommentHeight
		bodyHeight = available - commentHeight
	}

	extra := available - bodyHeight - commentHeight
	if extra > 0 {
		bodyNeed := maxInt(0, bodyLines-bodyHeight)
		commentNeed := maxInt(0, commentLines-commentHeight)
		switch {
		case commentNeed > bodyNeed:
			add := minInt(extra, commentNeed)
			commentHeight += add
			extra -= add
		case bodyNeed > 0:
			add := minInt(extra, minInt(bodyNeed, maxBodyHeight-bodyHeight))
			bodyHeight += add
			extra -= add
		}
		if extra > 0 {
			commentHeight += extra
		}
	}

	return bodyHeight, commentHeight
}

func (p PostsPageModel) detailFixedLineCount(width int) int {
	titleStyle := vSectionTitleStyle
	if p.DetailFocus == DetailFocusComments {
		titleStyle = vSectionTitleFocused
	}
	return lipgloss.Height(p.renderDetailHeader(width)) +
		p.detailDividerLineCount() +
		lipgloss.Height(p.renderDetailCommentsTitle(width, titleStyle, p.detailCommentsTitle()))
}

func (p PostsPageModel) detailDividerLineCount() int {
	return 1
}

func (p PostsPageModel) detailHeaderPlain() string {
	if p.CurrentPost == nil {
		return ""
	}
	ts := time.Unix(int64(p.CurrentPost.Timestamp), 0).In(shanghaiLocation).Format("2006-01-02 15:04")
	praiseState := "♡"
	if p.CurrentPost.IsPraise {
		praiseState = "♥"
	}
	followState := "☆"
	if p.CurrentPost.IsFollow {
		followState = "★"
	}
	return fmt.Sprintf("#%d  %s  ❝ %d  (%s) %d  (%s) %d", p.CurrentPost.Pid, ts, p.CurrentPost.Reply, praiseState, p.CurrentPost.PraiseNum, followState, p.CurrentPost.Likenum)
}

func (p PostsPageModel) detailCommentsTitle() string {
	sortLabel := "▲"
	if !p.CommentSortAsc {
		sortLabel = "▼"
	}
	title := fmt.Sprintf("评论 %d  %s", len(p.CommentList), sortLabel)
	if p.CommentListLoading {
		title += ", 加载中"
	}
	return title
}

func (p PostsPageModel) renderDetailHeader(width int) string {
	if width < 1 {
		width = 1
	}
	ts := time.Unix(int64(p.CurrentPost.Timestamp), 0).In(shanghaiLocation).Format("2006-01-02 15:04")
	styled := strings.Join([]string{
		vPostPidStyle.Render(fmt.Sprintf("#%d", p.CurrentPost.Pid)),
		vPostTimeStyle.Render(ts),
		vPostReplyStyle.Render(fmt.Sprintf("❝ %d", p.CurrentPost.Reply)),
		vPostLikeStyle.Render(p.postPraiseState()),
		vPostLikeStyle.Render(fmt.Sprintf("%d", p.CurrentPost.PraiseNum)),
		vPostLikeStyle.Render(p.postFollowState()),
		vPostLikeStyle.Render(fmt.Sprintf("%d", p.CurrentPost.Likenum)),
	}, "  ")
	if lipgloss.Width(styled) <= width {
		return styled
	}
	return strings.Join(wrapVisibleLine(p.detailHeaderPlain(), width), "\n")
}

func (p PostsPageModel) renderDetailCommentsTitle(width int, style lipgloss.Style, title string) string {
	availableWidth := width - style.GetHorizontalFrameSize()
	if availableWidth < 1 {
		availableWidth = 1
	}
	return style.Render(strings.Join(wrapVisibleLine(title, availableWidth), "\n"))
}

func (p PostsPageModel) postPraiseState() string {
	if p.CurrentPost != nil && p.CurrentPost.IsPraise {
		return "♥"
	}
	return "♡"
}

func (p PostsPageModel) postFollowState() string {
	if p.CurrentPost != nil && p.CurrentPost.IsFollow {
		return "★"
	}
	return "☆"
}

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (p *PostsPageModel) detailBodyLineCount() int {
	if p.CurrentPost == nil || p.CurrentPost.Text == "" {
		if p.CurrentPost == nil {
			return 1
		}
	}
	width := 20
	if p.PostBodyViewport != nil && p.PostBodyViewport.Width > 0 {
		width = p.PostBodyViewport.Width
	}
	lines := p.postDetailLines(*p.CurrentPost, p.detailBodyTextWidth(width))
	return len(lines)
}

func (p *PostsPageModel) commentLineCount() int {
	width := 20
	if p.CommentViewport != nil && p.CommentViewport.Width > 0 {
		width = p.CommentViewport.Width
	}
	if len(p.CommentList) == 0 {
		if p.CommentListLoading || p.CommentListError != "" {
			return len(strings.Split(p.renderEmptyCommentState(), "\n"))
		}
		return 1
	}
	lines := 0
	for _, c := range p.orderedComments() {
		lines += len(p.commentLogicalLines(c, width))
	}
	if p.CommentListError != "" || p.CommentListLoading {
		lines += 2
	}
	return lines
}

func (p *PostsPageModel) shouldPrefetchCommentsMore() bool {
	if p.CommentListLoading || !p.CommentListHasMore || p.CommentViewport == nil {
		return false
	}
	if p.CommentViewport.AtBottom() || p.CommentViewport.PastBottom() {
		return true
	}
	totalLines := p.CommentViewport.TotalLineCount()
	if totalLines == 0 {
		totalLines = p.commentLineCount()
	}
	bottom := p.CommentViewport.YOffset + p.CommentViewport.VisibleLineCount()
	return totalLines-bottom <= 3
}

func (p PostsPageModel) currentListContentWidth() int {
	if p.PostViewport != nil && p.PostViewport.Width > 0 {
		return p.PostViewport.Width
	}
	return 20
}

func (p PostsPageModel) postListTextLines(post models.Post, width int) []string {
	text := normalizeRenderedText(post.Text)
	lines := p.wrapPlainTextLines(text, width)
	if strings.TrimSpace(text) == "" {
		lines = nil
	}
	if !p.hasPostMedia(post) {
		if len(lines) == 0 {
			return []string{""}
		}
		return lines
	}

	return p.wrapPlainTextLines(p.postDisplayText(post), width)
}

func (p PostsPageModel) postMentionLine(post models.Post, width int) string {
	pid := mentionedPostID(post)
	if pid <= 0 || post.MentionedPost == nil {
		return ""
	}
	previewWidth := width - vCommentQuoteStyle.GetHorizontalFrameSize()
	if previewWidth < 1 {
		previewWidth = 1
	}
	text := oneLinePreviewText(p.postDisplayText(*post.MentionedPost))
	if strings.TrimSpace(text) == "" {
		text = "空内容"
	}
	preview := fmt.Sprintf("#%d: %s", pid, text)
	return vCommentQuoteStyle.Render(truncateVisibleLine(preview, previewWidth, "..."))
}

func (p PostsPageModel) postDetailLines(post models.Post, width int) []string {
	text := normalizeRenderedText(post.Text)
	lines := p.wrapPlainTextLines(text, width)
	if strings.TrimSpace(text) == "" {
		lines = nil
	}
	if !p.hasPostMedia(post) {
		if len(lines) == 0 {
			return []string{""}
		}
		return lines
	}

	return p.wrapPlainTextLines(p.postDisplayText(post), width)
}

func (p PostsPageModel) postDisplayText(post models.Post) string {
	text := normalizeRenderedText(post.Text)
	if p.hasPostMedia(post) {
		if text == "" {
			return "[图片]"
		}
		return text + "\n[图片]"
	}
	return text
}

func (p PostsPageModel) commentDisplayText(c models.Comment, name string) string {
	text := fmt.Sprintf("%s: %s", name, normalizeRenderedText(c.Text))
	if p.hasCommentMedia(c) {
		return text + "\n[图片]"
	}
	return text
}

func (p PostsPageModel) commentBodyText(c models.Comment) string {
	text := normalizeRenderedText(c.Text)
	if p.hasCommentMedia(c) {
		if text == "" {
			return "[图片]"
		}
		return text + "\n[图片]"
	}
	return text
}

func (p PostsPageModel) hasPostMedia(post models.Post) bool {
	return post.Type == "image" || strings.TrimSpace(post.MediaIds) != ""
}

func (p PostsPageModel) hasCommentMedia(c models.Comment) bool {
	return strings.TrimSpace(c.MediaIds) != ""
}

func (p PostsPageModel) listLineTextWidth(contentWidth int, selected bool) int {
	width := contentWidth - lipgloss.Width("  ")
	if selected {
		width -= 2
	}
	if width < 1 {
		width = 1
	}
	return width
}

func (p PostsPageModel) detailBodyTextWidth(contentWidth int) int {
	width := contentWidth - vDetailSection.GetHorizontalFrameSize() - vPostTextStyle.GetHorizontalFrameSize()
	if focusedWidth := contentWidth - vDetailSectionFocused.GetHorizontalFrameSize() - vPostTextStyle.GetHorizontalFrameSize(); focusedWidth < width {
		width = focusedWidth
	}
	return maxInt(1, width)
}

func (p PostsPageModel) commentBodyTextWidth(contentWidth int) int {
	width := contentWidth - vDetailSection.GetHorizontalFrameSize() - 2
	if focusedWidth := contentWidth - vDetailSectionFocused.GetHorizontalFrameSize() - 2; focusedWidth < width {
		width = focusedWidth
	}
	width -= vCommentSelectedStyle.GetHorizontalFrameSize()
	width -= lipgloss.Width("  ")
	return maxInt(1, width)
}

func (p PostsPageModel) commentQuoteTextWidth(contentWidth int) int {
	width := p.commentBodyTextWidth(contentWidth) - vCommentQuoteStyle.GetHorizontalFrameSize()
	return maxInt(1, width)
}

func (p PostsPageModel) postHeader(post models.Post) string {
	ts := time.Unix(int64(post.Timestamp), 0).In(shanghaiLocation).Format("2006-01-02 15:04")
	replyStr := vPostReplyStyle.Render(fmt.Sprintf("❝ %d", post.Reply))
	praiseState := "♡"
	if post.IsPraise {
		praiseState = "♥"
	}
	followState := "☆"
	if post.IsFollow {
		followState = "★"
	}
	praiseStr := vPostLikeStyle.Render(fmt.Sprintf("%s %d", praiseState, post.PraiseNum))
	followStr := vPostLikeStyle.Render(fmt.Sprintf("%s %d", followState, post.Likenum))
	meta := replyStr + " " + praiseStr + " " + followStr
	pidStr := vPostPidStyle.Render(fmt.Sprintf("#%-6d", post.Pid))
	tsStr := vPostTimeStyle.Render(ts)
	if !post.Anonymous {
		return pidStr + " [实名] " + tsStr + "  " + meta
	}
	return pidStr + " " + tsStr + "  " + meta
}

func (p PostsPageModel) postHeaderPlain(post models.Post) string {
	ts := time.Unix(int64(post.Timestamp), 0).In(shanghaiLocation).Format("2006-01-02 15:04")
	praiseState := "♡"
	if post.IsPraise {
		praiseState = "♥"
	}
	followState := "☆"
	if post.IsFollow {
		followState = "★"
	}
	header := fmt.Sprintf("#%-6d %s  ❝ %d %s %d %s %d", post.Pid, ts, post.Reply, praiseState, post.PraiseNum, followState, post.Likenum)
	if !post.Anonymous {
		header = fmt.Sprintf("#%-6d [实名] %s  ❝ %d %s %d %s %d", post.Pid, ts, post.Reply, praiseState, post.PraiseNum, followState, post.Likenum)
	}
	return header
}

func (p PostsPageModel) postHeaderLines(post models.Post, width int) []string {
	styled := p.postHeader(post)
	if lipgloss.Width(styled) <= width {
		return []string{styled}
	}
	return wrapVisibleLine(p.postHeaderPlain(post), width)
}

func (p PostsPageModel) wrapPlainTextLines(text string, width int) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")
	if len(rawLines) == 0 {
		return []string{""}
	}

	var wrapped []string
	for _, line := range rawLines {
		wrapped = append(wrapped, wrapVisibleLine(line, width)...)
	}
	if len(wrapped) == 0 {
		return []string{""}
	}
	return wrapped
}

func wrapVisibleLine(line string, width int) []string {
	if width < 1 {
		width = 1
	}
	if line == "" {
		return []string{""}
	}

	var wrapped []string
	var current []rune
	currentWidth := 0

	for _, r := range []rune(line) {
		runeWidth := lipgloss.Width(string(r))
		if runeWidth < 1 {
			runeWidth = 1
		}
		if len(current) > 0 && currentWidth+runeWidth > width {
			wrapped = append(wrapped, string(current))
			current = current[:0]
			currentWidth = 0
		}
		current = append(current, r)
		currentWidth += runeWidth
	}

	if len(current) > 0 {
		wrapped = append(wrapped, string(current))
	}
	if len(wrapped) == 0 {
		return []string{""}
	}
	return wrapped
}

func normalizeRenderedText(text string) string {
	if text == "" {
		return ""
	}
	runes := []rune(text)
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		out = append(out, r)

		// Skip any Unicode modifiers or variation selectors that follow the base character
		for i+1 < len(runes) && isUnicodeModifier(runes[i+1]) {
			i++
		}
	}
	return string(out)
}

// isUnicodeModifier checks if a rune is a Unicode modifier or variation selector
// that should be skipped when normalizing text.
func isUnicodeModifier(r rune) bool {
	// Variation Selectors (VS1-VS16): U+FE00-U+FE0F
	if r >= '\uFE00' && r <= '\uFE0F' {
		return true
	}

	// Combining Diacritical Marks: U+0300-U+036F
	if r >= '\u0300' && r <= '\u036F' {
		return true
	}

	// Emoji Modifiers (skin tone): U+1F3FB-U+1F3FF
	if r >= '\U0001F3FB' && r <= '\U0001F3FF' {
		return true
	}

	// Keycap symbol: U+20E3
	if r == '\u20E3' {
		return true
	}

	// Other common combining characters
	// Combining Diacritical Marks Supplement: U+1DC0-U+1DFF
	if r >= '\u1DC0' && r <= '\u1DFF' {
		return true
	}

	// Combining Diacritical Marks for Symbols: U+20D0-U+20FF
	if r >= '\u20D0' && r <= '\u20FF' {
		return true
	}

	return false
}

func truncateVisibleLine(line string, width int, suffix string) string {
	if width < 1 {
		return ""
	}
	if suffix == "" {
		suffix = "..."
	}
	if lipgloss.Width(line) <= width {
		return line
	}

	suffixWidth := lipgloss.Width(suffix)
	if suffixWidth >= width {
		return string([]rune(suffix)[:1])
	}

	var current []rune
	currentWidth := 0
	for _, r := range []rune(line) {
		runeWidth := lipgloss.Width(string(r))
		if runeWidth < 1 {
			runeWidth = 1
		}
		if currentWidth+runeWidth+suffixWidth > width {
			break
		}
		current = append(current, r)
		currentWidth += runeWidth
	}
	return string(current) + suffix
}

func (p *PostsPageModel) moveCommentSelection(delta int) {
	if len(p.CommentList) == 0 {
		return
	}
	totalLines := p.commentLineCount()
	if totalLines <= 0 {
		return
	}
	p.syncCommentCursorToSelection()
	p.CommentCursorLine = clampInt(p.CommentCursorLine+delta, 0, totalLines-1)
	p.SelectedCommentIdx = p.resolveCommentSelectionAtLine(p.CommentCursorLine, delta)
	p.scrollCommentCursorIntoView()
}

func (p *PostsPageModel) commentPageMove(direction int) {
	if len(p.CommentList) == 0 || p.CommentViewport == nil || direction == 0 {
		return
	}
	totalLines := p.commentLineCount()
	if totalLines <= 0 {
		return
	}
	p.syncCommentCursorToSelection()
	step := p.commentPageStep()
	delta := step
	if direction < 0 {
		delta = -step
	}
	p.CommentCursorLine = clampInt(p.CommentCursorLine+delta, 0, totalLines-1)
	p.SelectedCommentIdx = p.resolveCommentSelectionAtLine(p.CommentCursorLine, delta)

	maxOffset := maxInt(0, totalLines-p.CommentViewport.VisibleLineCount())
	p.CommentViewport.SetYOffset(clampInt(p.CommentViewport.YOffset+delta, 0, maxOffset))
	p.scrollCommentCursorIntoView()
}

func (p *PostsPageModel) commentPageStep() int {
	if p.CommentViewport == nil {
		return 1
	}
	visibleLines := p.CommentViewport.VisibleLineCount()
	if visibleLines <= 1 {
		return 1
	}
	step := visibleLines - 2
	if step < 1 {
		step = 1
	}
	return step
}

func (p *PostsPageModel) scrollCommentCursorIntoView() {
	if p.CommentViewport == nil || len(p.CommentList) == 0 {
		return
	}
	visible := p.CommentViewport.VisibleLineCount()
	if visible <= 0 {
		return
	}

	topMargin := 1
	bottomMargin := 3
	if maxTop := visible / 4; maxTop < topMargin {
		topMargin = maxTop
	}
	if maxBottom := visible - 2; maxBottom < bottomMargin {
		bottomMargin = maxBottom
	}
	if topMargin < 0 {
		topMargin = 0
	}
	if bottomMargin < 0 {
		bottomMargin = 0
	}

	topThreshold := p.CommentViewport.YOffset + topMargin
	bottomThreshold := p.CommentViewport.YOffset + visible - bottomMargin - 1
	if p.CommentCursorLine < topThreshold {
		p.CommentViewport.SetYOffset(maxInt(0, p.CommentCursorLine-topMargin))
		return
	}
	if p.CommentCursorLine > bottomThreshold {
		p.CommentViewport.SetYOffset(maxInt(0, p.CommentCursorLine-visible+bottomMargin+1))
	}
}

func (p *PostsPageModel) commentLineRangeAt(index int) (int, int) {
	if index < 0 || index >= len(p.CommentList) {
		return 0, 0
	}
	line := 0
	width := 20
	if p.CommentViewport != nil && p.CommentViewport.Width > 0 {
		width = p.CommentViewport.Width
	}
	for i, c := range p.orderedComments() {
		start := line
		line += len(p.commentLogicalLines(c, width))
		if i == index {
			return start, line - 1
		}
	}
	return 0, 0
}

func (p *PostsPageModel) syncCommentCursorToSelection() {
	if len(p.CommentList) == 0 {
		p.CommentCursorLine = 0
		p.SelectedCommentIdx = 0
		return
	}
	if p.SelectedCommentIdx < 0 {
		p.SelectedCommentIdx = 0
	}
	if p.SelectedCommentIdx >= len(p.CommentList) {
		p.SelectedCommentIdx = len(p.CommentList) - 1
	}
	start, end := p.commentLineRangeAt(p.SelectedCommentIdx)
	if p.CommentCursorLine < start || p.CommentCursorLine > end {
		p.CommentCursorLine = start
	}
}

func (p *PostsPageModel) reconcileCommentSelectionWithCursor() {
	if len(p.CommentList) == 0 {
		p.CommentCursorLine = 0
		p.SelectedCommentIdx = 0
		return
	}
	totalLines := p.commentLineCount()
	if totalLines <= 0 {
		p.CommentCursorLine = 0
		p.SelectedCommentIdx = 0
		return
	}
	p.CommentCursorLine = clampInt(p.CommentCursorLine, 0, totalLines-1)
	if p.SelectedCommentIdx < 0 {
		p.SelectedCommentIdx = 0
	}
	if p.SelectedCommentIdx >= len(p.CommentList) {
		p.SelectedCommentIdx = len(p.CommentList) - 1
	}
	start, end := p.commentLineRangeAt(p.SelectedCommentIdx)
	if p.CommentCursorLine >= start && p.CommentCursorLine <= end {
		return
	}
	bias := -1
	if p.CommentCursorLine > end {
		bias = 1
	}
	p.SelectedCommentIdx = p.commentIndexAtLineWithBias(p.CommentCursorLine, bias)
}

func (p *PostsPageModel) commentIndexAtLine(target int) int {
	return p.commentIndexAtLineWithBias(target, -1)
}

func (p *PostsPageModel) commentIndexAtLineWithBias(target, bias int) int {
	line := 0
	width := 20
	if p.CommentViewport != nil && p.CommentViewport.Width > 0 {
		width = p.CommentViewport.Width
	}
	for i, c := range p.orderedComments() {
		commentLines := len(p.commentLogicalLines(c, width))
		if target < line+commentLines {
			return i
		}
		line += commentLines
	}
	return maxInt(0, len(p.CommentList)-1)
}

func (p *PostsPageModel) resolveCommentSelectionAtLine(target, bias int) int {
	return p.commentIndexAtLineWithBias(target, bias)
}

func (p *PostsPageModel) SelectedPost() *models.Post {
	if p.SelectedPostIdx < 0 || p.SelectedPostIdx >= len(p.PostList) {
		return nil
	}
	if mentioned := p.selectedMentionedPost(); mentioned != nil {
		return mentioned
	}
	post := p.PostList[p.SelectedPostIdx]
	return &post
}

func (p *PostsPageModel) selectedMentionedPost() *models.Post {
	if p.SelectedPostIdx < 0 || p.SelectedPostIdx >= len(p.PostList) {
		return nil
	}
	start, _, ok := p.postMentionLineRangeAt(p.SelectedPostIdx)
	if !ok || p.CursorLine != start {
		return nil
	}
	post := p.PostList[p.SelectedPostIdx]
	if post.MentionedPost != nil {
		mentioned := *post.MentionedPost
		return &mentioned
	}
	pid := mentionedPostID(post)
	if pid <= 0 {
		return nil
	}
	return &models.Post{Pid: pid}
}

func (p *PostsPageModel) postMentionLineRangeAt(index int) (startLine, endLine int, ok bool) {
	if index < 0 || index >= len(p.PostList) {
		return 0, 0, false
	}
	line := 0
	for i := 0; i < index; i++ {
		if i > 0 {
			line++
		}
		line += p.postRenderedLinesAt(i)
	}
	if index > 0 {
		line++
	}

	post := p.PostList[index]
	selected := index == p.SelectedPostIdx
	lineWidth := p.listLineTextWidth(p.currentListContentWidth(), selected)
	if p.postMentionLine(post, lineWidth) == "" {
		return 0, 0, false
	}
	line += len(p.postHeaderLines(post, lineWidth))
	line += len(p.postListTextLines(post, lineWidth))
	return line, line, true
}

func (p *PostsPageModel) SelectedComment() *models.Comment {
	if p.SelectedCommentIdx < 0 || p.SelectedCommentIdx >= len(p.CommentList) {
		return nil
	}
	comment := p.CommentList[p.SelectedCommentIdx]
	return &comment
}

func (p *PostsPageModel) updatePost(updated *models.Post) {
	if updated == nil {
		return
	}
	for i := range p.PostList {
		if p.PostList[i].Pid == updated.Pid {
			mentioned := p.PostList[i].MentionedPost
			p.PostList[i] = *updated
			if p.PostList[i].MentionedPost == nil {
				p.PostList[i].MentionedPost = mentioned
			}
			p.postContent = ""
			continue
		}
		if p.PostList[i].MentionedPost != nil && p.PostList[i].MentionedPost.Pid == updated.Pid {
			mentioned := *updated
			p.PostList[i].MentionedPost = &mentioned
			p.postContent = ""
		}
	}
}
