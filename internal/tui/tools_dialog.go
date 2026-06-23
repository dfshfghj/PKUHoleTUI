package tui

import (
	"strings"

	"treehole/internal/config"

	"github.com/charmbracelet/lipgloss"
)

type ToolsSection int

const (
	ToolsSectionConfig ToolsSection = iota
	ToolsSectionLogs
	ToolsSectionInteractive
	ToolsSectionSystem
)

type ToolsDialogModel struct {
	section       ToolsSection
	Config        ConfigDialogModel
	Logs          LogsDialogModel
	Notifications NotificationDialogModel
}

func NewToolsDialog(cfg *config.Config) ToolsDialogModel {
	return ToolsDialogModel{
		section:       ToolsSectionConfig,
		Config:        NewConfigDialog(cfg),
		Logs:          NewLogsDialog(),
		Notifications: NewNotificationDialog(),
	}
}

func (m ToolsDialogModel) initialized() bool {
	return m.Config.initialized() && m.Logs.initialized() && m.Notifications.initialized()
}

func (m ToolsDialogModel) Section() ToolsSection {
	return m.section
}

func (m *ToolsDialogModel) Switch(section ToolsSection) {
	m.section = section
}

func (m *ToolsDialogModel) View(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	bodyHeight := maxInt(3, height-2)
	switch m.section {
	case ToolsSectionLogs:
		b.WriteString(m.Logs.View(width, bodyHeight))
	case ToolsSectionInteractive, ToolsSectionSystem:
		b.WriteString(m.Notifications.View(width, bodyHeight))
	default:
		b.WriteString(m.Config.View(width, bodyHeight))
	}
	return lipgloss.NewStyle().
		Background(colorBg).
		ColorWhitespace(true).
		Render(b.String())
}

func renderToolsBodyWithFooter(body, footer string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 2 {
		height = 2
	}
	bodyHeight := height - 1
	body = lipgloss.Place(
		width,
		bodyHeight,
		lipgloss.Left,
		lipgloss.Top,
		body,
	)
	footer = clipToVisibleWidth(footer, width)
	footer = vDialogHelpStyle.
		Padding(0).
		Width(width).
		Render(footer)
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m ToolsDialogModel) renderTabs() string {
	tabs := []struct {
		label   string
		section ToolsSection
	}{
		{"配置 (1)", ToolsSectionConfig},
		{"日志 (2)", ToolsSectionLogs},
		{"互动 (3)", ToolsSectionInteractive},
		{"系统 (4)", ToolsSectionSystem},
	}
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		style := vStatLabelStyle
		if m.section == tab.section {
			style = vStatValueStyle
		}
		parts = append(parts, style.Render(tab.label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts[0], "  ", parts[1], "  ", parts[2], "  ", parts[3])
}
