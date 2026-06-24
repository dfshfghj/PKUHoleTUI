package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var shanghaiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}()

func (m Model) View() tea.View {
	m.ensureDialogModels()

	w := m.Width
	if w < 1 {
		w = 80
	}
	h := m.Height
	if h < 1 {
		h = 24
	}

	rendered, placements := m.renderScreen(w, h)
	rendered = baseStyle.Render(rendered)
	rendered = stripPresentationSelectors(rendered)
	rendered = trimTerminalWrapRiskPadding(rendered)
	if m.Images != nil && m.Images.Enabled() {
		m.Images.SetFrame(placements)
	}
	if m.Capture != nil {
		m.Capture.RecordFrame(rendered)
	}

	view := tea.NewView(rendered)
	view.AltScreen = true
	return view
}

func trimTerminalWrapRiskPadding(frame string) string {
	lines := strings.Split(frame, "\n")
	for i, line := range lines {
		lines[i] = trimTrailingSpacesBeforeANSISuffix(line, terminalWrapRiskExtraWidth(line))
	}
	return strings.Join(lines, "\n")
}

func stripPresentationSelectors(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '\uFE00' && r <= '\uFE0F' {
			return -1
		}
		return r
	}, s)
}

func terminalWrapRiskExtraWidth(line string) int {
	extra := 0
	for _, r := range stripANSISequences(line) {
		if terminalMayRenderDoubleWidth(r) && lipgloss.Width(string(r)) < 2 {
			extra++
		}
	}
	return extra
}

func terminalMayRenderDoubleWidth(r rune) bool {
	switch {
	case r >= 0x2300 && r <= 0x23FF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	}
	return false
}

func trimTrailingSpacesBeforeANSISuffix(line string, count int) string {
	if count <= 0 {
		return line
	}
	suffixStart := len(line)
	for {
		matches := ansiControlPattern.FindAllStringIndex(line[:suffixStart], -1)
		if len(matches) == 0 {
			break
		}
		last := matches[len(matches)-1]
		if last[1] != suffixStart {
			break
		}
		suffixStart = last[0]
	}
	for count > 0 && suffixStart > 0 && line[suffixStart-1] == ' ' {
		line = line[:suffixStart-1] + line[suffixStart:]
		suffixStart--
		count--
	}
	return line
}

func (m Model) renderScreen(width, height int) (string, []imagePlacement) {
	var body string
	var placements []imagePlacement

	switch m.Dialog {
	case DialogHelp:
		body, placements = m.renderHelpScreen(width, height)
	case DialogTools:
		body, placements = m.renderPanelScreenWithStyle(width, height, m.renderToolsPanelContent, true)
	case DialogComposer:
		body, placements = m.renderPanelScreen(width, height, m.renderComposerPanelContent)
	case DialogImage:
		body, placements = m.renderImagePanelScreen(width, height)
	default:
		var base string
		var p []imagePlacement
		if m.Page == PageDashboard {
			base = m.renderDashboardScreen(width, height)
		} else {
			base, p = m.renderMainLayout(width, height)
		}
		placements = p
		if m.Dialog != DialogNone {
			dialog := m.renderDialog()
			base = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
		}
		body = lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, base)
	}

	if m.ToastMsg != "" {
		body = m.overlayToast(width, body)
	}
	return body, placements
}

func (m Model) renderDashboardScreen(width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, m.Dashboard.View(width, height))
}

func (m Model) renderMainLayout(width, height int) (string, []imagePlacement) {
	m.Width = width
	m.Height = height

	var tabBar string
	if m.Page != PageDashboard {
		tabs := []string{"同步", "帖子", "课表", "成绩"}
		var segments []powerlineSegment
		for i, t := range tabs {
			if i == m.TabCursor {
				segments = append(segments, powerlineSegment{Text: t, Style: tabItemActiveStyle})
			} else {
				segments = append(segments, powerlineSegment{Text: t, Style: tabItemStyle})
			}
		}
		tabBar = m.renderTabBar(width, m.renderPowerlineGroup(segments, powerlineRight))
	}
	var footer string
	if m.Page != PageDashboard {
		footer = m.renderStatusLine(width)
	}

	contentHeight := m.contentAreaHeightForSize(width, height)
	content, placements := m.renderContent(contentHeight)
	for i := range placements {
		placements[i].top += lipgloss.Height(tabBar)
	}
	contentBlock := lipgloss.Place(
		width,
		contentHeight,
		lipgloss.Left,
		lipgloss.Top,
		content,
	)

	parts := make([]string, 0, 3)
	if tabBar != "" {
		parts = append(parts, tabBar)
	}
	parts = append(parts, contentBlock)
	if footer != "" {
		parts = append(parts, footer)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) renderHelpScreen(width, height int) (string, []imagePlacement) {
	panelWidth := m.helpPanelWidth(width)
	mainModel := m
	mainModel.Dialog = DialogNone
	main, placements := mainModel.renderMainLayout(width, height)

	panel := m.renderHelpPanel(panelWidth)
	cardHeight := lipgloss.Height(panel)

	statusLineHeight := lipgloss.Height(m.renderStatusLine(width))
	availableHeight := height - statusLineHeight

	baseLayer := lipgloss.NewLayer(main)
	panelX := width - panelWidth
	if panelX < 0 {
		panelX = 0
	}
	panelY := availableHeight - cardHeight
	if panelY < 0 {
		panelY = 0
	}
	panelLayer := lipgloss.NewLayer(panel).X(panelX).Y(panelY).Z(1)
	body := lipgloss.NewCompositor(baseLayer, panelLayer).Render()
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) renderPanelScreen(width, height int, renderContent func(panelW, panelH int) string) (string, []imagePlacement) {
	return m.renderPanelScreenWithStyle(width, height, renderContent, false)
}

func (m Model) renderPanelScreenWithStyle(width, height int, renderContent func(panelW, panelH int) string, fillPanel bool) (string, []imagePlacement) {
	mainModel := m
	mainModel.Dialog = DialogNone
	main, placements := mainModel.renderMainLayout(width, height)

	panelW := width * 4 / 5
	panelH := height * 4 / 5
	if panelW < 40 {
		panelW = 40
	}
	if panelH < 22 {
		panelH = 22
	}
	if panelW > width {
		panelW = width
	}
	if panelH > height {
		panelH = height
	}

	panelX := (width - panelW) / 2
	panelY := (height - panelH) / 2

	panelStyle := panelContentStyle.Width(panelW).MaxHeight(panelH)
	if fillPanel {
		panelStyle = panelStyle.
			Padding(1, 3).
			Height(maxInt(1, panelH-2))
	}
	panel := panelStyle.Render(renderContent(panelW, panelH))

	baseLayer := lipgloss.NewLayer(main)
	panelLayer := lipgloss.NewLayer(panel).X(panelX).Y(panelY).Z(1)
	body := lipgloss.NewCompositor(baseLayer, panelLayer).Render()
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) renderToolsPanelContent(panelW, panelH int) string {
	return m.ToolsDialog.View(
		maxInt(20, panelW-panelContentStyle.GetHorizontalFrameSize()),
		maxInt(8, panelH-panelContentStyle.GetVerticalFrameSize()),
	)
}

func (m Model) renderComposerPanelContent(panelW, panelH int) string {
	return m.Composer.View(panelW, panelH)
}

func (m Model) renderImagePanelScreen(width, height int) (string, []imagePlacement) {
	mainModel := m
	mainModel.Dialog = DialogNone
	main, _ := mainModel.renderMainLayout(width, height)

	panelW := width * 4 / 5
	panelH := height * 4 / 5
	if panelW < 40 {
		panelW = 40
	}
	if panelH < 22 {
		panelH = 22
	}
	if panelW > width {
		panelW = width
	}
	if panelH > height {
		panelH = height
	}

	panelX := (width - panelW) / 2
	panelY := (height - panelH) / 2

	content, placements := m.ImageDialog.View(panelW, panelH, m.Images != nil && m.Images.Enabled())
	for i := range placements {
		placements[i].left += panelX + 3
		placements[i].top += panelY + 1
		placements[i].winCols = width
		placements[i].winRows = height
	}
	panel := panelContentStyle.Width(panelW).MaxHeight(panelH).Render(content)

	baseLayer := lipgloss.NewLayer(main)
	panelLayer := lipgloss.NewLayer(panel).X(panelX).Y(panelY).Z(1)
	body := lipgloss.NewCompositor(baseLayer, panelLayer).Render()
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) overlayToast(screenWidth int, body string) string {
	toastW := max(screenWidth/4, 20)
	toast := toastStyle.Width(toastW).Render(m.ToastMsg)

	baseLayer := lipgloss.NewLayer(body)
	toastLayer := lipgloss.NewLayer(toast).X(screenWidth - toastW - 2).Y(1).Z(2)
	return lipgloss.NewCompositor(baseLayer, toastLayer).Render()
}

func (m Model) contentAreaHeightForSize(width, height int) int {
	if m.Page == PageDashboard {
		if height < 1 {
			return 1
		}
		return height
	}
	tabBar := m.renderTabBar(width, "")
	footer := m.renderStatusLine(width)
	contentHeight := height - lipgloss.Height(tabBar) - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentHeight
}

func (m Model) renderTabBar(width int, content string) string {
	innerWidth := width
	return tabBarStyle.Width(innerWidth).MaxWidth(innerWidth).Render(content)
}

func (m Model) renderStatusLine(width int) string {
	if width < 1 {
		width = 1
	}
	innerWidth := width

	left := m.renderPowerlineGroup([]powerlineSegment{
		{Text: m.currentModeLabel(), Style: statusModeStyle},
		{Text: m.currentPageLabel(), Style: statusPageStyle},
	}, powerlineRight)
	right := m.renderPowerlineGroup([]powerlineSegment{
		{Text: m.renderSessionLabel(), Style: m.sessionBadgeStyle()},
		{Text: time.Now().In(shanghaiLocation).Format("15:04:05"), Style: statusClockStyle},
	}, powerlineLeft)

	summary := m.currentStatusSummary()
	line := joinStatusSections(innerWidth, left, summary, right)
	return footerStyle.Width(innerWidth).MaxWidth(innerWidth).Render(line)
}

type powerlineSegment struct {
	Text  string
	Style lipgloss.Style
}

type powerlineDirection int

const (
	powerlineRight powerlineDirection = iota
	powerlineLeft
)

func (m Model) renderPowerlineGroup(segments []powerlineSegment, direction powerlineDirection) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	background := footerStyle.GetBackground()
	if background == nil {
		background = colorBg
	}

	if direction == powerlineRight {
		for i, seg := range segments {
			b.WriteString(seg.Style.Render(seg.Text))
			if i == len(segments)-1 {
				continue
			}
			next := segments[i+1]
			b.WriteString(powerlineSeparatorRight(seg.Style.GetBackground(), next.Style.GetBackground()))
		}
		last := segments[len(segments)-1]
		b.WriteString(powerlineSeparatorRight(last.Style.GetBackground(), background))
		return b.String()
	}

	b.WriteString(powerlineSeparatorLeft(background, segments[0].Style.GetBackground()))
	for i, seg := range segments {
		b.WriteString(seg.Style.Render(seg.Text))
		if i == len(segments)-1 {
			continue
		}
		next := segments[i+1]
		b.WriteString(powerlineSeparatorLeft(seg.Style.GetBackground(), next.Style.GetBackground()))
	}
	return b.String()
}

func powerlineSeparatorRight(left, right color.Color) string {
	return lipgloss.NewStyle().
		Foreground(left).
		Background(right).
		Render("")
}

func powerlineSeparatorLeft(left, right color.Color) string {
	return lipgloss.NewStyle().
		Foreground(right).
		Background(left).
		Render("")
}

func joinStatusSections(width int, left, middle, right string) string {
	if width < 1 {
		return ""
	}

	fillStyle := lipgloss.NewStyle().Background(colorSurface)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 >= width {
		return truncateVisibleLine(left+" "+right, width, "...")
	}

	availableMiddle := width - leftWidth - rightWidth - 2
	if availableMiddle < 0 {
		availableMiddle = 0
	}
	if availableMiddle == 0 {
		return left + fillStyle.Render(strings.Repeat(" ", maxInt(0, width-leftWidth-rightWidth))) + right
	}

	middleInnerWidth := maxInt(0, availableMiddle-statusInfoStyle.GetHorizontalFrameSize())
	middle = truncateVisibleLine(middle, middleInnerWidth, "...")
	middleWidth := lipgloss.Width(middle)
	if middleWidth < middleInnerWidth {
		middle += fillStyle.Render(strings.Repeat(" ", middleInnerWidth-middleWidth))
	}
	middle = statusInfoStyle.Render(middle)
	core := left + fillStyle.Render(" ") + middle
	padding := width - lipgloss.Width(core) - rightWidth
	if padding < 1 {
		padding = 1
	}
	return core + fillStyle.Render(strings.Repeat(" ", padding)) + right
}

func (m Model) currentModeLabel() string {
	switch m.Dialog {
	case DialogTools:
		switch m.ToolsDialog.Section() {
		case ToolsSectionLogs:
			return "TOOLS-LOGS"
		case ToolsSectionInteractive:
			return "TOOLS-NOTIFY"
		case ToolsSectionSystem:
			return "TOOLS-SYSTEM"
		default:
			return "TOOLS-CONFIG"
		}
	case DialogHelp:
		return "HELP"
	case DialogSessionPrompt:
		return "LOGIN"
	case DialogAuthChallenge:
		return "AUTH"
	case DialogComposer:
		return "COMPOSE"
	case DialogTags:
		return "TAGS"
	}

	if m.Page == PageDashboard {
		return "DASHBOARD"
	}
	if m.Page == PageHome {
		if m.Home.CrawlerState == CrawlerRunning {
			return "SYNC"
		}
		return "HOME"
	}
	if m.Page == PageSchedule {
		return "SCHEDULE"
	}
	if m.Page == PageScores {
		return "SCORES"
	}

	if m.Posts.Searching {
		return "SEARCH"
	}
	if m.Posts.ShowPostDetail {
		if m.Posts.DetailFocus == DetailFocusPost {
			return "DETAIL-POST"
		}
		return "DETAIL-CMT"
	}
	if m.Posts.SearchActive {
		return "RESULTS"
	}
	return "NORMAL"
}

func (m Model) currentPageLabel() string {
	switch m.Page {
	case PageDashboard:
		return "TreeHole"
	case PageHome:
		return m.Home.modeLabel()
	case PagePosts:
		if m.Posts.ShowPostDetail && m.Posts.CurrentPost != nil {
			return fmt.Sprintf("#%d", m.Posts.CurrentPost.Pid)
		}
		if m.Posts.SearchActive {
			query := strings.TrimSpace(m.Posts.SearchInput)
			if query == "" {
				return "搜索结果"
			}
			return fmt.Sprintf("搜索 %s", query)
		}
		return "帖子列表"
	case PageSchedule:
		return "课表"
	case PageScores:
		return "成绩"
	default:
		return "TreeHole TUI"
	}
}

func (m Model) currentStatusSummary() string {
	if m.LastError != "" {
		return "错误: " + m.LastError
	}
	if m.Dialog != DialogNone {
		return m.dialogStatusSummary()
	}
	if m.Page == PageDashboard {
		if m.Dashboard.Loading {
			return "正在加载通知"
		}
		return "e: 浏览 | n: 通知 | c: 配置"
	}
	if m.Page == PageHome {
		return m.homeStatusSummary()
	}
	if m.Page == PageSchedule {
		return m.scheduleStatusSummary()
	}
	if m.Page == PageScores {
		return m.scoresStatusSummary()
	}
	return m.postsStatusSummary()
}

func (m Model) dialogStatusSummary() string {
	switch m.Dialog {
	case DialogTools:
		return "1/2/3/4: 配置/日志/互动/系统 | Ctrl+S: 保存配置 | Esc: 关闭"
	case DialogImage:
		return "o: 图片预览 | Left/Right: 切换 | Esc: 关闭"
	case DialogHelp:
		return "当前快捷键"
	case DialogSessionPrompt:
		return "登录态不可用，按 Enter 选择恢复方式"
	case DialogAuthChallenge:
		return "需要补充登录验证信息"
	case DialogComposer:
		return "Ctrl+S: 提交 | Esc: 取消"
	case DialogTags:
		return "Enter: 选择标签 | c: 清除筛选 | Esc: 关闭"
	default:
		return "h: 帮助 | Ctrl+Q: 退出"
	}
}

func (m Model) homeStatusSummary() string {
	switch m.Home.CrawlerState {
	case CrawlerRunning:
		elapsed := "0s"
		if !m.Home.CrawlerStart.IsZero() {
			elapsed = time.Since(m.Home.CrawlerStart).Round(time.Second).String()
		}
		progress := "等待首轮抓取"
		if m.Home.LastCrawlPage > 0 {
			progress = fmt.Sprintf("已抓到第%d页", m.Home.LastCrawlPage)
		}
		return fmt.Sprintf("%s | 已运行 %s", progress, elapsed)
	case CrawlerError:
		if m.Home.HomeLastError != "" {
			return "爬虫错误: " + m.Home.HomeLastError
		}
		return "爬虫错误"
	default:
		return "爬虫已停止"
	}
}

func (m Model) scheduleStatusSummary() string {
	if m.Schedule.Loading {
		return "加载课表中..."
	}
	if m.Schedule.Error != "" {
		return "课表错误: " + m.Schedule.Error
	}
	return fmt.Sprintf("%d 节", len(m.Schedule.Rows))
}

func (m Model) scoresStatusSummary() string {
	if m.Scores.Loading {
		return "加载成绩中..."
	}
	if m.Scores.Error != "" {
		return "成绩错误: " + m.Scores.Error
	}
	if m.Scores.Summary == nil {
		return "成绩未加载"
	}
	return fmt.Sprintf("GPA %s | %d 门成绩", emptyDash(m.Scores.Summary.GPA), len(m.Scores.Summary.Scores))
}

func (m Model) postsStatusSummary() string {
	if m.Posts.Searching {
		return "搜索输入中"
	}
	if m.Posts.ShowPostDetail && m.Posts.CurrentPost != nil {
		comments := fmt.Sprintf("评论 %d", len(m.Posts.CommentList))
		if m.Posts.CommentListLoading {
			comments += " | 加载评论中..."
		} else if m.Posts.CommentListHasMore {
			comments += " | 可继续下翻加载更多"
		}
		focus := "评论"
		if m.Posts.DetailFocus == DetailFocusPost {
			focus = "正文"
		}
		return fmt.Sprintf("%s | %s", comments, focus)
	}

	scope := fmt.Sprintf("%d 条", len(m.Posts.PostList))
	if m.Posts.SearchActive {
		scope = fmt.Sprintf("%d 条", len(m.Posts.PostList))
	}
	if m.Posts.ActiveTag != "" {
		scope += " | 标签 #" + m.Posts.ActiveTag
	}
	if m.Posts.PostListLoading {
		if len(m.Posts.PostList) == 0 {
			scope += " | 加载帖子中..."
		} else {
			scope += " | 正在加载更多..."
		}
	}
	return scope
}

func (m Model) renderSessionLabel() string {
	if m.Session.Mode == SessionModeOnline {
		if m.Session.CanWriteOnline {
			return "ONLINE WRITE"
		}
		return "ONLINE READ"
	}
	return "OFFLINE"
}

func (m Model) sessionBadgeStyle() lipgloss.Style {
	if m.Session.Mode == SessionModeOnline {
		return statusSessionOnlineStyle
	}
	return statusSessionOfflineStyle
}

func (m Model) renderContent(contentHeight int) (string, []imagePlacement) {
	switch m.Page {
	case PageDashboard:
		return m.Dashboard.View(m.Width, contentHeight), nil
	case PageHome:
		return m.renderHome(contentHeight), nil
	case PagePosts:
		return m.renderPosts(contentHeight)
	case PageSchedule:
		return m.renderSchedule(contentHeight), nil
	case PageScores:
		return m.renderScores(contentHeight), nil
	default:
		return "Unknown page", nil
	}
}

func (m Model) renderDialog() string {
	switch m.Dialog {
	case DialogHelp:
		return m.renderHelpDialog()
	case DialogSessionPrompt:
		return m.renderDialogCard(m.SessionDialog.View(m.Width))
	case DialogAuthChallenge:
		return m.renderDialogCard(m.AuthDialog.View(m.Width))
	case DialogTags:
		return m.renderDialogCard(m.TagsDialog.View(m.Width))
	default:
		return ""
	}
}

func (m Model) renderHome(contentHeight int) string {
	return m.Home.View(m.Width, contentHeight)
}

func (m Model) renderPosts(contentHeight int) (string, []imagePlacement) {
	return m.Posts.View(m.Width, contentHeight)
}

func (m Model) renderSchedule(contentHeight int) string {
	return m.Schedule.View(m.Width, contentHeight)
}

func (m Model) renderScores(contentHeight int) string {
	return m.Scores.View(m.Width, contentHeight)
}

func (m Model) renderHelpDialog() string {
	return m.renderHelpPanel(m.helpPanelWidth(m.Width))
}

func (m Model) renderHelpPanel(width int) string {
	panelWidth := maxInt(24, width)
	cardWidth := maxInt(18, panelWidth-helpCard.GetHorizontalFrameSize())
	innerWidth := cardWidth - helpCard.GetHorizontalPadding()
	keyWidth := maxInt(6, minInt(10, innerWidth/3))
	descWidth := maxInt(4, innerWidth-keyWidth-1)

	var b strings.Builder

	b.WriteString(vDialogTitleStyle.Render("快捷键"))
	b.WriteString("\n")
	b.WriteString(vStatLabelStyle.Render(m.helpContextTitle()))
	b.WriteString("\n\n")

	for _, item := range m.helpItems() {
		descLines := wrapVisibleLine(item.desc, descWidth)
		keyText := clipToVisibleWidth(item.key, keyWidth)
		keyText = vStatValueStyle.Width(keyWidth).Render(keyText)
		b.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top,
			keyText,
			" ",
			vStatLabelStyle.Width(descWidth).Render(descLines[0]),
		))
		for _, line := range descLines[1:] {
			b.WriteString("\n")
			b.WriteString(lipgloss.JoinHorizontal(
				lipgloss.Top,
				vStatValueStyle.Width(keyWidth).Render(""),
				" ",
				vStatLabelStyle.Width(descWidth).Render(line),
			))
		}
		b.WriteString("\n")
	}
	b.WriteString(vDialogHelpStyle.Render("Esc: 关闭"))

	card := helpCard.Width(cardWidth).Render(b.String())
	return lipgloss.Place(panelWidth, lipgloss.Height(card), lipgloss.Right, lipgloss.Top, card)
}

type helpItem struct {
	key  string
	desc string
}

func (m Model) helpPanelWidth(totalWidth int) int {
	return clampInt(totalWidth/4, 24, 34)
}

func (m Model) helpContextTitle() string {
	if m.Page == PageDashboard {
		return "Dashboard"
	}
	if m.Page == PageHome {
		if m.Home.CrawlerState == CrawlerRunning {
			return "同步页"
		}
		return "同步页"
	}
	if m.Posts.Searching {
		return "搜索"
	}
	if m.Posts.ShowPostDetail {
		return "帖子详情"
	}
	if m.Page == PageSchedule {
		return "课表"
	}
	if m.Page == PageScores {
		return "成绩"
	}
	return "帖子列表"
}

func (m Model) helpItems() []helpItem {
	items := []helpItem{{key: "Esc", desc: "关闭帮助"}}

	switch {
	case m.Dialog == DialogImage:
		items = append(items,
			helpItem{key: "←→", desc: "切换图片"},
			helpItem{key: "Esc", desc: "关闭图片"},
		)
	case m.Page == PageDashboard:
		items = append(items,
			helpItem{key: "e", desc: "进入浏览"},
			helpItem{key: "n", desc: "打开通知"},
			helpItem{key: "c", desc: "打开配置"},
		)
	case m.Page == PageHome:
		items = append(items,
			helpItem{key: "Tab", desc: "切换页面"},
			helpItem{key: "1-4", desc: "选择模式"},
			helpItem{key: "←→", desc: "切换按钮"},
			helpItem{key: "Enter", desc: "启动/停止"},
			helpItem{key: "c", desc: "打开配置"},
			helpItem{key: "l", desc: "查看日志"},
			helpItem{key: "b", desc: "查看通知"},
		)
		if m.Home.CrawlerState != CrawlerRunning {
			items = append(items, helpItem{key: "m", desc: "切换模式"})
		}
	case m.Page == PageSchedule || m.Page == PageScores:
		items = append(items,
			helpItem{key: "Tab", desc: "切换页面"},
			helpItem{key: "r", desc: "刷新"},
			helpItem{key: "c", desc: "打开配置"},
			helpItem{key: "l", desc: "查看日志"},
			helpItem{key: "b", desc: "查看通知"},
		)
		if m.Page == PageScores {
			items = append(items,
				helpItem{key: "↑↓", desc: "滚动成绩"},
				helpItem{key: "PgUp/PgDn", desc: "成绩翻页"},
			)
		}
	case m.Posts.Searching:
		items = append(items,
			helpItem{key: "Enter", desc: "开始搜索"},
			helpItem{key: "←→", desc: "移动光标"},
			helpItem{key: "Backspace", desc: "删除字符"},
		)
	case m.Posts.ShowPostDetail:
		items = append(items,
			helpItem{key: "Tab", desc: "切换正文/评论"},
			helpItem{key: "↑↓", desc: "滚动当前区域"},
			helpItem{key: "PgUp/PgDn", desc: "快速翻页"},
			helpItem{key: "o", desc: "打开图片"},
			helpItem{key: "s", desc: "切换排序"},
			helpItem{key: "r", desc: "刷新详情"},
		)
		if m.Posts.CanWrite {
			items = append(items,
				helpItem{key: "p", desc: "切换点赞"},
				helpItem{key: "f", desc: "切换关注"},
				helpItem{key: "c", desc: "发表评论"},
				helpItem{key: "q", desc: "引用评论"},
			)
		}
	default:
		items = append(items,
			helpItem{key: "Tab", desc: "切换页面"},
			helpItem{key: "↑↓", desc: "选择帖子"},
			helpItem{key: "PgUp/PgDn", desc: "快速翻页"},
			helpItem{key: "Enter", desc: "打开详情"},
			helpItem{key: "o", desc: "打开图片"},
			helpItem{key: "/", desc: "搜索帖子"},
			helpItem{key: "r", desc: "刷新列表"},
			helpItem{key: "c", desc: "打开配置"},
			helpItem{key: "l", desc: "查看日志"},
			helpItem{key: "b", desc: "查看通知"},
		)
		if m.Session.Mode == SessionModeOnline {
			items = append(items, helpItem{key: "t", desc: "标签筛选"})
		}
		if m.Posts.CanWrite {
			items = append(items,
				helpItem{key: "n", desc: "发布帖子"},
				helpItem{key: "p", desc: "切换点赞"},
				helpItem{key: "f", desc: "切换关注"},
			)
		}
	}

	return items
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clipToVisibleWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if rw < 1 {
			rw = 1
		}
		if used+rw > width {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

func (m Model) renderDialogCard(content string) string {
	width := minInt(70, maxInt(40, m.Width-8))
	return dialogCard.Width(width).Render(content)
}
