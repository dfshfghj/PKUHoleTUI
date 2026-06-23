package tui

import (
	"fmt"
	"strings"
	"time"

	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NotificationDialogModel struct {
	messageType models.NotificationType
	items       []models.Notification
	selected    int
	total       int
	loading     bool
	action      bool
	lastErr     string
}

func NewNotificationDialog() NotificationDialogModel {
	return NotificationDialogModel{messageType: models.NotificationTypeInteractive}
}

func (m NotificationDialogModel) initialized() bool {
	return m.messageType != ""
}

func (m *NotificationDialogModel) MessageType() models.NotificationType {
	return m.messageType
}

func (m *NotificationDialogModel) SetMessageType(messageType models.NotificationType) {
	if m.messageType == messageType {
		return
	}
	m.messageType = messageType
	m.selected = 0
	m.loading = true
	m.lastErr = ""
}

func (m *NotificationDialogModel) SetLoading(loading bool) {
	m.loading = loading
	if loading {
		m.lastErr = ""
	}
}

func (m *NotificationDialogModel) Loading() bool {
	return m.loading
}

func (m *NotificationDialogModel) SetNotifications(messageType models.NotificationType, items []models.Notification, total int) {
	m.messageType = messageType
	m.items = items
	m.total = total
	m.loading = false
	m.action = false
	m.lastErr = ""
	if m.selected >= len(items) {
		m.selected = maxInt(0, len(items)-1)
	}
}

func (m *NotificationDialogModel) SetError(err error) {
	m.loading = false
	m.action = false
	if err == nil {
		m.lastErr = ""
		return
	}
	m.lastErr = err.Error()
}

func (m *NotificationDialogModel) SetAction(action bool) {
	m.action = action
	if action {
		m.lastErr = ""
	}
}

func (m *NotificationDialogModel) Selected() *models.Notification {
	if m.selected < 0 || m.selected >= len(m.items) {
		return nil
	}
	return &m.items[m.selected]
}

func (m *NotificationDialogModel) CanMarkSelectedRead() bool {
	selected := m.Selected()
	return m.messageType == models.NotificationTypeInteractive && selected != nil && !selected.Read
}

func (m *NotificationDialogModel) MarkRead(id int) {
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Read = true
			break
		}
	}
	m.action = false
}

func (m *NotificationDialogModel) MarkAllRead() {
	for i := range m.items {
		m.items[i].Read = true
	}
	m.action = false
}

// Update returns true when the active message type changed and must be reloaded.
func (m *NotificationDialogModel) Update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up":
		if m.selected > 0 {
			m.selected--
		}
	case "down":
		if m.selected < len(m.items)-1 {
			m.selected++
		}
	case "pgup":
		m.selected = maxInt(0, m.selected-10)
	case "pgdown":
		m.selected = minInt(maxInt(0, len(m.items)-1), m.selected+10)
	}
	return false
}

func (m NotificationDialogModel) View(width, height int) string {
	var b strings.Builder
	innerWidth := maxInt(24, width-panelContentStyle.GetHorizontalFrameSize())

	switch {
	case m.loading:
		b.WriteString(vLoadingStyle.Render("加载通知中..."))
	case len(m.items) == 0:
		b.WriteString(vEmptyStyle.Render("暂无通知"))
	default:
		renderedItems := make([]string, len(m.items))
		for i := range m.items {
			renderedItems[i] = m.renderItem(m.items[i], i == m.selected, innerWidth)
		}
		availableHeight := maxInt(1, height-3)
		start, end := notificationVisibleRange(m.selected, renderedItems, availableHeight)
		for i := start; i < end; i++ {
			b.WriteString(renderedItems[i])
			if i != end-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(vPaginationStyle.Render(fmt.Sprintf("共 %d 条 | 当前 %d/%d", m.total, m.selected+1, len(m.items))))
	}

	if m.action {
		b.WriteString("\n")
		b.WriteString(vLoadingStyle.Render("更新已读状态中..."))
	}
	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(vErrorStyle.Render("错误: " + m.lastErr))
	}

	help := "↑↓/PgUp/PgDn: 选择 | a: 全部已读 | r: 刷新 | Esc: 关闭"
	if m.messageType == models.NotificationTypeInteractive {
		help = "↑↓/PgUp/PgDn: 选择 | Enter: 当前已读 | a: 全部已读 | r: 刷新 | Esc: 关闭"
	}
	return renderToolsBodyWithFooter(b.String(), help, width, height)
}

func (m NotificationDialogModel) renderItem(item models.Notification, selected bool, width int) string {
	unreadMarker := " "
	if !item.Read {
		unreadMarker = vStatValueStyle.Render("●")
	}
	meta := unreadMarker
	if item.PID > 0 {
		meta += fmt.Sprintf("  #%d", item.PID)
	}
	if item.CreatedAt != "" {
		meta += "  " + item.CreatedAt
	} else if item.Timestamp > 0 {
		meta += "  " + time.Unix(item.Timestamp, 0).In(shanghaiLocation).Format("2006-01-02 15:04")
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = meta
	} else {
		title += "  " + meta
	}
	contentWidth := maxInt(12, width-4)
	content := strings.Join(wrapVisibleLine(strings.TrimSpace(item.Content), contentWidth), "\n")
	body := title + "\n" + content
	style := lipgloss.NewStyle().
		Width(maxInt(12, width-2)).
		Padding(0, 1).
		Background(colorBg).
		ColorWhitespace(true).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)
	if selected {
		style = style.BorderForeground(colorAccent)
	}
	return style.Render(body)
}

func notificationVisibleRange(selected int, items []string, height int) (int, int) {
	if len(items) == 0 {
		return 0, 0
	}
	selected = clampInt(selected, 0, len(items)-1)
	start, end := selected, selected+1
	used := lipgloss.Height(items[selected])

	for start > 0 || end < len(items) {
		added := false
		if end < len(items) {
			nextHeight := lipgloss.Height(items[end])
			if used+nextHeight <= height {
				used += nextHeight
				end++
				added = true
			}
		}
		if start > 0 {
			prevHeight := lipgloss.Height(items[start-1])
			if used+prevHeight <= height {
				used += prevHeight
				start--
				added = true
			}
		}
		if !added {
			break
		}
	}

	return start, end
}
