package academy

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
)

func analyticsFixture(t testing.TB) *analyticsDataset {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	f, e := parseAnalyticsFilters(url.Values{"period": {"all"}, "timezone": {"UTC"}}, now)
	if e != nil {
		t.Fatal(e)
	}
	start := now.Add(-10 * 24 * time.Hour)
	last := now.Add(-7 * 24 * time.Hour)
	opened := now.Add(-8 * 24 * time.Hour)
	done := now.Add(-6 * 24 * time.Hour)
	p := Program{Modules: []Module{{Key: "module-one", Title: "Module", Status: "published", Lessons: []Lesson{{Key: "lesson-one", Title: "Open", Status: "published", Required: true, SelfPaced: true, CompletionRule: "manual"}, {Key: "lesson-two", Title: "Future", Status: "published", Required: true, SelfPaced: true, CompletionRule: "manual"}}}}}
	d := &analyticsDataset{Course: 1, Actor: 10, Owner: true, Communications: true, Now: now, Filters: f, Settings: defaultAnalyticsSettings(), Programs: map[int64]Program{1: p}, VersionNumbers: map[int64]int{1: 1}, Cohorts: map[int64]*Cohort{1: {ID: 1, VersionID: 1, VersionNumber: 1, Status: "active", Name: "October", StartsAt: start, Timezone: "UTC", Mode: "calendar", Rules: []CohortRule{{Kind: "lesson", Key: "lesson-two", At: now.Add(7 * 24 * time.Hour).Format(time.RFC3339)}}}}, Students: []*AnalyticsStudent{{ID: 1, User: 100, Name: "Anna", Version: 1, VersionNumber: 1, CohortID: 1, Access: "active", Enrolled: start, Started: &opened, Activity: &last}, {ID: 2, User: 100, Name: "Anna", Version: 1, VersionNumber: 1, Access: "active", Enrolled: start, Started: &opened, Activity: &last}, {ID: 3, User: 200, Name: "Pavel", Version: 1, VersionNumber: 1, Access: "active", Enrolled: start}}, Progress: []analyticsProgress{{Enrollment: 1, Key: "lesson-one", Status: "in_progress", First: opened, Last: last}, {Enrollment: 2, Key: "lesson-one", Status: "completed", First: opened, Last: done, Completed: &done}, {Enrollment: 2, Key: "lesson-two", Status: "completed", First: opened, Last: done, Completed: &done}}, Events: []analyticsEvent{{Enrollment: 1, User: 100, Type: "lesson_opened", Lesson: "lesson-one", At: opened}, {Enrollment: 2, User: 100, Type: "lesson_opened", Lesson: "lesson-one", At: opened}, {Enrollment: 2, User: 100, Type: "lesson_completed", Lesson: "lesson-one", At: done}, {Enrollment: 2, User: 10, Type: "lesson_completed", Lesson: "lesson-two", At: done}}}
	d.prepare()
	return d
}
func TestAnalyticsPopulationRiskAndSchedule(t *testing.T) {
	d := analyticsFixture(t)
	o := d.overview()
	if o["enrollments"] != 3 || o["unique_students"] != 2 || o["active_students"] != 1 || o["completed"] != 1 {
		t.Fatal(o)
	}
	f := o["funnel"].([]int)
	if f[0] != 3 || f[1] != 2 || f[2] != 1 || f[5] != 1 {
		t.Fatal(f)
	}
	if d.ByID[1].Expected != 1 || d.ByID[1].Risk.Behind != 1 {
		t.Fatal("locked lesson counted", d.ByID[1])
	}
	if d.ByID[2].Scheduled || len(d.ByID[2].Risk.Reasons) != 0 {
		t.Fatal("finished self-paced learner marked at risk")
	}
	lessons := d.lessonRows("")
	if lessons[1]["stopped"] != 0 {
		t.Fatal("closed lesson counted as abandonment")
	}
	if n := lessons[0]["elapsed_hours"].(*float64); n == nil || *n != 48 {
		t.Fatal("elapsed should use actual opening event", n)
	}
	if ratio(0, 0) != nil || mean(nil) != nil || median(nil) != nil {
		t.Fatal("empty data turned into zero")
	}
	if *median([]float64{0, 100, 80, 40}) != 60 {
		t.Fatal("median")
	}
}
func TestAnalyticsFiltersDSTAndValidation(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	f, e := parseAnalyticsFilters(url.Values{"period": {"custom"}, "from": {"2026-03-29"}, "to": {"2026-03-29"}, "timezone": {"Europe/Prague"}}, now)
	if e != nil || f.until.Sub(f.from) != 23*time.Hour {
		t.Fatal(f, e)
	}
	if _, e = parseAnalyticsFilters((AnalyticsFilters{}).values(), now); e != nil {
		t.Fatal("empty report filters", e)
	}
	for _, q := range []url.Values{{"cohort": {"1 OR 1=1"}}, {"timezone": {"../private"}}, {"progress_min": {"101"}}, {"size": {"100000"}}, {"period": {"custom"}, "from": {"2020-01-01"}, "to": {"2026-10-18"}}} {
		if _, e = parseAnalyticsFilters(q, now); e == nil {
			t.Fatal("unsafe/unbounded filter", q)
		}
	}
}
func TestAnalyticsNoRegradingOrAnswerLeak(t *testing.T) {
	d := analyticsFixture(t)
	finish := d.Now.Add(-time.Hour)
	truth := true
	d.Assessments = []analyticsAssessment{{ID: 5, Version: 1, Kind: "test", Title: "Frozen test", Lesson: "lesson-one"}}
	d.Tests = []analyticsTest{{ID: 1, Enrollment: 1, Assessment: 5, Number: 1, Status: "finished", Percent: 20, Score: 1, Passed: false, Finished: &finish, Grades: []analyticsGrade{{Question: 99, Correct: &truth}}, Answered: []int64{99}}, {ID: 2, Enrollment: 2, Assessment: 5, Number: 1, Status: "finished", Percent: 80, Score: 4, Passed: true, Finished: &finish}}
	rows := d.testRows(5)
	if *rows[0]["average"].(*float64) != 50 || *rows[0]["success_rate"].(*float64) != 50 {
		t.Fatal(rows)
	}
	raw, _ := json.Marshal(rows)
	for _, private := range []string{"is_correct", "answers", "questions_snapshot", "explanation"} {
		if strings.Contains(string(raw), `"`+private+`"`) {
			t.Fatal("private answer field", private)
		}
	}
}
func TestAnalyticsCommunicationsWindowAndMultipleEnrollments(t *testing.T) {
	d := analyticsFixture(t)
	a := d.Now.Add(-5 * 24 * time.Hour)
	b := d.Now.Add(-4 * 24 * time.Hour)
	recent := d.Now.Add(-time.Hour)
	activity := b.Add(time.Hour)
	d.Events = append(d.Events, analyticsEvent{Enrollment: 1, User: 100, Type: "lesson_opened", At: activity})
	d.Deliveries = []analyticsDelivery{{ID: 1, Enrollment: 1, User: 100, Campaign: 1, Name: "First", Channel: "email", Status: "sent", Queued: a, Sent: &a}, {ID: 2, Enrollment: 2, User: 100, Campaign: 2, Name: "Second", Channel: "email", Status: "sent", Queued: b, Sent: &b}, {ID: 3, Enrollment: 3, User: 200, Campaign: 3, Name: "Recent", Channel: "email", Status: "sent", Queued: recent, Sent: &recent}, {ID: 4, Enrollment: 1, User: 100, Campaign: 4, Channel: "in_app", Status: "sent", Queued: a, Sent: &a}}
	c := d.communications()
	if c["smtp_sent"] != 3 || c["in_app_sent"] != 1 || c["enqueued"] != 4 {
		t.Fatal(c)
	}
	if c["confirmed_deliveries"].(*int) != nil {
		t.Fatal("fake delivery confirmation")
	}
	rows := c["rows"].([]analyticsRow)
	for _, r := range rows {
		switch r["key"] {
		case "campaign:1":
			if r["returned"] != 0 || r["overlap_windows"] != 1 {
				t.Fatal("same return attributed to previous reminder", r)
			}
		case "campaign:2":
			if r["returned"] != 1 {
				t.Fatal("return not found across enrollments", r)
			}
		case "campaign:3":
			if r["return_rate"].(*float64) != nil {
				t.Fatal("unfinished observation counted")
			}
		}
	}
}
func TestAnalyticsExportSafetyAndOOXML(t *testing.T) {
	rows := [][]string{{"Имя", "Результат"}, {" \t=HYPERLINK(\"https://evil\")", "100"}, {"@SUM(A1)", "Недостаточно данных"}}
	var csv bytes.Buffer
	if e := writeAnalyticsCSV(&csv, rows); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(csv.String(), "' \t=HYPERLINK") || !strings.Contains(csv.String(), "'@SUM") {
		t.Fatal("CSV formula injection", csv.String())
	}
	var buf bytes.Buffer
	if e := writeAnalyticsXLSX(&buf, rows); e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		raw, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, e = decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(f.Name, e)
			}
		}
		if strings.Contains(string(raw), "<f>") || strings.Contains(string(raw), "TargetMode=\"External\"") {
			t.Fatal("active spreadsheet content")
		}
	}
	for _, key := range []string{"../../private.csv", "abc.xlsx/../../../secrets", "abc.exe"} {
		if _, e = reportPath(t.TempDir(), key); e == nil {
			t.Fatal("unsafe report path", key)
		}
	}
}
func BenchmarkAcademyAnalyticsAggregation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		d := analyticsFixture(b)
		for n := 0; n < 1000; n++ {
			s := *d.Students[0]
			s.ID = int64(n + 10)
			d.Students = append(d.Students, &s)
		}
		d.prepare()
		d.overview()
		d.lessonRows("")
	}
}

func TestAnalyticsRelativeWindowAndHistoricalQueue(t *testing.T) {
	d := analyticsFixture(t)
	d.Filters.Compare = "relative"
	d.Filters.Days = 3
	start := d.Cohorts[1].StartsAt
	end := start.AddDate(0, 0, 3)
	submitted := start.Add(24 * time.Hour)
	reviewed := end.Add(time.Hour)
	d.Students = append(d.Students, &AnalyticsStudent{ID: 4, User: 300, Name: "Late enrollment", Version: 1, CohortID: 1, Access: "active", Enrolled: end.Add(time.Hour)})
	d.Submissions = []analyticsSubmission{{ID: 1, Enrollment: 1, Assessment: 5, Number: 1, Status: "accepted", Submitted: submitted, Reviewed: &reviewed}}
	d.Reviews = []analyticsReview{{ID: 1, Submission: 1, Action: "accepted", At: reviewed}}
	d.Events = append(d.Events, analyticsEvent{Enrollment: 1, User: 100, Type: "lesson_completed", Lesson: "lesson-one", At: submitted})
	d.prepare()
	rows := d.cohortRows()
	r := rows[0]
	if r["enrollments"] != 1 || r["pending"] != 1 || r["overdue"] != nil {
		t.Fatal("relative boundary used current queue or late enrollment", r)
	}
	if v := r["average_progress"].(*float64); v == nil || *v != 50 {
		t.Fatal("historical milestone", r)
	}
	if v := r["assignment_acceptance"].(*float64); v == nil || *v != 0 {
		t.Fatal("future review changed past ratio", r)
	}
}
func TestAnalyticsStopsRequireStudentInactivity(t *testing.T) {
	d := analyticsFixture(t)
	recent := d.Now.Add(-time.Hour)
	d.Students[0].Activity = &recent
	d.prepare()
	if d.lessonRows("")[0]["stopped"] != 0 {
		t.Fatal("active student classified as stopped")
	}
	d.Students[0].Access = "suspended"
	d.prepare()
	if analyticsStudentMatch(d.Students[0], AnalyticsFilters{StudentFilter: "inactive"}, d.Settings) {
		t.Fatal("suspended student classified as inactive learning risk")
	}
}
func TestAnalyticsStaffScopeAndNumericWorkbook(t *testing.T) {
	d := analyticsFixture(t)
	d.Staff = []analyticsStaff{{ID: 10, Name: "Owner", Role: "owner", CanReview: true}, {ID: 20, Name: "Teacher", Role: "teacher", Cohort: 1, CanReview: true}, {ID: 30, Name: "Curator", Role: "curator", Cohort: 1}}
	d.Students[0].CuratorID = 30
	rows := d.staffRows()
	if rows[0]["assigned"] != 3 || rows[1]["assigned"] != 1 || rows[2]["assigned"] != 1 {
		t.Fatal(rows)
	}
	var buf bytes.Buffer
	if e := writeAnalyticsXLSX(&buf, [][]string{{"Ученик", "Прогресс, %"}, {"=2+2", "50"}}); e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range z.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			r, _ := f.Open()
			raw, _ := io.ReadAll(r)
			r.Close()
			if !strings.Contains(string(raw), `<c r="B2"><v>50</v></c>`) || strings.Contains(string(raw), "<f>") {
				t.Fatal(string(raw))
			}
		}
	}
}

func TestAnalyticsContextDoesNotInventUnfinishedResults(t *testing.T) {
	d := analyticsFixture(t)
	d.Tests = []analyticsTest{{ID: 1, Enrollment: 1, Status: "started"}}
	value, err := d.studentContext(1)
	if err != nil {
		t.Fatal(err)
	}
	attempt := value["tests"].([]analyticsRow)[0]
	if attempt["percent"] != nil || attempt["passed"] != nil {
		t.Fatal("unfinished attempt has a fabricated result", attempt)
	}
}
