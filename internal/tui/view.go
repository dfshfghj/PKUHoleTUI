package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/lipgloss"
)

var shanghaiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return loc
}()

func (m Model) View() string {
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
	if m.Images != nil && m.Images.Enabled() {
		m.Images.SetFrame(placements)
	}
	if m.Capture != nil {
		m.Capture.RecordFrame(rendered)
	}

	return rendered
}

func (m Model) renderScreen(width, height int) (string, []imagePlacement) {
	if m.Dialog == DialogHelp {
		return m.renderHelpScreen(width, height)
	}

	body, placements := m.renderMainLayout(width, height)
	if m.Dialog != DialogNone {
		dialog := m.renderDialog()
		body = lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) renderMainLayout(width, height int) (string, []imagePlacement) {
	m.Width = width
	m.Height = height

	tabs := []string{"同步", "帖子"}
	var segments []powerlineSegment
	for i, t := range tabs {
		if i == m.TabCursor {
			segments = append(segments, powerlineSegment{Text: t, Style: tabItemActiveStyle})
		} else {
			segments = append(segments, powerlineSegment{Text: t, Style: tabItemStyle})
		}
	}
	tabBar := m.renderTabBar(width, m.renderPowerlineGroup(segments, powerlineRight))
	footer := m.renderStatusLine(width)

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

	body := lipgloss.JoinVertical(lipgloss.Left, tabBar, contentBlock, footer)
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

	baseLayer := lipgloss2.NewLayer(main)
	panelX := width - panelWidth
	if panelX < 0 {
		panelX = 0
	}
	panelY := availableHeight - cardHeight
	if panelY < 0 {
		panelY = 0
	}
	panelLayer := lipgloss2.NewLayer(panel).X(panelX).Y(panelY).Z(1)
	body := lipgloss2.NewCompositor(baseLayer, panelLayer).Render()
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, body), placements
}

func (m Model) contentAreaHeightForSize(width, height int) int {
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

func powerlineSeparatorRight(left, right lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().
		Foreground(left).
		Background(right).
		Render("")
}

func powerlineSeparatorLeft(left, right lipgloss.TerminalColor) string {
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
	case DialogConfig:
		return "CONFIG"
	case DialogLogs:
		return "LOGS"
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

	if m.Page == PageHome {
		if m.Home.CrawlerState == CrawlerRunning {
			return "SYNC"
		}
		return "HOME"
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
	case PageHome:
		if m.Home.CrawlMode == CrawlMonitor {
			return fmt.Sprintf("监控前%d页", m.Home.MonitorPages)
		}
		return "顺序抓取"
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
	default:
		return "TreeHole TUI"
	}
}

func (m Model) currentStatusSummary() string {
	if m.LastError != "" {
		return "错误: " + m.LastError
	}
	if m.Posts.StatusText != "" {
		return m.Posts.StatusText
	}
	if m.Dialog != DialogNone {
		return m.dialogStatusSummary()
	}
	if m.Page == PageHome {
		return m.homeStatusSummary()
	}
	return m.postsStatusSummary()
}

func (m Model) dialogStatusSummary() string {
	switch m.Dialog {
	case DialogConfig:
		return "c: 配置编辑 | Enter: 保存 | Esc: 关闭"
	case DialogLogs:
		return "l: 运行日志 | Esc: 关闭"
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
	case PageHome:
		return m.renderHome(contentHeight), nil
	case PagePosts:
		return m.renderPosts(contentHeight)
	default:
		return "Unknown page", nil
	}
}

func (m Model) renderDialog() string {
	switch m.Dialog {
	case DialogConfig:
		return m.renderConfigDialog()
	case DialogLogs:
		return m.renderLogsDialog()
	case DialogHelp:
		return m.renderHelpDialog()
	case DialogSessionPrompt:
		return m.renderDialogCard(m.SessionDialog.View(m.Width))
	case DialogAuthChallenge:
		return m.renderDialogCard(m.AuthDialog.View(m.Width))
	case DialogComposer:
		return m.renderDialogCard(m.Composer.View(m.Width))
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

func (m Model) renderConfigDialog() string {
	return m.renderDialogCard(m.ConfigDialog.View(m.Width, m.Height))
}

func (m Model) renderLogsDialog() string {
	return m.renderDialogCard(m.LogsDialog.View(m.Width))
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
	return "帖子列表"
}

func (m Model) helpItems() []helpItem {
	items := []helpItem{{key: "Esc", desc: "关闭帮助"}}

	switch {
	case m.Page == PageHome:
		items = append(items,
			helpItem{key: "Tab", desc: "切到帖子页"},
			helpItem{key: "←→", desc: "切换按钮"},
			helpItem{key: "Enter", desc: "启动/停止"},
			helpItem{key: "c", desc: "打开配置"},
			helpItem{key: "l", desc: "查看日志"},
		)
		if m.Home.CrawlerState != CrawlerRunning {
			items = append(items, helpItem{key: "m", desc: "切换模式"})
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
			helpItem{key: "Tab", desc: "切到同步页"},
			helpItem{key: "↑↓", desc: "选择帖子"},
			helpItem{key: "PgUp/PgDn", desc: "快速翻页"},
			helpItem{key: "Enter", desc: "打开详情"},
			helpItem{key: "/", desc: "搜索帖子"},
			helpItem{key: "r", desc: "刷新列表"},
			helpItem{key: "c", desc: "打开配置"},
			helpItem{key: "l", desc: "查看日志"},
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
