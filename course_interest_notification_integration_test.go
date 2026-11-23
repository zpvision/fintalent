package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"FinTalent/internal/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// No workers or real SMTP: application/outbox atomicity in a fresh cloud schema.
func TestCourseInterestNotificationIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_COURSE_INTEREST_NOTIFICATION_DB_TESTS") != "1" {
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
	schema := fmt.Sprintf("course_interest_mail_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := base.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "course_interest_mail_") {
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
	exec(`CREATE TABLE users(id bigint PRIMARY KEY,full_name text NOT NULL,email text NOT NULL,is_blocked boolean NOT NULL DEFAULT false,is_system boolean NOT NULL DEFAULT false,courses_access boolean NOT NULL DEFAULT true);
 CREATE TABLE courses(id bigint PRIMARY KEY,owner_id bigint REFERENCES users(id),title text NOT NULL,slug text NOT NULL,status text NOT NULL,pricing_mode text NOT NULL,price bigint NOT NULL DEFAULT 0);
 CREATE TABLE course_tariffs(id bigint PRIMARY KEY,course_id bigint REFERENCES courses(id),title text NOT NULL,price bigint NOT NULL);
 CREATE TABLE course_access_settings(id boolean PRIMARY KEY,public_access boolean);INSERT INTO course_access_settings VALUES(true,true);
 CREATE TABLE profimarket_purchases(id bigint PRIMARY KEY,buyer_user_id bigint);
 INSERT INTO users(id,full_name,email) VALUES(1,'Автор Первый','first-author@example.invalid'),(2,'Автор Второй','second-author@example.invalid'),(3,'Анна Ученица','account@example.invalid'),(4,'Борис Ученик','boris@example.invalid'),(5,'Ученик отката','rollback@example.invalid');
 INSERT INTO courses VALUES(101,1,'Финансовая модель','finance','published','tariffs',0),(102,2,'Бесплатный курс','free','published','single',0),(103,1,'Черновик','draft','draft','single',0);
 INSERT INTO course_tariffs VALUES(501,101,'С сопровождением',1490000),(502,102,'Чужой тариф',30000)`)
	exec(courseReferralsSchema)
	exec(courseInterestContactsSchema)
	exec(courseInterestPriceSchema)
	exec(notificationOutboxSchema)
	exec(`ALTER TABLE course_interests ADD COLUMN paid_at timestamptz`)
	h := courses.New(isolated, func(r *http.Request) (int64, error) {
		id, _ := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		return id, nil
	}, func(*http.Request) bool { return false })
	h.SetInterestNotifier(enqueueCourseInterestNotification)
	mux := http.NewServeMux()
	h.Register(mux)
	handler := requestSecurity(mux)
	request := func(user int64, slug string, tariff int64) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"slug": slug, "tariff_id": tariff, "contact_email": "contact@example.invalid", "contact_phone": "+7 (999) 123-45-67", "student_name": "FORGED"})
		r := httptest.NewRequest("POST", "/api/courses/interest", strings.NewReader(string(raw)))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
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
	count := func(table string, want int) {
		t.Helper()
		var n int
		if err := isolated.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != want {
			t.Fatalf("%s count %d want %d: %v", table, n, want, err)
		}
	}
	check(request(0, "finance", 501), 401)
	check(request(3, "finance", 502), 400)
	check(request(3, "draft", 0), 404)
	count("notification_outbox", 0)
	check(request(3, "finance", 501), 204)
	count("course_interests", 1)
	count("notification_outbox", 1)
	var recipient, key string
	var payload []byte
	if err := isolated.QueryRowContext(ctx, `SELECT recipient_email,dedup_key,payload FROM notification_outbox`).Scan(&recipient, &key, &payload); err != nil {
		t.Fatal(err)
	}
	var data eventNotificationEmailData
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatal(err)
	}
	if recipient != "first-author@example.invalid" || !strings.HasPrefix(key, "course-interest-author:") || len(data.Facts) != 3 || data.Facts[0].Value != "Анна Ученица" || data.Facts[1].Value != "contact@example.invalid" || data.Facts[2].Value != "+79991234567" || !strings.Contains(data.Details, "14 900,00 ₽") || !strings.Contains(data.ButtonURL, "/profile/courses/101/academy/analytics?") {
		t.Fatalf("wrong author or contact snapshot: %s %+v", recipient, data)
	}
	check(request(3, "finance", 501), 204)
	count("notification_outbox", 1)
	// A delivery failure leaves the application saved and the mail queued for retry.
	original := db
	db = isolated
	defer func() { db = original }()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	processOutbox(ctx)
	var status string
	var attempts int
	if err := isolated.QueryRowContext(ctx, `SELECT status,attempts FROM notification_outbox`).Scan(&status, &attempts); err != nil || status != "pending" || attempts != 1 {
		t.Fatal("failed SMTP was not retained for retry", err)
	}
	count("course_interests", 1)
	// Sent messages also retain the deduplication key after their payload is erased.
	exec(`UPDATE notification_outbox SET status='sent',payload='{}',recipient_email='',recipient_name=''`)
	check(request(3, "finance", 501), 204)
	count("notification_outbox", 1)
	// Two concurrent submissions represent one application and one notification.
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; codes[i] = request(4, "finance", 501).Code }(i)
	}
	close(start)
	wg.Wait()
	if codes[0] != 204 || codes[1] != 204 {
		t.Fatalf("concurrent interest: %v", codes)
	}
	count("notification_outbox", 2)
	count("course_interests", 2)
	exec(`UPDATE course_interests SET paid_at=now() WHERE user_id=4`)
	check(request(4, "finance", 501), 409)
	count("notification_outbox", 2)
	// A failed enqueue rolls both records back, so a later retry can recover.
	h.SetInterestNotifier(func(ctx context.Context, tx *sql.Tx, event courses.InterestNotification) error {
		if err := enqueueCourseInterestNotification(ctx, tx, event); err != nil {
			return err
		}
		return errors.New("synthetic enqueue failure")
	})
	check(request(5, "finance", 501), 500)
	count("course_interests", 2)
	count("notification_outbox", 2)
	h.SetInterestNotifier(enqueueCourseInterestNotification)
	check(request(5, "finance", 501), 204)
	count("course_interests", 3)
	count("notification_outbox", 3)
	check(request(3, "free", 0), 204)
	count("notification_outbox", 4)
	var freePayload []byte
	if err := isolated.QueryRowContext(ctx, `SELECT payload FROM notification_outbox WHERE recipient_email='second-author@example.invalid'`).Scan(&freePayload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(freePayload), "Основной тариф") || !strings.Contains(string(freePayload), "0,00 ₽") {
		t.Fatal("single-price notification missing")
	}
}
