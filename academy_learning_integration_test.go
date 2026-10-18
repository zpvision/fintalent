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
	"time"
)

func testAcademyStageB(t *testing.T, db *sql.DB, mux *http.ServeMux, owner, student, course int64) {
	t.Helper()
	request := func(method, path string, user int64, v any, status int) []byte {
		t.Helper()
		b, _ := json.Marshal(v)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		if user > 0 {
			r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	base := fmt.Sprintf("/api/academy/courses/%d", course)
	var stranger int64
	if err := db.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES('Academy student','academy-learner@example.invalid',repeat('*',60)) RETURNING id`).Scan(&stranger); err != nil {
		t.Fatal(err)
	}
	var p academy.Program
	_ = json.Unmarshal(request("GET", base+"/program", owner, nil, 200), &p)
	module := academy.NewModule()
	module.Title = "Опубликованная программа"
	module.Status = "published"
	for i := 0; i < 3; i++ {
		l := academy.NewLesson()
		l.Title = fmt.Sprintf("Учебный урок %d", i+1)
		l.Status = "published"
		l.Blocks = []academy.Block{{Key: academy.Key(), Type: "text", Data: academy.BlockData{Rich: []academy.RichBlock{{Type: "paragraph", Text: "Материалы исходной версии"}}}}}
		if i == 1 {
			l.Required = false
		}
		if i == 2 {
			l.CompletionRule = "materials"
		}
		module.Lessons = append(module.Lessons, l)
	}
	p.Modules = []academy.Module{module}
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	first, optional, last := p.Modules[0].Lessons[0].Key, p.Modules[0].Lessons[1].Key, p.Modules[0].Lessons[2].Key
	// A private asset belongs only to the first lesson of the frozen version.
	var buffer bytes.Buffer
	mw := multipart.NewWriter(&buffer)
	_ = mw.WriteField("entity_key", first)
	part, _ := mw.CreateFormFile("file", "learning.pdf")
	_, _ = part.Write([]byte("%PDF-1.7\nStage B fixture"))
	_ = mw.Close()
	r := httptest.NewRequest("POST", base+"/assets", &buffer)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-Test-User", strconv.FormatInt(owner, 10))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var asset struct{ ID int64 }
	_ = json.Unmarshal(w.Body.Bytes(), &asset)
	p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, academy.Block{Key: academy.Key(), Type: "file", Data: academy.BlockData{AssetID: asset.ID}})
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	snapshot := func() int64 {
		t.Helper()
		request("POST", base+"/versions", owner, map[string]int64{"revision": p.Revision}, 200)
		var id int64
		if err := db.QueryRow(`SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, p.ID, p.Revision).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	version1 := snapshot()
	request("POST", base+"/students", owner, map[string]any{"user_id": student, "version_id": version1, "request_key": academy.Key()}, 400)
	request("POST", base+"/releases", owner, map[string]any{"version_id": version1}, 200)
	if _, err := db.Exec(`UPDATE academy_version_releases SET test_metadata='[]' WHERE version_id=$1`, version1); err == nil {
		t.Fatal("release metadata is mutable")
	}
	request("POST", base+"/releases", student, map[string]any{"version_id": version1}, 404)
	key := academy.Key()
	grantBody := map[string]any{"user_id": student, "version_id": version1, "request_key": key}
	var grant struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, grantBody, 200), &grant)
	var retry struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, grantBody, 200), &retry)
	if grant.ID != retry.ID {
		t.Fatal("duplicate enrollment")
	}
	epath := fmt.Sprintf("/api/academy/enrollments/%d", grant.ID)
	room := func(user int64) struct {
		Enrollment academy.Enrollment `json:"enrollment"`
		Program    *academy.Program   `json:"program"`
		Resume     string             `json:"resume_lesson_key"`
	} {
		t.Helper()
		var value struct {
			Enrollment academy.Enrollment `json:"enrollment"`
			Program    *academy.Program   `json:"program"`
			Resume     string             `json:"resume_lesson_key"`
		}
		_ = json.Unmarshal(request("GET", epath, user, nil, 200), &value)
		return value
	}
	list := request("GET", "/api/academy/learning", student, nil, 200)
	if !bytes.Contains(list, []byte(fmt.Sprintf(`"id":%d`, grant.ID))) {
		t.Fatal("enrollment not listed")
	}
	request("GET", epath, 0, nil, 401)
	request("GET", epath, stranger, nil, 404)
	request("GET", epath+"/lessons/unknown-key", student, nil, 404)
	download := fmt.Sprintf("%s/assets/%d?lesson=%s", epath, asset.ID, first)
	request("GET", download, student, nil, 200)
	request("GET", download, stranger, nil, 404)
	request("GET", fmt.Sprintf("%s/assets/%d?lesson=%s", epath, asset.ID, last), student, nil, 404)
	request("GET", fmt.Sprintf("%s/assets/%d", base, asset.ID), student, nil, 404)
	open := func(lesson string) {
		request("POST", epath+"/lessons/"+lesson+"/open", student, map[string]any{"event_key": academy.Key()}, 200)
	}
	complete := func(lesson string, ack bool, status int) {
		request("POST", epath+"/lessons/"+lesson+"/complete", student, map[string]any{"materials_acknowledged": ack, "percent": 100}, status)
	}
	complete(first, false, 400)
	openKey := "course_started" // Also exercises separation from server event keys.
	openBody := map[string]string{"event_key": openKey}
	request("POST", epath+"/lessons/"+first+"/open", student, openBody, 200)
	opened := room(student).Enrollment.LastActivityAt
	request("POST", epath+"/lessons/"+first+"/open", student, openBody, 200)
	if !room(student).Enrollment.LastActivityAt.Equal(*opened) {
		t.Fatal("retried open changed activity")
	}
	complete(first, false, 200)
	complete(first, false, 200)
	view := room(student)
	if view.Enrollment.Summary.Percent != 50 {
		t.Fatal(view.Enrollment.Summary)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM academy_learning_events WHERE enrollment_id=$1 AND type='lesson_completed'`, grant.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("completion counted twice", count, err)
	}
	open(optional)
	complete(optional, false, 200)
	if room(student).Enrollment.Summary.Percent != 50 {
		t.Fatal("optional lesson distorted progress")
	}
	open(last)
	complete(last, false, 400)
	complete(last, true, 200)
	complete(last, false, 200) // A successful acknowledgement is not required a second time.
	if room(student).Enrollment.Summary.Percent != 100 || room(student).Enrollment.CompletedAt == nil {
		t.Fatal("course not completed")
	}
	for _, typ := range []string{"course_started", "course_completed"} {
		if err := db.QueryRow(`SELECT count(*) FROM academy_learning_events WHERE enrollment_id=$1 AND type=$2`, grant.ID, typ).Scan(&count); err != nil || count != 1 {
			t.Fatal("course event not idempotent", typ, count, err)
		}
	}
	card := request("GET", fmt.Sprintf("%s/students/%d", base, grant.ID), owner, nil, 200)
	if !bytes.Contains(card, []byte(first)) {
		t.Fatal("author progress missing")
	}
	for _, status := range []string{"suspended", "revoked"} {
		request("PUT", fmt.Sprintf("%s/students/%d", base, grant.ID), owner, map[string]string{"status": status}, 200)
		if room(student).Program != nil {
			t.Fatal("closed access leaked program")
		}
		request("GET", download, student, nil, 403)
		request("GET", epath+"/lessons/"+first, student, nil, 403)
		request("PUT", fmt.Sprintf("%s/students/%d", base, grant.ID), owner, map[string]string{"status": "active"}, 200)
		if room(student).Enrollment.Summary.Percent != 100 {
			t.Fatal("restoring erased progress")
		}
	}
	if _, err := db.Exec(`UPDATE academy_enrollments SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, grant.ID); err != nil {
		t.Fatal(err)
	}
	if room(student).Enrollment.EffectiveStatus != "expired" || room(student).Program != nil {
		t.Fatal("expired access exposed materials")
	}
	request("GET", download, student, nil, 403)
	request("PUT", fmt.Sprintf("%s/students/%d", base, grant.ID), owner, map[string]any{"status": "active", "set_expiration": true, "expires_at": time.Now().Add(time.Hour)}, 200)
	// New draft and new release cannot replace the old student's content.
	p.Modules[0].Lessons[0].Title = "Только новая версия"
	for _, kind := range []string{"test", "assignment", "video"} {
		l := academy.NewLesson()
		l.Status = "published"
		l.Title = "Требование " + kind
		l.Blocks = []academy.Block{{Key: academy.Key(), Type: kind, Data: academy.BlockData{}}}
		if kind == "assignment" {
			l.Blocks[0].Data.AnswerFormat = "text"
		}
		if kind == "video" {
			l.CompletionRule = "materials"
			l.Blocks[0].Data.URL = "https://youtu.be/abc123"
		}
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
	}
	_ = json.Unmarshal(request("PUT", base+"/program", owner, p, 200), &p)
	if room(student).Program.Modules[0].Lessons[0].Title == "Только новая версия" {
		t.Fatal("draft leaked into student version")
	}
	version2 := snapshot()
	request("POST", base+"/releases", owner, map[string]any{"version_id": version2}, 400)
	request("POST", base+"/releases", owner, map[string]any{"version_id": version2, "acknowledge_warnings": true}, 200)
	var second struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/students", owner, map[string]any{"user_id": student, "version_id": version2, "request_key": academy.Key()}, 200), &second)
	secondPath := fmt.Sprintf("/api/academy/enrollments/%d", second.ID)
	raw := request("GET", secondPath, student, nil, 200)
	if !bytes.Contains(raw, []byte("Только новая версия")) {
		t.Fatal("new enrollment wrong version")
	}
	if room(student).Enrollment.VersionID != version1 {
		t.Fatal("old enrollment moved versions")
	}
	page := request("GET", fmt.Sprintf("/api/academy/learning?before=%d", second.ID), student, nil, 200)
	if !bytes.Contains(page, []byte(fmt.Sprintf(`"id":%d`, grant.ID))) || bytes.Contains(page, []byte(fmt.Sprintf(`"id":%d`, second.ID))) {
		t.Fatal("enrollment cursor incorrect")
	}
	request("GET", "/api/academy/learning?before=invalid", student, nil, 400)
	for _, l := range p.Modules[0].Lessons[3:] {
		request("POST", secondPath+"/lessons/"+l.Key+"/open", student, map[string]string{"event_key": academy.Key()}, 200)
		request("POST", secondPath+"/lessons/"+l.Key+"/complete", student, map[string]bool{"materials_acknowledged": true}, 400)
	}
	// Author preview does not create enrollment/progress/events.
	var before int
	_ = db.QueryRow(`SELECT count(*) FROM academy_enrollments`).Scan(&before)
	request("GET", fmt.Sprintf("%s/releases/%d/preview", base, version1), owner, nil, 200)
	_ = db.QueryRow(`SELECT count(*) FROM academy_enrollments`).Scan(&count)
	if count != before {
		t.Fatal("preview enrolled author")
	}
	request("POST", base+"/students", owner, map[string]any{"user_id": owner, "version_id": version1, "request_key": academy.Key()}, 200)
	if !bytes.Contains(request("GET", "/api/academy/learning", owner, nil, 200), []byte(fmt.Sprintf(`"program_version_id":%d`, version1))) {
		t.Fatal("author cannot also learn")
	}
	search := request("GET", base+"/user-search?q=academy-learner@example.invalid", owner, nil, 200)
	if !bytes.Contains(search, []byte("Academy student")) || bytes.Contains(search, []byte("email")) {
		t.Fatal("privacy search failed")
	}
	if string(bytes.TrimSpace(request("GET", base+"/user-search?q=ac", owner, nil, 200))) != "[]" {
		t.Fatal("short search exposed users")
	}
	request("GET", base+"/students", stranger, nil, 404)
	request("GET", fmt.Sprintf("%s/students/%d", base, grant.ID), stranger, nil, 404)
}
