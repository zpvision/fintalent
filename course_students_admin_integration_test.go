package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"FinTalent/internal/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Opt-in cloud test: synthetic records in a fresh verified schema only.
func TestCourseStudentsAdminIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_COURSE_STUDENTS_ADMIN_DB_TESTS") != "1" {
		t.Skip("requires opt-in isolated cloud PostgreSQL test")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || os.Getenv("DATABASE_URL") == "" {
		t.Fatal("cloud DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	base := stdlib.OpenDB(*config)
	defer base.Close()
	schema := fmt.Sprintf("course_students_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "course_students_test_") {
			t.Fatal("unsafe cleanup target")
		}
		if _, err := base.ExecContext(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	defer isolated.Close()
	var actual string
	if err := isolated.QueryRowContext(ctx, "SELECT current_schema()").Scan(&actual); err != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := isolated.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE users(id bigint PRIMARY KEY,full_name text NOT NULL,email text NOT NULL);
 CREATE TABLE courses(id bigint PRIMARY KEY,title text NOT NULL,owner_id bigint REFERENCES users(id),pricing_mode text NOT NULL,price bigint);
 CREATE TABLE course_tariffs(id bigint PRIMARY KEY,course_id bigint REFERENCES courses(id),title text NOT NULL,price bigint);
 INSERT INTO users VALUES(1,'Автор А','author-a@example.invalid'),(2,'Автор Б','author-b@example.invalid'),(3,'Текущая Ирина','account@example.invalid'),(4,'Алексей Тест','alex@example.invalid'),(5,'Ольга Тест','olga@example.invalid');
 INSERT INTO courses VALUES(10,'Первый курс',1,'tariffs',NULL),(11,'Бесплатный курс',1,'single',0),(12,'Второй автор',2,'tariffs',NULL),(13,'Удалённый тариф',2,'tariffs',NULL);
 INSERT INTO course_tariffs VALUES(100,10,'Текущий тариф',999999),(200,12,'Текущий старый тариф',200000)`)
	exec(courseReferralsSchema)
	exec(courseInterestContactsSchema)
	exec(courseInterestPriceSchema)
	exec(`ALTER TABLE course_interests ADD COLUMN paid_at timestamptz,ADD COLUMN paid_enrollment_id bigint,ADD COLUMN invitation_outbox_id bigint;
 INSERT INTO course_referral_guests(token_hash) VALUES(repeat('a',64));
 INSERT INTO course_interests(course_id,user_id,tariff_id,student_name,contact_email,tariff_name,selected_price) VALUES
 (10,3,100,'Ирина Историческая','snapshot@example.invalid','Выбранный тариф',150000),
 (11,3,NULL,'','','',0),(12,4,200,'','','',NULL),(13,5,999,'','','',NULL),
 (10,4,100,'Алексей Оплатил','paid@example.invalid','Оплаченный тариф',300000);
 UPDATE course_interests SET paid_at=now(),paid_enrollment_id=42,invitation_outbox_id=43 WHERE user_id=4 AND course_id=10;
 INSERT INTO course_interests(course_id,guest_hash) VALUES(10,repeat('a',64))`)
	mux := http.NewServeMux()
	courses.New(isolated, func(*http.Request) (int64, error) { return 3, nil }, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "yes" }).Register(mux)
	handler := requestSecurity(mux)
	request := func(method, path string, admin bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if admin {
			r.Header.Set("X-Test-Admin", "yes")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("got %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	type stats struct {
		Applications int   `json:"applications"`
		Students     int   `json:"students"`
		TotalPrice   int64 `json:"total_price"`
		Unknown      int   `json:"unknown_prices"`
		Current      int   `json:"current_prices"`
	}
	type report struct {
		Total   int   `json:"total"`
		Summary stats `json:"summary"`
		Overall stats `json:"overall"`
		Authors []any `json:"authors"`
		Items   []struct {
			ID        int64  `json:"id"`
			Tariff    string `json:"tariff_name"`
			Price     *int64 `json:"selected_price"`
			CanDelete bool   `json:"can_delete"`
		} `json:"items"`
	}
	get := func(query string) report {
		t.Helper()
		w := request("GET", "/api/admin/courses/students"+query, true)
		check(w, 200)
		var d report
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	path := "/api/admin/courses/students"
	for _, method := range []string{"GET", "DELETE", "POST"} {
		check(request(method, path, false), 401)
		check(request(method, path+"/1", false), 401)
	}
	check(request("POST", path, true), 405)
	check(request("GET", path+"/1", true), 405)
	for _, query := range []string{"?page=0", "?page=bad", "?author_id=-1", "?author_id=bad", "?student_name=" + strings.Repeat("x", 201)} {
		check(request("GET", path+query, true), 400)
	}
	d := get("")
	if d.Total != 5 || d.Overall.Applications != 5 || d.Overall.Students != 3 || d.Overall.TotalPrice != 650000 || d.Overall.Unknown != 1 || d.Overall.Current != 1 || len(d.Authors) != 2 {
		t.Fatalf("incorrect global totals: %+v", d)
	}
	var unpaid, paid int64
	for _, item := range d.Items {
		if item.Tariff == "Выбранный тариф" {
			unpaid = item.ID
			if item.Price == nil || *item.Price != 150000 || !item.CanDelete {
				t.Fatal("snapshot changed or deletion missing")
			}
		}
		if item.Tariff == "Оплаченный тариф" {
			paid = item.ID
			if item.CanDelete {
				t.Fatal("paid deletion enabled")
			}
		}
	}
	if unpaid == 0 || paid == 0 {
		t.Fatal("missing fixture applications")
	}
	q := "?author_id=1&student_name=" + url.QueryEscape("ирина") + "&email=" + url.QueryEscape("SNAPSHOT@")
	d = get(q)
	if d.Total != 1 || d.Summary.Students != 1 || d.Summary.TotalPrice != 150000 || d.Overall.TotalPrice != 650000 {
		t.Fatalf("combined filters or totals failed: %+v", d)
	}
	d = get("?author_id=1")
	if d.Total != 3 || d.Summary.Students != 2 || d.Summary.TotalPrice != 450000 {
		t.Fatalf("author filter: %+v", d)
	}
	for _, q := range []string{"?student_name=" + url.QueryEscape("Алексей"), "?email=alex@"} {
		want := 1
		if strings.Contains(q, "student_name") {
			want = 2
		}
		if get(q).Total != want {
			t.Fatal("legacy contact filter failed")
		}
	}
	if get("?student_name=%25").Total != 0 || get("?author_id=999999").Total != 0 {
		t.Fatal("empty or literal search failed")
	}
	check(request("DELETE", fmt.Sprintf("%s/%d", path, paid), true), 409)
	check(request("DELETE", path+"/bad", true), 400)
	check(request("DELETE", path+"/99999999", true), 404)
	r := httptest.NewRequest("DELETE", fmt.Sprintf("%s/%d", path, unpaid), nil)
	r.Header.Set("X-Test-Admin", "yes")
	r.Header.Set("Origin", "https://evil.invalid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	check(w, 403)
	check(request("DELETE", fmt.Sprintf("%s/%d", path, unpaid), true), 200)
	check(request("DELETE", fmt.Sprintf("%s/%d", path, unpaid), true), 404)
	d = get("")
	if d.Total != 4 || d.Overall.TotalPrice != 500000 || d.Overall.Students != 3 {
		t.Fatalf("totals did not update after delete: %+v", d)
	}
	var count int
	if err := isolated.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil || count != 5 {
		t.Fatal("deletion touched account")
	}
	// Concurrent payment wins: a DELETE waiting on that row must recheck paid_at.
	var raceID int64
	if err := isolated.QueryRowContext(ctx, `SELECT id FROM course_interests WHERE course_id=11 AND user_id=3`).Scan(&raceID); err != nil {
		t.Fatal(err)
	}
	tx, err := isolated.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE course_interests SET paid_at=now() WHERE id=$1`, raceID); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request("DELETE", fmt.Sprintf("%s/%d", path, raceID), true) }()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(<-done, 409)
	exec(`INSERT INTO users SELECT n,'Ученик '||n,'student'||n||'@example.invalid' FROM generate_series(6,60) n;
 INSERT INTO course_interests(course_id,user_id,selected_price) SELECT 10,n,10000 FROM generate_series(6,60) n`)
	d = get("?page=1")
	second := get("?page=2")
	if d.Total != 59 || len(d.Items) != 50 || len(second.Items) != 9 || d.Overall.TotalPrice != second.Overall.TotalPrice {
		t.Fatal("pagination totals incorrect")
	}
}
