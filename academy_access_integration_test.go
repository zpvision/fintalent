package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/courses"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestAcademyCoursesGateIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	user := func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }
	c := courses.New(f.DB, user, func(*http.Request) bool { return false })
	mux := http.NewServeMux()
	academy.New(f.DB, user, c.Gate, f.Root).Register(mux)
	var enrollment int64
	if err := f.DB.QueryRow(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key) VALUES($1,$2,$3,$4,$5) RETURNING id`, f.Course, f.Student, f.Version, f.Owner, academy.Key()).Scan(&enrollment); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(`UPDATE course_access_settings SET public_access=false`); err != nil {
		t.Fatal(err)
	}
	request := func(path string, uid int64, status int) {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("X-Test-User", strconv.FormatInt(uid, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s user %d got %d want %d: %s", path, uid, w.Code, status, w.Body.String())
		}
	}
	room := fmt.Sprintf("/api/academy/enrollments/%d", enrollment)
	request(room, f.Student, 404)
	request(fmt.Sprintf("/api/academy/courses/%d", f.Course), f.Owner, 404)
	request("/api/academy/workspaces", f.Teacher, 404)
	if _, err := f.DB.Exec(`UPDATE users SET courses_access=true WHERE id IN($1,$2,$3)`, f.Student, f.Owner, f.Teacher); err != nil {
		t.Fatal(err)
	}
	request(room, f.Student, 200)
	request(room, f.Teacher, 404) // Section permission never grants another learner's enrollment.
	request(fmt.Sprintf("/api/academy/courses/%d", f.Course), f.Teacher, 404)
	if _, err := f.DB.Exec(`UPDATE users SET courses_access=false WHERE id=$1`, f.Student); err != nil {
		t.Fatal(err)
	}
	request(room, f.Student, 404)
	if _, err := f.DB.Exec(`UPDATE course_access_settings SET public_access=true`); err != nil {
		t.Fatal(err)
	}
	request(room, f.Student, 200) // Preserve the existing global launch switch.
}
