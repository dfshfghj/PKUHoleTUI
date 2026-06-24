package tui

import (
	"strings"

	"treehole/internal/config"

	"charm.land/lipgloss/v2"
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
	rendered := lipgloss.NewStyle().
		Background(colorBg).
		Render(b.String())
	return preserveBackgroundAfterReset(rendered, colorBg)
}

func renderToolsBodyWithFooter(body, footer string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 2 {
		height = 2
	}
	fill := dialogBackgroundFillStyle()
	bodyHeight := height - 1
	body = fillRenderedBackground(body, width, fill)
	body = lipgloss.Place(
		width,
		bodyHeight,
		lipgloss.Left,
		lipgloss.Top,
		body,
		lipgloss.WithWhitespaceStyle(fill),
	)
	footer = clipToVisibleWidth(footer, width)
	footer = vDialogHelpStyle.
		Padding(0).
		Width(width).
		Render(footer)
	footer = fillRenderedBackground(footer, width, fill)
	return preserveBackgroundAfterReset(lipgloss.JoinVertical(lipgloss.Left, body, footer), colorBg)
}

func dialogBackgroundFillStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(colorBg)
}

func (m ToolsDialogModel) renderTabs() string {
	fill := dialogBackgroundFillStyle()
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
		parts = append(parts, style.Background(colorBg).Render(tab.label))
	}
	return parts[0] +
		fill.Render("  ") +
		parts[1] +
		fill.Render("  ") +
		parts[2] +
		fill.Render("  ") +
		parts[3]
}
