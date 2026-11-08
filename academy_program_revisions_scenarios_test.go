package main

import (
	"FinTalent/internal/academy"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func TestAcademyProgramRevisionScenariosPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := f.DB.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	number := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := f.DB.QueryRow(q, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	request := func(method, path string, actor int64, body any, status int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	base := fmt.Sprintf("/api/academy/courses/%d", f.Course)
	draft := func(v int64) academy.Program {
		t.Helper()
		var p academy.Program
		if e := json.Unmarshal(request("GET", fmt.Sprintf("%s/versions/%d/draft", base, v), f.Owner, nil, 200), &p); e != nil {
			t.Fatal(e)
		}
		return p
	}
	save := func(p academy.Program) academy.Program {
		t.Helper()
		var out academy.Program
		if e := json.Unmarshal(request("PUT", fmt.Sprintf("%s/versions/%d/draft", base, p.VersionID), f.Owner, p, 200), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	publish := func(p academy.Program) {
		t.Helper()
		request("POST", fmt.Sprintf("%s/versions/%d/publish", base, p.VersionID), f.Owner, map[string]any{"revision": p.Revision, "acknowledge_warnings": true}, 200)
	}
	type roomData struct {
		Program    academy.Program    `json:"program"`
		Enrollment academy.Enrollment `json:"enrollment"`
	}
	room := func(eid int64) roomData {
		t.Helper()
		var out roomData
		if e := json.Unmarshal(request("GET", fmt.Sprintf("/api/academy/enrollments/%d", eid), f.Student, nil, 200), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	exec(academyProgramRevisionsSchema)
	p := draft(f.Version)
	for len(p.Modules[0].Lessons) < 10 {
		l := academy.NewLesson()
		l.Description = "Lesson content"
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
	}
	p = save(p)
	publish(p)
	cohorts := []int64{}
	enrollments := []int64{}
	for i := 0; i < 2; i++ {
		ch := number(`INSERT INTO academy_cohorts(course_id,program_version_id,name,status,starts_at,created_by,request_key) VALUES($1,$2,$3,'active',now()-interval '1 day',$4,$5) RETURNING id`, f.Course, f.Version, fmt.Sprint("Cohort", i), f.Owner, academy.Key())
		eid := number(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,cohort_id,request_key,enrolled_by,started_at) VALUES($1,$2,$3,$4,$5,$6,now()) RETURNING id`, f.Course, f.Student, f.Version, ch, academy.Key(), f.Owner)
		cohorts = append(cohorts, ch)
		enrollments = append(enrollments, eid)
	}
	for _, l := range p.Modules[0].Lessons[:8] {
		exec(`INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status,completed_at) VALUES($1,$2,$3,'completed',now())`, enrollments[0], f.Version, l.Key)
	}
	t.Run("A_C_G_H_two_cohorts_and_8_of_10", func(t *testing.T) {
		previousPublication := room(enrollments[0]).Program.PublicationID
		l := academy.NewLesson()
		l.Description = "Eleventh"
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
		p = save(p)
		if len(room(enrollments[0]).Program.Modules[0].Lessons) != 10 {
			t.Fatal("draft exposed")
		}
		publish(p)
		request("POST", fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/open?publication_revision=%d", enrollments[0], l.Key, previousPublication), f.Student, map[string]string{"event_key": academy.Key()}, 409)
		for _, eid := range enrollments {
			if len(room(eid).Program.Modules[0].Lessons) != 11 {
				t.Fatal("cohort not updated")
			}
		}
		r := room(enrollments[0])
		if r.Enrollment.Summary.Completed != 8 || r.Enrollment.Summary.Required != 11 || r.Enrollment.Summary.Percent != 72 {
			t.Fatalf("progress %+v", r.Enrollment.Summary)
		}
		if number(`SELECT percent FROM academy_student_metrics WHERE enrollment_id=$1`, enrollments[0]) != 72 {
			t.Fatal("SQL metrics disagree with room")
		}
		if number(`SELECT count(*) FROM academy_lesson_progress WHERE enrollment_id=$1 AND status='completed'`, enrollments[0]) != 8 {
			t.Fatal("progress lost")
		}
	})
	var v2 academy.Program
	t.Run("B_independent_version", func(t *testing.T) {
		if e := json.Unmarshal(request("POST", base+"/versions", f.Owner, map[string]int64{"source_version_id": f.Version}, 200), &v2); e != nil {
			t.Fatal(e)
		}
		if v2.Modules[0].Lessons[0].Key == p.Modules[0].Lessons[0].Key {
			t.Fatal("identities shared")
		}
		m := academy.NewModule()
		l := academy.NewLesson()
		l.Description = "V2 only"
		m.Lessons = []academy.Lesson{l}
		v2.Modules = append(v2.Modules, m)
		v2 = save(v2)
		publish(v2)
		bonus := academy.NewLesson()
		bonus.Description = "V1 only"
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, bonus)
		p = save(p)
		publish(p)
		if len(draft(v2.VersionID).Modules) != 2 || len(draft(v2.VersionID).Modules[0].Lessons) != 11 {
			t.Fatal("version 2 changed")
		}
		if room(enrollments[0]).Enrollment.VersionID != f.Version {
			t.Fatal("enrollment rebound")
		}
	})
	t.Run("D_completion_is_retained", func(t *testing.T) {
		exec(`UPDATE academy_enrollments SET completed_at='2026-01-01T00:00:00Z' WHERE id=$1`, enrollments[1])
		bonus := academy.NewLesson()
		bonus.Required = false
		bonus.Description = "Bonus"
		bonus.AccessMode = "immediate"
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, bonus)
		p = save(p)
		publish(p)
		r := room(enrollments[1])
		if r.Enrollment.Summary.Status != "completed" || r.Enrollment.CompletedAt == nil || r.Enrollment.CompletedAt.Year() != 2026 {
			t.Fatal("completion revoked")
		}
		request("POST", fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/open", enrollments[1], bonus.Key), f.Student, map[string]string{"event_key": academy.Key()}, 200)
		if number(`SELECT count(*) FROM academy_enrollments WHERE id=$1 AND completed_at='2026-01-01T00:00:00Z'`, enrollments[1]) != 1 {
			t.Fatal("completion date changed")
		}
	})
	t.Run("draft_conflict_retry_and_access", func(t *testing.T) {
		stale := draft(f.Version)
		p.Modules[0].Title = "Changed draft"
		p = save(p)
		request("PUT", fmt.Sprintf("%s/versions/%d/draft", base, f.Version), f.Owner, stale, 409)
		if room(enrollments[0]).Program.Modules[0].Title == "Changed draft" {
			t.Fatal("draft visible")
		}
		publish(p)
		before := number(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version)
		publish(p)
		if number(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version) != before {
			t.Fatal("duplicate publication")
		}
		request("GET", fmt.Sprintf("%s/versions/%d/draft", base, f.Version), f.Student, nil, 404)
		if room(enrollments[0]).Program.Modules[0].Title != "Changed draft" {
			t.Fatal("new publication missing")
		}
	})
	t.Run("I_archive_retains_history", func(t *testing.T) {
		key := p.Modules[0].Lessons[0].Key
		p.Modules[0].Lessons = p.Modules[0].Lessons[1:]
		p = save(p)
		publish(p)
		if number(`SELECT count(*) FROM academy_lesson_progress WHERE enrollment_id=$1 AND lesson_key=$2 AND status='completed'`, enrollments[0], key) != 1 {
			t.Fatal("archived history lost")
		}
	})
	t.Run("atomic_concurrent_read_and_publish", func(t *testing.T) {
		before := number(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version)
		oldCount := len(p.Modules[0].Lessons)
		l := academy.NewLesson()
		l.Description = "Atomic lesson"
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
		p.Modules[0].Title = "Atomic publication"
		p = save(p)
		payload, _ := json.Marshal(map[string]any{"revision": p.Revision, "acknowledge_warnings": true, "request_key": "concurrent-publication"})
		var wg sync.WaitGroup
		failures := make(chan string, 100)
		for worker := 0; worker < 6; worker++ {
			wg.Add(1)
			go func(writer bool) {
				defer wg.Done()
				for n := 0; n < 8; n++ {
					method, path, actor, body := "GET", fmt.Sprintf("/api/academy/enrollments/%d", enrollments[0]), f.Student, []byte("null")
					if writer {
						method, path, actor, body = "POST", fmt.Sprintf("%s/versions/%d/publish", base, f.Version), f.Owner, payload
					}
					r := httptest.NewRequest(method, path, bytes.NewReader(body))
					r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
					w := httptest.NewRecorder()
					f.Mux.ServeHTTP(w, r)
					if w.Code != 200 {
						failures <- fmt.Sprintf("concurrent HTTP %d: %s", w.Code, w.Body.String())
						return
					}
					if !writer {
						var read roomData
						if e := json.Unmarshal(w.Body.Bytes(), &read); e != nil {
							failures <- e.Error()
							return
						}
						m := read.Program.Modules[0]
						if m.Title == "Atomic publication" {
							if len(m.Lessons) != oldCount+1 {
								failures <- "mixed new publication"
							}
						} else if len(m.Lessons) != oldCount {
							failures <- "mixed old publication"
						}
					}
				}
			}(worker%2 == 0)
		}
		wg.Wait()
		close(failures)
		for message := range failures {
			t.Error(message)
		}
		if number(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version) != before+1 {
			t.Fatal("concurrent duplicate revisions")
		}
	})
	t.Run("E_F_historical_and_active_tests", func(t *testing.T) {
		tid := number(`INSERT INTO tests(author_id,slug) VALUES($1,$2) RETURNING id`, f.Owner, academy.Key())
		tv := number(`INSERT INTO test_versions(test_id,version,title,created_by) VALUES($1,1,'Revision test',$2) RETURNING id`, tid, f.Owner)
		qid := number(`INSERT INTO test_questions(test_version_id,question,question_type,points) VALUES($1,'Original question','single_choice',10) RETURNING id`, tv)
		correct := number(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'Original correct',true) RETURNING id`, qid)
		wrong := number(`INSERT INTO test_answers(question_id,answer,is_correct) VALUES($1,'Originally wrong',false) RETURNING id`, qid)
		l := academy.NewLesson()
		l.Required = false
		l.Description = "Tests"
		pass := 70.0
		b := academy.Block{Key: academy.Key(), Type: "test", Data: academy.BlockData{TestVersionID: tv, Assessment: &academy.AssessmentPolicy{PassingPercent: &pass, MaxAttempts: 100, ShowCorrect: true}}}
		l.Blocks = []academy.Block{b}
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
		p = save(p)
		publish(p)
		aid := number(`SELECT id FROM academy_assessment_versions WHERE program_version_id=$1 AND block_key=$2 ORDER BY id DESC LIMIT 1`, f.Version, b.Key)
		answer, _ := json.Marshal([]academy.TestAnswer{{QuestionID: qid, SelectedIDs: []int64{correct}}})
		exec(`INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,status,answers,score,max_score,percent,passed,finished_at)
   SELECT $1,$2,$3,n,'historical-'||n,'finished',$4,10,10,100,true,now() FROM generate_series(1,50)n`, enrollments[0], f.Version, aid, string(answer))
		active := number(`INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,answers) VALUES($1,$2,$3,51,'active-before-publication',$4) RETURNING id`, enrollments[0], f.Version, aid, string(answer))
		var before string
		if e := f.DB.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(t) ORDER BY id)::text) FROM academy_test_attempts t WHERE assessment_id=$1 AND status='finished'`, aid).Scan(&before); e != nil {
			t.Fatal(e)
		}
		exec(`UPDATE test_questions SET question='New question' WHERE id=$1`, qid)
		exec(`UPDATE test_answers SET is_correct=(id=$2) WHERE question_id=$1`, qid, wrong)
		pass = 95
		p.Modules[0].Lessons[len(p.Modules[0].Lessons)-1].Blocks[0].Data.Assessment.PassingPercent = &pass
		p = save(p)
		if e := json.Unmarshal(request("POST", fmt.Sprintf("%s/versions/%d/refresh-tests", base, f.Version), f.Owner, map[string]int64{"revision": p.Revision}, 200), &p); e != nil {
			t.Fatal(e)
		}
		publish(p)
		newAid := number(`SELECT id FROM academy_assessment_versions WHERE program_version_id=$1 AND block_key=$2 ORDER BY id DESC LIMIT 1`, f.Version, b.Key)
		if aid == newAid {
			t.Fatal("definition overwritten")
		}
		var after string
		if e := f.DB.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(t) ORDER BY id)::text) FROM academy_test_attempts t WHERE assessment_id=$1 AND status='finished'`, aid).Scan(&after); e != nil {
			t.Fatal(e)
		}
		if before != after {
			t.Fatal("historical results changed")
		}
		path := fmt.Sprintf("/api/academy/enrollments/%d/lessons/%s/assessments/%s", enrollments[0], l.Key, b.Key)
		request("POST", fmt.Sprintf("%s/attempts/%d/finish", path, active), f.Student, map[string]any{}, 200)
		if number(`SELECT count(*) FROM academy_test_attempts WHERE id=$1 AND status='finished' AND passed AND percent=100 AND assessment_id=$2`, active, aid) != 1 {
			t.Fatal("active attempt regraded on new definition")
		}
		request("POST", path+"/start", f.Student, map[string]string{"request_key": academy.Key()}, 200)
		if number(`SELECT count(*) FROM academy_test_attempts WHERE enrollment_id=$1 AND assessment_id=$2 AND status='started'`, enrollments[0], newAid) != 1 {
			t.Fatal("new attempt did not use new definition")
		}
		request("GET", fmt.Sprintf("%s?attempt=%d", path, active), f.Student, nil, 200)
		var v3 academy.Program
		if e := json.Unmarshal(request("POST", base+"/versions", f.Owner, map[string]int64{"source_version_id": f.Version}, 200), &v3); e != nil {
			t.Fatal(e)
		}
		exec(`UPDATE test_answers SET is_correct=(id=$2) WHERE question_id=$1`, qid, correct)
		if e := json.Unmarshal(request("POST", fmt.Sprintf("%s/versions/%d/refresh-tests", base, f.Version), f.Owner, map[string]int64{"revision": p.Revision}, 200), &p); e != nil {
			t.Fatal(e)
		}
		publish(p)
		publish(v3)
		if number(`SELECT count(*) FROM academy_current_assessment_versions a CROSS JOIN LATERAL jsonb_array_elements(a.questions) q CROSS JOIN LATERAL jsonb_array_elements(q->'answers') answer WHERE a.program_version_id=$1 AND (answer->>'id')::bigint=$2 AND (answer->>'is_correct')::boolean`, v3.VersionID, wrong) != 1 {
			t.Fatal("copied version inherited later source edits")
		}
	})
	t.Run("historical_assignment_conditions_and_archival", func(t *testing.T) {
		l := academy.NewLesson()
		l.Required = false
		l.Description = "Practice"
		b := academy.Block{Key: academy.Key(), Type: "assignment", Data: academy.BlockData{Title: "Practice", Text: "Original assignment terms", AnswerFormat: "text", Assessment: &academy.AssessmentPolicy{MaxScore: 100, MaxAttempts: 10, Formats: []string{"text"}}}}
		l.Blocks = []academy.Block{b}
		p.Modules[0].Lessons = append(p.Modules[0].Lessons, l)
		p = save(p)
		publish(p)
		aid := number(`SELECT id FROM academy_current_assessment_versions WHERE program_version_id=$1 AND block_key=$2`, f.Version, b.Key)
		sid := number(`INSERT INTO academy_assignment_submissions(enrollment_id,program_version_id,assessment_id,number,request_key,status,answer,submitted_at) VALUES($1,$2,$3,1,'historical-practice','submitted','{"text":"Original work","links":[],"asset_ids":[]}',now()) RETURNING id`, enrollments[0], f.Version, aid)
		p.Modules[0].Lessons[len(p.Modules[0].Lessons)-1].Blocks[0].Data.Text = "Updated assignment terms"
		p.Modules[0].Lessons[len(p.Modules[0].Lessons)-1].Blocks[0].Data.Assessment.MaxScore = 200
		p = save(p)
		publish(p)
		path := fmt.Sprintf("%s/reviews/%d", base, sid)
		raw := request("GET", path, f.Owner, nil, 200)
		var review struct {
			Definition academy.BlockData `json:"definition"`
		}
		if e := json.Unmarshal(raw, &review); e != nil {
			t.Fatal(e)
		}
		if review.Definition.Text != "Original assignment terms" || review.Definition.Assessment.MaxScore != 100 {
			t.Fatal("old assignment terms replaced")
		}
		request("POST", path, f.Owner, map[string]any{"status": "accepted", "revision": 0, "score": 90, "comment": "Accepted original work", "request_key": academy.Key()}, 200)
		p.Modules[0].Lessons = p.Modules[0].Lessons[:len(p.Modules[0].Lessons)-1]
		p = save(p)
		publish(p)
		raw = request("GET", path, f.Owner, nil, 200)
		if !bytes.Contains(raw, []byte("Original assignment terms")) {
			t.Fatal("archived review inaccessible")
		}
		if number(`SELECT count(*) FROM academy_assignment_submissions WHERE id=$1 AND assessment_id=$2 AND status='accepted' AND score=90`, sid, aid) != 1 {
			t.Fatal("historical evaluation lost")
		}
		request("GET", base+"/reviews", f.Owner, nil, 200)
		request("GET", base+"/analytics/tests?period=all", f.Owner, nil, 200)
		request("GET", base+"/analytics/assignments?period=all", f.Owner, nil, 200)
	})
	t.Run("J_existing_relations_and_landing", func(t *testing.T) {
		if number(`SELECT count(*) FROM academy_cohorts WHERE id IN($1,$2) AND program_version_id=$3`, cohorts[0], cohorts[1], f.Version) != 2 {
			t.Fatal("cohorts changed")
		}
		if number(`SELECT count(*) FROM course_modules WHERE course_id=$1`, f.Course) != 0 {
			t.Fatal("landing modified")
		}
		exec(academyProgramRevisionsSchema)
		for _, eid := range enrollments {
			if room(eid).Enrollment.VersionID != f.Version {
				t.Fatal("migration changed enrollment")
			}
		}
	})
	t.Run("versioned_media_upload_clone_and_history_protection", func(t *testing.T) {
		png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN0sAAAAASUVORK5CYII=")
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("entity_key", p.Modules[0].Lessons[0].Key)
		part, e := form.CreateFormFile("file", "revision.png")
		if e != nil {
			t.Fatal(e)
		}
		part.Write(png)
		form.Close()
		r := httptest.NewRequest("POST", base+"/assets", &body)
		r.Header.Set("X-Test-User", strconv.FormatInt(f.Owner, 10))
		r.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatal("versioned lesson upload", w.Code, w.Body.String())
		}
		var asset struct {
			ID int64 `json:"id"`
		}
		if e = json.Unmarshal(w.Body.Bytes(), &asset); e != nil || asset.ID <= 0 {
			t.Fatal("asset", e)
		}
		p.Modules[0].Lessons[0].Blocks = append(p.Modules[0].Lessons[0].Blocks, academy.Block{Key: academy.Key(), Type: "image", Data: academy.BlockData{AssetID: asset.ID}})
		p = save(p)
		publish(p)
		var copy academy.Program
		if e = json.Unmarshal(request("POST", base+"/versions", f.Owner, map[string]int64{"source_version_id": f.Version}, 200), &copy); e != nil {
			t.Fatal(e)
		}
		if number(`SELECT count(*) FROM academy_asset_links WHERE asset_id=$1 AND entity_key=$2`, asset.ID, copy.Modules[0].Lessons[0].Key) != 1 {
			t.Fatal("copy lost authorized asset link")
		}
		if number(`SELECT count(*) FROM academy_media_assets WHERE id=$1`, asset.ID) != 1 {
			t.Fatal("physical media duplicated")
		}
		request("DELETE", fmt.Sprintf("%s/assets/%d", base, asset.ID), f.Owner, nil, 409)
		blocks := p.Modules[0].Lessons[0].Blocks
		p.Modules[0].Lessons[0].Blocks = blocks[:len(blocks)-1]
		p = save(p)
		publish(p)
		request("DELETE", fmt.Sprintf("%s/assets/%d", base, asset.ID), f.Owner, nil, 409)
		request("GET", fmt.Sprintf("/api/academy/enrollments/%d/assets/%d?lesson=%s", enrollments[0], asset.ID, copy.Modules[0].Lessons[0].Key), f.Student, nil, 404)
		request("GET", fmt.Sprintf("%s/assets/%d", base, asset.ID), f.Teacher, nil, 404)
	})
	t.Run("first_publication_and_foreign_version_IDOR", func(t *testing.T) {
		course := number(`INSERT INTO courses(owner_id,slug,title,published_at) VALUES($1,$2,'New synthetic course',now()) RETURNING id`, f.Owner, academy.Key())
		otherBase := fmt.Sprintf("/api/academy/courses/%d", course)
		var initial academy.Program
		if e := json.Unmarshal(request("POST", otherBase+"/program", f.Owner, map[string]string{"source": "empty"}, 200), &initial); e != nil {
			t.Fatal(e)
		}
		m := academy.NewModule()
		l := academy.NewLesson()
		l.Description = "First version content"
		m.Lessons = []academy.Lesson{l}
		initial.Modules = []academy.Module{m}
		if e := json.Unmarshal(request("PUT", otherBase+"/program", f.Owner, initial, 200), &initial); e != nil {
			t.Fatal(e)
		}
		var first academy.Program
		if e := json.Unmarshal(request("POST", otherBase+"/versions", f.Owner, map[string]int64{"source_version_id": 0}, 200), &first); e != nil {
			t.Fatal(e)
		}
		if first.VersionID <= 0 || number(`SELECT count(*) FROM academy_version_releases WHERE version_id=$1`, first.VersionID) != 0 {
			t.Fatal("version published before confirmation")
		}
		request("POST", fmt.Sprintf("%s/versions/%d/publish", otherBase, first.VersionID), f.Owner, map[string]any{"revision": first.Revision, "acknowledge_warnings": true}, 200)
		if number(`SELECT count(*) FROM academy_current_version_releases WHERE version_id=$1 AND course_id=$2`, first.VersionID, course) != 1 {
			t.Fatal("first publication unavailable")
		}
		request("GET", fmt.Sprintf("%s/versions/%d/draft", base, first.VersionID), f.Owner, nil, 404)
		request("PUT", fmt.Sprintf("%s/versions/%d/draft", base, first.VersionID), f.Owner, first, 404)
		request("GET", fmt.Sprintf("%s/versions/%d/history", otherBase, first.VersionID), f.Student, nil, 404)
		request("POST", base+"/versions", f.Owner, map[string]int64{"source_version_id": 0}, 400)
	})
}
