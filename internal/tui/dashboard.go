package tui

import (
	"fmt"
	"strings"
	"time"

	"treehole/internal/models"

	"charm.land/lipgloss/v2"
)

const dashboardLogo = `
████████╗██████╗ ███████╗███████╗██╗  ██╗ ██████╗ ██╗     ███████╗
╚══██╔══╝██╔══██╗██╔════╝██╔════╝██║  ██║██╔═══██╗██║     ██╔════╝
   ██║   ██████╔╝█████╗  █████╗  ███████║██║   ██║██║     █████╗
   ██║   ██╔══██╗██╔══╝  ██╔══╝  ██╔══██║██║   ██║██║     ██╔══╝
   ██║   ██║  ██║███████╗███████╗██║  ██║╚██████╔╝███████╗███████╗
   ╚═╝   ╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚══════╝`

type DashboardModel struct {
	Notifications []models.Notification
	Loading       bool
	Error         string
	HotPosts      []DashboardHotPost
	HotLoading    bool
	HotError      string
}

type DashboardHotPost struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	FollowNum int    `json:"follownum"`
}

func NewDashboardModel() DashboardModel {
	return DashboardModel{Loading: true, HotLoading: true}
}

func (m *DashboardModel) SetNotifications(items []models.Notification, err error) {
	m.Loading = false
	if err != nil {
		m.Error = err.Error()
		return
	}
	m.Error = ""
	m.Notifications = m.Notifications[:0]
	for _, item := range items {
		if !item.Read {
			m.Notifications = append(m.Notifications, item)
		}
	}
}

func (m *DashboardModel) SetHotPosts(items []DashboardHotPost, err error) {
	m.HotLoading = false
	if err != nil {
		m.HotError = err.Error()
		return
	}
	m.HotError = ""
	m.HotPosts = append(m.HotPosts[:0], items...)
}

func (m *DashboardModel) MarkRead(id int) {
	filtered := m.Notifications[:0]
	for _, item := range m.Notifications {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	m.Notifications = filtered
}

func (m *DashboardModel) MarkAllRead(messageType models.NotificationType) {
	filtered := m.Notifications[:0]
	for _, item := range m.Notifications {
		if item.Type != messageType {
			filtered = append(filtered, item)
		}
	}
	m.Notifications = filtered
}

func (m DashboardModel) View(width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 1
	}

	contentWidth := clampInt(width-2, 44, 96)
	logo := equalizeLogoLineWidths(dashboardLogo)
	logoWidth := lipgloss.Width(logo)
	if logoWidth > contentWidth {
		logo = "T R E E H O L E"
		logoWidth = lipgloss.Width(logo)
	}

	blockWidth := dashboardBlockWidth(m, logoWidth, contentWidth)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		Align(lipgloss.Left).
		Width(blockWidth).
		Render(logo))
	b.WriteString("\n\n")
	b.WriteString(m.renderNotifications(blockWidth))
	b.WriteString("\n\n")
	b.WriteString(m.renderHotPosts(blockWidth))
	b.WriteString("\n\n")
	b.WriteString(renderDashboardAction("󰈔", "Explore", "e", blockWidth))
	b.WriteString("\n")
	b.WriteString(renderDashboardAction("", "Config", "c", blockWidth))

	block := b.String()
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

func dashboardBlockWidth(m DashboardModel, logoWidth, maxWidth int) int {
	w := logoWidth
	if w < 1 {
		w = 1
	}
	type actionItem struct {
		icon, label, key string
	}
	actions := []actionItem{
		{"󰈔", "Explore", "e"},
		{"", "Config", "c"},
	}
	for _, a := range actions {
		if lw := lipgloss.Width(renderDashboardAction(a.icon, a.label, a.key, w)); lw > w {
			w = lw
		}
	}
	for _, line := range strings.Split(m.renderNotificationsBody(), "\n") {
		if lw := lipgloss.Width(line); lw > w {
			w = lw
		}
	}
	for _, line := range strings.Split(m.renderHotPostsBody(w), "\n") {
		if lw := lipgloss.Width(line); lw > w {
			w = lw
		}
	}
	if w > maxWidth {
		w = maxWidth
	}
	if w < 1 {
		w = 1
	}
	return w
}

func padRowToWidth(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).Render(s)
}

func (m DashboardModel) renderNotifications(width int) string {
	var b strings.Builder
	b.WriteString(renderDashboardAction("", "Notifications", "n", width))
	b.WriteString("\n")
	lines := strings.Split(m.renderNotificationsBody(), "\n")
	for i, line := range lines {
		b.WriteString(padRowToWidth(line, width))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m DashboardModel) renderHotPosts(width int) string {
	var b strings.Builder
	b.WriteString(renderDashboardAction("󰓎", "热榜", "", width))
	b.WriteString("\n")
	lines := strings.Split(m.renderHotPostsBody(width), "\n")
	for i, line := range lines {
		b.WriteString(padRowToWidth(line, width))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m DashboardModel) renderHotPostsBody(width int) string {
	var b strings.Builder
	switch {
	case m.HotLoading:
		b.WriteString(vLoadingStyle.Render("loading..."))
	case m.HotError != "":
		b.WriteString(vStatLabelStyle.Render("hot posts unavailable"))
	case len(m.HotPosts) == 0:
		b.WriteString(vStatLabelStyle.Render("no hot posts"))
	default:
		limit := minInt(5, len(m.HotPosts))
		for i := 0; i < limit; i++ {
			b.WriteString(vStatValueStyle.Render("●"))
			b.WriteString("  ")
			b.WriteString(vStatLabelStyle.Render(dashboardHotPostLine(m.HotPosts[i], maxInt(1, width-3))))
			if i != limit-1 {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func (m DashboardModel) renderNotificationsBody() string {
	var b strings.Builder
	switch {
	case m.Loading:
		b.WriteString(vLoadingStyle.Render("loading..."))
	case m.Error != "":
		b.WriteString(vStatLabelStyle.Render("notifications unavailable"))
	case len(m.Notifications) == 0:
		b.WriteString(vStatLabelStyle.Render("no unread notifications"))
	default:
		limit := minInt(5, len(m.Notifications))
		for i := 0; i < limit; i++ {
			item := m.Notifications[i]
			line := dashboardNotificationLine(item)
			b.WriteString(vStatValueStyle.Render("●"))
			b.WriteString("  ")
			b.WriteString(vStatLabelStyle.Render(line))
			if i != limit-1 {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func dashboardHotPostLine(item DashboardHotPost, width int) string {
	if width < 1 {
		width = 1
	}
	likes := fmt.Sprintf("★ %d", item.FollowNum)
	left := strings.TrimSpace(fmt.Sprintf("#%d %s", item.ID, normalizeDashboardHotPostText(item.Text)))
	if left == "" {
		left = fmt.Sprintf("#%d", item.ID)
	}
	likesWidth := lipgloss.Width(likes)
	if width <= likesWidth+1 {
		return truncateVisibleLine(likes, width, "...")
	}
	leftWidth := width - likesWidth - 2
	left = truncateVisibleLine(left, leftWidth, "...")
	gap := maxInt(2, width-lipgloss.Width(left)-likesWidth)
	return left + strings.Repeat(" ", gap) + likes
}

func normalizeDashboardHotPostText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.Join(strings.Fields(text), " ")
}

func dashboardNotificationLine(item models.Notification) string {
	kind := "互动"
	if item.Type == models.NotificationTypeSystem {
		kind = "系统"
	}
	when := item.CreatedAt
	if when == "" && item.Timestamp > 0 {
		when = time.Unix(item.Timestamp, 0).In(shanghaiLocation).Format("02/Jan 15:04")
	}
	text := strings.TrimSpace(item.Title)
	if text == "" {
		text = strings.TrimSpace(item.Content)
	}
	pid := ""
	if item.PID > 0 {
		pid = fmt.Sprintf(" #%d", item.PID)
	}
	return strings.TrimSpace(fmt.Sprintf("%s  %s%s  %s", when, kind, pid, text))
}

func equalizeLogoLineWidths(logo string) string {
	lines := strings.Split(strings.TrimRight(logo, "\n"), "\n")
	if len(lines) == 0 {
		return logo
	}
	maxW := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	for i, line := range lines {
		if pad := maxW - lipgloss.Width(line); pad > 0 {
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	return strings.Join(lines, "\n")
}

func renderDashboardAction(icon, label, key string, width int) string {
	left := icon + "  " + label
	gap := maxInt(2, width-lipgloss.Width(left)-lipgloss.Width(key))
	return vStatValueStyle.Render(left) +
		strings.Repeat(" ", gap) +
		vStatLabelStyle.Render(key)
}
