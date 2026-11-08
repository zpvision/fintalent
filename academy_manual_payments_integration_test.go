package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCourseManualPaymentsIsolatedPostgres(t *testing.T) {
	dsn := os.Getenv("COURSE_REFERRALS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated cloud test DSN is not configured")
	}
	c, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	if c.Host == "200.169.191.202" || c.Host == "localhost" || c.Host == "127.0.0.1" || !strings.Contains(strings.ToLower(c.Database), "test") {
		t.Fatal("refusing non-cloud-test database")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("RUN_COURSES_DB_TESTS", "1")
	f := newAcademyFocusedFixture(t)
	f.Handler.SetPaidInvitationEnqueuer(enqueueAcademyPaidInvitation)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, e := f.DB.Exec(query, args...); e != nil {
			t.Fatal(e)
		}
	}
	scalar := func(query string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := f.DB.QueryRow(query, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	var source, id int64
	if e := f.DB.QueryRow(`INSERT INTO course_referral_links(course_id,name,code,created_by) VALUES($1,'Telegram','telegram',$2) RETURNING id`, f.Course, f.Owner).Scan(&source); e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO course_referral_attributions(user_id,course_id,referral_link_id,first_clicked_at) VALUES($1,$2,$3,now())`, f.Student, f.Course, source)
	if e := f.DB.QueryRow(`INSERT INTO course_interests(user_id,course_id,referral_link_id) VALUES($1,$2,$3) RETURNING id`, f.Student, f.Course, source).Scan(&id); e != nil {
		t.Fatal(e)
	}
	request := func(actor, interest, version int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", fmt.Sprintf("/api/academy/courses/%d/analytics/interests/%d/paid", f.Course, interest), strings.NewReader(fmt.Sprintf(`{"confirm":true,"version_id":%d,"commission_bps":0}`, version)))
		r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		return w
	}
	if w := request(f.Teacher, id, f.Version); w.Code != 404 {
		t.Fatal("outsider can confirm payment", w.Code)
	}
	if w := request(f.Owner, id, f.Version+99999); w.Code != 400 {
		t.Fatal("unpublished version accepted", w.Code)
	}
	if n := scalar(`SELECT count(*) FROM notification_outbox WHERE dedup_key LIKE 'academy-paid-interest:%'`); n != 0 {
		t.Fatal("failed operation queued mail")
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); statuses <- request(f.Owner, id, f.Version).Code }()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatal("concurrent payment failed", status)
		}
	}
	if scalar(`SELECT paid_commission_bps FROM course_interests WHERE id=$1`, id) != 1200 {
		t.Fatal("partner rate or client override")
	}
	if scalar(`SELECT count(*) FROM academy_enrollments WHERE user_id=$1 AND course_id=$2`, f.Student, f.Course) != 1 {
		t.Fatal("duplicate enrollment")
	}
	if scalar(`SELECT count(*) FROM notification_outbox WHERE dedup_key=$1`, fmt.Sprintf("academy-paid-interest:%d", id)) != 1 {
		t.Fatal("duplicate invitation")
	}
	exec(`UPDATE course_commission_terms SET standard_bps=1800,partner_bps=900 WHERE course_id=$1`, f.Course)
	exec(academyManualPaymentsSchema)
	if scalar(`SELECT paid_commission_bps FROM course_interests WHERE id=$1`, id) != 1200 {
		t.Fatal("paid rate changed")
	}
	if scalar(`SELECT standard_bps FROM course_commission_terms WHERE course_id=$1`, f.Course) != 1800 {
		t.Fatal("migration overwrote custom rate")
	}
	if _, e := f.DB.Exec(`UPDATE course_interests SET paid_commission_bps=100 WHERE id=$1`, id); e == nil {
		t.Fatal("paid snapshot mutable")
	}
	// Queue errors roll back both the payment and the new enrollment.
	var other int64
	if e := f.DB.QueryRow(`INSERT INTO course_interests(user_id,course_id) VALUES($1,$2) RETURNING id`, f.Teacher, f.Course).Scan(&other); e != nil {
		t.Fatal(e)
	}
	f.Handler.SetPaidInvitationEnqueuer(func(context.Context, *sql.Tx, int64, int64, int64, int64) (int64, error) {
		return 0, errors.New("mock queue unavailable")
	})
	if w := request(f.Owner, other, f.Version); w.Code != 500 {
		t.Fatal("queue error not reported")
	}
	if scalar(`SELECT count(*) FROM academy_enrollments WHERE user_id=$1 AND course_id=$2`, f.Teacher, f.Course) != 0 {
		t.Fatal("partial enrollment committed")
	}
	if scalar(`SELECT count(*) FROM course_interests WHERE id=$1 AND paid_at IS NOT NULL`, other) != 0 {
		t.Fatal("partial payment committed")
	}
	f.Handler.SetPaidInvitationEnqueuer(enqueueAcademyPaidInvitation)
	if w := request(f.Owner, other, f.Version); w.Code != 200 {
		t.Fatal("retry after rollback failed", w.Body.String())
	}
	if scalar(`SELECT paid_commission_bps FROM course_interests WHERE id=$1`, other) != 1800 {
		t.Fatal("standard per-course rate not used")
	}
	exec(`UPDATE course_commission_defaults SET standard_bps=1700,partner_bps=800 WHERE id=true`)
	var newCourse int64
	if e := f.DB.QueryRow(`INSERT INTO courses(owner_id,slug) VALUES($1,'new-rate-course') RETURNING id`, f.Owner).Scan(&newCourse); e != nil {
		t.Fatal(e)
	}
	if scalar(`SELECT standard_bps FROM course_commission_terms WHERE course_id=$1`, newCourse) != 1700 {
		t.Fatal("new course default not copied")
	}
	if scalar(`SELECT standard_bps FROM course_commission_terms WHERE course_id=$1`, f.Course) != 1800 {
		t.Fatal("defaults affected existing course")
	}
	var individual, application int64
	if e := f.DB.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES('Individual student','individual-payment@example.invalid',repeat('*',60)) RETURNING id`).Scan(&individual); e != nil {
		t.Fatal(e)
	}
	if e := f.DB.QueryRow(`INSERT INTO course_interests(course_id,user_id,commission_override_bps) VALUES($1,$2,0) RETURNING id`, f.Course, individual).Scan(&application); e != nil {
		t.Fatal(e)
	}
	if w := request(f.Owner, application, f.Version); w.Code != 200 {
		t.Fatal("individual rate confirmation failed", w.Body.String())
	}
	if scalar(`SELECT paid_commission_bps FROM course_interests WHERE id=$1`, application) != 0 {
		t.Fatal("zero individual override ignored")
	}
}
