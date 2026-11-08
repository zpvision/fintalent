package courses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Dedicated cloud test DSN only. Never load .env or use the app DATABASE_URL.
func TestReferralIsolatedPostgres(t *testing.T) {
	dsn := os.Getenv("COURSE_REFERRALS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated cloud COURSE_REFERRALS_TEST_DSN is not configured")
	}
	config, e := pgx.ParseConfig(dsn)
	if e != nil {
		t.Fatal("invalid test DSN")
	}
	if config.Host == "200.169.191.202" || !strings.Contains(strings.ToLower(config.Database), "test") {
		t.Fatal("refusing non-test database")
	}
	base := stdlib.OpenDB(*config)
	t.Cleanup(func() { base.Close() })
	schema := fmt.Sprintf("referrals_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = base.Exec("CREATE SCHEMA " + quoted); e != nil {
		t.Fatal("cannot create isolated test schema")
	}
	t.Cleanup(func() {
		if _, err := base.Exec("DROP SCHEMA " + quoted + " CASCADE"); err != nil {
			t.Error("test schema cleanup failed")
		}
	})
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	defer db.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,full_name TEXT NOT NULL DEFAULT '',is_blocked BOOLEAN NOT NULL DEFAULT false);CREATE TABLE courses(id BIGINT PRIMARY KEY,owner_id BIGINT REFERENCES users(id),slug TEXT UNIQUE,status TEXT,pricing_mode TEXT DEFAULT 'single');CREATE TABLE course_tariffs(id BIGINT PRIMARY KEY,course_id BIGINT REFERENCES courses(id));INSERT INTO users(id) SELECT generate_series(1,10);INSERT INTO courses(id,owner_id,slug,status) VALUES(1,1,'course-a','published'),(2,2,'course-b','published'),(3,1,'draft','draft');`)
	migration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "091_course_referrals.sql"))
	if e != nil {
		t.Fatal(e)
	}
	exec(string(migration))
	exec(string(migration))
	h := New(db, func(r *http.Request) (int64, error) {
		id, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		return id, nil
	}, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "yes" })
	request := func(method, path, body string, user int64, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		switch path {
		case "/visit":
			h.referralVisit(w, r)
		case "/interest":
			h.courseInterest(w, r)
		default:
			h.owned(w, r)
		}
		return w
	}
	create := func(course int, name, code string, status int) int64 {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"name": name, "code": code})
		w := request("POST", fmt.Sprintf("/api/courses/my/%d/referrals", course), string(body), int64(course), nil)
		if w.Code != status {
			t.Fatalf("create %q: %d %s", code, w.Code, w.Body.String())
		}
		var out struct{ ID int64 }
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out.ID
	}
	first := create(1, "Telegram", "Telegram2026", 201)
	second := create(1, "Email", "email_2026", 201)
	create(1, "Duplicate", "TELEGRAM2026", 409)
	create(2, "Telegram B", "telegram2026", 201)
	create(1, "Bad", "bad code", 400)
	if w := request("GET", "/api/courses/my/1/referrals", "", 2, nil); w.Code != 404 {
		t.Fatal("foreign course link leak")
	}
	visit := func(slug, code string, user int64, cookie *http.Cookie) *http.Cookie {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"slug": slug, "code": code})
		w := request("POST", "/visit", string(body), user, cookie)
		if w.Code != 204 {
			t.Fatalf("visit: %d %s", w.Code, w.Body.String())
		}
		if cookies := w.Result().Cookies(); len(cookies) > 0 {
			return cookies[0]
		}
		return cookie
	}
	cookie := visit("course-a", "telegram2026", 0, nil)
	if cookie == nil {
		t.Fatal("missing guest cookie")
	}
	visit("course-a", "email_2026", 0, cookie)
	visit("course-b", "telegram2026", 0, cookie)
	interest := func(slug string, user int64, c *http.Cookie) {
		t.Helper()
		w := request("POST", "/interest", `{"slug":"`+slug+`","referral_link_id":999,"commission_percent":1}`, user, c)
		if w.Code != 204 {
			t.Fatalf("interest %d %s", w.Code, w.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		if w := request("POST", "/interest", `{"slug":"course-a"}`, 0, cookie); w.Code != 401 {
			t.Fatal("guest interest accepted")
		}
	}
	var n int
	check := func(q string, want int, args ...any) {
		t.Helper()
		if e := db.QueryRow(q, args...).Scan(&n); e != nil || n != want {
			t.Fatalf("assertion got=%d want=%d error=%v", n, want, e)
		}
	}
	check(`SELECT count(*) FROM course_interests`, 0)
	r := httptest.NewRequest("POST", "/login", nil)
	r.AddCookie(cookie)
	if e = BindReferralGuest(context.Background(), db, r, 3); e != nil {
		t.Fatal(e)
	}
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=3`, 2)
	check(`SELECT referral_link_id FROM course_referral_attributions WHERE user_id=3 AND course_id=1`, int(first))
	check(`SELECT count(*) FROM course_interests`, 0)
	interest("course-a", 3, nil)
	interest("course-a", 3, nil)
	check(`SELECT count(*) FROM course_interests WHERE user_id=3`, 1)
	visit("course-a", "email_2026", 3, nil)
	interest("course-a", 3, nil)
	check(`SELECT referral_link_id FROM course_referral_attributions WHERE user_id=3 AND course_id=1`, int(first))
	exec(`UPDATE course_referral_attributions SET first_clicked_at=now()-interval '10 years' WHERE user_id=3`)
	if e = BindReferralGuest(context.Background(), db, r, 3); e != nil {
		t.Fatal(e)
	}
	check(`SELECT count(*) FROM course_interests`, 1)
	// Deleted codes stay reserved and existing attribution survives.
	w := request("DELETE", fmt.Sprintf("/api/courses/my/1/referrals/%d", first), "", 1, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	create(1, "Cannot reuse", "telegram2026", 409)
	visit("course-a", "telegram2026", 4, nil)
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=4`, 0)
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=3`, 2)
	// A claim on a removed link cannot bind a new account.
	c := visit("course-a", "email_2026", 0, nil)
	exec(`UPDATE course_referral_links SET deleted_at=now() WHERE id=$1`, second)
	r2 := httptest.NewRequest("POST", "/login", nil)
	r2.AddCookie(c)
	if e = BindReferralGuest(context.Background(), db, r2, 5); e != nil {
		t.Fatal(e)
	}
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=5`, 0)
	// Plain visits and interests on another course do not erase account source.
	interest("course-b", 3, nil)
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=3`, 2)
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request("POST", "/visit", `{"slug":"course-b","code":"telegram2026"}`, 6, nil)
			statuses <- w.Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 204 {
			t.Fatal("concurrent visit failed", status)
		}
	}
	check(`SELECT count(*) FROM course_referral_attributions WHERE user_id=6 AND course_id=2`, 1)
	if w := request("POST", "/interest", `{"slug":"draft"}`, 3, nil); w.Code != 404 {
		t.Fatal("draft accepted")
	}
	interest("course-b", 9, nil)
	visit("course-b", "telegram2026", 9, nil)
	interest("course-b", 9, nil)
	check(`SELECT count(*) FROM course_interests WHERE user_id=9 AND course_id=2 AND referral_link_id IS NOT NULL`, 1)
	// Failed attempts to delete a foreign course's link cannot erase history.
	w = request("DELETE", fmt.Sprintf("/api/courses/my/2/referrals/%d", first), "", 2, nil)
	if w.Code != 404 {
		t.Fatal("foreign course deletion accepted")
	}
	// Existing unique constraints prevent a cross-course link association.
	_, e = db.Exec(`INSERT INTO course_referral_attributions(user_id,course_id,referral_link_id,first_clicked_at) VALUES(7,2,$1,now())`, first)
	if e == nil {
		t.Fatal("cross-course attribution allowed")
	}
}
