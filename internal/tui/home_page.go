package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HomePageModel struct {
	LoggedIn     bool
	LoginUser    string
	CrawlerState CrawlerState
	CrawlerStart time.Time
	CrawlMode    CrawlMode
	MonitorPages int

	LastCrawlPage int
	LastCrawlTime time.Duration

	HomeButtonIdx int
	HomeLastError string
}

type HomeAction int

const (
	HomeActionNone HomeAction = iota
	HomeActionStartCrawler
	HomeActionStopCrawler
)

func NewHomePageModel() HomePageModel {
	return HomePageModel{
		CrawlerState:  CrawlerStopped,
		CrawlMode:     CrawlSequential,
		MonitorPages:  3,
		HomeButtonIdx: int(HomeFocusStart),
	}
}

func (h HomePageModel) View(width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 1
	}

	var b strings.Builder

	b.WriteString(vTitleStyle.Render("TreeHole TUI"))
	b.WriteString("\n")

	if h.LoggedIn {
		b.WriteString(compactCardStyle(width).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				vStatLabelStyle.Render("登录状态: "),
				vStatusRunningStyle.Render("已登录"),
				vStatLabelStyle.Render("  用户: "),
				vStatValueStyle.Render(h.LoginUser),
			),
		))
	} else {
		b.WriteString(compactCardStyle(width).Render(vStatusStoppedStyle.Render("未登录")))
	}

	var crawlerStatus string
	switch h.CrawlerState {
	case CrawlerRunning:
		crawlerStatus = vStatusRunningStyle.Render("运行中")
	case CrawlerStopped:
		crawlerStatus = vStatusStoppedStyle.Render("已停止")
	case CrawlerError:
		crawlerStatus = vStatusStoppedStyle.Render("错误")
	}

	elapsed := "0s"
	if h.CrawlerState == CrawlerRunning && !h.CrawlerStart.IsZero() {
		elapsed = time.Since(h.CrawlerStart).Round(time.Second).String()
	}

	b.WriteString("\n")
	b.WriteString(compactCardStyle(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			vStatLabelStyle.Render("爬虫状态: "),
			crawlerStatus,
			vStatLabelStyle.Render("  运行时长: "),
			vStatValueStyle.Render(elapsed),
		),
	))

	b.WriteString("\n")
	b.WriteString(compactCardStyle(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			vStatLabelStyle.Render("上次爬取: "),
			vStatValueStyle.Render(fmt.Sprintf("第%d页", h.LastCrawlPage)),
			vStatLabelStyle.Render("  耗时: "),
			vStatValueStyle.Render(h.LastCrawlTime.Round(time.Millisecond).String()),
		),
	))

	modeLabel := "模式: 顺序爬取"
	if h.CrawlMode == CrawlMonitor {
		modeLabel = fmt.Sprintf("模式: 监控模式(前%d页)", h.MonitorPages)
	}
	b.WriteString("\n")
	b.WriteString(compactCardStyle(width).Render(vStatValueStyle.Render(modeLabel)))

	b.WriteString("\n")
	buttons := []string{"启动爬虫", "停止爬虫", modeLabel}
	var btns []string
	for i, label := range buttons {
		disabled := (i == 0 && h.CrawlerState == CrawlerRunning) ||
			(i == 1 && h.CrawlerState != CrawlerRunning)
		if disabled {
			btns = append(btns, vButtonDisabled.Render(label))
		} else if h.HomeButtonIdx == i {
			if i == 1 {
				btns = append(btns, vButtonActiveDanger.Render(label))
			} else {
				btns = append(btns, vButtonActive.Render(label))
			}
		} else {
			btns = append(btns, vButtonDefault.Render(label))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, btns...))

	if h.HomeLastError != "" {
		b.WriteString("\n")
		b.WriteString(vErrorStyle.Render("错误: " + h.HomeLastError))
	}

	return lipgloss.Place(
		width,
		height,
		lipgloss.Left,
		lipgloss.Top,
		b.String(),
	)
}

func compactCardStyle(width int) lipgloss.Style {
	contentWidth := width - 6
	if contentWidth < 10 {
		contentWidth = 10
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(contentWidth)
}

func (h *HomePageModel) Update(msg tea.KeyMsg) HomeAction {
	switch msg.String() {
	case "m":
		if h.CrawlerState == CrawlerStopped {
			if h.CrawlMode == CrawlSequential {
				h.CrawlMode = CrawlMonitor
			} else {
				h.CrawlMode = CrawlSequential
			}
		}
	case "left":
		if h.HomeButtonIdx > 0 {
			h.HomeButtonIdx--
		}
	case "right":
		if h.HomeButtonIdx < int(HomeFocusMode) {
			h.HomeButtonIdx++
		}
	case "enter":
		if h.HomeButtonIdx == int(HomeFocusStart) && h.CrawlerState == CrawlerStopped {
			h.CrawlerState = CrawlerRunning
			h.CrawlerStart = time.Now()
			h.HomeLastError = ""
			return HomeActionStartCrawler
		}
		if h.HomeButtonIdx == int(HomeFocusStop) && h.CrawlerState == CrawlerRunning {
			h.CrawlerState = CrawlerStopped
			return HomeActionStopCrawler
		}
		if h.HomeButtonIdx == int(HomeFocusMode) {
			if h.CrawlMode == CrawlSequential {
				h.CrawlMode = CrawlMonitor
			} else {
				h.CrawlMode = CrawlSequential
			}
		}
	}
	return HomeActionNone
}
