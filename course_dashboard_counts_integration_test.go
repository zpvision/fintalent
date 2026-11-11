package main

import (
	"FinTalent/internal/courses"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestCourseDashboardCountsPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	if _, err := f.DB.Exec(`UPDATE users SET courses_access=true WHERE id=$1`, f.Owner); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	courses.New(f.DB, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, func(*http.Request) bool { return false }).Register(mux)
	get := func(path string) []map[string]json.RawMessage {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("X-Test-User", strconv.FormatInt(f.Owner, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	check := func(students, unpaid int) {
		t.Helper()
		rows := get("/api/courses/my")
		if len(rows) != 1 {
			t.Fatal("expected owner's course")
		}
		var a, b int
		json.Unmarshal(rows[0]["student_count"], &a)
		json.Unmarshal(rows[0]["unpaid_interest_count"], &b)
		if a != students || b != unpaid {
			t.Fatalf("counts %d/%d want %d/%d", a, b, students, unpaid)
		}
		r := httptest.NewRequest("GET", "/api/academy/courses/"+strconv.FormatInt(f.Course, 10)+"?summary=1", nil)
		r.Header.Set("X-Test-User", strconv.FormatInt(f.Owner, 10))
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal("workspace summary failed", w.Code)
		}
		var summary struct {
			Course struct {
				Unpaid int `json:"unpaid_interest_count"`
			}
		}
		if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil || summary.Course.Unpaid != unpaid {
			t.Fatal("workspace badge count", err, summary.Course.Unpaid)
		}
	}
	check(0, 0)
	for i, uid := range []int64{f.Student, f.Student, f.Teacher} {
		status := "active"
		if i == 1 {
			status = "revoked"
		}
		if _, err := f.DB.Exec(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key,access_status) VALUES($1,$2,$3,$4,$5,$6)`, f.Course, uid, f.Version, f.Owner, "dashboard-count-"+strconv.Itoa(i), status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.DB.Exec(`INSERT INTO course_interests(course_id,user_id) VALUES($1,$2),($1,$3)`, f.Course, f.Student, f.Teacher); err != nil {
		t.Fatal(err)
	}
	check(2, 2)
	if _, err := f.DB.Exec(`WITH mail AS (INSERT INTO notification_outbox(kind,recipient_email,status) VALUES('test_dashboard','dashboard@example.invalid','failed') RETURNING id) UPDATE course_interests SET paid_at=now(),paid_commission_bps=2000,paid_enrollment_id=(SELECT id FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 ORDER BY id LIMIT 1),invitation_outbox_id=(SELECT id FROM mail) WHERE course_id=$1 AND user_id=$2`, f.Course, f.Student); err != nil {
		t.Fatal(err)
	}
	check(2, 1)
	for _, row := range get("/api/courses") {
		if row["student_count"] != nil || row["unpaid_interest_count"] != nil {
			t.Fatal("private dashboard counters leaked into public catalog")
		}
	}
}
