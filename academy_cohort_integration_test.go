package main

import (
	"FinTalent/internal/academy"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func testAcademyStageD(t *testing.T, db *sql.DB, mux *http.ServeMux, owner, student, course int64) {
	t.Helper()
	base := fmt.Sprintf("/api/academy/courses/%d", course)
	call := func(method, path string, user int64, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	request := func(method, path string, user int64, body any, status int) []byte {
		t.Helper()
		w := call(method, path, user, body)
		if w.Code != status {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var curator, teacher, student2 int64
	for i, dst := range []*int64{&curator, &teacher, &student2} {
		if err := db.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES($1,$2,repeat('*',60)) RETURNING id`, fmt.Sprintf("Academy D user %d", i), fmt.Sprintf("academy-d-%d@example.invalid", i)).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	var p academy.Program
	_ = json.Unmarshal(request("GET", base+"/program", owner, nil, 200), &p)
	m := academy.NewModule()
	m.Status = "published"
	l := academy.NewLesson()
	l.Status = "published"
	l.Title = "Cohort lesson"
	l.Required = true
	l.SelfPaced = true
	block := academy.Block{Key: academy.Key(), Type: "assignment", Data: academy.BlockData{Title: "Cohort task", Text: "Submit text", AnswerFormat: "text", Assessment: &academy.AssessmentPolicy{MaxAttempts: 3, MaxScore: 100, Formats: []string{"text"}}}}
	l.Blocks = []academy.Block{{Key: academy.Key(), Type: "text", Data: academy.BlockData{Text: "COHORT_PRIVATE_CONTENT"}}, block}
	m.Lessons = []academy.Lesson{l}
	p.Modules = []academy.Module{m}
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	var upload bytes.Buffer
	mw := multipart.NewWriter(&upload)
	_ = mw.WriteField("entity_key", l.Key)
	part, _ := mw.CreateFormFile("file", "cohort-private.pdf")
	_, _ = part.Write([]byte("%PDF-1.7\nCohort private file"))
	_ = mw.Close()
	ur := httptest.NewRequest("POST", base+"/assets", &upload)
	ur.Header.Set("Content-Type", mw.FormDataContentType())
	ur.Header.Set("X-Test-User", strconv.FormatInt(owner, 10))
	uw := httptest.NewRecorder()
	mux.ServeHTTP(uw, ur)
	if uw.Code != 201 {
		t.Fatal(uw.Code, uw.Body.String())
	}
	var asset struct{ ID int64 }
	_ = json.Unmarshal(uw.Body.Bytes(), &asset)
	p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, academy.Block{Key: academy.Key(), Type: "file", Data: academy.BlockData{AssetID: asset.ID}})
	var testVersion int64
	_ = db.QueryRow(`SELECT v.id FROM test_versions v JOIN tests t ON t.id=v.test_id WHERE t.author_id=$1 AND EXISTS(SELECT 1 FROM test_questions q WHERE q.test_version_id=v.id) ORDER BY v.id DESC LIMIT 1`, owner).Scan(&testVersion)
	if testVersion == 0 {
		var tid, qid int64
		if err := db.QueryRow(`INSERT INTO tests(author_id,slug) VALUES($1,$2) RETURNING id`, owner, academy.Key()).Scan(&tid); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,1,'Cohort test',$2) RETURNING id`, tid, owner).Scan(&testVersion); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO test_questions(test_version_id,question,question_type,points) VALUES($1,'Question','single_choice',1) RETURNING id`, testVersion).Scan(&qid); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'Correct',true)`, qid); err != nil {
			t.Fatal(err)
		}
	}
	optional := false
	testBlock := academy.Block{Key: academy.Key(), Type: "test", Data: academy.BlockData{TestVersionID: testVersion, Assessment: &academy.AssessmentPolicy{Required: &optional, MaxAttempts: 1}}}
	p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, testBlock)
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	request("POST", base+"/versions", owner, map[string]any{"revision": p.Revision}, 200)
	var version int64
	if err := db.QueryRow(`SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, p.ID, p.Revision).Scan(&version); err != nil {
		t.Fatal(err)
	}
	request("POST", base+"/releases", owner, map[string]any{"version_id": version}, 200)
	create := func(name string, capacity int) academy.Cohort {
		t.Helper()
		c := academy.Cohort{Name: name, Description: "Real cohort", Type: "group", VersionID: version, StartsAt: time.Now().Add(-24 * time.Hour), Timezone: "Europe/Prague", Capacity: &capacity, Mode: "calendar", Rules: []academy.CohortRule{{Kind: "module", Key: m.Key, At: time.Now().Add(time.Hour).Format(time.RFC3339)}}, RequestKey: academy.Key()}
		raw := request("POST", base+"/cohorts", owner, c, 200)
		var got academy.Cohort
		_ = json.Unmarshal(raw, &got)
		var again academy.Cohort
		_ = json.Unmarshal(request("POST", base+"/cohorts", owner, c, 200), &again)
		if got.ID != again.ID {
			t.Fatal("cohort create not idempotent")
		}
		return got
	}
	c := create("October", 10)
	cp := fmt.Sprintf("%s/cohorts/%d", base, c.ID)
	request("GET", cp, student, nil, 404)
	request("POST", base+"/cohorts", student, c, 404)
	foreign := c
	foreign.VersionID = 9223372036854775806
	foreign.RequestKey = academy.Key()
	request("POST", base+"/cohorts", owner, foreign, 404)
	var enrolled struct {
		IDs []int64 `json:"ids"`
	}
	_ = json.Unmarshal(request("POST", cp+"/enroll", owner, map[string]any{"user_ids": []int64{student, student2}, "request_key": academy.Key()}, 200), &enrolled)
	if len(enrolled.IDs) != 2 {
		t.Fatal("bulk enrollment failed")
	}
	eid := enrolled.IDs[0]
	ep := fmt.Sprintf("/api/academy/enrollments/%d", eid)
	locked := request("GET", ep, student, nil, 200)
	if bytes.Contains(locked, []byte("COHORT_PRIVATE_CONTENT")) {
		t.Fatal("draft materials leaked")
	}
	request("POST", ep+"/lessons/"+l.Key+"/open", student, map[string]any{"event_key": academy.Key()}, 403)
	c.Status = "active"
	_ = json.Unmarshal(request("PUT", cp, owner, c, 200), &c)
	request("GET", ep+"/lessons/"+l.Key, student, nil, 403)
	request("GET", ep+"/lessons/"+l.Key+"/assessments/"+block.Key, student, nil, 403)
	request("GET", ep+"/lessons/"+l.Key+"/assessments/"+testBlock.Key, student, nil, 403)
	assetPath := fmt.Sprintf("%s/assets/%d?lesson=%s", ep, asset.ID, l.Key)
	request("GET", assetPath, student, nil, 403)
	request("POST", ep+"/lessons/"+l.Key+"/assessments/"+block.Key+"/draft", student, map[string]any{"request_key": academy.Key()}, 403)
	// An explicit lesson rule overrides the later module rule.
	c.Rules = append(c.Rules, academy.CohortRule{Kind: "lesson", Key: l.Key, At: time.Now().Add(-time.Hour).Format(time.RFC3339)})
	c.Deadlines = []academy.CohortRule{{Kind: "assignment", Key: block.Key, At: time.Now().Add(-time.Minute).Format(time.RFC3339), AllowLate: false}}
	_ = json.Unmarshal(request("PUT", cp, owner, c, 200), &c)
	if !bytes.Contains(request("GET", ep, student, nil, 200), []byte("COHORT_PRIVATE_CONTENT")) {
		t.Fatal("available content missing")
	}
	request("POST", ep+"/lessons/"+l.Key+"/open", student, map[string]any{"event_key": academy.Key()}, 200)
	request("GET", assetPath, student, nil, 200)
	request("GET", assetPath, student2, nil, 404)
	ap := ep + "/lessons/" + l.Key + "/assessments/" + block.Key
	var state struct {
		Submissions []academy.Submission `json:"submissions"`
	}
	_ = json.Unmarshal(request("POST", ap+"/draft", student, map[string]any{"request_key": academy.Key()}, 200), &state)
	s := state.Submissions[0]
	sp := fmt.Sprintf("%s/submissions/%d", ap, s.ID)
	_ = json.Unmarshal(request("PUT", sp, student, map[string]any{"revision": s.Revision, "answer": map[string]any{"text": "Real homework", "links": []any{}, "asset_ids": []any{}}}, 200), &state)
	s = state.Submissions[0]
	request("POST", sp+"/submit", student, map[string]any{"revision": s.Revision}, 403)
	c.Deadlines[0].AllowLate = true
	_ = json.Unmarshal(request("PUT", cp, owner, c, 200), &c)
	_ = json.Unmarshal(request("POST", sp+"/submit", student, map[string]any{"revision": s.Revision}, 200), &state)
	s = state.Submissions[0]
	request("PUT", cp+"/staff", owner, map[string]any{"user_id": curator, "role": "curator", "can_review": true}, 200)
	request("PUT", cp+"/staff", owner, map[string]any{"user_id": teacher, "role": "teacher", "can_review": false}, 200)
	request("GET", base+"/program", curator, nil, 403)
	request("GET", base+"/crm", curator, nil, 200)
	rp := fmt.Sprintf("%s/reviews/%d", base, s.ID)
	request("GET", rp, curator, nil, 404)
	request("PUT", fmt.Sprintf("%s/crm/%d/curator", base, eid), owner, map[string]any{"user_id": curator}, 200)
	request("GET", rp, curator, nil, 200)
	request("GET", rp, teacher, nil, 404)
	raw := request("GET", base+"/crm", curator, nil, 200)
	var rows []struct {
		EnrollmentID int64 `json:"enrollment_id"`
	}
	_ = json.Unmarshal(raw, &rows)
	if len(rows) != 1 || rows[0].EnrollmentID != eid {
		t.Fatal("curator saw unassigned students", string(raw))
	}
	request("GET", fmt.Sprintf("%s/crm/%d", base, enrolled.IDs[1]), curator, nil, 404)
	request("GET", cp+"/preview", curator, nil, 200)
	closedGate := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { http.Error(w, "beta closed", 403) }
	}
	staffHandler := academy.New(db, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, closedGate, t.TempDir())
	for _, scenario := range []struct {
		path  string
		actor int64
		code  int
	}{{fmt.Sprintf("/profile/courses/%d/academy/cohorts/%d", course, c.ID), curator, 200}, {fmt.Sprintf("/profile/courses/%d/edit", course), curator, 403}, {fmt.Sprintf("/profile/courses/%d/academy", course), curator, 403}, {fmt.Sprintf("/profile/courses/%d/academy/crm", course), student2, 403}} {
		r := httptest.NewRequest("GET", scenario.path, nil)
		r.Header.Set("X-Test-User", strconv.FormatInt(scenario.actor, 10))
		w := httptest.NewRecorder()
		staffHandler.CoursePage(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })(w, r)
		if w.Code != scenario.code {
			t.Fatalf("staff page gate %s got %d", scenario.path, w.Code)
		}
	}
	var sqlTime time.Time
	if err := db.QueryRow(`SELECT academy_cohort_rule_at('2026-03-28T09:00:00Z'::timestamptz,'Europe/Prague','{"day":1,"clock":"10:00"}'::jsonb)`).Scan(&sqlTime); err != nil || sqlTime.UTC().Format(time.RFC3339) != "2026-03-29T08:00:00Z" {
		t.Fatal("SQL DST rule", sqlTime, err)
	}
	request("GET", base+"/releases", curator, nil, 403)
	request("PUT", cp, curator, c, 403)
	request("GET", "/api/academy/workspaces", curator, nil, 200)
	np := fmt.Sprintf("%s/crm/%d/notes", base, eid)
	var n struct{ ID int64 }
	_ = json.Unmarshal(request("POST", np, curator, map[string]any{"text": "PRIVATE_CURATOR_NOTE"}, 200), &n)
	request("PUT", fmt.Sprintf("%s/%d", np, n.ID), curator, map[string]any{"text": "Edited private note", "revision": 1}, 200)
	request("PUT", fmt.Sprintf("%s/%d", np, n.ID), curator, map[string]any{"text": "stale", "revision": 1}, 409)
	if bytes.Contains(request("GET", ep, student, nil, 200), []byte("PRIVATE_CURATOR_NOTE")) {
		t.Fatal("notes leaked to learner")
	}
	request("DELETE", fmt.Sprintf("%s/%d", np, n.ID), owner, map[string]any{"revision": 2}, 200)
	var tag struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/crm/tags", owner, map[string]any{"name": "VIP", "color": "teal"}, 200), &tag)
	request("PUT", fmt.Sprintf("%s/crm/%d/tags", base, eid), owner, map[string]any{"tag_ids": []int64{tag.ID}}, 200)
	request("POST", base+"/crm/tags", curator, map[string]any{"name": "Forbidden"}, 403)
	request("PUT", fmt.Sprintf("%s/crm/tags/%d", base, tag.ID), owner, map[string]any{"name": "VIP updated", "color": "purple"}, 200)
	request("DELETE", fmt.Sprintf("%s/crm/tags/%d", base, tag.ID), owner, map[string]any{}, 200)
	request("POST", rp, curator, map[string]any{"status": "accepted", "revision": s.Revision, "score": 90, "comment": "Accepted", "request_key": academy.Key()}, 200)
	var progress academy.Enrollment
	_ = json.Unmarshal(request("GET", ep+"/progress", student, nil, 200), &progress)
	if progress.Summary.Percent != 100 {
		t.Fatal("curator review did not reconcile progress", progress.Summary)
	}
	// Version lock, atomic capacity and old results survive a safe transfer.
	var anotherVersion int64
	_ = db.QueryRow(`SELECT version_id FROM academy_version_releases WHERE course_id=$1 AND version_id<>$2 LIMIT 1`, course, version).Scan(&anotherVersion)
	if anotherVersion > 0 {
		bad := c
		bad.VersionID = anotherVersion
		bad.Rules = nil
		bad.Deadlines = nil
		request("PUT", cp, owner, bad, 409)
	}
	small := create("Capacity", 1)
	smallPath := fmt.Sprintf("%s/cohorts/%d", base, small.ID)
	codes := make(chan int, 2)
	for _, uid := range []int64{student, student2} {
		go func(uid int64) {
			codes <- call("POST", smallPath+"/enroll", owner, map[string]any{"user_ids": []int64{uid}, "request_key": academy.Key()}).Code
		}(uid)
	}
	a, b := <-codes, <-codes
	if !((a == 200 && b == 409) || (a == 409 && b == 200)) {
		t.Fatalf("concurrent capacity failed %d %d", a, b)
	}
	destination := create("November", 10)
	dp := fmt.Sprintf("%s/cohorts/%d", base, destination.ID)
	request("POST", cp+"/enroll", owner, map[string]any{"user_ids": []int64{student}, "request_key": academy.Key(), "transfer_enrollment_id": enrolled.IDs[1]}, 409)
	transfer := map[string]any{"user_ids": []int64{student}, "request_key": academy.Key(), "transfer_enrollment_id": eid}
	request("POST", dp+"/enroll", owner, transfer, 200)
	request("POST", dp+"/enroll", owner, transfer, 200)
	request("POST", ep+"/lessons/"+l.Key+"/open", student, map[string]any{"event_key": academy.Key()}, 403)
	var count int
	_ = db.QueryRow(`SELECT count(*) FROM academy_assignment_submissions WHERE enrollment_id=$1 AND status='accepted'`, eid).Scan(&count)
	if count != 1 {
		t.Fatal("transfer destroyed history")
	}
	_ = db.QueryRow(`SELECT count(*) FROM academy_enrollments WHERE cohort_id=$1 AND user_id=$2`, destination.ID, student).Scan(&count)
	if count != 1 {
		t.Fatal("transfer retry duplicated enrollment")
	}
	c.Status = "completed"
	_ = json.Unmarshal(request("PUT", cp, owner, c, 200), &c)
	c.Status = "archived"
	_ = json.Unmarshal(request("PUT", cp, owner, c, 200), &c)
	request("POST", cp+"/enroll", owner, map[string]any{"user_ids": []int64{teacher}, "request_key": academy.Key()}, 409)
	// Revoking staff removes CRM and review access; historical reviews remain intact.
	request("DELETE", cp+"/staff", owner, map[string]any{"user_id": curator}, 200)
	request("GET", rp, curator, nil, 404)
	if err := academy.DeliverCohortNotifications(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT count(*) FROM notifications WHERE type LIKE 'academy_%'`).Scan(&count)
	if err := academy.DeliverCohortNotifications(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var after int
	_ = db.QueryRow(`SELECT count(*) FROM notifications WHERE type LIKE 'academy_%'`).Scan(&after)
	if count != after {
		t.Fatal("duplicate notifications")
	}
	request("GET", cp, owner, nil, 200)
	request("GET", base+"/cohorts", owner, nil, 200)
	request("GET", fmt.Sprintf("%s/crm/%d", base, eid), owner, nil, 200)
}
