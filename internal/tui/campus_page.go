package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	"charm.land/lipgloss/v2"
)

type CourseSchedulePageModel struct {
	Rows    []models.CourseScheduleRow
	Loading bool
	Error   string
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
	b.WriteString(vStatLabelStyle.Width(timeWidth).Render("节次"))
	for _, day := range days {
		b.WriteString(" ")
		b.WriteString(vStatLabelStyle.Width(cellWidth).Render(day.label))
	}
	for _, row := range p.Rows {
		b.WriteString("\n")
		b.WriteString(vStatValueStyle.Width(timeWidth).Render(shortenCell(row.TimeNum, timeWidth)))
		for _, day := range days {
			b.WriteString(" ")
			value := shortenCell(day.get(row), cellWidth)
			if value == "" {
				value = "-"
			}
			b.WriteString(vStatValueStyle.Width(cellWidth).MaxWidth(cellWidth).Render(value))
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
	bodyWidth := maxInt(20, width-4)
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

	nameWidth := maxInt(14, bodyWidth-28)
	b.WriteString("\n\n")
	b.WriteString(vStatLabelStyle.Width(10).Render("学期"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(nameWidth).Render("课程"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(5).Render("学分"))
	b.WriteString(" ")
	b.WriteString(vStatLabelStyle.Width(6).Render("成绩"))
	availableRows := maxInt(1, height-16)
	maxOffset := maxInt(0, len(p.Summary.Scores)-availableRows)
	offset := clampInt(p.Offset, 0, maxOffset)
	end := minInt(len(p.Summary.Scores), offset+availableRows)
	for _, score := range p.Summary.Scores[offset:end] {
		b.WriteString("\n")
		b.WriteString(vPostMetaStyle.Width(10).Render(shortenCell(score.YearTerm, 10)))
		b.WriteString(" ")
		b.WriteString(vPostTextStyle.Width(nameWidth).Render(shortenCell(score.Name, nameWidth)))
		b.WriteString(" ")
		b.WriteString(vPostMetaStyle.Width(5).Render(shortenCell(score.Credit, 5)))
		b.WriteString(" ")
		b.WriteString(vPostLikeStyle.Width(6).Render(shortenCell(score.Score, 6)))
	}
	if len(p.Summary.Scores) > availableRows {
		b.WriteString("\n")
		b.WriteString(vPaginationStyle.Render(fmt.Sprintf(
			"%d-%d / %d  ↑↓ 滚动  PgUp/PgDn 翻页",
			offset+1,
			end,
			len(p.Summary.Scores),
		)))
	}
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, b.String())
}

func (p *ScorePageModel) Move(delta, height int) {
	if p.Summary == nil {
		return
	}
	availableRows := maxInt(1, height-16)
	maxOffset := maxInt(0, len(p.Summary.Scores)-availableRows)
	p.Offset = clampInt(p.Offset+delta, 0, maxOffset)
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
