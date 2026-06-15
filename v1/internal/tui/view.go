package tui

import (
	"fmt"
	"strings"
	"time"

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

	// Tab bar - fixed top
	tabs := []string{"同步", "帖子"}
	var segments []powerlineSegment
	for i, t := range tabs {
		if i == m.TabCursor {
			segments = append(segments, powerlineSegment{Text: t, Style: tabItemActiveStyle})
		} else {
			segments = append(segments, powerlineSegment{Text: t, Style: tabItemStyle})
		}
	}
	tabBar := m.renderTabBar(w, m.renderPowerlineGroup(segments, powerlineRight))

	// Footer - fixed bottom
	footer := m.renderStatusLine(w)

	contentHeight := m.contentAreaHeightForSize(w, h)
	content, placements := m.renderContent(contentHeight)
	for i := range placements {
		placements[i].top += lipgloss.Height(tabBar)
	}
	contentBlock := lipgloss.Place(
		w,
		contentHeight,
		lipgloss.Left,
		lipgloss.Top,
		content,
	)

	body := lipgloss.JoinVertical(lipgloss.Left, tabBar, contentBlock, footer)

	// Dialog overlay
	if m.Dialog != DialogNone {
		dialog := m.renderDialog()
		body = lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, dialog)
	}

	rendered := lipgloss.Place(
		w,
		h,
		lipgloss.Left,
		lipgloss.Top,
		body,
	)
	rendered = baseStyle.Render(rendered)
	if m.Images != nil && m.Images.Enabled() {
		m.Images.SetFrame(placements)
	}
	if m.Capture != nil {
		m.Capture.RecordFrame(rendered)
	}

	return rendered
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
			return fmt.Sprintf("同步 监控前%d页", m.Home.MonitorPages)
		}
		return "同步 顺序抓取"
	case PagePosts:
		if m.Posts.ShowPostDetail && m.Posts.CurrentPost != nil {
			return fmt.Sprintf("帖子 #%d", m.Posts.CurrentPost.Pid)
		}
		if m.Posts.SearchActive {
			query := strings.TrimSpace(m.Posts.SearchInput)
			if query == "" {
				return "帖子 搜索结果"
			}
			return fmt.Sprintf("搜索 %s", query)
		}
		return "帖子 列表"
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
		return "h: 快捷键帮助 | Esc: 关闭"
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
		return fmt.Sprintf("%s | 已运行 %s | Enter: 启停 | m: 切换模式", progress, elapsed)
	case CrawlerError:
		if m.Home.HomeLastError != "" {
			return "爬虫错误: " + m.Home.HomeLastError
		}
		return "爬虫错误，按 Enter 或 m 调整后重试"
	default:
		return "爬虫已停止 | Enter: 启动/停止 | m: 切换模式"
	}
}

func (m Model) postsStatusSummary() string {
	if m.Posts.Searching {
		return "输入关键字后按 Enter 搜索，Esc 取消"
	}
	if m.Posts.ShowPostDetail && m.Posts.CurrentPost != nil {
		comments := fmt.Sprintf("评论 %d", len(m.Posts.CommentList))
		if m.Posts.CommentListLoading {
			comments += " | 加载评论中..."
		} else if m.Posts.CommentListHasMore {
			comments += " | 可继续下翻加载更多"
		}
		focus := "焦点: 评论"
		if m.Posts.DetailFocus == DetailFocusPost {
			focus = "焦点: 正文"
		}
		return fmt.Sprintf("%s | %s | Tab: 切换正文/评论 | q: 引用评论", comments, focus)
	}

	scope := fmt.Sprintf("%d", len(m.Posts.PostList))
	if m.Posts.SearchActive {
		scope = fmt.Sprintf("%d", len(m.Posts.PostList))
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
	return scope + " | /: 搜索 | Enter: 详情 | r: 刷新 | h: 帮助"
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
	var b strings.Builder

	b.WriteString(vDialogTitleStyle.Render("快捷键帮助"))
	b.WriteString("\n\n")

	helpItems := []struct {
		key  string
		desc string
	}{
		{"h", "打开/关闭此帮助菜单"},
		{"c", "打开配置管理对话框"},
		{"l", "打开运行日志查看器"},
		{"Tab", "在首页和帖子列表之间切换"},
		{"Ctrl+Q", "退出程序"},
		{"", ""},
		{"m", "切换爬取模式（顺序/监控）"},
		{"←→", "选择启动/停止爬虫按钮"},
		{"Enter", "执行选中的操作"},
		{"", ""},
		{"/", "搜索帖子（#pid / :follow）"},
		{"r", "刷新帖子列表 / 详情"},
		{"t", "打开标签筛选（在线模式）"},
		{"n", "发帖（可写在线模式）"},
		{"Enter", "查看帖子详情"},
		{"↑↓", "选择帖子 / 滚动评论"},
		{"PgUp/PgDn", "快速滚动"},
		{"", ""},
		{"详情页 p", "点赞 / 取消点赞"},
		{"详情页 f", "关注 / 取消关注"},
		{"详情页 c", "发评论"},
		{"详情页 q", "引用当前选中评论"},
		{"详情页 s", "评论正序/逆序切换"},
		{"模式提示", "在线失败时会提示重新登录，短信/令牌验证会弹出输入框"},
	}

	for _, item := range helpItems {
		if item.key == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			vStatValueStyle.Width(12).Render(item.key),
			vStatLabelStyle.Render(item.desc),
		))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(vDialogHelpStyle.Render("Esc: 关闭"))

	return m.renderDialogCard(b.String())
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

func (m Model) renderDialogCard(content string) string {
	width := minInt(70, maxInt(40, m.Width-8))
	return dialogCard.Width(width).Render(content)
}
