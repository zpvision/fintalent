package main

import (
	"FinTalent/internal/academy"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func testAcademyStageC(t *testing.T, db *sql.DB, mux *http.ServeMux, owner, student, course int64) {
	request := func(method, path string, user int64, body any, status int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if user > 0 {
			r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	base := fmt.Sprintf("/api/academy/courses/%d", course)
	var testID, testVersion, q1, q2, correct, wrong int64
	for _, step := range []struct {
		q    string
		args []any
		id   *int64
	}{
		{`INSERT INTO tests(author_id,slug) VALUES($1,$2) RETURNING id`, []any{owner, academy.Key()}, &testID},
		{`INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,1,'Academy C test',$2) RETURNING id`, nil, &testVersion},
	} {
		args := step.args
		if args == nil {
			args = []any{testID, owner}
		}
		if err := db.QueryRow(step.q, args...).Scan(step.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(`INSERT INTO test_questions(test_version_id,question,question_type,explanation,points,sort_order) VALUES($1,'Choose','single_choice','SECRET_EXPLANATION',10,1) RETURNING id`, testVersion).Scan(&q1); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO test_questions(test_version_id,question,question_type,points,sort_order) VALUES($1,'Write','text',10,2) RETURNING id`, testVersion).Scan(&q2); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'Correct',true) RETURNING id`, q1).Scan(&correct); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'Wrong',false) RETURNING id`, q1).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'SECRET_CORRECT_TEXT',true)`, q2); err != nil {
		t.Fatal(err)
	}
	var professionalBefore int
	_ = db.QueryRow(`SELECT count(*) FROM test_attempts`).Scan(&professionalBefore)
	var p academy.Program
	_ = json.Unmarshal(request("GET", base+"/program", owner, nil, 200), &p)
	required, pass, best := true, 70.0, true
	m := academy.NewModule()
	m.Status = "published"
	l := academy.NewLesson()
	l.Title = "Test and assignment"
	l.Status = "published"
	testBlock := academy.Block{Key: academy.Key(), Type: "test", Data: academy.BlockData{TestVersionID: testVersion, Assessment: &academy.AssessmentPolicy{Required: &required, MaxAttempts: 2, PassingPercent: &pass, TimeLimitSeconds: 300, ShowCorrect: true, ShowExplanations: true, UseBest: &best}}}
	c1, c2 := academy.Key(), academy.Key()
	assignment := academy.Block{Key: academy.Key(), Type: "assignment", Data: academy.BlockData{Title: "Practical work", AnswerFormat: "text", Text: "Complete all formats", Assessment: &academy.AssessmentPolicy{Required: &required, MaxAttempts: 2, MaxScore: 100, PassingScore: &pass, Formats: []string{"text", "file", "link"}, Criteria: []academy.Criterion{{Key: c1, Title: "Accuracy", MaxScore: 60}, {Key: c2, Title: "Completeness", MaxScore: 40}}}}}
	l.Blocks = []academy.Block{testBlock, assignment}
	m.Lessons = []academy.Lesson{l}
	timed := academy.NewLesson()
	timed.Status = "published"
	timed.Required = false
	timed.Title = "Timed"
	timerBlock := testBlock
	timerBlock.Key = academy.Key()
	timerBlock.Data.Assessment = &academy.AssessmentPolicy{MaxAttempts: 1, TimeLimitSeconds: 1}
	timed.Blocks = []academy.Block{timerBlock}
	m.Lessons = append(m.Lessons, timed)
	p.Modules = []academy.Module{m}
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	request("POST", base+"/versions", owner, map[string]int64{"revision": p.Revision}, 200)
	var version int64
	_ = db.QueryRow(`SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, p.ID, p.Revision).Scan(&version)
	request("POST", base+"/releases", owner, map[string]any{"version_id": version}, 200)
	var enrolled struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, map[string]any{"user_id": student, "version_id": version, "request_key": academy.Key()}, 200), &enrolled)
	ep := fmt.Sprintf("/api/academy/enrollments/%d", enrolled.ID)
	tp := ep + "/lessons/" + l.Key + "/assessments/" + testBlock.Key
	ap := ep + "/lessons/" + l.Key + "/assessments/" + assignment.Key
	request("POST", ep+"/lessons/"+l.Key+"/open", student, map[string]string{"event_key": academy.Key()}, 200)
	request("POST", ep+"/lessons/"+l.Key+"/complete", student, map[string]bool{}, 400)
	raw := request("GET", tp, student, nil, 200)
	if bytes.Contains(raw, []byte("SECRET_")) || bytes.Contains(raw, []byte(`"question_type"`)) {
		t.Fatal("questions leaked before start")
	}
	var state struct {
		Attempts    []academy.AcademyTestAttempt `json:"attempts"`
		Submissions []academy.Submission         `json:"submissions"`
	}
	decode := func(raw []byte) {
		t.Helper()
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
	}
	key := academy.Key()
	decode(request("POST", tp+"/start", student, map[string]string{"request_key": key}, 200))
	first := state.Attempts[0]
	raw = request("GET", tp, student, nil, 200)
	if bytes.Contains(raw, []byte("SECRET_")) || bytes.Contains(raw, []byte(`"is_correct":true`)) {
		t.Fatal("solutions leaked in active test")
	}
	decode(request("POST", tp+"/start", student, map[string]string{"request_key": key}, 200))
	if len(state.Attempts) != 1 {
		t.Fatal("duplicate start")
	}
	attemptPath := tp + fmt.Sprintf("/attempts/%d", first.ID)
	decode(request("PUT", attemptPath+"/answer", student, map[string]any{"question_id": q1, "selected_answer_ids": []int64{wrong}, "revision": 0}, 200))
	request("PUT", attemptPath+"/answer", student, map[string]any{"question_id": q1, "selected_answer_ids": []int64{correct}, "revision": 0}, 409)
	decode(request("POST", attemptPath+"/finish", student, map[string]int{"score": 9999}, 200))
	if state.Attempts[0].Percent != 0 || state.Attempts[0].Passed {
		t.Fatal("client score trusted")
	}
	raw = request("GET", tp, student, nil, 200)
	if !bytes.Contains(raw, []byte("SECRET_CORRECT_TEXT")) {
		t.Fatal("allowed review not shown")
	}
	// Editing the original draft after Academy freeze must not alter grading.
	if _, err := db.Exec(`UPDATE test_answers SET answer='CHANGED_SOURCE' WHERE question_id=$1`, q2); err != nil {
		t.Fatal(err)
	}
	decode(request("POST", tp+"/start", student, map[string]string{"request_key": academy.Key()}, 200))
	second := state.Attempts[0]
	secondPath := tp + fmt.Sprintf("/attempts/%d", second.ID)
	decode(request("PUT", secondPath+"/answer", student, map[string]any{"question_id": q1, "selected_answer_ids": []int64{correct}, "revision": 0}, 200))
	decode(request("PUT", secondPath+"/answer", student, map[string]any{"question_id": q2, "text_answer": "SECRET_CORRECT_TEXT", "revision": 1}, 200))
	decode(request("POST", secondPath+"/finish", student, nil, 200))
	if state.Attempts[0].Percent != 100 || !state.Attempts[0].Passed {
		t.Fatal("snapshot grading failed")
	}
	request("POST", secondPath+"/finish", student, nil, 200)
	request("POST", tp+"/start", student, map[string]string{"request_key": academy.Key()}, 409)
	room := request("GET", ep, student, nil, 200)
	if bytes.Contains(room, []byte(`"percent":100`)) {
		t.Fatal("assignment requirement bypassed")
	}
	decode(request("POST", ap+"/draft", student, map[string]string{"request_key": academy.Key()}, 200))
	sub := state.Submissions[0]
	sp := ap + fmt.Sprintf("/submissions/%d", sub.ID)
	request("PUT", sp, student, map[string]any{"revision": sub.Revision, "answer": map[string]any{"text": "My draft", "links": []string{"javascript:alert(1)"}, "asset_ids": []int64{}}}, 400)
	decode(request("PUT", sp, student, map[string]any{"revision": sub.Revision, "answer": map[string]any{"text": "My draft", "links": []string{"https://example.invalid/work"}, "asset_ids": []int64{}}}, 200))
	sub = state.Submissions[0]
	request("POST", sp+"/submit", student, map[string]int64{"revision": sub.Revision}, 400)
	upload := func(path string, user int64) []byte {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		f, _ := mw.CreateFormFile("file", "../../answer.pdf")
		_, _ = f.Write([]byte("%PDF-1.7\nAcademy answer"))
		_ = mw.Close()
		r := httptest.NewRequest("POST", path, &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	decode(upload(sp+"/files", student))
	sub = state.Submissions[0]
	file := sub.Answer.AssetIDs[0]
	fp := sp + fmt.Sprintf("/files/%d", file)
	request("GET", fp, student, nil, 200)
	request("GET", fp, owner, nil, 404)
	request("GET", fmt.Sprintf("%s/assets/%d?lesson=%s", ep, file, l.Key), student, nil, 404)
	decode(request("POST", sp+"/submit", student, map[string]int64{"revision": sub.Revision}, 200))
	sub = state.Submissions[0]
	request("POST", sp+"/submit", student, map[string]int64{"revision": sub.Revision}, 200)
	request("PUT", sp, student, map[string]any{"revision": sub.Revision, "answer": sub.Answer}, 409)
	rp := base + fmt.Sprintf("/reviews/%d", sub.ID)
	request("GET", rp, student, nil, 404)
	request("POST", rp, student, map[string]any{"status": "accepted", "score": 100}, 404)
	request("GET", rp, owner, nil, 200)
	request("GET", rp+fmt.Sprintf("/files/%d", file), owner, nil, 200)
	request("GET", base+"/reviews?status=pending", owner, nil, 200)
	request("POST", rp, owner, map[string]any{"status": "accepted", "revision": sub.Revision, "request_key": academy.Key(), "criterion_scores": map[string]int{c1: 99, c2: 40}}, 400)
	request("POST", rp, owner, map[string]any{"status": "revision_requested", "revision": sub.Revision, "request_key": academy.Key(), "comment": "Please revise"}, 200)
	decode(request("POST", ap+"/draft", student, map[string]string{"request_key": academy.Key()}, 200))
	sub2 := state.Submissions[0]
	if sub2.ID == sub.ID || len(state.Submissions) != 2 || state.Submissions[1].Answer.Text != "My draft" {
		t.Fatal("history overwritten")
	}
	sp2 := ap + fmt.Sprintf("/submissions/%d", sub2.ID)
	decode(upload(sp2+"/files", student))
	sub2 = state.Submissions[0]
	decode(request("POST", sp2+"/submit", student, map[string]int64{"revision": sub2.Revision}, 200))
	sub2 = state.Submissions[0]
	rp2 := base + fmt.Sprintf("/reviews/%d", sub2.ID)
	request("POST", rp2, owner, map[string]any{"status": "in_review", "revision": sub2.Revision, "request_key": academy.Key()}, 200)
	decode(request("GET", ap, student, nil, 200))
	sub2 = state.Submissions[0]
	reviewKey := academy.Key()
	review := map[string]any{"status": "accepted", "revision": sub2.Revision, "request_key": reviewKey, "score": 99, "criterion_scores": map[string]int{c1: 50, c2: 30}, "comment": "Good work"}
	request("POST", rp2, owner, review, 200)
	request("POST", rp2, owner, review, 200)
	var reviewCount int
	_ = db.QueryRow(`SELECT count(*) FROM academy_submission_reviews WHERE submission_id=$1 AND action='accepted'`, sub2.ID).Scan(&reviewCount)
	if reviewCount != 1 {
		t.Fatal("duplicate review")
	}
	decode(request("GET", ap, student, nil, 200))
	if state.Submissions[0].Score == nil || *state.Submissions[0].Score != 80 || state.Submissions[0].Status != "accepted" {
		t.Fatal("criterion grading failed")
	}
	if !bytes.Contains(request("GET", ep, student, nil, 200), []byte(`"percent":100`)) {
		t.Fatal("progress did not reconcile")
	}
	if _, err := db.Exec(`UPDATE academy_assignment_submissions SET answer='{}' WHERE id=$1`, sub.ID); err == nil {
		t.Fatal("historical answer mutable")
	}
	if _, err := db.Exec(`UPDATE academy_assessment_versions SET questions='[]' WHERE program_version_id=$1`, version); err == nil {
		t.Fatal("assessment mutable")
	}
	if _, err := db.Exec(`UPDATE academy_test_attempts SET score=999 WHERE id=$1`, second.ID); err == nil {
		t.Fatal("finished score mutable")
	}
	timedPath := ep + "/lessons/" + timed.Key + "/assessments/" + timerBlock.Key
	decode(request("POST", timedPath+"/start", student, map[string]string{"request_key": academy.Key()}, 200))
	timerID := state.Attempts[0].ID
	if _, err := db.Exec(`UPDATE academy_test_attempts SET deadline_at=NOW()-INTERVAL '1 second' WHERE id=$1`, timerID); err != nil {
		t.Fatal(err)
	}
	request("PUT", timedPath+fmt.Sprintf("/attempts/%d/answer", timerID), student, map[string]any{"revision": 0, "question_id": q1, "selected_answer_ids": []int64{correct}}, 409)
	decode(request("GET", timedPath, student, nil, 200))
	if state.Attempts[0].Status != "finished" || state.Attempts[0].Passed {
		t.Fatal("timer bypass")
	}
	if bytes.Contains(request("GET", timedPath, student, nil, 200), []byte("SECRET_")) {
		t.Fatal("solutions shown against policy")
	}
	request("GET", base+"/assessment-results", owner, nil, 200)
	request("GET", fmt.Sprintf("%s/students/%d", base, enrolled.ID), owner, nil, 200)
	if !bytes.Contains(request("GET", "/api/academy/notifications", student, nil, 200), []byte("Работа проверена")) {
		t.Fatal("review notification missing")
	}
	var notices []struct {
		ID   int64 `json:"id"`
		Read bool  `json:"is_read"`
	}
	_ = json.Unmarshal(request("GET", "/api/academy/notifications", student, nil, 200), &notices)
	if len(notices) == 0 {
		t.Fatal("no notifications")
	}
	request("PUT", "/api/academy/notifications", student, map[string]any{"ids": []int64{notices[0].ID}}, 200)
	var read bool
	_ = db.QueryRow(`SELECT is_read FROM notifications WHERE id=$1 AND user_id=$2`, notices[0].ID, student).Scan(&read)
	if !read {
		t.Fatal("notification did not mark read")
	}
	// The course owner may also study, but cannot manually grade their own answer.
	var own struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, map[string]any{"user_id": owner, "version_id": version, "request_key": academy.Key()}, 200), &own)
	ownAP := fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/assessments/%s", own.ID, l.Key, assignment.Key)
	decode(request("POST", ownAP+"/draft", owner, map[string]string{"request_key": academy.Key()}, 200))
	ownSub := state.Submissions[0]
	ownSP := ownAP + fmt.Sprintf("/submissions/%d", ownSub.ID)
	decode(request("PUT", ownSP, owner, map[string]any{"revision": ownSub.Revision, "answer": map[string]any{"text": "Own work", "links": []string{"https://example.invalid/own"}, "asset_ids": []int64{}}}, 200))
	decode(upload(ownSP+"/files", owner))
	ownSub = state.Submissions[0]
	decode(request("POST", ownSP+"/submit", owner, map[string]int64{"revision": ownSub.Revision}, 200))
	request("POST", base+fmt.Sprintf("/reviews/%d", ownSub.ID), owner, map[string]any{"status": "accepted", "revision": state.Submissions[0].Revision, "request_key": academy.Key(), "criterion_scores": map[string]int{c1: 60, c2: 40}}, 403)
	// A new edition freezes changed source questions and criteria; old results are untouched.
	p.Modules[0].Lessons[0].Blocks[1].Data.Assessment.MaxScore = 120
	p.Modules[0].Lessons[0].Blocks[1].Data.Assessment.Criteria[1].MaxScore = 60
	auto := academy.NewLesson()
	auto.Title = "Format only"
	auto.Status = "published"
	auto.Required = false
	autoBlock := academy.Block{Key: academy.Key(), Type: "assignment", Data: academy.BlockData{Title: "Self-check", AnswerFormat: "text", Assessment: &academy.AssessmentPolicy{ReviewMode: "automatic", MaxAttempts: 1, Formats: []string{"text"}}}}
	auto.Blocks = []academy.Block{autoBlock}
	p.Modules[0].Lessons = append(p.Modules[0].Lessons, auto)
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	request("POST", base+"/versions", owner, map[string]int64{"revision": p.Revision}, 200)
	var newVersion int64
	_ = db.QueryRow(`SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, p.ID, p.Revision).Scan(&newVersion)
	request("POST", base+"/releases", owner, map[string]any{"version_id": newVersion}, 200)
	var currentMax float64
	_ = db.QueryRow(`SELECT (config->'assessment'->>'max_score')::numeric FROM academy_assessment_versions WHERE program_version_id=$1 AND block_key=$2`, version, assignment.Key).Scan(&currentMax)
	if currentMax != 100 {
		t.Fatal("old criteria changed")
	}
	_ = json.Unmarshal(request("POST", base+"/students", owner, map[string]any{"user_id": owner, "version_id": newVersion, "request_key": academy.Key()}, 200), &own)
	autoPath := fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/assessments/%s", own.ID, auto.Key, autoBlock.Key)
	decode(request("POST", autoPath+"/draft", owner, map[string]string{"request_key": academy.Key()}, 200))
	as := state.Submissions[0]
	autoSP := autoPath + fmt.Sprintf("/submissions/%d", as.ID)
	decode(request("PUT", autoSP, owner, map[string]any{"revision": as.Revision, "answer": map[string]any{"text": "Completed independently", "links": []string{}, "asset_ids": []int64{}}}, 200))
	as = state.Submissions[0]
	decode(request("POST", autoSP+"/submit", owner, map[string]int64{"revision": as.Revision}, 200))
	if state.Submissions[0].Status != "accepted" || state.Submissions[0].Score != nil || state.Submissions[0].ReviewSource != "automatic" {
		t.Fatal("format-only review invented grade")
	}
	if !bytes.Contains(request("GET", ep, student, nil, 200), []byte(`"percent":100`)) {
		t.Fatal("new edition altered old progress")
	}
	var later struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, map[string]any{"user_id": student, "version_id": newVersion, "request_key": academy.Key()}, 200), &later)
	laterAP := fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/assessments/%s", later.ID, l.Key, assignment.Key)
	decode(request("POST", laterAP+"/draft", student, map[string]string{"request_key": academy.Key()}, 200))
	ls := state.Submissions[0]
	laterSP := laterAP + fmt.Sprintf("/submissions/%d", ls.ID)
	decode(request("PUT", laterSP, student, map[string]any{"revision": ls.Revision, "answer": map[string]any{"text": "New edition work", "links": []string{"https://example.invalid/new"}, "asset_ids": []int64{}}}, 200))
	decode(upload(laterSP+"/files", student))
	ls = state.Submissions[0]
	decode(request("POST", laterSP+"/submit", student, map[string]int64{"revision": ls.Revision}, 200))
	ls = state.Submissions[0]
	var nextResult struct {
		Next int64 `json:"next_id"`
	}
	_ = json.Unmarshal(request("POST", rp2, owner, review, 200), &nextResult)
	if nextResult.Next != ls.ID {
		t.Fatal("next work missing or included owner's own work", nextResult.Next, ls.ID)
	}
	request("POST", base+fmt.Sprintf("/reviews/%d", ls.ID), owner, map[string]any{"status": "rejected", "revision": ls.Revision, "comment": "Final rejection", "request_key": academy.Key()}, 200)
	request("GET", laterSP+fmt.Sprintf("/files/%d", file), student, nil, 404)
	var after int
	_ = db.QueryRow(`SELECT count(*) FROM test_attempts`).Scan(&after)
	if after != professionalBefore {
		t.Fatal("Academy affected professional attempts")
	}
	request("PUT", fmt.Sprintf("%s/students/%d", base, enrolled.ID), owner, map[string]string{"status": "suspended"}, 200)
	request("GET", fp, student, nil, 403)
	request("GET", tp, student, nil, 403)
}
