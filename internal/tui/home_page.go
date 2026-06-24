package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type HomePageModel struct {
	LoggedIn         bool
	LoginUser        string
	CrawlerState     CrawlerState
	CrawlerStart     time.Time
	CrawlMode        CrawlMode
	MonitorPages     int
	PostsPerRequest  int
	CommentsPerPost  int
	FetchImages      bool
	SaveJSON         bool
	ConvertWebp      bool
	ThumbnailStartID int
	ThumbnailEndID   int

	LastCrawlPage    int
	LastCrawlTime    time.Duration
	LastCrawlSummary string

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
		CrawlerState:     CrawlerStopped,
		CrawlMode:        CrawlSequential,
		MonitorPages:     3,
		PostsPerRequest:  200,
		CommentsPerPost:  200,
		ConvertWebp:      true,
		ThumbnailStartID: 30000,
		ThumbnailEndID:   31000,
		HomeButtonIdx:    int(HomeFocusStart),
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

	modeLabel := "模式: " + h.modeLabel()
	b.WriteString("\n")
	b.WriteString(compactCardStyle(width).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			vStatValueStyle.Render(modeLabel),
			vHelpStyle.Render(h.modeHelp()),
		),
	))

	b.WriteString("\n")
	b.WriteString(compactCardStyle(width).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top,
				vStatLabelStyle.Render("请求: "),
				vStatValueStyle.Render(fmt.Sprintf("posts=%d comments=%d", h.PostsPerRequest, h.CommentsPerPost)),
				vStatLabelStyle.Render("  图片: "),
				vStatValueStyle.Render(onOff(h.FetchImages)),
				vStatLabelStyle.Render("  JSON: "),
				vStatValueStyle.Render(onOff(h.SaveJSON)),
			),
			lipgloss.JoinHorizontal(lipgloss.Top,
				vStatLabelStyle.Render("补图/WebP: "),
				vStatValueStyle.Render(onOff(h.ConvertWebp)),
				vStatLabelStyle.Render("  缩略图: "),
				vStatValueStyle.Render(fmt.Sprintf("%d-%d", h.ThumbnailStartID, h.ThumbnailEndID)),
			),
		),
	))

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
	if h.LastCrawlSummary != "" {
		b.WriteString("\n")
		b.WriteString(vPaginationStyle.Render(h.LastCrawlSummary))
	}

	return lipgloss.Place(
		width,
		height,
		lipgloss.Left,
		lipgloss.Top,
		b.String(),
	)
}

func (h HomePageModel) modeLabel() string {
	switch h.CrawlMode {
	case CrawlMonitor:
		return fmt.Sprintf("监控模式(前%d页)", h.MonitorPages)
	case CrawlFetchImages:
		return "补齐数据库图片"
	case CrawlFetchThumbnails:
		return "下载缩略图"
	default:
		return "顺序爬取"
	}
}

func (h HomePageModel) modeHelp() string {
	return "1 顺序 | 2 监控 | 3 补图 | 4 缩略图 | m 循环 | +/- 调整上限 | [/] 调整起点"
}

func onOff(value bool) string {
	if value {
		return "开"
	}
	return "关"
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

func (h *HomePageModel) Update(msg tea.KeyPressMsg) HomeAction {
	switch msg.String() {
	case "m":
		if h.CrawlerState == CrawlerStopped {
			h.CrawlMode = CrawlMode((int(h.CrawlMode) + 1) % 4)
		}
	case "1":
		h.setMode(CrawlSequential)
	case "2":
		h.setMode(CrawlMonitor)
	case "3":
		h.setMode(CrawlFetchImages)
	case "4":
		h.setMode(CrawlFetchThumbnails)
	case "+", "=":
		h.adjustModeParam(1)
	case "-", "_":
		h.adjustModeParam(-1)
	case "[":
		h.adjustThumbnailStart(-100)
	case "]":
		h.adjustThumbnailStart(100)
	case "i":
		if h.CrawlerState == CrawlerStopped {
			h.FetchImages = !h.FetchImages
		}
	case "j":
		if h.CrawlerState == CrawlerStopped {
			h.SaveJSON = !h.SaveJSON
		}
	case "w":
		if h.CrawlerState == CrawlerStopped {
			h.ConvertWebp = !h.ConvertWebp
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
			h.CrawlMode = CrawlMode((int(h.CrawlMode) + 1) % 4)
		}
	}
	return HomeActionNone
}

func (h *HomePageModel) setMode(mode CrawlMode) {
	if h.CrawlerState == CrawlerStopped {
		h.CrawlMode = mode
	}
}

func (h *HomePageModel) adjustModeParam(delta int) {
	if h.CrawlerState != CrawlerStopped {
		return
	}
	switch h.CrawlMode {
	case CrawlMonitor:
		h.MonitorPages = clampInt(h.MonitorPages+delta, 1, 50)
	case CrawlFetchThumbnails:
		step := 100
		h.ThumbnailEndID += delta * step
		if h.ThumbnailEndID < h.ThumbnailStartID {
			h.ThumbnailEndID = h.ThumbnailStartID
		}
	default:
		h.PostsPerRequest = clampInt(h.PostsPerRequest+delta*20, 20, 200)
		h.CommentsPerPost = clampInt(h.CommentsPerPost+delta*20, 20, 200)
	}
}

func (h *HomePageModel) adjustThumbnailStart(delta int) {
	if h.CrawlerState != CrawlerStopped || h.CrawlMode != CrawlFetchThumbnails {
		return
	}
	h.ThumbnailStartID = maxInt(1, h.ThumbnailStartID+delta)
	if h.ThumbnailEndID < h.ThumbnailStartID {
		h.ThumbnailEndID = h.ThumbnailStartID
	}
}
