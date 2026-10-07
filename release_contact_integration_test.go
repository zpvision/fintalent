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

func testReleaseContacts(t *testing.T, ctx context.Context) {
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
	users := []int64{}
	for i := 0; i < 3; i++ {
		id := insert(`INSERT INTO users(full_name,email,password_hash) VALUES('Contact fixture',$1,repeat('*',60)) RETURNING id`, fmt.Sprintf("contact%d@example.invalid", i))
		users = append(users, id)
		w := httptest.NewRecorder()
		if err := createSession(w, id); err != nil {
			t.Fatal(err)
		}
		cookies[id] = w.Result().Cookies()[0]
	}
	sender, recipient, other := users[0], users[1], users[2]
	resume := insert(`INSERT INTO resumes(user_id,status,visibility) VALUES($1,'published','public') RETURNING id`, recipient)
	request := func(uid int64, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if uid != 0 {
			r.AddCookie(cookies[uid])
		}
		w := httptest.NewRecorder()
		if path == "/api/v1/contact-threads" {
			contactThreads(w, r)
		} else {
			contactThreadAction(w, r)
		}
		return w
	}
	body := fmt.Sprintf(`{"resume_id":%d,"subject":"Fixture","message":"Private fixture message"}`, resume)
	create := func(uid int64) *httptest.ResponseRecorder {
		return request(uid, "POST", "/api/v1/contact-threads", body)
	}
	if w := create(0); w.Code != 401 {
		t.Fatalf("guest create: %d", w.Code)
	}
	if w := create(recipient); w.Code != 400 {
		t.Fatalf("self create: %d", w.Code)
	}
	for _, state := range []string{"draft", "private", "blocked", "deleted"} {
		switch state {
		case "draft":
			exec(`UPDATE resumes SET status='draft' WHERE id=$1`, resume)
		case "private":
			exec(`UPDATE resumes SET status='published',visibility='private' WHERE id=$1`, resume)
		case "blocked":
			exec(`UPDATE resumes SET visibility='public' WHERE id=$1`, resume)
			exec(`UPDATE users SET is_blocked=TRUE WHERE id=$1`, recipient)
		case "deleted":
			exec(`UPDATE users SET is_blocked=FALSE WHERE id=$1`, recipient)
			exec(`UPDATE resumes SET deleted_at=NOW() WHERE id=$1`, resume)
		}
		if w := create(sender); w.Code != 404 {
			t.Fatalf("%s parent: %d %s", state, w.Code, w.Body.String())
		}
	}
	assertCount(t, `SELECT count(*) FROM contact_threads WHERE sender_id=$1`, 0, sender)
	exec(`UPDATE resumes SET deleted_at=NULL WHERE id=$1`, resume)
	// Four concurrent requests must respect the existing two-per-week rule.
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- create(sender) }()
	}
	wg.Wait()
	close(results)
	var thread int64
	accepted, limited := 0, 0
	for w := range results {
		if w.Code == 201 {
			accepted++
			var result struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			thread = result.ID
		} else if w.Code == 429 {
			limited++
		} else {
			t.Fatalf("concurrent create: %d %s", w.Code, w.Body.String())
		}
	}
	if accepted != 2 || limited != 2 {
		t.Fatalf("create limits: %d/%d", accepted, limited)
	}
	assertCount(t, `SELECT count(*) FROM contact_threads WHERE sender_id=$1`, 2, sender)
	assertCount(t, `SELECT count(*) FROM notification_outbox WHERE recipient_email=$1`, 2, "contact1@example.invalid")
	path := fmt.Sprintf("/api/v1/contact-threads/%d/", thread)
	for _, action := range []string{"messages", "accept", "decline", "block", "report"} {
		method := "POST"
		if action == "messages" {
			method = "GET"
		}
		if w := request(other, method, path+action, `{"reason":"Fixture reason"}`); w.Code != 404 {
			t.Fatalf("foreign %s: %d", action, w.Code)
		}
	}
	if w := request(sender, "POST", path+"accept", ""); w.Code != 409 {
		t.Fatalf("sender accept: %d", w.Code)
	}
	if w := request(sender, "POST", path+"messages", `{"message":"premature"}`); w.Code != 409 {
		t.Fatalf("pending message: %d", w.Code)
	}
	if w := request(recipient, "POST", path+"accept", ""); w.Code != 200 {
		t.Fatalf("recipient accept: %d %s", w.Code, w.Body.String())
	}
	for _, uid := range []int64{sender, recipient} {
		if w := request(uid, "POST", path+"messages", `{"message":"Accepted reply"}`); w.Code != 200 {
			t.Fatalf("participant reply: %d", w.Code)
		}
		if w := request(uid, "GET", path+"messages", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Private fixture message") {
			t.Fatalf("participant history: %d", w.Code)
		}
	}
	exec(`UPDATE users SET is_blocked=TRUE WHERE id=$1`, recipient)
	if w := request(sender, "POST", path+"messages", `{"message":"blocked reply"}`); w.Code != 404 {
		t.Fatalf("blocked participant write: %d", w.Code)
	}
	if w := request(sender, "GET", path+"messages", ""); w.Code != 200 {
		t.Fatalf("retained personal history: %d", w.Code)
	}
	exec(`UPDATE users SET is_blocked=FALSE WHERE id=$1`, recipient)
	if w := request(recipient, "POST", path+"block", ""); w.Code != 200 {
		t.Fatalf("block: %d", w.Code)
	}
	if w := create(sender); w.Code != 403 {
		t.Fatalf("new request after block: %d", w.Code)
	}
	if w := request(sender, "POST", path+"messages", `{"message":"blocked thread reply"}`); w.Code != 409 {
		t.Fatalf("blocked thread message: %d", w.Code)
	}
	assertCount(t, `SELECT count(*) FROM contact_messages WHERE thread_id=$1`, 3, thread)
	// Inject an outbox failure in this disposable schema: no partial conversation.
	exec(`CREATE FUNCTION reject_contact_fixture_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.recipient_email='contact1@example.invalid' THEN RAISE EXCEPTION 'fixture outbox failure'; END IF; RETURN NEW; END $$`)
	exec(`CREATE TRIGGER reject_contact_fixture_outbox BEFORE INSERT ON notification_outbox FOR EACH ROW EXECUTE FUNCTION reject_contact_fixture_outbox()`)
	defer func() {
		exec(`DROP TRIGGER reject_contact_fixture_outbox ON notification_outbox`)
		exec(`DROP FUNCTION reject_contact_fixture_outbox()`)
	}()
	if w := create(other); w.Code != 500 {
		t.Fatalf("outbox failure: %d", w.Code)
	}
	assertCount(t, `SELECT count(*) FROM contact_threads WHERE sender_id=$1`, 0, other)
}
