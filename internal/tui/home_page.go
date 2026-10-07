package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// HomeFocus is the focused region of the sync page.
type HomeFocus int

const (
	HomeFocusJobs HomeFocus = iota
	HomeFocusParams
	HomeFocusEditValue
)

type HomePageModel struct {
	LoggedIn  bool
	LoginUser string

	Jobs        []CrawlerJob
	SelectedJob int

	Focus    HomeFocus
	ParamRow int
	Input    textinput.Model

	PendingDeleteID string
	PendingDeleteAt time.Time

	LastError string
}

func NewHomePageModel() HomePageModel {
	applyTheme("")
	return HomePageModel{
		SelectedJob: -1,
		Focus:       HomeFocusJobs,
		Input:       newJobValueInput(),
	}
}

func newJobValueInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "0"
	input.CharLimit = 9
	input.SetWidth(10)
	styleTextInput(&input, colorSurface, colorText, colorMuted)
	return input
}

func (h *HomePageModel) Selected() *CrawlerJob {
	if h.SelectedJob < 0 || h.SelectedJob >= len(h.Jobs) {
		return nil
	}
	return &h.Jobs[h.SelectedJob]
}

func (h *HomePageModel) clampSelection() {
	if len(h.Jobs) == 0 {
		h.SelectedJob = -1
		return
	}
	h.SelectedJob = clampInt(h.SelectedJob, 0, len(h.Jobs)-1)
}

// AddJob appends an unsaved job with default parameters and focuses its form.
func (h *HomePageModel) AddJob() {
	h.Jobs = append(h.Jobs, newCrawlerJob(h.defaultJobName(), defaultCrawlerParams()))
	h.SelectedJob = len(h.Jobs) - 1
	h.Focus = HomeFocusParams
	h.ParamRow = 0
}

func (h *HomePageModel) defaultJobName() string {
	count := 0
	for i := range h.Jobs {
		if strings.HasPrefix(h.Jobs[i].Name, "新任务") {
			count++
		}
	}
	return fmt.Sprintf("新任务 %d", count+1)
}

func (h *HomePageModel) removeSelectedJob() {
	if h.SelectedJob < 0 || h.SelectedJob >= len(h.Jobs) {
		return
	}
	h.Jobs = append(h.Jobs[:h.SelectedJob], h.Jobs[h.SelectedJob+1:]...)
	h.clampSelection()
	if h.SelectedJob < 0 {
		h.Focus = HomeFocusJobs
	}
}

func (h *HomePageModel) jobCounts() (running, total int) {
	for i := range h.Jobs {
		if h.Jobs[i].State() == JobRunning || h.Jobs[i].State() == JobStopping {
			running++
		}
	}
	return running, len(h.Jobs)
}

// RefreshJobs re-checks process liveness and reloads the last progress line.
func (h *HomePageModel) RefreshJobs() {
	for i := range h.Jobs {
		if h.Jobs[i].PID == 0 {
			continue
		}
		if !jobAlive(h.Jobs[i].PID) {
			h.Jobs[i].Stopping = false
			h.Jobs[i].Progress = jobProgressLine(h.Jobs[i])
			continue
		}
		if h.Jobs[i].Progress == "" || h.Jobs[i].State() == JobRunning {
			h.Jobs[i].Progress = jobProgressLine(h.Jobs[i])
		}
	}
}

// homeActionKind is what a key press asks the Model to do.
type homeActionKind int

const (
	homeActionNone homeActionKind = iota
	homeActionStartJob
	homeActionStopJob
	homeActionSaveJobs
	homeActionDeleteJob
	homeActionDeleteConfirm
	homeActionOpenJobLog
	homeActionRefresh
)

type homeAction struct {
	Kind  homeActionKind
	Index int
}

const deleteConfirmWindow = 3 * time.Second

func (h *HomePageModel) Update(msg tea.KeyPressMsg) homeAction {
	if h.Focus == HomeFocusEditValue {
		switch {
		case key.Matches(msg, shortcut.Enter):
			h.commitParamEdit()
			return homeAction{Kind: homeActionNone}
		case key.Matches(msg, shortcut.Escape):
			h.cancelParamEdit()
			return homeAction{Kind: homeActionNone}
		default:
			h.Input, _ = h.Input.Update(msg)
			return homeAction{Kind: homeActionNone}
		}
	}
	if h.Focus == HomeFocusParams {
		return h.updateParamsFocus(msg)
	}
	return h.updateJobsFocus(msg)
}

func (h *HomePageModel) updateJobsFocus(msg tea.KeyPressMsg) homeAction {
	switch {
	case key.Matches(msg, shortcut.Up, shortcut.VimUp):
		if h.SelectedJob > 0 {
			h.SelectedJob--
		}
	case key.Matches(msg, shortcut.Down, shortcut.VimDown):
		if h.SelectedJob < len(h.Jobs)-1 {
			h.SelectedJob++
		}
	case key.Matches(msg, shortcut.Enter):
		if h.Selected() != nil {
			h.Focus = HomeFocusParams
			h.ParamRow = 0
		}
	case key.Matches(msg, shortcut.HomeNewJob):
		h.AddJob()
	case key.Matches(msg, shortcut.HomeJobStartStop):
		job := h.Selected()
		if job == nil {
			return homeAction{Kind: homeActionNone}
		}
		index := h.SelectedJob
		if job.State() == JobRunning || job.State() == JobStopping {
			return homeAction{Kind: homeActionStopJob, Index: index}
		}
		return homeAction{Kind: homeActionStartJob, Index: index}
	case key.Matches(msg, shortcut.HomeJobDelete):
		job := h.Selected()
		if job == nil {
			return homeAction{Kind: homeActionNone}
		}
		now := time.Now()
		if h.PendingDeleteID == job.ID && now.Sub(h.PendingDeleteAt) < deleteConfirmWindow {
			index := h.SelectedJob
			h.removeSelectedJob()
			h.PendingDeleteID = ""
			return homeAction{Kind: homeActionDeleteJob, Index: index}
		}
		h.PendingDeleteID = job.ID
		h.PendingDeleteAt = now
		return homeAction{Kind: homeActionDeleteConfirm, Index: h.SelectedJob}
	case key.Matches(msg, shortcut.HomeJobLog):
		if h.Selected() != nil {
			return homeAction{Kind: homeActionOpenJobLog, Index: h.SelectedJob}
		}
	case key.Matches(msg, keymap.Direct.Refresh):
		return homeAction{Kind: homeActionRefresh}
	}
	return homeAction{Kind: homeActionNone}
}

func (h *HomePageModel) updateParamsFocus(msg tea.KeyPressMsg) homeAction {
	rows := h.paramRows()
	switch {
	case key.Matches(msg, shortcut.Up, shortcut.VimUp):
		if h.ParamRow > 0 {
			h.ParamRow--
		}
	case key.Matches(msg, shortcut.Down, shortcut.VimDown):
		if h.ParamRow < len(rows)-1 {
			h.ParamRow++
		}
	case key.Matches(msg, shortcut.Escape):
		h.Focus = HomeFocusJobs
	case key.Matches(msg, shortcut.Save):
		return homeAction{Kind: homeActionSaveJobs}
	case key.Matches(msg, shortcut.Left, shortcut.VimLeft, shortcut.Decrease):
		h.adjustParam(rows, -1)
	case key.Matches(msg, shortcut.Right, shortcut.VimRight, shortcut.Increase):
		h.adjustParam(rows, 1)
	case key.Matches(msg, keymap.Prefix.Leader):
		if h.ParamRow >= 0 && h.ParamRow < len(rows) && rows[h.ParamRow].Kind != paramRowInt {
			return h.activateParamRow(rows)
		}
	case key.Matches(msg, shortcut.Enter):
		return h.activateParamRow(rows)
	}
	return homeAction{Kind: homeActionNone}
}

func (h *HomePageModel) activateParamRow(rows []homeParamRow) homeAction {
	if h.ParamRow < 0 || h.ParamRow >= len(rows) {
		return homeAction{Kind: homeActionNone}
	}
	row := rows[h.ParamRow]
	job := h.Selected()
	if job == nil {
		return homeAction{Kind: homeActionNone}
	}
	switch row.Kind {
	case paramRowMode:
		h.setParamMode(CrawlMode((int(job.Params.Mode) + 1) % 4))
	case paramRowBool:
		*row.Bool(&job.Params) = !*row.Bool(&job.Params)
	default:
		value := *row.Int(&job.Params)
		h.Focus = HomeFocusEditValue
		h.Input.SetValue(strconv.Itoa(value))
		h.Input.CursorEnd()
		_ = h.Input.Focus()
	}
	return homeAction{Kind: homeActionNone}
}

func (h *HomePageModel) adjustParam(rows []homeParamRow, delta int) {
	if h.ParamRow < 0 || h.ParamRow >= len(rows) {
		return
	}
	job := h.Selected()
	if job == nil {
		return
	}
	row := rows[h.ParamRow]
	switch row.Kind {
	case paramRowMode:
		mode := (int(job.Params.Mode) + delta) % 4
		if mode < 0 {
			mode += 4
		}
		h.setParamMode(CrawlMode(mode))
	case paramRowBool:
		*row.Bool(&job.Params) = delta > 0
	default:
		value := *row.Int(&job.Params) + delta*row.Step
		*row.Int(&job.Params) = clampRowValue(row, value)
		h.normalizeParams()
	}
}

// clampRowValue applies the row's bounds. Max=0 means "no upper bound" and
// Min=0 means "no lower bound beyond zero".
func clampRowValue(row homeParamRow, value int) int {
	if value < 0 {
		value = 0
	}
	if row.Min != 0 && value < row.Min {
		value = row.Min
	}
	if row.Max > 0 && value > row.Max {
		value = row.Max
	}
	return value
}

func (h *HomePageModel) setParamMode(mode CrawlMode) {
	job := h.Selected()
	if job == nil {
		return
	}
	job.Params.Mode = mode
	if h.ParamRow >= len(h.paramRows()) {
		h.ParamRow = maxInt(0, len(h.paramRows())-1)
	}
}

// normalizeParams keeps the thumbnail range ordered.
func (h *HomePageModel) normalizeParams() {
	job := h.Selected()
	if job == nil {
		return
	}
	if job.Params.ThumbnailStartID < 1 {
		job.Params.ThumbnailStartID = 1
	}
	if job.Params.ThumbnailEndID < job.Params.ThumbnailStartID {
		job.Params.ThumbnailEndID = job.Params.ThumbnailStartID
	}
}

func (h *HomePageModel) commitParamEdit() {
	rows := h.paramRows()
	value, err := strconv.Atoi(strings.TrimSpace(h.Input.Value()))
	if err == nil && h.ParamRow >= 0 && h.ParamRow < len(rows) {
		row := rows[h.ParamRow]
		if job := h.Selected(); job != nil && row.Kind == paramRowInt {
			*row.Int(&job.Params) = clampRowValue(row, value)
			h.normalizeParams()
		}
	}
	h.cancelParamEdit()
}

func (h *HomePageModel) cancelParamEdit() {
	h.Focus = HomeFocusParams
	h.Input.Blur()
}

func (h *HomePageModel) ParamEditing() bool {
	return h.Focus == HomeFocusEditValue
}

// homeParamRow describes one editable parameter row.
type homeParamRow struct {
	Label string
	Kind  homeParamKind
	Int   func(*CrawlerParams) *int
	Bool  func(*CrawlerParams) *bool
	Step  int
	Min   int
	Max   int
}

type homeParamKind int

const (
	paramRowMode homeParamKind = iota
	paramRowInt
	paramRowBool
)

func intRow(label string, field func(*CrawlerParams) *int, step, minV, maxV int) homeParamRow {
	return homeParamRow{Label: label, Kind: paramRowInt, Int: field, Step: step, Min: minV, Max: maxV}
}

func boolRow(label string, field func(*CrawlerParams) *bool) homeParamRow {
	return homeParamRow{Label: label, Kind: paramRowBool, Bool: field}
}

func (h *HomePageModel) paramsMode() CrawlMode {
	if job := h.Selected(); job != nil {
		return job.Params.Mode
	}
	return CrawlSequential
}

func (h *HomePageModel) paramRows() []homeParamRow {
	rows := []homeParamRow{{Label: "类型", Kind: paramRowMode}}
	switch h.paramsMode() {
	case CrawlMonitor:
		// Monitor always loops, so 循环 is implicit; the loop delay stays adjustable.
		rows = append(rows,
			intRow("监控页数", func(p *CrawlerParams) *int { return &p.MonitorPages }, 1, 1, 50),
			intRow("页延迟", func(p *CrawlerParams) *int { return &p.PageDelaySeconds }, 1, 0, 0),
			intRow("轮间隔", func(p *CrawlerParams) *int { return &p.LoopDelaySeconds }, 10, 0, 0),
			boolRow("抓取图片", func(p *CrawlerParams) *bool { return &p.FetchImages }),
			boolRow("保存 JSON", func(p *CrawlerParams) *bool { return &p.SaveRawJSON }),
		)
	case CrawlFetchImages:
		// No extra parameters.
	case CrawlFetchThumbnails:
		rows = append(rows,
			intRow("起始 ID", func(p *CrawlerParams) *int { return &p.ThumbnailStartID }, 100, 1, 0),
			intRow("结束 ID", func(p *CrawlerParams) *int { return &p.ThumbnailEndID }, 100, 1, 0),
		)
	default:
		// Sequential is a single round; looping belongs to the monitor mode.
		rows = append(rows,
			intRow("起始页", func(p *CrawlerParams) *int { return &p.StartPage }, 1, 1, 0),
			intRow("页数", func(p *CrawlerParams) *int { return &p.Pages }, 10, 0, 0),
			intRow("页延迟", func(p *CrawlerParams) *int { return &p.PageDelaySeconds }, 1, 0, 0),
			boolRow("抓取图片", func(p *CrawlerParams) *bool { return &p.FetchImages }),
			boolRow("保存 JSON", func(p *CrawlerParams) *bool { return &p.SaveRawJSON }),
		)
	}
	rows = append(rows,
		intRow("帖子批次", func(p *CrawlerParams) *int { return &p.PostBatchSize }, 20, 1, 200),
		intRow("评论上限", func(p *CrawlerParams) *int { return &p.CommentLimit }, 20, 1, 200),
		boolRow("转换 WebP", func(p *CrawlerParams) *bool { return &p.ConvertWebp }),
	)
	return rows
}

// homePageLayout records where the interactive rows ended up, so mouse
// hit-testing uses the same geometry as the rendered frame.
type homePageLayout struct {
	JobRows   map[int]int
	ParamRows map[int]int
}

func (h HomePageModel) View(width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 1
	}
	lines, _ := h.buildLines(width, height)
	if len(lines) > height {
		lines = lines[:height]
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, strings.Join(lines, "\n"))
}

func homeSectionTitle(text string) string {
	marker := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▍")
	return marker + " " + vStatValueStyle.Render(text)
}

// homeHintStyle has no vertical padding so a hint occupies exactly one row.
var homeHintStyle = lipgloss.NewStyle().Foreground(colorMuted)

// homeEmptyStyle has no vertical padding for the same reason.
var homeEmptyStyle = lipgloss.NewStyle().Foreground(colorBorder)

func (h HomePageModel) buildLines(width, height int) ([]string, homePageLayout) {
	layout := homePageLayout{JobRows: map[int]int{}, ParamRows: map[int]int{}}
	var lines []string
	push := func(line string) {
		lines = append(lines, lipgloss.NewStyle().MaxWidth(width).Render(line))
	}

	push(vTitleStyle.Render("TreeHole TUI"))
	push(h.loginLine())
	push("")

	running, total := h.jobCounts()
	jobRows := h.paramRows()
	paramBlock := 2 // heading + hint
	if h.Selected() != nil {
		paramBlock += len(jobRows)
	}
	jobBudget := maxInt(1, height-len(lines)-paramBlock-4) // 4: jobs heading/hint/blank/detail
	jobBudget = minInt(jobBudget, maxInt(1, len(h.Jobs)))

	heading := homeSectionTitle("后台任务")
	countText := vStatLabelStyle.Render(fmt.Sprintf("%d 运行中 / 共 %d", running, total))
	gap := width - lipgloss.Width(heading) - lipgloss.Width(countText)
	push(heading + strings.Repeat(" ", maxInt(1, gap)) + countText)

	start := 0
	if h.SelectedJob >= jobBudget {
		start = h.SelectedJob - jobBudget + 1
	}
	end := minInt(len(h.Jobs), start+jobBudget)
	if len(h.Jobs) == 0 {
		push("  " + homeEmptyStyle.Render("暂无后台任务，按 n 新建"))
	}
	for i := start; i < end; i++ {
		layout.JobRows[i] = len(lines)
		push(h.renderJobRow(i, width, i == h.SelectedJob))
	}
	if running >= 2 {
		push("  " + lipgloss.NewStyle().Foreground(colorWarning).Render("提示: 多个爬虫同时写入 SQLite 可能触发写锁等待"))
	}
	if job := h.Selected(); job != nil {
		push(h.renderJobDetail(*job))
	}
	hint := "n 新建 · Enter 参数 · s 启动/停止 · x 删除 · L 日志 · r 刷新"
	if h.PendingDeleteID != "" && time.Since(h.PendingDeleteAt) < deleteConfirmWindow {
		push("  " + lipgloss.NewStyle().Foreground(colorWarning).Render("再按一次 x 确认删除该任务"))
	} else {
		push("  " + homeHintStyle.Render(hint))
	}
	push("")

	selected := h.Selected()
	title := "参数"
	if selected != nil {
		title += " — " + selected.Name
	}
	push(homeSectionTitle(title))
	if selected == nil {
		push("  " + homeHintStyle.Render("选择或新建一个任务后在此编辑爬虫参数"))
	} else {
		for i, row := range jobRows {
			layout.ParamRows[i] = len(lines)
			push(h.renderParamRow(row, selected.Params, i))
		}
		if h.Focus == HomeFocusJobs {
			push("  " + homeHintStyle.Render("Enter 进入参数编辑"))
		} else {
			push("  " + homeHintStyle.Render("↑↓ 选择 · ←→ 调整 · Enter 编辑/切换 · Ctrl+S 保存 · Esc 返回"))
		}
	}
	if h.LastError != "" {
		push(vErrorStyle.Render("错误: " + h.LastError))
	}
	return lines, layout
}

func (h HomePageModel) loginLine() string {
	if h.LoggedIn {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			vStatLabelStyle.Render("登录: "),
			vStatusRunningStyle.Render("已登录"),
			vStatLabelStyle.Render("  用户: "),
			vStatValueStyle.Render(h.LoginUser),
		)
	}
	return vStatusStoppedStyle.Render("未登录")
}

func jobStateLabel(job CrawlerJob) string {
	switch job.State() {
	case JobRunning:
		uptime := job.Uptime().Round(time.Second)
		return fmt.Sprintf("运行中 %s", uptime)
	case JobStopping:
		return "停止中"
	case JobFinished:
		return "已结束"
	default:
		return "未启动"
	}
}

func jobStateGlyph(state CrawlerJobState) string {
	switch state {
	case JobRunning:
		return "●"
	case JobStopping:
		return "◐"
	case JobFinished:
		return "✓"
	default:
		return "○"
	}
}

func jobStateStyle(state CrawlerJobState) lipgloss.Style {
	switch state {
	case JobRunning:
		return vStatusRunningStyle
	case JobStopping:
		return lipgloss.NewStyle().Foreground(colorWarning)
	case JobFinished:
		return vStatLabelStyle
	default:
		return vStatusStoppedStyle
	}
}

func (h HomePageModel) renderJobRow(index, width int, selected bool) string {
	job := h.Jobs[index]
	state := job.State()
	prefix := "  "
	if selected {
		prefix = lipgloss.NewStyle().Foreground(colorAccent).Render("▸") + " "
	}
	glyph := jobStateStyle(state).Render(jobStateGlyph(state))
	name := padVisibleRight(vStatValueStyle.Render(clipToPlain(job.Name, 16)), 16)
	summary := vStatLabelStyle.Render(clipToPlain(jobSummary(job), 40))
	left := prefix + glyph + " " + name + " " + summary

	status := jobStateStyle(state).Render(jobStateLabel(job))
	gap := width - lipgloss.Width(left) - lipgloss.Width(status)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + status
}

func (h HomePageModel) renderJobDetail(job CrawlerJob) string {
	state := job.State()
	parts := []string{"状态: " + jobStateLabel(job)}
	if job.PID > 0 && state != JobNeverStarted {
		parts = append(parts, fmt.Sprintf("pid %d", job.PID))
	}
	if job.Progress != "" {
		parts = append(parts, "进度: "+clipToPlain(progressText(job.Progress), 36))
	}
	return "  " + vStatLabelStyle.Render(strings.Join(parts, " · "))
}

// progressText drops the log timestamp so the crawler message itself fits.
func progressText(line string) string {
	if idx := strings.Index(line, "[Crawler]"); idx > 0 {
		return line[idx:]
	}
	return line
}

// The 类型 row layout is "  " + label(width homeParamLabelWidth) + " " + the
// mode options joined by two spaces. Keeping the widths here lets mouse
// hit-testing derive the option spans from the same numbers as the renderer.
const (
	homeParamPrefixWidth = 2
	homeParamLabelWidth  = 10
)

// homeModeOptions is the display order of the 类型 radio buttons.
var homeModeOptions = []CrawlMode{CrawlSequential, CrawlMonitor, CrawlFetchImages, CrawlFetchThumbnails}

// homeModeOptionSpans returns the [start,end) screen columns of each 类型
// option, matching renderParamRow so a click lands on the option it points at.
func homeModeOptionSpans() [][2]int {
	col := homeParamPrefixWidth + homeParamLabelWidth + 1 // prefix + label + space
	spans := make([][2]int, len(homeModeOptions))
	for i, mode := range homeModeOptions {
		width := lipgloss.Width("● " + crawlModeShort(mode))
		spans[i] = [2]int{col, col + width}
		col += width + 2 // option + the two-space separator
	}
	return spans
}

// homeModeAtColumn maps a column on the 类型 row to the mode option under it.
func homeModeAtColumn(x int) (CrawlMode, bool) {
	for i, span := range homeModeOptionSpans() {
		if x >= span[0] && x < span[1] {
			return homeModeOptions[i], true
		}
	}
	return CrawlSequential, false
}

func (h HomePageModel) renderParamRow(row homeParamRow, params CrawlerParams, index int) string {
	prefix := "  "
	if h.Focus != HomeFocusJobs && index == h.ParamRow {
		prefix = lipgloss.NewStyle().Foreground(colorAccent).Render("▸") + " "
	}
	label := vStatLabelStyle.Width(homeParamLabelWidth).Render(row.Label)
	accent := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	muted := vStatLabelStyle

	switch row.Kind {
	case paramRowMode:
		parts := make([]string, 0, len(homeModeOptions))
		for _, mode := range homeModeOptions {
			if params.Mode == mode {
				parts = append(parts, accent.Render("● "+crawlModeShort(mode)))
				continue
			}
			parts = append(parts, muted.Render("○ "+crawlModeShort(mode)))
		}
		return prefix + label + " " + strings.Join(parts, "  ")
	case paramRowBool:
		if *row.Bool(&params) {
			return prefix + label + " " + accent.Render("[x]")
		}
		return prefix + label + " " + muted.Render("[ ]")
	default:
		if h.Focus == HomeFocusEditValue && index == h.ParamRow {
			return prefix + label + " [ " + h.Input.View() + " ]"
		}
		return prefix + label + " " + vStatValueStyle.Render(fmt.Sprintf("[ %d ]", *row.Int(&params)))
	}
}

// clipToPlain truncates plain text (no ANSI) to a visible width.
func clipToPlain(s string, width int) string {
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

func padVisibleRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

func onOff(value bool) string {
	if value {
		return "开"
	}
	return "关"
}

// compactCardStyle is kept for the schedule page, which still uses cards.
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
