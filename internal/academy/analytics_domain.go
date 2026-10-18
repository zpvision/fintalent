package academy

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AnalyticsFilters struct {
	Cohort        int64  `json:"cohort"`
	Version       int64  `json:"version"`
	Curator       int64  `json:"curator"`
	Status        string `json:"status"`
	Period        string `json:"period"`
	From          string `json:"from"`
	To            string `json:"to"`
	Timezone      string `json:"timezone"`
	Compare       string `json:"compare"`
	Days          int    `json:"days"`
	Search        string `json:"search"`
	StudentFilter string `json:"student_filter"`
	Page          int    `json:"page"`
	Size          int    `json:"size"`
	ProgressMin   int    `json:"progress_min"`
	Enrollment    int64  `json:"enrollment"`
	from, until   time.Time
}

func parseAnalyticsFilters(q url.Values, now time.Time) (AnalyticsFilters, error) {
	f := AnalyticsFilters{Period: q.Get("period"), From: q.Get("from"), To: q.Get("to"), Timezone: q.Get("timezone"), Status: q.Get("status"), Compare: q.Get("compare"), Search: strings.TrimSpace(q.Get("search")), StudentFilter: q.Get("student_filter"), Page: 1, Size: 50, Days: 14}
	for name, p := range map[string]*int64{"cohort": &f.Cohort, "version": &f.Version, "curator": &f.Curator, "enrollment": &f.Enrollment} {
		if s := q.Get(name); s != "" {
			v, e := strconv.ParseInt(s, 10, 64)
			if e != nil || v < 0 && !(name == "cohort" && v == -1) {
				return f, ErrInvalid
			}
			*p = v
		}
	}
	for name, p := range map[string]*int{"page": &f.Page, "size": &f.Size, "days": &f.Days, "progress_min": &f.ProgressMin} {
		if s := q.Get(name); s != "" {
			v, e := strconv.Atoi(s)
			if e != nil {
				return f, ErrInvalid
			}
			*p = v
		}
	}
	if f.ProgressMin < 0 || f.ProgressMin > 100 {
		return f, ErrInvalid
	}
	if f.Page < 1 || f.Page > 100000 || f.Size < 1 || f.Size > 100 || f.Days < 1 || f.Days > 365 || len(f.Search) > 200 || !oneOf(f.Status, "", "not_started", "in_progress", "completed") || !oneOf(f.StudentFilter, "", "not_started", "started", "inactive", "behind", "successful", "completed", "attention", "high", "medium", "low") {
		return f, ErrInvalid
	}
	if f.Timezone == "" {
		f.Timezone = "UTC"
	}
	loc, e := time.LoadLocation(f.Timezone)
	if e != nil || f.Timezone == "Local" {
		return f, inputError{"Выберите временную зону IANA"}
	}
	if f.Compare == "" {
		f.Compare = "calendar"
	}
	if !oneOf(f.Compare, "calendar", "relative") {
		return f, ErrInvalid
	}
	if f.Period == "" {
		f.Period = "all"
	}
	today := now.In(loc)
	end := time.Date(today.Year(), today.Month(), today.Day()+1, 0, 0, 0, 0, loc)
	switch f.Period {
	case "all":
		f.from = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		f.until = end
	case "7", "30", "90":
		days, _ := strconv.Atoi(f.Period)
		f.until = end
		f.from = end.AddDate(0, 0, -days)
	case "custom":
		start, er := time.ParseInLocation("2006-01-02", f.From, loc)
		if er != nil {
			return f, inputError{"Укажите начальную дату"}
		}
		last, er := time.ParseInLocation("2006-01-02", f.To, loc)
		if er != nil || last.Before(start) || last.After(today) || last.Sub(start) > 366*5*24*time.Hour {
			return f, inputError{"Выберите период до пяти лет, не позднее сегодняшнего дня"}
		}
		f.from = start
		f.until = last.AddDate(0, 0, 1)
	default:
		return f, ErrInvalid
	}
	return f, nil
}
func (f AnalyticsFilters) values() url.Values {
	q := url.Values{}
	for k, v := range map[string]string{"period": f.Period, "from": f.From, "to": f.To, "timezone": f.Timezone, "status": f.Status, "compare": f.Compare, "search": f.Search, "student_filter": f.StudentFilter} {
		q.Set(k, v)
	}
	for k, v := range map[string]int64{"cohort": f.Cohort, "version": f.Version, "curator": f.Curator, "enrollment": f.Enrollment} {
		q.Set(k, strconv.FormatInt(v, 10))
	}
	for k, v := range map[string]int{"page": f.Page, "size": f.Size, "days": f.Days, "progress_min": f.ProgressMin} {
		if v > 0 {
			q.Set(k, strconv.Itoa(v))
		}
	}
	return q
}
func (f AnalyticsFilters) in(t time.Time) bool { return !t.Before(f.from) && t.Before(f.until) }

type AnalyticsSettings struct {
	GraceDays    int   `json:"grace_days"`
	InactiveDays int   `json:"inactive_days"`
	FailedTests  int   `json:"failed_tests"`
	Revisions    int   `json:"revisions"`
	Overdue      int   `json:"overdue"`
	Behind       int   `json:"behind"`
	HighSignals  int   `json:"high_signals"`
	ReturnDays   int   `json:"return_days"`
	Revision     int64 `json:"revision"`
}

func defaultAnalyticsSettings() AnalyticsSettings {
	return AnalyticsSettings{GraceDays: 1, InactiveDays: 5, FailedTests: 3, Revisions: 2, Overdue: 1, Behind: 2, HighSignals: 2, ReturnDays: 3}
}
func (s AnalyticsSettings) validate() error {
	if s.GraceDays < 0 || s.GraceDays > 30 {
		return ErrInvalid
	}
	if s.InactiveDays < 1 || s.InactiveDays > 365 || s.FailedTests < 1 || s.FailedTests > 100 || s.Revisions < 1 || s.Revisions > 100 || s.Overdue < 1 || s.Overdue > 100 || s.Behind < 1 || s.Behind > 1000 || s.HighSignals < 1 || s.HighSignals > 5 || s.ReturnDays < 1 || s.ReturnDays > 30 {
		return inputError{"Проверьте пороги риска и окно активности"}
	}
	return nil
}

type AnalyticsRisk struct {
	Level        string   `json:"level"`
	Reasons      []string `json:"reasons"`
	InactiveDays int      `json:"inactive_days"`
	Behind       int      `json:"behind"`
	Overdue      int      `json:"overdue"`
}
type AnalyticsStudent struct {
	ID              int64         `json:"enrollment_id"`
	User            int64         `json:"user_id"`
	Name            string        `json:"name"`
	Avatar          string        `json:"avatar"`
	Version         int64         `json:"version_id"`
	VersionNumber   int           `json:"version_number"`
	CohortID        int64         `json:"cohort_id"`
	CohortName      string        `json:"cohort_name"`
	CuratorID       int64         `json:"curator_id"`
	CuratorName     string        `json:"curator_name"`
	Enrolled        time.Time     `json:"enrolled_at"`
	Started         *time.Time    `json:"started_at"`
	Finished        *time.Time    `json:"completed_at"`
	Activity        *time.Time    `json:"last_activity_at"`
	LastLesson      string        `json:"last_lesson_key"`
	Access          string        `json:"access_status"`
	Summary         Summary       `json:"summary"`
	Progress        *float64      `json:"percent"`
	Scheduled       bool          `json:"scheduled"`
	ExpectedPercent *float64      `json:"expected_percent"`
	Expected        int           `json:"expected_lessons"`
	TestAverage     *float64      `json:"test_average"`
	TestCount       int           `json:"test_attempts"`
	Pending         int           `json:"pending"`
	Revisions       int           `json:"revisions"`
	Accepted        int           `json:"accepted"`
	Risk            AnalyticsRisk `json:"risk"`
}
type analyticsProgress struct {
	Enrollment int64      `json:"enrollment_id"`
	Key        string     `json:"lesson_key"`
	Status     string     `json:"status"`
	First      time.Time  `json:"first_opened_at"`
	Last       time.Time  `json:"last_opened_at"`
	Completed  *time.Time `json:"completed_at"`
}
type analyticsEvent struct {
	Enrollment int64     `json:"enrollment_id"`
	User       int64     `json:"user_id"`
	Type       string    `json:"type"`
	Lesson     string    `json:"lesson_key"`
	At         time.Time `json:"created_at"`
}
type analyticsGrade struct {
	Question int64   `json:"question_id"`
	Correct  *bool   `json:"is_correct"`
	Points   float64 `json:"earned_points"`
}
type analyticsTest struct {
	ID         int64            `json:"id"`
	Enrollment int64            `json:"enrollment_id"`
	Assessment int64            `json:"assessment_id"`
	Number     int              `json:"number"`
	Status     string           `json:"status"`
	Percent    float64          `json:"percent"`
	Score      float64          `json:"score"`
	Passed     bool             `json:"passed"`
	Started    time.Time        `json:"started_at"`
	Finished   *time.Time       `json:"finished_at"`
	Grades     []analyticsGrade `json:"grades"`
	Answered   []int64          `json:"answered"`
}
type analyticsSubmission struct {
	ID         int64      `json:"id"`
	Enrollment int64      `json:"enrollment_id"`
	Assessment int64      `json:"assessment_id"`
	Number     int        `json:"number"`
	Status     string     `json:"status"`
	Submitted  time.Time  `json:"submitted_at"`
	Score      *float64   `json:"score"`
	Reviewer   int64      `json:"reviewed_by"`
	Reviewed   *time.Time `json:"reviewed_at"`
	Assigned   int64      `json:"assigned_to"`
}
type analyticsReview struct {
	ID         int64              `json:"id"`
	Submission int64              `json:"submission_id"`
	Reviewer   int64              `json:"reviewer_id"`
	Name       string             `json:"name"`
	Action     string             `json:"action"`
	At         time.Time          `json:"created_at"`
	Score      *float64           `json:"score"`
	Criteria   map[string]float64 `json:"criterion_scores"`
}
type analyticsAssessment struct {
	ID        int64     `json:"id"`
	Version   int64     `json:"program_version_id"`
	Lesson    string    `json:"lesson_key"`
	Block     string    `json:"block_key"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Config    BlockData `json:"config"`
	Questions []struct {
		ID   int64  `json:"id"`
		Text string `json:"question"`
	} `json:"questions"`
}
type analyticsDelivery struct {
	ID         int64      `json:"id"`
	Enrollment int64      `json:"enrollment_id"`
	User       int64      `json:"recipient_id"`
	Campaign   int64      `json:"campaign_id"`
	Automation int64      `json:"automation_id"`
	Name       string     `json:"name"`
	Channel    string     `json:"channel"`
	Status     string     `json:"status"`
	Queued     time.Time  `json:"queued_at"`
	Sent       *time.Time `json:"sent_at"`
	Error      string     `json:"error_code"`
	Run        int64      `json:"run_id"`
}
type analyticsStaff struct {
	ID        int64  `json:"user_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Cohort    int64  `json:"cohort_id"`
	CanReview bool   `json:"can_review"`
}
type analyticsDataset struct {
	Course, Actor         int64
	Owner, Communications bool
	Filters               AnalyticsFilters
	Settings              AnalyticsSettings
	Now                   time.Time
	Students              []*AnalyticsStudent
	ByID                  map[int64]*AnalyticsStudent
	Programs              map[int64]Program
	VersionNumbers        map[int64]int
	Cohorts               map[int64]*Cohort
	Progress              []analyticsProgress
	Events                []analyticsEvent
	Tests                 []analyticsTest
	Submissions           []analyticsSubmission
	Reviews               []analyticsReview
	Assessments           []analyticsAssessment
	Deliveries            []analyticsDelivery
	Staff                 []analyticsStaff
	Runs                  int
	progress              map[int64]map[string]analyticsProgress
	opens                 map[int64]map[string]time.Time
	completions           map[int64]map[string]time.Time
}

func ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := math.Round(float64(n)/float64(d)*10000) / 100
	return &v
}
func mean(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sum := 0.
	for _, v := range values {
		sum += v
	}
	v := math.Round(sum/float64(len(values))*100) / 100
	return &v
}
func median(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	x := v[len(v)/2]
	if len(v)%2 == 0 {
		x = (x + v[len(v)/2-1]) / 2
	}
	return &x
}
func publishedLessons(p Program, cohort bool) []struct {
	Module      string
	ModuleTitle string
	Lesson      Lesson
} {
	out := []struct {
		Module      string
		ModuleTitle string
		Lesson      Lesson
	}{}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" || !cohort && !l.SelfPaced {
				continue
			}
			if cohort {
				l.SelfPaced = true
			}
			out = append(out, struct {
				Module      string
				ModuleTitle string
				Lesson      Lesson
			}{m.Key, m.Title, l})
		}
	}
	return out
}
func (d *analyticsDataset) prepare() {
	d.ByID = map[int64]*AnalyticsStudent{}
	d.progress = map[int64]map[string]analyticsProgress{}
	d.opens = map[int64]map[string]time.Time{}
	d.completions = map[int64]map[string]time.Time{}
	for _, s := range d.Students {
		s.Pending = 0
		s.Revisions = 0
		s.Accepted = 0
		s.Expected = 0
		s.ExpectedPercent = nil
		s.Progress = nil
		s.Scheduled = false
		s.TestCount = 0
		d.ByID[s.ID] = s
		d.progress[s.ID] = map[string]analyticsProgress{}
		d.opens[s.ID] = map[string]time.Time{}
		d.completions[s.ID] = map[string]time.Time{}
	}
	for _, p := range d.Progress {
		if d.ByID[p.Enrollment] != nil {
			d.progress[p.Enrollment][p.Key] = p
		}
	}
	for _, s := range d.Students {
		if s.LastLesson == "" {
			var last time.Time
			for _, p := range d.progress[s.ID] {
				if p.Last.After(last) {
					last, s.LastLesson = p.Last, p.Key
				}
			}
		}
	}
	for _, ev := range d.Events {
		if d.ByID[ev.Enrollment] == nil {
			continue
		}
		target := d.opens[ev.Enrollment]
		if ev.Type == "lesson_completed" {
			target = d.completions[ev.Enrollment]
		} else if ev.Type != "lesson_opened" {
			continue
		}
		old, ok := target[ev.Lesson]
		if !ok || ev.At.Before(old) {
			target[ev.Lesson] = ev.At
		}
	}
	tests := map[int64][]analyticsTest{}
	subs := map[int64][]analyticsSubmission{}
	for _, t := range d.Tests {
		tests[t.Enrollment] = append(tests[t.Enrollment], t)
	}
	for _, s := range d.Submissions {
		subs[s.Enrollment] = append(subs[s.Enrollment], s)
	}
	normalized := map[int64]Program{}
	for vid, p := range d.Programs {
		copyP := p
		copyP.Modules = append([]Module(nil), p.Modules...)
		for i := range copyP.Modules {
			copyP.Modules[i].Lessons = append([]Lesson(nil), p.Modules[i].Lessons...)
			for j := range copyP.Modules[i].Lessons {
				copyP.Modules[i].Lessons[j].SelfPaced = true
			}
		}
		normalized[vid] = copyP
	}
	for _, s := range d.Students {
		p := d.Programs[s.Version]
		progress := []Progress{}
		for _, v := range d.progress[s.ID] {
			progress = append(progress, Progress{LessonKey: v.Key, Status: v.Status, CompletedAt: v.Completed})
		}
		if s.CohortID != 0 {
			p = normalized[s.Version]
		}
		s.Summary = SummaryFor(p, progress, s.Started != nil)
		if s.Summary.Required > 0 {
			v := float64(s.Summary.Percent)
			s.Progress = &v
		}
		scores := []float64{}
		fail := map[int64]int{}
		latestTest := map[int64]analyticsTest{}
		passed := map[int64]bool{}
		for _, t := range tests[s.ID] {
			if t.Status != "finished" {
				continue
			}
			if t.Finished != nil && d.Filters.in(*t.Finished) {
				scores = append(scores, t.Percent)
				s.TestCount++
			}
			if !t.Passed {
				fail[t.Assessment]++
			}
			if t.Passed {
				passed[t.Assessment] = true
			}
			if t.Number > latestTest[t.Assessment].Number {
				latestTest[t.Assessment] = t
			}
		}
		s.TestAverage = mean(scores)
		latest := map[int64]analyticsSubmission{}
		returns := map[int64]int{}
		for _, sub := range subs[s.ID] {
			if sub.Number > latest[sub.Assessment].Number {
				latest[sub.Assessment] = sub
			}
			if sub.Status == "revision_requested" {
				returns[sub.Assessment]++
			}
		}
		for _, sub := range latest {
			switch sub.Status {
			case "submitted", "in_review":
				s.Pending++
			case "revision_requested":
				s.Revisions++
			case "accepted":
				s.Accepted++
			}
		}
		r := AnalyticsRisk{Level: "low", Reasons: []string{}}
		anchor := s.Enrolled
		if s.Activity != nil {
			anchor = *s.Activity
		}
		r.InactiveDays = max(0, int(d.Now.Sub(anchor).Hours()/24))
		c := d.Cohorts[s.CohortID]
		s.Scheduled = c != nil && c.Mode != "all"
		if s.Scheduled {
			for _, l := range publishedLessons(d.Programs[s.Version], true) {
				av := availability(c, l.Module, l.Lesson.Key, d.Now.Add(-time.Duration(d.Settings.GraceDays)*24*time.Hour))
				if l.Lesson.Required && av.Available {
					s.Expected++
					if v, ok := d.progress[s.ID][l.Lesson.Key]; !ok || v.Status != "completed" || CompletionBlock(l.Lesson) != "" {
						r.Behind++
					}
				}
			}
			s.ExpectedPercent = ratio(s.Expected, s.Summary.Required)
		}

		for _, a := range d.Assessments {
			if a.Version != s.Version || a.Kind != "assignment" {
				continue
			}
			module := ""
			for _, lesson := range publishedLessons(d.Programs[s.Version], s.CohortID != 0) {
				if lesson.Lesson.Key == a.Lesson {
					module = lesson.Module
					break
				}
			}
			if module == "" || !availability(c, module, a.Lesson, d.Now).Available {
				continue
			}
			if at, _ := assignmentDeadline(d.Cohorts[s.CohortID], a.Block); at != nil && at.Before(d.Now) {
				sub := latest[a.ID]
				if !oneOf(sub.Status, "submitted", "in_review", "accepted") {
					r.Overdue++
				}
			}
		}
		if s.Access == "active" && s.Summary.Status != "completed" {
			if r.InactiveDays >= d.Settings.InactiveDays {
				r.Reasons = append(r.Reasons, fmt.Sprintf("Нет активности %d дней", r.InactiveDays))
			}
			if s.CohortID != 0 && r.Behind >= d.Settings.Behind {
				r.Reasons = append(r.Reasons, fmt.Sprintf("Не завершены %d открытых обязательных уроков", r.Behind))
			}
			for _, a := range d.Assessments {
				if a.Version != s.Version || a.Kind != "test" {
					continue
				}
				ok := passed[a.ID]
				if !yes(policy(Block{Type: "test", Data: a.Config}).UseBest) {
					ok = latestTest[a.ID].Passed
				}
				if !ok && fail[a.ID] >= d.Settings.FailedTests {
					r.Reasons = append(r.Reasons, fmt.Sprintf("Тест «%s»: %d неуспешных попыток", a.Title, fail[a.ID]))
				}
			}
			for aid, sub := range latest {
				if sub.Status == "revision_requested" && returns[aid] >= d.Settings.Revisions {
					r.Reasons = append(r.Reasons, fmt.Sprintf("Задание возвращено на доработку %d раз", returns[aid]))
				}
			}
			if r.Overdue >= d.Settings.Overdue {
				r.Reasons = append(r.Reasons, fmt.Sprintf("Просрочено заданий: %d", r.Overdue))
			}
		}
		if len(r.Reasons) > 0 {
			r.Level = "medium"
		}
		if len(r.Reasons) >= d.Settings.HighSignals {
			r.Level = "high"
		}
		s.Risk = r
	}
}
func analyticsStudentMatch(s *AnalyticsStudent, f AnalyticsFilters, settings AnalyticsSettings) bool {
	if f.ProgressMin > 0 && (s.Progress == nil || *s.Progress < float64(f.ProgressMin)) {
		return false
	}
	if f.Status != "" && s.Summary.Status != f.Status {
		return false
	}
	if f.Search != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(f.Search)) && strconv.FormatInt(s.User, 10) != f.Search {
		return false
	}
	switch f.StudentFilter {
	case "not_started":
		return s.Started == nil
	case "started":
		return s.Started != nil
	case "inactive":
		return s.Access == "active" && s.Summary.Status != "completed" && s.Risk.InactiveDays >= settings.InactiveDays
	case "behind":
		return s.Access == "active" && s.Summary.Status != "completed" && s.Scheduled && s.Risk.Behind > 0
	case "successful":
		return s.Access == "active" && len(s.Risk.Reasons) == 0 && s.Started != nil
	case "completed":
		return s.Summary.Status == "completed"
	case "attention":
		return len(s.Risk.Reasons) > 0
	case "low", "medium", "high":
		return s.Risk.Level == f.StudentFilter
	}
	return true
}
