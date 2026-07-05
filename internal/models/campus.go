package models

type CourseDay struct {
	CourseName      string `json:"courseName"`
	Parity          string `json:"parity"`
	Style           string `json:"sty"`
	FullDescription string `json:"-"`
}

type CourseScheduleRow struct {
	TimeNum string
	Mon     CourseDay
	Tue     CourseDay
	Wed     CourseDay
	Thu     CourseDay
	Fri     CourseDay
	Sat     CourseDay
	Sun     CourseDay
}

type CourseScore struct {
	YearTerm string
	Name     string
	Credit   string
	Score    string
	Category string
}

type GPATerm struct {
	YearTerm string
	GPA      string
}

type ScoreSummary struct {
	GPA          string
	TotalCredit  string
	PassedCredit string
	CourseCount  string
	Scores       []CourseScore
	GPATerms     []GPATerm
}
