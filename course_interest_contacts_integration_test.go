package main

import (
	"FinTalent/internal/courses"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestCourseInterestContactsPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := f.DB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE course_access_settings SET public_access=true WHERE id=true`)
	exec(`UPDATE courses SET status='published',pricing_mode='tariffs' WHERE id=$1`, f.Course)
	var slug string
	f.DB.QueryRow(`SELECT slug FROM courses WHERE id=$1`, f.Course).Scan(&slug)
	var tariff int64
	if err := f.DB.QueryRow(`INSERT INTO course_tariffs(course_id,title,price) VALUES($1,'Contact test tariff',120000) RETURNING id`, f.Course).Scan(&tariff); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	courses.New(f.DB, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "true" }).Register(mux)
	request := func(method, path string, user int64, admin bool, body any, status int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		if admin {
			r.Header.Set("X-Test-Admin", "true")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	body := map[string]any{"slug": slug, "tariff_id": tariff, "contact_email": "contact@example.invalid", "contact_phone": "+7 (999) 123-45-67", "student_name": "FORGED", "user_id": f.Teacher}
	request("POST", "/api/courses/interest", 0, false, body, 401)
	invalid := map[string]any{"slug": slug, "tariff_id": tariff, "contact_email": "bad", "contact_phone": "123"}
	request("POST", "/api/courses/interest", f.Student, false, invalid, 400)
	request("POST", "/api/courses/interest", f.Student, false, body, 204)
	var name, accountName, email, phone, tariffName, at string
	var user int64
	if err := f.DB.QueryRow(`SELECT user_id,student_name,contact_email,contact_phone,tariff_name,first_clicked_at::text FROM course_interests WHERE course_id=$1`, f.Course).Scan(&user, &name, &email, &phone, &tariffName, &at); err != nil {
		t.Fatal(err)
	}
	f.DB.QueryRow(`SELECT full_name FROM users WHERE id=$1`, f.Student).Scan(&accountName)
	if user != f.Student || name != accountName || name == "FORGED" || email != "contact@example.invalid" || phone != "+79991234567" || tariffName != "Contact test tariff" {
		t.Fatal("contact snapshot or identity mismatch")
	}
	body["contact_phone"] = "+7 999 765 43 21"
	request("POST", "/api/courses/interest", f.Student, false, body, 204)
	var count int
	f.DB.QueryRow(`SELECT count(*) FROM course_interests WHERE course_id=$1`, f.Course).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate application")
	}
	var repeatedAt string
	f.DB.QueryRow(`SELECT first_clicked_at::text FROM course_interests WHERE course_id=$1`, f.Course).Scan(&repeatedAt)
	if repeatedAt != at {
		t.Fatal("original application date changed")
	}
	exec(`UPDATE users SET full_name='Changed account name' WHERE id=$1`, f.Student)
	exec(`UPDATE course_tariffs SET title='Changed tariff' WHERE id=$1`, tariff)
	exec(courseInterestContactsSchema)
	exec(courseInterestContactsSchema)
	request("GET", "/api/admin/courses/students", f.Student, false, nil, 401)
	request("POST", "/api/admin/courses/students", f.Owner, true, nil, 405)
	var result struct {
		Total int
		Items []struct {
			Course  string `json:"course_title"`
			Tariff  string `json:"tariff_name"`
			Student string `json:"student_name"`
			Email   string `json:"contact_email"`
			Phone   string `json:"contact_phone"`
		}
	}
	if err := json.Unmarshal(request("GET", "/api/admin/courses/students", f.Owner, true, nil, 200), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].Student != name || result.Items[0].Tariff != tariffName || result.Items[0].Phone != "+79997654321" {
		t.Fatal("admin snapshots missing or overwritten")
	}
	var enrollment, outbox int64
	if err := f.DB.QueryRow(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key) VALUES($1,$2,$3,$4,'contacts-test') RETURNING id`, f.Course, f.Student, f.Version, f.Owner).Scan(&enrollment); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.QueryRow(`INSERT INTO notification_outbox(kind,recipient_email,status) VALUES('test_contacts','contact@example.invalid','failed') RETURNING id`).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE course_interests SET paid_at=now(),paid_commission_bps=2000,paid_enrollment_id=$2,invitation_outbox_id=$3 WHERE course_id=$1`, f.Course, enrollment, outbox)
	request("POST", "/api/courses/interest", f.Student, false, body, 409)
	exec(`INSERT INTO course_interests(course_id,user_id) VALUES($1,$2)`, f.Course, f.Owner)
	var legacy struct {
		Total int
		Items []map[string]any
	}
	json.Unmarshal(request("GET", "/api/admin/courses/students", f.Owner, true, nil, 200), &legacy)
	if legacy.Total != 2 {
		t.Fatal("legacy applications omitted")
	}
	request("GET", "/api/admin/courses/students?page=0", f.Owner, true, nil, 400)
	request("GET", fmt.Sprintf("/api/admin/courses/students?page=%d", 2), f.Owner, true, nil, 200)
}
