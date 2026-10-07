package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// JobLogDialogModel shows the tail of one background crawler job's log.
type JobLogDialogModel struct {
	JobID string
	Title string
	Logs  LogsDialogModel
}

func NewJobLogDialog() JobLogDialogModel {
	return JobLogDialogModel{Logs: NewLogsDialog()}
}

func (m JobLogDialogModel) initialized() bool {
	return true
}

func (m *JobLogDialogModel) Open(job CrawlerJob) {
	m.JobID = job.ID
	m.Title = job.Name
	m.Logs = NewLogsDialog()
	m.Logs.SetLoading(true)
}

func (m *JobLogDialogModel) SetLines(lines []string) {
	m.Logs.SetLines(lines)
}

func (m *JobLogDialogModel) SetError(err error) {
	m.Logs.SetError(err)
}

func (m *JobLogDialogModel) Loading() bool {
	return m.Logs.Loading()
}

func (m JobLogDialogModel) Update(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, shortcut.Up, shortcut.VimUp):
		m.Logs.Update(msg)
		return nil
	case key.Matches(msg, shortcut.Down, shortcut.VimDown):
		m.Logs.Update(msg)
		return nil
	case key.Matches(msg, shortcut.PgUp, shortcut.PgDown):
		m.Logs.Update(msg)
		return nil
	case key.Matches(msg, keymap.Direct.Refresh):
		m.Logs.SetLoading(true)
		return loadJobLogCmd(m.JobID)
	}
	return nil
}

func (m JobLogDialogModel) View(width, height int) string {
	title := "任务日志"
	if m.Title != "" {
		title += " — " + m.Title
	}
	body := m.Logs.View(width, maxInt(3, height-2))
	return vDialogTitleStyle.Render(clipToPlain(title, width)) + "\n\n" + body
}
