package main

import (
	"FinTalent/internal/academy"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func testAcademyStageE(t *testing.T, db *sql.DB, mux *http.ServeMux, owner, student, course int64) {
	t.Helper()
	base := fmt.Sprintf("/api/academy/courses/%d", course)
	comm := base + "/communications"
	request := func(method, path string, user int64, body any, status int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var teacher int64
	_ = db.QueryRow(`SELECT id FROM users WHERE email='academy-d-1@example.invalid'`).Scan(&teacher)
	if teacher == 0 {
		t.Fatal("D fixture missing")
	}
	var version int64
	_ = db.QueryRow(`SELECT max(version_id) FROM academy_version_releases WHERE course_id=$1`, course).Scan(&version)
	cohort := academy.Cohort{Name: "Communications", Type: "group", Mode: "all", VersionID: version, StartsAt: time.Now().Add(-time.Hour), Timezone: "UTC", RequestKey: academy.Key()}
	_ = json.Unmarshal(request("POST", base+"/cohorts", owner, cohort, 200), &cohort)
	cohort.Status = "active"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/cohorts/%d", base, cohort.ID), owner, cohort, 200), &cohort)
	var enrolled struct {
		IDs []int64 `json:"ids"`
	}
	_ = json.Unmarshal(request("POST", fmt.Sprintf("%s/cohorts/%d/enroll", base, cohort.ID), owner, map[string]any{"user_ids": []int64{student}, "request_key": academy.Key()}, 200), &enrolled)
	eid := enrolled.IDs[0]
	request("GET", comm+"/campaigns", student, nil, 404)
	request("GET", comm+"/templates", owner, nil, 200)
	message := academy.Message{Subject: "Hello {{student.first_name}}", Preheader: "Your learning", Blocks: []academy.EmailBlock{{Key: academy.Key(), Type: "heading", Text: "{{course.title}}"}, {Key: academy.Key(), Type: "text", Text: "Welcome {{student.name}}"}, {Key: academy.Key(), Type: "button", Text: "Learn", URL: "{{lesson.url}}"}}}
	audience := academy.Segment{Join: "and", Cohorts: []int64{cohort.ID}, Conditions: []academy.Condition{{Field: "progress", Op: "lt", Value: "40"}, {Field: "activity_days", Op: "gte", Value: "0"}}}
	request("POST", comm+"/audience", owner, map[string]any{"audience": academy.Segment{Conditions: []academy.Condition{{Field: "email; DROP TABLE users", Op: "eq", Value: "x"}}}, "channel": "email", "category": "service"}, 400)
	var found struct {
		Count    int `json:"count"`
		Eligible int `json:"eligible"`
	}
	_ = json.Unmarshal(request("POST", comm+"/audience", owner, map[string]any{"audience": audience, "channel": "email", "category": "service"}, 200), &found)
	if found.Count != 1 || found.Eligible != 1 {
		t.Fatal("segment", found)
	}
	var tag struct{ ID int64 }
	_ = json.Unmarshal(request("POST", base+"/crm/tags", owner, map[string]any{"name": "E segment", "color": "blue"}, 200), &tag)
	request("PUT", fmt.Sprintf("%s/crm/%d/tags", base, eid), owner, map[string]any{"tag_ids": []int64{tag.ID}}, 200)
	tagged := audience
	tagged.Join = "or"
	tagged.Conditions = []academy.Condition{{Field: "progress", Op: "gt", Value: "90"}, {Field: "tag", Op: "eq", Value: strconv.FormatInt(tag.ID, 10)}}
	_ = json.Unmarshal(request("POST", comm+"/audience", owner, map[string]any{"audience": tagged, "channel": "email", "category": "service"}, 200), &found)
	if found.Count != 1 {
		t.Fatal("OR/tag segment")
	}
	request("POST", comm+"/preview", owner, map[string]any{"message": message}, 200)
	bad := message
	bad.Subject = "{{student.password}}"
	request("POST", comm+"/preview", owner, map[string]any{"message": bad}, 400)
	makeCampaign := func(channel, category string, when *time.Time) academy.Campaign {
		t.Helper()
		c := academy.Campaign{Name: "Real campaign", Message: message, Audience: audience, Channel: channel, Category: category, Timezone: "UTC", SendAt: when, RequestKey: academy.Key()}
		var got academy.Campaign
		_ = json.Unmarshal(request("POST", comm+"/campaigns", owner, c, 201), &got)
		return got
	}
	launch := func(c academy.Campaign, status int) {
		t.Helper()
		request("POST", fmt.Sprintf("%s/campaigns/%d/launch", comm, c.ID), owner, map[string]any{"confirm": true, "revision": c.Revision}, status)
	}
	c := makeCampaign("email", "service", nil)
	request("POST", fmt.Sprintf("%s/campaigns/%d/launch", comm, c.ID), owner, map[string]any{"confirm": false, "revision": c.Revision}, 400)
	launch(c, 200)
	launch(c, 200)
	request("PUT", fmt.Sprintf("%s/campaigns/%d", comm, c.ID), owner, c, 409)
	var sent atomic.Int32
	mock := func(ctx context.Context, email, subject, html, id, unsub string) academy.CommunicationResult {
		sent.Add(1)
		if !bytes.Contains([]byte(html), []byte("FinTalent Academy")) || unsub == "" {
			t.Error("invalid personalized email")
		}
		return academy.CommunicationResult{}
	}
	process := func(transport academy.CommunicationTransport) {
		t.Helper()
		if err := academy.ProcessCommunications(context.Background(), db, "https://fintalent.ru", transport); err != nil {
			t.Fatal(err)
		}
	}
	process(mock)
	process(mock)
	if sent.Load() != 1 {
		t.Fatal("duplicate send", sent.Load())
	}
	var status string
	_ = db.QueryRow(`SELECT status FROM academy_campaigns WHERE id=$1`, c.ID).Scan(&status)
	if status != "completed" {
		t.Fatal(status)
	}
	future := time.Now().Add(time.Hour)
	scheduled := makeCampaign("email", "service", &future)
	launch(scheduled, 200)
	process(mock)
	if sent.Load() != 1 {
		t.Fatal("scheduled sent early")
	}
	request("POST", fmt.Sprintf("%s/campaigns/%d/cancel", comm, scheduled.ID), owner, map[string]any{}, 200)
	marketing := makeCampaign("email", "marketing", nil)
	launch(marketing, 400)
	request("PUT", "/api/academy/communication-preferences", student, map[string]any{"course_id": course, "email_service": true, "email_marketing": true, "in_app": true}, 400)
	request("PUT", "/api/academy/communication-preferences", student, map[string]any{"course_id": course, "email_service": true, "email_marketing": true, "in_app": true, "confirm_marketing_consent": true}, 200)
	launch(marketing, 200)
	var token string
	_ = db.QueryRow(`SELECT unsubscribe_token FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2`, student, course).Scan(&token)
	request("POST", "/api/academy/unsubscribe/"+token, 0, map[string]any{}, 200)
	process(mock)
	if sent.Load() != 1 {
		t.Fatal("unsubscribe bypassed")
	}
	request("PUT", "/api/academy/communication-preferences", student, map[string]any{"course_id": course, "email_service": true, "in_app": true}, 200)
	retry := makeCampaign("email", "service", nil)
	launch(retry, 200)
	process(func(context.Context, string, string, string, string, string) academy.CommunicationResult {
		return academy.CommunicationResult{ErrorCode: "smtp_temporary", Retryable: true}
	})
	var did int64
	var attempts int
	_ = db.QueryRow(`SELECT id,attempts,status FROM academy_message_deliveries WHERE campaign_id=$1`, retry.ID).Scan(&did, &attempts, &status)
	if status != "queued" || attempts != 1 {
		t.Fatal("retry not scheduled", status, attempts)
	}
	_, _ = db.Exec(`UPDATE academy_message_deliveries SET next_attempt_at=now() WHERE id=$1`, did)
	process(mock)
	if sent.Load() != 2 {
		t.Fatal("retry did not send")
	}
	// Age preceding attempts only inside this isolated fixture; rate limits count retries too.
	_, _ = db.Exec("UPDATE academy_delivery_attempts SET started_at=now()-INTERVAL '2 hours' WHERE delivery_id IN(SELECT id FROM academy_message_deliveries WHERE campaign_id IN($1,$2))", c.ID, retry.ID)
	unknown := makeCampaign("email", "service", nil)
	launch(unknown, 200)
	process(func(context.Context, string, string, string, string, string) academy.CommunicationResult {
		return academy.CommunicationResult{ErrorCode: "smtp_result_unknown", Uncertain: true}
	})
	_ = db.QueryRow(`SELECT id,status FROM academy_message_deliveries WHERE campaign_id=$1`, unknown.ID).Scan(&did, &status)
	if status != "failed" {
		t.Fatal("uncertain retried automatically")
	}
	request("POST", fmt.Sprintf("%s/deliveries/%d/retry", comm, did), owner, map[string]any{}, 409)
	// Test delivery is restricted to the authenticated author's address and remains queued for a safe mock.
	request("POST", comm+"/test", owner, map[string]any{"message": message, "email": "somebody@example.invalid"}, 400)
	var ownEmail string
	_ = db.QueryRow(`SELECT email FROM users WHERE id=$1`, owner).Scan(&ownEmail)
	for i := 0; i < 3; i++ {
		request("POST", comm+"/test", owner, map[string]any{"message": message, "email": ownEmail}, 202)
	}
	request("POST", comm+"/test", owner, map[string]any{"message": message, "email": ownEmail}, 429)
	// Consume the three test jobs with an explicit mock, never SMTP.
	for i := 0; i < 3; i++ {
		process(func(context.Context, string, string, string, string, string) academy.CommunicationResult {
			return academy.CommunicationResult{}
		})
	}
	staffPath := fmt.Sprintf("%s/cohorts/%d/staff", base, cohort.ID)
	request("PUT", staffPath, owner, map[string]any{"user_id": teacher, "role": "curator", "can_communicate": false}, 200)
	request("GET", comm+"/campaigns", teacher, nil, 403)
	request("PUT", staffPath, owner, map[string]any{"user_id": teacher, "role": "curator", "can_communicate": true}, 200)
	request("PUT", fmt.Sprintf("%s/crm/%d/curator", base, eid), owner, map[string]any{"user_id": teacher}, 200)
	request("POST", comm+"/audience", teacher, map[string]any{"audience": academy.Segment{}, "channel": "in_app", "category": "service"}, 200)
	a := academy.Automation{Name: "Lesson companion", Trigger: "lesson_completed", Status: "draft", RequestKey: academy.Key(), Config: academy.AutomationConfig{Audience: audience, InactiveDays: 5, LeadDays: 1, Steps: []academy.CommunicationStep{{Key: academy.Key(), Channel: "in_app", Category: "service", Target: "student", Timing: "immediate", Message: message}}}}
	_ = json.Unmarshal(request("POST", comm+"/automations", owner, a, 200), &a)
	a.Status = "active"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, a.ID), owner, a, 200), &a)
	_, err := db.Exec(`INSERT INTO academy_learning_events(enrollment_id,actor_id,type,event_key,lesson_key) VALUES($1,$2,'lesson_completed',$3,'lesson_fixture')`, eid, student, academy.Key())
	if err != nil {
		t.Fatal(err)
	}
	if err = academy.ObserveAutomations(context.Background(), db, "https://fintalent.ru"); err != nil {
		t.Fatal(err)
	}
	if err = academy.ObserveAutomations(context.Background(), db, "https://fintalent.ru"); err != nil {
		t.Fatal(err)
	}
	var runs int
	_ = db.QueryRow(`SELECT count(*) FROM academy_automation_runs WHERE automation_id=$1`, a.ID).Scan(&runs)
	if runs != 1 {
		t.Fatal("duplicate automation", runs)
	}
	a.Status = "paused"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, a.ID), owner, a, 200), &a)
	process(mock)
	_ = db.QueryRow(`SELECT status FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1`, a.ID).Scan(&status)
	if status != "queued" {
		t.Fatal("paused automation ran", status)
	}
	a.Status = "active"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, a.ID), owner, a, 200), &a)
	_, _ = db.Exec(`UPDATE academy_message_deliveries SET next_attempt_at=now() WHERE run_id IN(SELECT id FROM academy_automation_runs WHERE automation_id=$1)`, a.ID)
	process(mock)
	request("GET", comm+"/deliveries", owner, nil, 200)
	request("GET", fmt.Sprintf("%s/automations/%d/history", comm, a.ID), owner, nil, 200)
	request("GET", "/api/academy/communication-preferences", student, nil, 200)
	var tmpl struct{ ID int64 }
	_ = json.Unmarshal(request("POST", comm+"/templates", owner, map[string]any{"name": "Custom", "message": message}, 200), &tmpl)
	request("PUT", fmt.Sprintf("%s/templates/%d", comm, tmpl.ID), owner, map[string]any{"name": "Updated", "message": message, "revision": 1}, 200)
	request("DELETE", fmt.Sprintf("%s/templates/%d", comm, tmpl.ID), owner, map[string]any{"revision": 2}, 200)
	// Simulate a crash after claiming SMTP: no automatic duplicate is permitted.
	_, _ = db.Exec(`UPDATE academy_message_deliveries SET status='sending',claimed_at=now()-INTERVAL '6 minutes' WHERE id=$1`, did)
	process(mock)
	var uncertain bool
	_ = db.QueryRow(`SELECT uncertain,status FROM academy_message_deliveries WHERE id=$1`, did).Scan(&uncertain, &status)
	if !uncertain || status != "failed" {
		t.Fatal("lost lease not marked uncertain")
	}
	// A new automation observes new source events, not historical retries or draft edits.
	createRule := func(trigger string, timing string, delay int) academy.Automation {
		t.Helper()
		rule := academy.Automation{Name: "Event " + trigger, Trigger: trigger, Status: "draft", RequestKey: academy.Key(), Config: academy.AutomationConfig{Audience: audience, InactiveDays: 5, LeadDays: 1, Steps: []academy.CommunicationStep{{Key: academy.Key(), Channel: "in_app", Category: "service", Target: "student", Timing: timing, DelayMinutes: delay, Message: message}}}}
		_ = json.Unmarshal(request("POST", comm+"/automations", owner, rule, 200), &rule)
		rule.Status = "active"
		_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, rule.ID), owner, rule, 200), &rule)
		return rule
	}
	observe := func() {
		t.Helper()
		if er := academy.ObserveAutomations(context.Background(), db, "https://fintalent.ru"); er != nil {
			t.Fatal(er)
		}
	}
	jobCount := func(id int64) int {
		t.Helper()
		var count int
		if er := db.QueryRow("SELECT count(*) FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1", id).Scan(&count); er != nil {
			t.Fatal(er)
		}
		return count
	}
	addEvent := func(typ string, data any) {
		t.Helper()
		raw, _ := json.Marshal(data)
		if _, er := db.Exec("INSERT INTO academy_learning_events(enrollment_id,actor_id,type,event_key,data) VALUES($1,$2,$3,$4,$5)", eid, student, typ, academy.Key(), raw); er != nil {
			t.Fatal(er)
		}
	}
	for _, trigger := range []string{"enrolled", "assignment_reviewed", "test_passed", "test_failed"} {
		rule := createRule(trigger, "delay", 60)
		typ := trigger
		data := map[string]any{}
		if trigger == "assignment_reviewed" {
			typ = "assignment_revision_requested"
		}
		if trigger == "test_passed" || trigger == "test_failed" {
			typ = "test_finished"
			data["details"] = map[string]bool{"passed": trigger == "test_passed"}
		}
		addEvent(typ, data)
		observe()
		observe()
		if jobCount(rule.ID) != 1 {
			t.Fatal("event not deduplicated", trigger, jobCount(rule.ID))
		}
		var due time.Time
		if er := db.QueryRow("SELECT d.next_attempt_at FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1", rule.ID).Scan(&due); er != nil || !due.After(time.Now().Add(59*time.Minute)) {
			t.Fatal("delay not persisted", er, due)
		}
	}
	inactive := createRule("inactive", "immediate", 0)
	if _, er := db.Exec("UPDATE academy_enrollments SET last_activity_at=now()-INTERVAL '6 days' WHERE id=$1", eid); er != nil {
		t.Fatal(er)
	}
	observe()
	observe()
	if jobCount(inactive.ID) != 1 {
		t.Fatal("inactive dedup")
	}
	if _, er := db.Exec("UPDATE academy_enrollments SET last_activity_at=now() WHERE id=$1", eid); er != nil {
		t.Fatal(er)
	}
	process(mock)
	if er := db.QueryRow("SELECT d.error_code FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1", inactive.ID).Scan(&status); er != nil || status != "activity_resumed" {
		t.Fatal("stale inactive message sent", status, er)
	}
	// Changing a paused configuration starts a fresh generation only on activation.
	paused := createRule("lesson_opened", "immediate", 0)
	paused.Status = "paused"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, paused.ID), owner, paused, 200), &paused)
	paused.Config.InactiveDays = 6
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, paused.ID), owner, paused, 200), &paused)
	addEvent("lesson_opened", map[string]any{})
	paused.Status = "active"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, paused.ID), owner, paused, 200), &paused)
	observe()
	if jobCount(paused.ID) != 0 {
		t.Fatal("old event replayed after editing paused rule")
	}
	// A curator cannot keep sending after their explicit permission is revoked.
	staffCampaign := academy.Campaign{Name: "Scoped", Message: message, Audience: audience, Channel: "in_app", Category: "service", Timezone: "UTC", RequestKey: academy.Key()}
	_ = json.Unmarshal(request("POST", comm+"/campaigns", teacher, staffCampaign, 201), &staffCampaign)
	request("POST", fmt.Sprintf("%s/campaigns/%d/launch", comm, staffCampaign.ID), teacher, map[string]any{"confirm": true, "revision": staffCampaign.Revision}, 200)
	request("PUT", staffPath, owner, map[string]any{"user_id": teacher, "role": "curator", "can_communicate": false}, 200)
	process(mock)
	if er := db.QueryRow("SELECT status FROM academy_message_deliveries WHERE campaign_id=$1", staffCampaign.ID).Scan(&status); er != nil || status != "cancelled" {
		t.Fatal("staff permission bypass", status, er)
	}
	request("GET", fmt.Sprintf("%s/automations/%d/runs", comm, a.ID), owner, nil, 200)
	// Empty explicit selections cannot expand into the whole course.
	_ = json.Unmarshal(request("POST", comm+"/audience", owner, map[string]any{"audience": academy.Segment{Mode: "students"}, "channel": "in_app", "category": "service"}, 200), &found)
	if found.Count != 0 {
		t.Fatal("empty selection expanded")
	}

	// Collection totals include every page; filters do not falsify the aggregate counts.
	rawMessage, _ := json.Marshal(message)
	rawAudience, _ := json.Marshal(audience)
	if _, er := db.Exec("INSERT INTO academy_campaigns(course_id,created_by,name,message,audience,channel,category,timezone,request_key) SELECT $1,$2,'Page '||n,$3,$4,'email','service','UTC','paging:'||$5::text||':'||n FROM generate_series(1,101) n", course, owner, rawMessage, rawAudience, academy.Key()); er != nil {
		t.Fatal(er)
	}
	var page struct {
		Items []academy.Campaign `json:"items"`
		Stats struct {
			Total int `json:"total"`
		} `json:"stats"`
	}
	_ = json.Unmarshal(request("GET", comm+"/campaigns", owner, nil, 200), &page)
	if len(page.Items) != 100 || page.Stats.Total < 101 {
		t.Fatal("campaign pagination/stats", len(page.Items), page.Stats.Total)
	}
	total := page.Stats.Total
	cursor := page.Items[len(page.Items)-1].ID
	_ = json.Unmarshal(request("GET", fmt.Sprintf("%s/campaigns?before=%d", comm, cursor), owner, nil, 200), &page)
	if len(page.Items) == 0 || page.Stats.Total != total {
		t.Fatal("campaign next page")
	}
	_ = json.Unmarshal(request("GET", comm+"/campaigns?status=completed", owner, nil, 200), &page)
	for _, item := range page.Items {
		if item.Status != "completed" {
			t.Fatal("campaign status filter")
		}
	}
	request("GET", comm+"/automations?status=active", owner, nil, 200)
	request("POST", fmt.Sprintf("%s/campaigns/%d/cancel", comm, c.ID), owner, map[string]any{}, 409)

	// Future schedule events may be observed early, but an absolute action never precedes its trigger.
	futureRule := createRule("lesson_unlocked", "immediate", 0)
	futureRule.Status = "paused"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, futureRule.ID), owner, futureRule, 200), &futureRule)
	futureRule.Config.Steps[0].Timing = "at"
	futureRule.Config.Steps[0].At = time.Now().Add(-time.Hour).Format(time.RFC3339)
	futureRule.Status = "active"
	_ = json.Unmarshal(request("PUT", fmt.Sprintf("%s/automations/%d", comm, futureRule.ID), owner, futureRule, 200), &futureRule)
	opens := time.Now().Add(72 * time.Hour)
	sourceKey := academy.Key()
	if _, er := db.Exec("INSERT INTO academy_cohort_events(course_id,cohort_id,enrollment_id,actor_id,type,event_key,data,notify_at) VALUES($1,$2,$3,$4,'lesson_unlocked',$5,'{}',$6)", course, cohort.ID, eid, owner, sourceKey, opens); er != nil {
		t.Fatal(er)
	}
	observe()
	var queuedAt time.Time
	if er := db.QueryRow("SELECT d.next_attempt_at FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1 ORDER BY d.id DESC LIMIT 1", futureRule.ID).Scan(&queuedAt); er != nil || queuedAt.Before(opens.Add(-time.Second)) {
		t.Fatal("absolute action preceded future trigger", queuedAt, er)
	}
	// Editing a schedule invalidates already queued actions through the original source event.
	if _, er := db.Exec("UPDATE academy_cohort_events SET cancelled=true WHERE event_key=$1", sourceKey); er != nil {
		t.Fatal(er)
	}
	if _, er := db.Exec("UPDATE academy_message_deliveries SET next_attempt_at=now() WHERE run_id IN(SELECT id FROM academy_automation_runs WHERE automation_id=$1)", futureRule.ID); er != nil {
		t.Fatal(er)
	}
	process(mock)
	if er := db.QueryRow("SELECT d.error_code FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=$1 ORDER BY d.id DESC LIMIT 1", futureRule.ID).Scan(&status); er != nil || status != "source_event_cancelled" {
		t.Fatal("obsolete schedule action sent", status, er)
	}

}
