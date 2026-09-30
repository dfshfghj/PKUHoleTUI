package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	"charm.land/lipgloss/v2"
)

type CourseSchedulePageModel struct {
	Rows      []models.CourseScheduleRow
	Loading   bool
	Error     string
	CursorRow int
	CursorCol int
}

type ScorePageModel struct {
	Summary *models.ScoreSummary
	Loading bool
	Error   string
	Offset  int
}

func (p CourseSchedulePageModel) View(width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 1
	}
	bodyWidth := maxInt(20, width-4)
	var b strings.Builder
	b.WriteString("\n\n")
	if p.Loading {
		b.WriteString(vLoadingStyle.Render("加载课表中..."))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}
	if p.Error != "" {
		b.WriteString(vErrorStyle.Render("错误: " + p.Error))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}
	if len(p.Rows) == 0 {
		b.WriteString(vEmptyStyle.Render("暂无课表数据"))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}

	days := []struct {
		label string
		get   func(models.CourseScheduleRow) string
	}{
		{"一", func(r models.CourseScheduleRow) string { return r.Mon.CourseName }},
		{"二", func(r models.CourseScheduleRow) string { return r.Tue.CourseName }},
		{"三", func(r models.CourseScheduleRow) string { return r.Wed.CourseName }},
		{"四", func(r models.CourseScheduleRow) string { return r.Thu.CourseName }},
		{"五", func(r models.CourseScheduleRow) string { return r.Fri.CourseName }},
		{"六", func(r models.CourseScheduleRow) string { return r.Sat.CourseName }},
		{"日", func(r models.CourseScheduleRow) string { return r.Sun.CourseName }},
	}
	timeWidth := 4
	cellWidth := maxInt(6, (bodyWidth-timeWidth-len(days))/len(days))
	cursorStyle := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true)
	b.WriteString(vStatLabelStyle.Width(timeWidth).Render("节次"))
	for col, day := range days {
		b.WriteString(" ")
		if p.CursorRow == -1 && p.CursorCol == col {
			b.WriteString(cursorStyle.Width(cellWidth).Render(day.label))
		} else {
			b.WriteString(vStatLabelStyle.Width(cellWidth).Render(day.label))
		}
	}
	for rowIdx, row := range p.Rows {
		b.WriteString("\n")
		if p.CursorRow == rowIdx && p.CursorCol == -1 {
			b.WriteString(cursorStyle.Width(timeWidth).Render(shortenCell(row.TimeNum, timeWidth)))
		} else {
			b.WriteString(vStatValueStyle.Width(timeWidth).Render(shortenCell(row.TimeNum, timeWidth)))
		}
		for colIdx, day := range days {
			b.WriteString(" ")
			value := shortenCell(day.get(row), cellWidth)
			if value == "" {
				value = "-"
			}
			if p.CursorRow == rowIdx && p.CursorCol == colIdx {
				b.WriteString(cursorStyle.Width(cellWidth).MaxWidth(cellWidth).Render(value))
			} else {
				b.WriteString(vStatValueStyle.Width(cellWidth).MaxWidth(cellWidth).Render(value))
			}
		}
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}

func (p ScorePageModel) View(width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 1
	}
	var b strings.Builder
	b.WriteString("\n\n")
	if p.Loading {
		b.WriteString(vLoadingStyle.Render("加载成绩中..."))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}
	if p.Error != "" {
		b.WriteString(vErrorStyle.Render("错误: " + p.Error))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}
	if p.Summary == nil {
		b.WriteString(vEmptyStyle.Render("暂无成绩数据"))
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
	}
	b.WriteString(p.scoresPrelude(width))
	availableRows := maxInt(1, height-16)
	offset := clampInt(p.Offset, 0, maxInt(0, len(p.Summary.Scores)-availableRows))
	end := minInt(len(p.Summary.Scores), offset+availableRows)
	bar, _ := p.scoresBar(height)
	var barLines []string
	if bar.Visible {
		barLines = bar.render()
	}
	nameWidth := p.scoresNameWidth(width)
	for i, score := range p.Summary.Scores[offset:end] {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(vPostMetaStyle.Width(10).Render(shortenCell(score.YearTerm, 10)))
		b.WriteString(" ")
		b.WriteString(vPostTextStyle.Width(nameWidth).Render(shortenCell(score.Name, nameWidth)))
		b.WriteString(" ")
		b.WriteString(vPostMetaStyle.Width(5).Render(shortenCell(score.Credit, 5)))
		b.WriteString(" ")
		b.WriteString(vPostLikeStyle.Width(6).Render(shortenCell(score.Score, 6)))
		if bar.Visible && i < len(barLines) {
			b.WriteString(barLines[i])
		}
	}
	if len(p.Summary.Scores) > availableRows {
		b.WriteString("\n")
		b.WriteString(vPaginationStyle.Render(fmt.Sprintf(
			"%d-%d / %d",
			offset+1,
			end,
			len(p.Summary.Scores),
		)))
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}

// scoresPrelude renders everything above the score rows (summary card, term
// line and table header) and ends with a newline so the first score row
// starts on the next line.
func (p ScorePageModel) scoresPrelude(width int) string {
	bodyWidth := maxInt(20, width-4)
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(compactCardStyle(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			vStatLabelStyle.Render("GPA: "), vStatValueStyle.Render(emptyDash(p.Summary.GPA)),
			vStatLabelStyle.Render("  已修: "), vStatValueStyle.Render(emptyDash(p.Summary.PassedCredit)),
			vStatLabelStyle.Render("  总学分: "), vStatValueStyle.Render(emptyDash(p.Summary.TotalCredit)),
			vStatLabelStyle.Render("  课程: "), vStatValueStyle.Render(emptyDash(p.Summary.CourseCount)),
		),
	))
	if len(p.Summary.GPATerms) > 0 {
		b.WriteString("\n")
		var parts []string
		for _, term := range p.Summary.GPATerms {
			parts = append(parts, fmt.Sprintf("%s %s", term.YearTerm, term.GPA))
		}
		b.WriteString(vPaginationStyle.Render(truncateVisibleLine(strings.Join(parts, "  "), bodyWidth, "...")))
	}
	nameWidth := p.scoresNameWidth(width)
	b.WriteString("\n\n")
	b.WriteString(vStatLabelStyle.Width(10).Render("学期"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(nameWidth).Render("课程"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(5).Render("学分"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(6).Render("成绩"))
	b.WriteString("\n")
	return b.String()
}

func (p ScorePageModel) scoresNameWidth(width int) int {
	bodyWidth := maxInt(20, width-4)
	return maxInt(14, bodyWidth-28)
}

// scoresBar describes the score table scrollbar for a given content height.
func (p ScorePageModel) scoresBar(height int) (scrollbarState, int) {
	if p.Summary == nil {
		return scrollbarState{}, 0
	}
	availableRows := maxInt(1, height-16)
	maxOffset := maxInt(0, len(p.Summary.Scores)-availableRows)
	offset := clampInt(p.Offset, 0, maxOffset)
	bar := scrollbarFor(availableRows, len(p.Summary.Scores), availableRows, offset)
	return bar, offset
}

func (p *ScorePageModel) Move(delta, height int) {
	if p.Summary == nil {
		return
	}
	availableRows := maxInt(1, height-16)
	maxOffset := maxInt(0, len(p.Summary.Scores)-availableRows)
	p.Offset = clampInt(p.Offset+delta, 0, maxOffset)
}

func (p *CourseSchedulePageModel) SelectedCourse() (day, timeNum, name, parity, description string) {
	if len(p.Rows) == 0 || p.CursorRow < 0 || p.CursorRow >= len(p.Rows) {
		return "", "", "", "", ""
	}
	row := p.Rows[p.CursorRow]
	timeNum = row.TimeNum
	dayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	getDays := []func(models.CourseScheduleRow) models.CourseDay{
		func(r models.CourseScheduleRow) models.CourseDay { return r.Mon },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Tue },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Wed },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Thu },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Fri },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Sat },
		func(r models.CourseScheduleRow) models.CourseDay { return r.Sun },
	}
	if p.CursorCol < 0 || p.CursorCol >= len(dayNames) {
		return "", timeNum, "", "", ""
	}
	day = dayNames[p.CursorCol]
	cd := getDays[p.CursorCol](row)
	return day, timeNum, cd.CourseName, cd.Parity, cd.FullDescription
}

func shortenCell(text string, width int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if width <= 1 || lipgloss.Width(text) <= width {
		return text
	}
	return truncateVisibleLine(text, width, "...")
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
