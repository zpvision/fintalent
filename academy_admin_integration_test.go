package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/courses"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAcademyAdministrationIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	ctx := context.Background()
	f.Handler.SetAdminResolver(func(r *http.Request) (academy.AdminPrincipal, error) {
		if r.Header.Get("X-Test-Admin") == "root" {
			return academy.AdminPrincipal{Key: "platform_admin", Name: "Platform administrator", Role: "superadmin", Contacts: true, Answers: true, Correct: true}, nil
		}
		uid, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		return academy.ResolveAdminGrant(r.Context(), f.DB, uid)
	})
	request := func(method, path, actor string, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		var text string
		if body != nil {
			raw, _ := json.Marshal(body)
			text = string(raw)
		}
		r := httptest.NewRequest(method, "http://academy.local/api/admin/academy"+path, strings.NewReader(text))
		r.Header.Set("Content-Type", "application/json")
		if actor == "root" {
			r.Header.Set("X-Test-Admin", "root")
		} else {
			r.Header.Set("X-Test-User", actor)
		}
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s = %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	get := func(path, actor string) map[string]any {
		t.Helper()
		var v map[string]any
		w := request("GET", path, actor, nil, 200)
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	command := func(path, actor, expected string, data any, status int) {
		t.Helper()
		request("PUT", path, actor, map[string]any{"reason": "Technical support correction", "confirm": true, "request_key": academy.Key(), "expected": expected, "data": data}, status)
	}
	root := "root"
	support := fmt.Sprint(f.Teacher)
	student := fmt.Sprint(f.Student)
	request("GET", "/overview", student, nil, 401)
	get("/session", root)
	for _, section := range []string{"overview", "courses", "authors", "students", "cohorts", "enrollments", "tests", "submissions", "campaigns", "automations", "deliveries", "audit", "settings", "grants"} {
		get("/"+section, root)
	}
	if _, err := f.DB.Exec("INSERT INTO academy_admin_grants(user_id,role) VALUES($1,'support')", f.Teacher); err != nil {
		t.Fatal(err)
	}
	get("/overview", support)
	request("GET", "/grants", support, nil, 403)
	var e int64
	if err := f.DB.QueryRow("INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key) VALUES($1,$2,$3,$4,$5) RETURNING id", f.Course, f.Student, f.Version, f.Owner, academy.Key()).Scan(&e); err != nil {
		t.Fatal(err)
	}
	enrollment := get(fmt.Sprintf("/enrollments/%d", e), support)
	expected := enrollment["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/enrollments/%d/access", e), support, expected, map[string]any{"status": "suspended"}, 200)
	command(fmt.Sprintf("/enrollments/%d/access", e), support, expected, map[string]any{"status": "active"}, 409)
	enrollment = get(fmt.Sprintf("/enrollments/%d", e), support)
	expected = enrollment["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/enrollments/%d/access", e), support, expected, map[string]any{"status": "active", "set_expiration": true, "expires_at": time.Now().Add(24 * time.Hour)}, 200)
	course := get(fmt.Sprintf("/courses/%d", f.Course), root)
	expected = course["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/courses/%d", f.Course), support, expected, map[string]any{"title": "Forbidden"}, 403)
	command(fmt.Sprintf("/courses/%d", f.Course), root, expected, map[string]any{"title": "Administratively corrected course"}, 200)
	get(fmt.Sprintf("/courses/%d/program", f.Course), root)
	get(fmt.Sprintf("/courses/%d/analytics/overview", f.Course), root)
	person := get(fmt.Sprintf("/students/%d", f.Student), support)
	if person["person"].(map[string]any)["email"] != nil {
		t.Fatal("contacts disclosed")
	}
	var count int
	if err := f.DB.QueryRow("SELECT count(*) FROM academy_admin_audit WHERE course_id=$1", f.Course).Scan(&count); err != nil || count < 3 {
		t.Fatal("missing audit", count, err)
	}

	// Immutable original results are overlaid by separately audited, enrollment-scoped corrections.
	required := true
	passing := 70.0
	program := academy.Program{ID: f.Program, CourseID: f.Course, Revision: 2, Source: "empty", Modules: []academy.Module{{Key: "module-correct", Title: "Assessment module", Status: "published", Lessons: []academy.Lesson{{Key: "lesson-correct", Title: "Assessment lesson", Status: "published", SelfPaced: true, Required: true, CompletionRule: "manual", Blocks: []academy.Block{{Key: "test-correct", Type: "test", Data: academy.BlockData{Title: "Frozen test", TestVersionID: 100, Assessment: &academy.AssessmentPolicy{Required: &required, PassingPercent: &passing}}}, {Key: "work-correct", Type: "assignment", Data: academy.BlockData{Title: "Frozen assignment", Assessment: &academy.AssessmentPolicy{Required: &required, MaxScore: 100, PassingScore: &passing, Criteria: []academy.Criterion{{Key: "accuracy", Title: "Accuracy", MaxScore: 100}}}}}}}}}}}
	raw, _ := json.Marshal(program)
	var version int64
	if err := f.DB.QueryRow("INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,2,2,$2,$3) RETURNING id", f.Program, raw, f.Owner).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec("INSERT INTO academy_version_releases(version_id,course_id,published_by) VALUES($1,$2,$3)", version, f.Course, f.Owner); err != nil {
		t.Fatal(err)
	}
	var eid, tid, sid, ta, sa int64
	if err := f.DB.QueryRow("INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key,started_at) VALUES($1,$2,$3,$4,$5,now()) RETURNING id", f.Course, f.Student, version, f.Owner, academy.Key()).Scan(&eid); err != nil {
		t.Fatal(err)
	}
	for i, dest := range []*int64{&ta, &sa} {
		b := program.Modules[0].Lessons[0].Blocks[i]
		config, _ := json.Marshal(b.Data)
		if err := f.DB.QueryRow("INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,title) VALUES($1,'lesson-correct',$2,$3,$4,$5) RETURNING id", version, b.Key, b.Type, config, b.Data.Title).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.DB.QueryRow("INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,status,score,max_score,percent,passed,finished_at,answers) VALUES($1,$2,$3,1,$4,'finished',20,100,20,false,now(),'[{\"question_id\":99,\"text_answer\":\"PRIVATE_STUDENT_ANSWER\"}]') RETURNING id", eid, version, ta, academy.Key()).Scan(&tid); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.QueryRow("INSERT INTO academy_assignment_submissions(enrollment_id,program_version_id,assessment_id,number,request_key,status,score,reviewed_by,reviewed_at,submitted_at,answer) VALUES($1,$2,$3,1,$4,'rejected',20,$5,now(),now(),'{\"text\":\"PRIVATE_SUBMISSION_TEXT\",\"links\":[],\"asset_ids\":[]}') RETURNING id", eid, version, sa, academy.Key(), f.Owner).Scan(&sid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec("INSERT INTO academy_submission_reviews(submission_id,reviewer_id,source,request_key,action,score,comment) VALUES($1,$2,'manual',$3,'rejected',20,'Original review')", sid, f.Owner, academy.Key()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec("INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status) VALUES($1,$2,'lesson-correct','in_progress')", eid, version); err != nil {
		t.Fatal(err)
	}
	limited := request("GET", fmt.Sprintf("/tests/%d", tid), support, nil, 200)
	if strings.Contains(limited.Body.String(), "PRIVATE_STUDENT_ANSWER") {
		t.Fatal("answers leaked to support")
	}
	item := get(fmt.Sprintf("/tests/%d", tid), root)
	expected = item["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/tests/%d/correct", tid), support, expected, map[string]any{"score": 80}, 403)
	command(fmt.Sprintf("/tests/%d/correct", tid), root, expected, map[string]any{"score": 80}, 200)
	var originalScore, effectiveScore float64
	if err := f.DB.QueryRow("SELECT original.score,effective.score FROM academy_test_attempts original JOIN academy_effective_test_attempts effective ON effective.id=original.id WHERE original.id=$1", tid).Scan(&originalScore, &effectiveScore); err != nil || originalScore != 20 || effectiveScore != 80 {
		t.Fatal("original score overwritten", originalScore, effectiveScore, err)
	}
	item = get(fmt.Sprintf("/submissions/%d", sid), root)
	expected = item["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/submissions/%d/correct", sid), root, expected, map[string]any{"score": 100, "status": "accepted", "criteria": map[string]float64{"accuracy": 100}, "comment": "Administrative correction"}, 200)
	detail := get(fmt.Sprintf("/enrollments/%d", eid), root)
	if detail["enrollment"].(map[string]any)["summary"].(map[string]any)["percent"] != float64(100) {
		t.Fatal("both requirements did not complete lesson", detail)
	}
	var originalStatus string
	if err := f.DB.QueryRow("SELECT status FROM academy_assignment_submissions WHERE id=$1", sid).Scan(&originalStatus); err != nil || originalStatus != "rejected" {
		t.Fatal("original review overwritten", err)
	}
	expected = detail["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/submissions/%d/correct", sid), root, expected, map[string]any{"score": 20, "status": "rejected", "criteria": map[string]float64{"accuracy": 20}}, 200)
	detail = get(fmt.Sprintf("/enrollments/%d", eid), root)
	if detail["enrollment"].(map[string]any)["summary"].(map[string]any)["percent"] != float64(0) {
		t.Fatal("reversed correction did not recalculate")
	}
	if _, err := f.DB.Exec("UPDATE academy_admin_corrections SET payload='{}' WHERE enrollment_id=$1", eid); err == nil {
		t.Fatal("corrections are mutable")
	}
	if _, err := f.DB.Exec("UPDATE academy_admin_audit SET reason='erase history' WHERE enrollment_id=$1", eid); err == nil {
		t.Fatal("audit is mutable")
	}
	// Platform reports use the existing queue but remain separate from the same user's author context.
	report := request("POST", fmt.Sprintf("/courses/%d/analytics/reports", f.Course), root, map[string]any{"kind": "students", "format": "csv", "request_key": academy.Key(), "filters": academy.AnalyticsFilters{}}, 202)
	var job struct{ ID int64 }
	_ = json.Unmarshal(report.Body.Bytes(), &job)
	if err := academy.ProcessAnalyticsReports(ctx, f.DB, f.Root); err != nil {
		t.Fatal(err)
	}
	request("GET", fmt.Sprintf("/courses/%d/analytics/reports/%d/file", f.Course, job.ID), root, nil, 200)
	request("GET", fmt.Sprintf("/courses/%d/analytics/reports/%d/file", f.Course, job.ID), support, nil, 404)
	// Every version remains byte-for-byte unchanged.
	var unchanged []byte
	if err := f.DB.QueryRow("SELECT content FROM academy_program_versions WHERE id=$1", version).Scan(&unchanged); err != nil {
		t.Fatal(err)
	}
	var persisted academy.Program
	if json.Unmarshal(unchanged, &persisted) != nil || persisted.Modules[0].Lessons[0].Title != "Assessment lesson" {
		t.Fatal("immutable program changed")
	}

	// Cohort management reuses schedule, staff, enrollment and transfer transactions.
	var cohortID int64
	if err := f.DB.QueryRow("INSERT INTO academy_cohorts(course_id,program_version_id,name,status,starts_at,timezone,created_by,request_key) VALUES($1,$2,'Support cohort','active',now()-interval '1 day','Europe/Prague',$3,$4) RETURNING id", f.Course, f.Version, f.Owner, academy.Key()).Scan(&cohortID); err != nil {
		t.Fatal(err)
	}
	cohortPath := fmt.Sprintf("/cohorts/%d", cohortID)
	c := get(cohortPath, root)
	expected = c["snapshot"].(map[string]any)["expected"].(string)
	command(cohortPath+"/staff", root, expected, map[string]any{"user_id": f.Teacher, "role": "curator", "can_review": true, "can_communicate": false}, 200)
	c = get(cohortPath, root)
	expected = c["snapshot"].(map[string]any)["expected"].(string)
	cohortData := c["cohort"].(map[string]any)
	cohortData["name"] = "Corrected cohort"
	command(cohortPath, root, expected, cohortData, 200)
	enrollment = get(fmt.Sprintf("/enrollments/%d", e), root)
	expected = enrollment["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/enrollments/%d/transfer", e), root, expected, map[string]any{"cohort_id": cohortID}, 200)
	var transferred int64
	if err := f.DB.QueryRow("SELECT id FROM academy_enrollments WHERE cohort_id=$1 AND user_id=$2", cohortID, f.Student).Scan(&transferred); err != nil || transferred == e {
		t.Fatal("transfer did not preserve old enrollment", err)
	}
	var oldAccess string
	if err := f.DB.QueryRow("SELECT access_status FROM academy_enrollments WHERE id=$1", e).Scan(&oldAccess); err != nil || oldAccess != "revoked" {
		t.Fatal("old enrollment not revoked", err, oldAccess)
	}
	enrollment = get(fmt.Sprintf("/enrollments/%d", transferred), root)
	expected = enrollment["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/enrollments/%d/curator", transferred), root, expected, map[string]any{"user_id": f.Teacher}, 200)
	// Effective result lists stay scoped to this particular enrollment, not user+course.
	list := get(fmt.Sprintf("/tests?enrollment=%d", transferred), root)
	if list["total"] != float64(0) {
		t.Fatal("another enrollment's test leaked into support card")
	}
	// Explicit confirmation, replay and restricted moderation.
	course = get(fmt.Sprintf("/courses/%d", f.Course), root)
	expected = course["snapshot"].(map[string]any)["expected"].(string)
	replay := map[string]any{"reason": "Hide invalid publication", "confirm": true, "request_key": academy.Key(), "expected": expected, "data": map[string]bool{"blocked": true}}
	request("PUT", fmt.Sprintf("/courses/%d/restriction", f.Course), root, replay, 200)
	request("PUT", fmt.Sprintf("/courses/%d/restriction", f.Course), root, replay, 200)
	replay["data"] = map[string]bool{"blocked": false}
	request("PUT", fmt.Sprintf("/courses/%d/restriction", f.Course), root, replay, 409)
	replay["confirm"] = false
	replay["request_key"] = academy.Key()
	request("PUT", fmt.Sprintf("/courses/%d/restriction", f.Course), root, replay, 400)
	if _, err := f.DB.Exec("UPDATE academy_admin_grants SET role='moderator' WHERE user_id=$1", f.Teacher); err != nil {
		t.Fatal(err)
	}
	get("/courses", support)
	request("GET", "/students", support, nil, 403)
	request("GET", "/overview", support, nil, 403)
	course = get(fmt.Sprintf("/courses/%d", f.Course), support)
	expected = course["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/courses/%d/restriction", f.Course), support, expected, map[string]bool{"blocked": false}, 200)
	if _, err := f.DB.Exec("UPDATE academy_admin_grants SET role='support' WHERE user_id=$1", f.Teacher); err != nil {
		t.Fatal(err)
	}
	// Cancellation affects queued messages only; already sent deliveries are retained.
	var campaign, automation int64
	if err := f.DB.QueryRow("INSERT INTO academy_campaigns(course_id,created_by,name,message,audience,channel,category,status,request_key) VALUES($1,$2,'Stop campaign','{}','{}','in_app','service','sending',$3) RETURNING id", f.Course, f.Owner, academy.Key()).Scan(&campaign); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec("INSERT INTO academy_message_deliveries(course_id,actor_id,campaign_id,enrollment_id,recipient_id,channel,category,message,dedup_key,next_attempt_at) VALUES($1,$2,$3,$4,$5,'in_app','service','{}',$6,now())", f.Course, f.Owner, campaign, eid, f.Student, academy.Key()); err != nil {
		t.Fatal(err)
	}
	item = get(fmt.Sprintf("/campaigns/%d", campaign), root)
	expected = item["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/campaigns/%d/cancel", campaign), root, expected, map[string]any{}, 200)
	var deliveryStatus string
	if err := f.DB.QueryRow("SELECT status FROM academy_message_deliveries WHERE campaign_id=$1", campaign).Scan(&deliveryStatus); err != nil || deliveryStatus != "cancelled" {
		t.Fatal("queued delivery not cancelled", err)
	}
	if err := f.DB.QueryRow("INSERT INTO academy_automations(course_id,created_by,name,trigger,config,status,request_key) VALUES($1,$2,'Stop automation','student_enrolled','{}','active',$3) RETURNING id", f.Course, f.Owner, academy.Key()).Scan(&automation); err != nil {
		t.Fatal(err)
	}
	item = get(fmt.Sprintf("/automations/%d", automation), root)
	expected = item["snapshot"].(map[string]any)["expected"].(string)
	command(fmt.Sprintf("/automations/%d/pause", automation), root, expected, map[string]any{}, 200)

	// Closed answer files require explicit answers permission, including for support.
	storage := strings.Repeat("a", 48)
	if err := os.WriteFile(filepath.Join(f.Root, storage), []byte("private synthetic answer"), 0600); err != nil {
		t.Fatal(err)
	}
	var asset int64
	if err := f.DB.QueryRow("INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by,submission_id) VALUES($1,'work-correct',$2,'answer.txt','text/plain',24,$3,$4) RETURNING id", f.Program, storage, f.Student, sid).Scan(&asset); err != nil {
		t.Fatal(err)
	}
	request("GET", fmt.Sprintf("/assets/%d", asset), support, nil, 403)
	request("GET", fmt.Sprintf("/assets/%d", asset), root, nil, 200)
	request("GET", fmt.Sprintf("/assets/%d", asset), student, nil, 401)
	// Concurrent retries serialize at the command key and produce exactly one successful audit.
	enrollment = get(fmt.Sprintf("/enrollments/%d", transferred), root)
	expected = enrollment["snapshot"].(map[string]any)["expected"].(string)
	concurrentKey := academy.Key()
	concurrentBody, _ := json.Marshal(map[string]any{"reason": "Concurrent idempotent support request", "confirm": true, "request_key": concurrentKey, "expected": expected, "data": map[string]any{"status": "suspended"}})
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("PUT", fmt.Sprintf("http://academy.local/api/admin/academy/enrollments/%d/access", transferred), strings.NewReader(string(concurrentBody)))
			req.Header.Set("X-Test-Admin", "root")
			rec := httptest.NewRecorder()
			f.Mux.ServeHTTP(rec, req)
			statuses <- rec.Code
		}()
	}
	for i := 0; i < 2; i++ {
		if status := <-statuses; status != 200 {
			t.Fatal("concurrent command", status)
		}
	}
	if err := f.DB.QueryRow("SELECT count(*) FROM academy_admin_audit WHERE request_key=$1", concurrentKey).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate administrative effect", err, count)
	}
	// Delegated administrators need explicit correction rights and cannot correct their own work.
	grant := get(fmt.Sprintf("/grants/%d", f.Student), root)
	command(fmt.Sprintf("/grants/%d", f.Student), root, grant["snapshot"].(map[string]any)["expected"].(string), map[string]any{"role": "academy_admin", "correct_results": true}, 200)
	item = get(fmt.Sprintf("/tests/%d", tid), root)
	command(fmt.Sprintf("/tests/%d/correct", tid), student, item["snapshot"].(map[string]any)["expected"].(string), map[string]any{"score": 100}, 403)
	token := strings.Repeat("b", 64)
	if _, err := f.DB.Exec("INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 day')", f.Student, hashSessionToken(token)); err != nil {
		t.Fatal(err)
	}
	staffRequest := httptest.NewRequest("GET", "http://academy.local/api/admin/academy/session", nil)
	staffRequest.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	staffPrincipal, staffErr := resolveAcademyAdministrator(staffRequest)
	if staffErr != nil || staffPrincipal.User != f.Student || staffPrincipal.Role != "academy_admin" || isAdmin(staffRequest) {
		t.Fatal("normal session authority was not isolated", staffErr)
	}
	grant = get(fmt.Sprintf("/grants/%d", f.Student), root)
	request("DELETE", fmt.Sprintf("/grants/%d", f.Student), root, map[string]any{"reason": "Revoke temporary test authority", "confirm": true, "request_key": academy.Key(), "expected": grant["snapshot"].(map[string]any)["expected"], "data": map[string]any{}}, 200)
	if _, err := resolveAcademyAdministrator(staffRequest); err == nil {
		t.Fatal("revoked grant remained active")
	}
	settings := get("/settings", root)
	command("/settings", root, settings["expected"].(string), map[string]any{"upload_mb": 10, "report_days": 3, "email_per_minute": 5, "automations_per_course": 5, "email_enabled": false, "automations_enabled": false, "moderation_required": true}, 200)
	// A global automation pause also defers already queued actions, including in-app messages.
	if _, err := f.DB.Exec("UPDATE academy_automations SET status='active' WHERE id=$1", automation); err != nil {
		t.Fatal(err)
	}
	var runID, deliveryID int64
	if err := f.DB.QueryRow("INSERT INTO academy_automation_runs(automation_id,generation,enrollment_id,source_key,event_type,event_at,conditions) VALUES($1,1,$2,'pause-test','enrolled',now(),'{}') RETURNING id", automation, eid).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.QueryRow("INSERT INTO academy_message_deliveries(course_id,actor_id,run_id,recipient_id,channel,category,message,dedup_key,next_attempt_at,test) VALUES($1,$2,$3,$4,'in_app','service','{}',$5,now(),true) RETURNING id", f.Course, f.Owner, runID, f.Student, academy.Key()).Scan(&deliveryID); err != nil {
		t.Fatal(err)
	}
	if err := academy.ProcessCommunications(ctx, f.DB, "https://academy.example.invalid", func(context.Context, string, string, string, string, string) academy.CommunicationResult {
		t.Fatal("unexpected external transport")
		return academy.CommunicationResult{}
	}); err != nil {
		t.Fatal(err)
	}
	var code string
	if err := f.DB.QueryRow("SELECT status,error_code FROM academy_message_deliveries WHERE id=$1", deliveryID).Scan(&deliveryStatus, &code); err != nil || deliveryStatus != "queued" || code != "academy_automations_paused" {
		t.Fatal("global pause did not defer queued action", deliveryStatus, code, err)
	}
	// Public administrative endpoints enforce same-origin even without the outer middleware.
	r := httptest.NewRequest("PUT", "http://academy.local/api/admin/academy/settings", strings.NewReader("{}"))
	r.Header.Set("X-Test-Admin", "root")
	r.Header.Set("Origin", "https://untrusted.invalid")
	w := httptest.NewRecorder()
	f.Mux.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("CSRF", w.Code)
	}
	// Production resolver accepts only existing cookie sessions, never test headers.
	t.Setenv("ADMIN_LOGIN", "academy-test-admin")
	t.Setenv("ADMIN_PASSWORD", "isolated-test-password")
	authRequest := httptest.NewRequest("GET", "http://academy.local/api/admin/academy/session", nil)
	authResponse := httptest.NewRecorder()
	if err := createAdminSession(authResponse, authRequest); err != nil {
		t.Fatal(err)
	}
	authRequest.AddCookie(authResponse.Result().Cookies()[0])
	principal, authErr := resolveAcademyAdministrator(authRequest)
	if authErr != nil || principal.Role != "superadmin" || principal.User != 0 {
		t.Fatal("existing root session was not reused", authErr)
	}
	noCookie := httptest.NewRequest("GET", "http://academy.local/api/admin/academy/session", nil)
	noCookie.Header.Set("X-Test-Admin", "root")
	if _, authErr = resolveAcademyAdministrator(noCookie); authErr == nil {
		t.Fatal("production accepted a header identity")
	}

	// Platform account restrictions cannot be counted as effective active access.
	if _, err := f.DB.Exec("UPDATE users SET is_blocked=true WHERE id=$1", f.Student); err != nil {
		t.Fatal(err)
	}
	overview := get("/overview", root)
	if overview["metrics"].(map[string]any)["active_enrollments"] != float64(0) {
		t.Fatal("blocked account counted as active access")
	}
	get("/students", root)
	if _, err := f.DB.Exec("UPDATE users SET is_blocked=false WHERE id=$1", f.Student); err != nil {
		t.Fatal(err)
	}
	// The original moderation route also requires a reason, confirmation and revision.
	courses.New(f.DB, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, isAdmin).Register(f.Mux)
	if _, err := f.DB.Exec("UPDATE courses SET status='moderation',revision=revision+1 WHERE id=$1", f.Course); err != nil {
		t.Fatal(err)
	}
	var landingRevision int64
	if err := f.DB.QueryRow("SELECT revision FROM courses WHERE id=$1", f.Course).Scan(&landingRevision); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		reason string
		status int
	}{{"", 400}, {"Legacy moderation correction", 200}} {
		raw, _ := json.Marshal(map[string]any{"reason": check.reason, "confirm": true, "revision": landingRevision})
		legacy := httptest.NewRequest("POST", fmt.Sprintf("http://academy.local/api/admin/courses/%d/reject", f.Course), strings.NewReader(string(raw)))
		legacy.AddCookie(authResponse.Result().Cookies()[0])
		rec := httptest.NewRecorder()
		f.Mux.ServeHTTP(rec, legacy)
		if rec.Code != check.status {
			t.Fatal("legacy moderation", rec.Code, rec.Body.String())
		}
	}
	if err := prepareAcademyDatabase(ctx); err != nil {
		t.Fatal("repeat migration", err)
	}
}
