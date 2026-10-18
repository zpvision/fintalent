package main

import (
	"FinTalent/internal/academy"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAcademyAnalyticsIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	db := f.DB
	ctx := context.Background()
	if e := prepareAcademyDatabase(ctx); e != nil {
		t.Fatal("repeat migration", e)
	}
	now := time.Now().UTC()
	old := now.Add(-10 * 24 * time.Hour)
	last := now.Add(-7 * 24 * time.Hour)
	finished := now.Add(-6 * 24 * time.Hour)
	no := false
	p := academy.Program{ID: f.Program, CourseID: f.Course, Revision: 2, Source: "empty", Modules: []academy.Module{{Key: "module-alpha", Title: "Module", Status: "published", Lessons: []academy.Lesson{{Key: "lesson-alpha", Title: "Introduction", Status: "published", Required: true, SelfPaced: true, CompletionRule: "manual", Blocks: []academy.Block{{Key: "test-alpha", Type: "test", Data: academy.BlockData{Title: "Frozen test", Assessment: &academy.AssessmentPolicy{Required: &no}}}, {Key: "assignment-alpha", Type: "assignment", Data: academy.BlockData{Title: "Practical task", Assessment: &academy.AssessmentPolicy{Required: &no, MaxScore: 100, Criteria: []academy.Criterion{{Key: "criterion-alpha", Title: "Accuracy", MaxScore: 100}}}}}}}, {Key: "lesson-bravo", Title: "Future lesson", Status: "published", Required: true, SelfPaced: true, CompletionRule: "manual", Blocks: []academy.Block{{Key: "text-bravo", Type: "text", Data: academy.BlockData{Text: "Learning"}}}}}}}}
	raw, _ := json.Marshal(p)
	var version int64
	if e := db.QueryRow(`INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,2,2,$2,$3) RETURNING id`, f.Program, raw, f.Owner).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO academy_version_releases(course_id,version_id,published_by) VALUES($1,$2,$3)`, f.Course, version, f.Owner); e != nil {
		t.Fatal(e)
	}
	var cohort, otherCohort int64
	rules, _ := json.Marshal([]academy.CohortRule{{Kind: "lesson", Key: "lesson-bravo", At: now.Add(7 * 24 * time.Hour).Format(time.RFC3339)}})
	if e := db.QueryRow(`INSERT INTO academy_cohorts(course_id,program_version_id,name,type,status,starts_at,timezone,schedule_mode,rules,request_key,created_by) VALUES($1,$2,'October','group','active',$3,'UTC','calendar',$4,$5,$6) RETURNING id`, f.Course, version, old, rules, academy.Key(), f.Owner).Scan(&cohort); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(`INSERT INTO academy_cohorts(course_id,program_version_id,name,type,status,starts_at,timezone,schedule_mode,request_key,created_by) VALUES($1,$2,'November','group','active',$3,'UTC','all',$4,$5) RETURNING id`, f.Course, f.Version, now.Add(-24*time.Hour), academy.Key(), f.Owner).Scan(&otherCohort); e != nil {
		t.Fatal(e)
	}
	var e1, e2, e3 int64
	for i, item := range []struct {
		uid, cid int64
		dest     *int64
	}{{f.Student, cohort, &e1}, {f.Student, 0, &e2}, {f.Teacher, cohort, &e3}} {
		if e := db.QueryRow(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,cohort_id,enrolled_by,request_key,enrolled_at,last_activity_at,started_at) VALUES($1,$2,$3,NULLIF($4,0),$5,$6,$7,$8,$8) RETURNING id`, f.Course, item.uid, version, item.cid, f.Owner, academy.Key(), old, last).Scan(item.dest); e != nil {
			t.Fatal(i, e)
		}
	}
	if _, e := db.Exec(`INSERT INTO academy_cohort_staff(cohort_id,user_id,role,can_review,can_communicate,assigned_by) VALUES($1,$2,'curator',true,false,$3)`, cohort, f.Teacher, f.Owner); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO academy_student_curators(enrollment_id,user_id,assigned_by) VALUES($1,$2,$3)`, e1, f.Teacher, f.Owner); e != nil {
		t.Fatal(e)
	}
	for _, row := range []struct {
		eid            int64
		lesson, status string
		done           *time.Time
	}{{e1, "lesson-alpha", "in_progress", nil}, {e2, "lesson-alpha", "completed", &finished}, {e2, "lesson-bravo", "completed", &finished}} {
		if _, e := db.Exec(`INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status,first_opened_at,last_opened_at,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, row.eid, version, row.lesson, row.status, old, last, row.done); e != nil {
			t.Fatal(e)
		}
	}
	for _, row := range []struct {
		eid, actor   int64
		lesson, kind string
		at           time.Time
	}{{e1, f.Student, "lesson-alpha", "lesson_opened", old}, {e2, f.Student, "lesson-alpha", "lesson_opened", old}, {e2, f.Student, "lesson-alpha", "lesson_completed", finished}, {e2, f.Owner, "lesson-bravo", "lesson_completed", finished}} {
		if _, e := db.Exec(`INSERT INTO academy_learning_events(enrollment_id,actor_id,lesson_key,type,event_key,created_at) VALUES($1,$2,$3,$4,$5,$6)`, row.eid, row.actor, row.lesson, row.kind, academy.Key(), row.at); e != nil {
			t.Fatal(e)
		}
	}
	var testID, assignment int64
	questions := `[{"id":9001,"question":"Frozen question","points":1,"answers":[{"id":901,"answer":"NEVER_RETURN_SOLUTION","is_correct":true}]}]`
	config, _ := json.Marshal(p.Modules[0].Lessons[0].Blocks[0].Data)
	if e := db.QueryRow(`INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,questions,title) VALUES($1,'lesson-alpha','test-alpha','test',$2,$3,'Frozen test') RETURNING id`, version, config, questions).Scan(&testID); e != nil {
		t.Fatal(e)
	}
	config, _ = json.Marshal(p.Modules[0].Lessons[0].Blocks[1].Data)
	if e := db.QueryRow(`INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,title) VALUES($1,'lesson-alpha','assignment-alpha','assignment',$2,'Practical task') RETURNING id`, version, config).Scan(&assignment); e != nil {
		t.Fatal(e)
	}
	for i, eid := range []int64{e1, e2} {
		grades := fmt.Sprintf(`[{"question_id":9001,"is_correct":%t,"earned_points":%d}]`, i == 1, i)
		if _, e := db.Exec(`INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,status,percent,score,max_score,passed,started_at,finished_at,grades,answers) VALUES($1,$2,$3,1,$4,'finished',$5,$5,100,$6,$7,$8,$9,'[{"question_id":9001,"selected_answer_id":901}]')`, eid, version, testID, academy.Key(), float64(20+i*60), i == 1, old, last, grades); e != nil {
			t.Fatal(e)
		}
	}
	var submission int64
	if e := db.QueryRow(`INSERT INTO academy_assignment_submissions(enrollment_id,program_version_id,assessment_id,number,request_key,status,submitted_at,reviewed_at,reviewed_by,score) VALUES($1,$2,$3,1,$4,'revision_requested',$5,$6,$7,40) RETURNING id`, e1, version, assignment, academy.Key(), old, last, f.Owner).Scan(&submission); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO academy_submission_reviews(submission_id,reviewer_id,source,request_key,action,score,criterion_scores,created_at) VALUES($1,$2,'manual',$3,'revision_requested',40,'{"criterion-alpha":40}',$4)`, submission, f.Owner, academy.Key(), last); e != nil {
		t.Fatal(e)
	}
	base := fmt.Sprintf("/api/academy/courses/%d/analytics", f.Course)
	request := func(method, path string, actor int64, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if actor > 0 {
			r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
		}
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	result := func(path string, actor int64) map[string]any {
		t.Helper()
		var v map[string]any
		w := request("GET", base+path, actor, nil, 200)
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		if strings.Contains(w.Body.String(), "NEVER_RETURN_SOLUTION") {
			t.Fatal("solution leaked")
		}
		return v
	}
	request("GET", base+"/overview", 0, nil, 401)
	request("GET", base+"/overview", f.Student, nil, 404)
	overview := result("/overview", f.Owner)
	metrics := overview["data"].(map[string]any)
	if metrics["enrollments"] != float64(3) || metrics["unique_students"] != float64(2) || metrics["completed"] != float64(1) {
		t.Fatal(metrics)
	}
	limited := result("/overview", f.Teacher)["data"].(map[string]any)
	if limited["enrollments"] != float64(1) || limited["unique_students"] != float64(1) {
		t.Fatal("curator aggregate leak", limited)
	}
	request("GET", base+"/communications", f.Teacher, nil, 403)
	request("GET", base+"/overview?cohort=9999999", f.Owner, nil, 404)
	request("GET", base+"/overview?version=9999999", f.Owner, nil, 404)
	request("GET", base+"/students?cohort=1%20OR%201=1", f.Owner, nil, 400)
	tests := result(fmt.Sprintf("/tests/%d", testID), f.Owner)["rows"].([]any)
	test := tests[0].(map[string]any)
	if test["average"] != float64(50) || test["median"] != float64(50) || test["success_rate"] != float64(50) {
		t.Fatal(test)
	}
	result(fmt.Sprintf("/assignments/%d", assignment), f.Owner)
	result("/modules", f.Owner)
	result("/cohorts?compare=relative&days=14", f.Owner)
	result("/staff", f.Owner)
	result("/communications", f.Owner)
	lessons := result(fmt.Sprintf("/lessons/lesson-bravo?version=%d", version), f.Owner)["rows"].([]any)
	if lessons[0].(map[string]any)["available"] != float64(1) || lessons[0].(map[string]any)["stopped"] != float64(0) {
		t.Fatal("schedule ignored", lessons)
	}
	context := result(fmt.Sprintf("/context/%d", e1), f.Teacher)
	if context["student"].(map[string]any)["enrollment_id"] != float64(e1) || context["communications_available"] != false {
		t.Fatal(context)
	}
	lessonDetail := result(fmt.Sprintf("/lessons/lesson-alpha?version=%d&student_page=2", version), f.Owner)["rows"].([]any)[0].(map[string]any)
	if lessonDetail["student_page"] != float64(2) || len(lessonDetail["students"].([]any)) != 0 {
		t.Fatal("lesson detail pagination", lessonDetail)
	}
	request("GET", fmt.Sprintf("%s/lessons/lesson-alpha?version=%d&student_page=0", base, version), f.Owner, nil, 400)
	request("GET", fmt.Sprintf("%s/context/%d", base, e2), f.Teacher, nil, 403)
	page := result("/students?size=1&page=2", f.Owner)
	if page["total"] != float64(3) || len(page["rows"].([]any)) != 1 {
		t.Fatal("pagination", page)
	}
	qualified := result("/students?progress_min=75", f.Owner)
	if qualified["total"] != float64(1) {
		t.Fatal("funnel detail filter", qualified)
	}
	result("/overview?period=7", f.Owner)
	settings := academy.AnalyticsSettings{InactiveDays: 3, GraceDays: 1, FailedTests: 2, Revisions: 1, Overdue: 1, Behind: 1, HighSignals: 2, ReturnDays: 3}
	request("PUT", base+"/settings", f.Teacher, settings, 403)
	request("PUT", base+"/settings", f.Owner, settings, 200)
	request("PUT", base+"/settings", f.Owner, settings, 409)
	create := func(actor int64, kind, format, key string) int64 {
		t.Helper()
		w := request("POST", base+"/reports", actor, map[string]any{"kind": kind, "format": format, "request_key": key, "filters": academy.AnalyticsFilters{}}, 202)
		var job struct{ ID int64 }
		_ = json.Unmarshal(w.Body.Bytes(), &job)
		return job.ID
	}
	key := academy.Key()
	csv := create(f.Owner, "students", "csv", key)
	if create(f.Owner, "students", "csv", key) != csv {
		t.Fatal("duplicate job")
	}
	request("POST", base+"/reports", f.Owner, map[string]any{"kind": "tests", "format": "csv", "request_key": key, "filters": academy.AnalyticsFilters{}}, 409)
	xlsx := create(f.Owner, "performance", "xlsx", academy.Key())
	workers := make(chan error, 2)
	for range 2 {
		go func() { workers <- academy.ProcessAnalyticsReports(ctx, db, f.Root) }()
	}
	for range 2 {
		if e := <-workers; e != nil {
			t.Fatal(e)
		}
	}
	file := request("GET", fmt.Sprintf("%s/reports/%d/file", base, csv), f.Owner, nil, 200)
	if !strings.Contains(file.Header().Get("Cache-Control"), "no-store") || !strings.Contains(file.Body.String(), "Stage E 1") {
		t.Fatal("CSV export", file.Body.String())
	}
	excel := request("GET", fmt.Sprintf("%s/reports/%d/file", base, xlsx), f.Owner, nil, 200)
	if _, e := zip.NewReader(bytes.NewReader(excel.Body.Bytes()), int64(excel.Body.Len())); e != nil {
		t.Fatal("invalid XLSX", e)
	}
	request("GET", fmt.Sprintf("%s/reports/%d/file", base, csv), f.Teacher, nil, 404)
	ownReport := create(f.Teacher, "students", "csv", academy.Key())
	if e := academy.ProcessAnalyticsReports(ctx, db, f.Root); e != nil {
		t.Fatal(e)
	}
	request("GET", fmt.Sprintf("%s/reports/%d/file", base, ownReport), f.Teacher, nil, 200)
	if _, e := db.Exec(`DELETE FROM academy_student_curators WHERE enrollment_id=$1`, e1); e != nil {
		t.Fatal(e)
	}
	request("GET", fmt.Sprintf("%s/reports/%d/file", base, ownReport), f.Teacher, nil, 403)
	if _, e := db.Exec(`UPDATE academy_report_jobs SET expires_at=now()-INTERVAL '1 minute' WHERE id=$1`, csv); e != nil {
		t.Fatal(e)
	}
	request("GET", fmt.Sprintf("%s/reports/%d/file", base, csv), f.Owner, nil, 409)
	expiredJob := request("GET", fmt.Sprintf("%s/reports/%d", base, csv), f.Owner, nil, 200)
	if !strings.Contains(expiredJob.Body.String(), `"status":"expired"`) {
		t.Fatal("expired report metadata", expiredJob.Body.String())
	}
	if e := academy.ProcessAnalyticsReports(ctx, db, f.Root); e != nil {
		t.Fatal(e)
	}
	request("DELETE", fmt.Sprintf("%s/reports/%d", base, xlsx), f.Owner, nil, 200)
	// A thousand additional real SQL fixtures measure aggregate reads and pagination without production data.
	if _, e := db.Exec(`INSERT INTO users(full_name,email,password_hash) SELECT 'Analytics student '||n,'analytics-f-'||n||'@example.invalid',repeat('*',60) FROM generate_series(1,1000) n`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key,enrolled_at) SELECT $1,id,$2,$3,'analytics-fixture-'||id,$4 FROM users WHERE email LIKE 'analytics-f-%@example.invalid'`, f.Course, version, f.Owner, old); e != nil {
		t.Fatal(e)
	}
	begin := time.Now()
	large := result("/overview", f.Owner)
	if large["data"].(map[string]any)["enrollments"] != float64(1003) {
		t.Fatal("large aggregation")
	}
	t.Logf("1003 enrollments: %s; server timing %.0f ms", time.Since(begin), large["elapsed_ms"])
	largeJob := create(f.Owner, "progress", "xlsx", academy.Key())
	if _, e := db.Exec(`UPDATE academy_report_jobs SET status='running',started_at=now()-INTERVAL '11 minutes',lease_key='stale-lease' WHERE id=$1`, largeJob); e != nil {
		t.Fatal(e)
	}
	if e := academy.ProcessAnalyticsReports(ctx, db, f.Root); e != nil {
		t.Fatal(e)
	}
	request("GET", fmt.Sprintf("%s/reports/%d/file", base, largeJob), f.Owner, nil, 200)
	var audit int
	if e := db.QueryRow(`SELECT count(*) FROM academy_communication_audit WHERE entity='report' AND action='downloaded' AND course_id=$1`, f.Course).Scan(&audit); e != nil || audit < 4 {
		t.Fatal("download audit", audit, e)
	}
}
