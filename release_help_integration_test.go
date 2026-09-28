package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func testReleaseHelp(t *testing.T, ctx context.Context) {
	insert := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx, q, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	cookies := map[int64]*http.Cookie{}
	var users []int64
	for i := 0; i < 3; i++ {
		id := insert(`INSERT INTO users(full_name,email,password_hash) VALUES('Help fixture',$1,repeat('*',60)) RETURNING id`, fmt.Sprintf("help%d@example.invalid", i))
		users = append(users, id)
		w := httptest.NewRecorder()
		if err := createSession(w, id); err != nil {
			t.Fatal(err)
		}
		cookies[id] = w.Result().Cookies()[0]
	}
	requester, expert, other := users[0], users[1], users[2]
	resume := insert(`INSERT INTO resumes(user_id,status,visibility) VALUES($1,'published','private') RETURNING id`, expert)
	topic := insert(`INSERT INTO help_topics(name) VALUES('Isolated help fixture') RETURNING id`)
	exec(`INSERT INTO resume_help_topics(resume_id,topic_id) VALUES($1,$2)`, resume, topic)
	request := func(uid int64, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if uid > 0 {
			r.AddCookie(cookies[uid])
		}
		w := httptest.NewRecorder()
		if path == "/api/v1/help/requests" {
			helpRequests(w, r)
		} else {
			helpRequestAction(w, r)
		}
		return w
	}
	create := func() *httptest.ResponseRecorder {
		return request(requester, "POST", "/api/v1/help/requests", fmt.Sprintf(`{"resume_id":%d,"topic_id":%d,"text":"Fixture help request"}`, resume, topic))
	}
	idFrom := func(w *httptest.ResponseRecorder) int64 {
		t.Helper()
		if w.Code != 201 {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		var v struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v.ID
	}
	action := func(uid, id int64, method, name, body string) *httptest.ResponseRecorder {
		return request(uid, method, fmt.Sprintf("/api/v1/help/requests/%d/%s", id, name), body)
	}
	expect := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("want %d got %d %s", code, w.Code, w.Body.String())
		}
	}
	// A hidden job-search profile may still offer explicitly selected public help.
	id := idFrom(create())
	exec(`UPDATE users SET is_blocked=TRUE WHERE id=$1`, expert)
	expect(create(), 400)
	exec(`UPDATE users SET is_system=TRUE WHERE id=$1`, expert)
	systemRequest := idFrom(create())
	exec(`UPDATE users SET is_blocked=FALSE,is_system=FALSE WHERE id=$1`, expert)
	exec(`UPDATE help_topics SET is_active=FALSE WHERE id=$1`, topic)
	expect(create(), 400)
	exec(`UPDATE help_topics SET is_active=TRUE WHERE id=$1`, topic)
	exec(`UPDATE resumes SET status='draft' WHERE id=$1`, resume)
	expect(create(), 400)
	exec(`UPDATE resumes SET status='published' WHERE id=$1`, resume)
	expect(action(other, id, "GET", "messages", ""), 404)
	expect(action(other, id, "POST", "messages", `{"text":"foreign"}`), 404)
	expect(action(other, id, "POST", "accept", `{"message":"foreign"}`), 404)
	expect(action(requester, id, "POST", "review", `{"rating":5,"text":"premature"}`), 400)
	expect(action(requester, id, "POST", "accept", `{"message":"wrong actor"}`), 400)
	expect(action(expert, id, "POST", "accept", `{"message":"Accepted fixture"}`), 200)
	expect(action(requester, id, "POST", "messages", `{"text":"Private help reply"}`), 201)
	for _, uid := range []int64{requester, expert} {
		w := action(uid, id, "GET", "messages", "")
		expect(w, 200)
		if !strings.Contains(w.Body.String(), "Private help reply") {
			t.Fatal("missing participant history")
		}
	}
	exec(`UPDATE users SET is_blocked=TRUE WHERE id=$1`, expert)
	expect(action(requester, id, "POST", "messages", `{"text":"blocked"}`), 404)
	expect(action(requester, id, "GET", "messages", ""), 200)
	exec(`UPDATE users SET is_blocked=FALSE WHERE id=$1`, expert)
	expect(action(expert, id, "POST", "complete", ""), 200)
	expect(action(requester, id, "POST", "messages", `{"text":"late"}`), 400)
	expect(action(other, id, "POST", "review", `{"rating":5}`), 404)
	expect(action(expert, id, "POST", "review", `{"rating":5}`), 400)
	expect(action(requester, id, "POST", "review", `{"rating":5,"text":"Fixture review"}`), 201)
	expect(action(requester, id, "POST", "review", `{"rating":5}`), 409)
	expect(action(expert, systemRequest, "POST", "decline", `{"reason":"Fixture decline"}`), 200)
	assertCount(t, `SELECT count(*) FROM notification_outbox WHERE recipient_email=$1`, 2, "help0@example.invalid")
	for i := 0; i < 3; i++ {
		raceID := insert(`INSERT INTO help_requests(requester_id,expert_id,topic_id,request_text,status) VALUES($1,$2,$3,'Race fixture','accepted') RETURNING id`, requester, expert, topic)
		var message, finish *httptest.ResponseRecorder
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			message = action(requester, raceID, "POST", "messages", `{"text":"Racing reply"}`)
		}()
		go func() { defer wg.Done(); finish = action(expert, raceID, "POST", "complete", "") }()
		wg.Wait()
		expect(finish, 200)
		count := 0
		if message.Code == 201 {
			count = 1
		} else {
			expect(message, 400)
		}
		assertCount(t, `SELECT count(*) FROM help_request_messages WHERE help_request_id=$1`, count, raceID)
		expect(action(requester, raceID, "POST", "messages", `{"text":"After finish"}`), 400)
	}
	// Notification failure rolls back creation and an expert's acceptance/message.
	pending := idFrom(create())
	exec(`CREATE FUNCTION reject_help_fixture_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.recipient_email IN ('help0@example.invalid','help1@example.invalid') THEN RAISE EXCEPTION 'fixture outbox failure'; END IF; RETURN NEW; END $$`)
	exec(`CREATE TRIGGER reject_help_fixture_outbox BEFORE INSERT ON notification_outbox FOR EACH ROW EXECUTE FUNCTION reject_help_fixture_outbox()`)
	defer func() {
		exec(`DROP TRIGGER reject_help_fixture_outbox ON notification_outbox`)
		exec(`DROP FUNCTION reject_help_fixture_outbox()`)
	}()
	var before int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM help_requests WHERE requester_id=$1`, requester).Scan(&before); err != nil {
		t.Fatal(err)
	}
	expect(create(), 500)
	assertCount(t, `SELECT count(*) FROM help_requests WHERE requester_id=$1`, before, requester)
	expect(action(expert, pending, "POST", "accept", `{"message":"Must roll back"}`), 500)
	assertCount(t, `SELECT count(*) FROM help_requests WHERE id=$1 AND status='new'`, 1, pending)
	assertCount(t, `SELECT count(*) FROM help_request_messages WHERE help_request_id=$1`, 0, pending)
}
