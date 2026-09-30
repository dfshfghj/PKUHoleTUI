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
	ToolsSectionHelp
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
	case ToolsSectionHelp:
		b.WriteString(renderToolsHelp(width, bodyHeight))
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
	if height < 1 {
		height = 1
	}
	fill := dialogBackgroundFillStyle()
	body = fillRenderedBackground(body, width, fill)
	body = lipgloss.Place(
		width,
		height,
		lipgloss.Left,
		lipgloss.Top,
		body,
		lipgloss.WithWhitespaceStyle(fill),
	)
	return preserveBackgroundAfterReset(body, colorBg)
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
		{"配置 (C)", ToolsSectionConfig},
		{"日志 (L)", ToolsSectionLogs},
		{"互动 (I)", ToolsSectionInteractive},
		{"系统 (S)", ToolsSectionSystem},
		{"帮助 (?)", ToolsSectionHelp},
	}
	parts := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		style := vStatLabelStyle
		if m.section == tab.section {
			style = vStatValueStyle
		}
		parts = append(parts, style.Background(colorBg).Render(tab.label))
	}
	return strings.Join(parts, fill.Render("  "))
}

func renderToolsHelp(width, height int) string {
	var lines []string
	add := func(line string) {
		lines = append(lines, clipToVisibleWidth(line, width))
	}
	addHeading := func(line string) {
		lines = append(lines, vStatValueStyle.Render(clipToVisibleWidth(line, width)))
	}

	addHeading("项目用法")
	add("")
	add("启动后先进入 Dashboard，可查看未读通知、热榜和常用入口。")
	add("按 e 进入浏览；按 c 编辑配置；按 n 打开互动通知；按 ? 打开本帮助。")
	add("进入帖子列表后，可搜索、打开详情、查看图片、发布帖子、点赞或关注。")
	add("")
	addHeading("全局快捷键")
	add("Tab: 切换主页面")
	add("?: 当前页面快捷键")
	add("space+?: 项目帮助")
	add("space+c: 配置")
	add("space+l: 日志")
	add("space+b: 通知")
	add("q: 退出")
	add("")
	addHeading("帖子快捷键")
	add("↑↓: 选择/滚动")
	add("PgUp/PgDn: 快速翻页")
	add("/: 搜索")
	add("Enter: 打开详情")
	add("o: 打开图片")
	add("r: 刷新")
	add("n: 发布帖子")
	add("p: 点赞")
	add("f: 关注")
	add("t: 标签筛选")
	add("")
	addHeading("鼠标")
	add("单击: 选中 / 聚焦；双击帖子: 打开详情")
	add("单击 ♡ / ☆: 点赞 / 关注")
	add("滚轮: 移动光标（页面跟随）；拖动右侧滚动条: 拖动浏览")
	add("M 或 :mouse: 关闭/开启鼠标上报")
	add("关闭后恢复终端原生选中；开启时可用 Shift+拖选")

	body := strings.Join(lines, "\n")
	return renderToolsBodyWithFooter(body, "?: 帮助 | Esc: 关闭", width, height)
}
