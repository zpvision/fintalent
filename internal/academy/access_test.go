package academy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoursesGateCoversAcademyEntrypoints(t *testing.T) {
	userCalled := false
	gate := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { http.Error(w, "closed", 404) }
	}
	h := New(nil, func(*http.Request) (int64, error) { userCalled = true; return 1, nil }, gate, t.TempDir())
	mux := http.NewServeMux()
	h.Register(mux)
	for _, path := range []string{"/api/academy/courses/1", "/api/academy/courses/1/cohorts", "/api/academy/courses/1/communications", "/api/academy/learning", "/api/academy/learning/access", "/api/academy/enrollments/1", "/api/academy/enrollments/1/assets/1?lesson=lesson", "/api/academy/enrollments/1/lessons/lesson/assessments/test/start", "/api/academy/notifications", "/api/academy/workspaces", "/api/academy/communication-preferences"} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != 404 {
				t.Errorf("%s %s = %d", method, path, w.Code)
			}
		}
	}
	for _, path := range []string{"/profile/academy", "/profile/learning/1", "/profile/courses/1/academy/cohorts/1"} {
		w := httptest.NewRecorder()
		h.CoursePage(func(w http.ResponseWriter, r *http.Request) { t.Error("page exposed") })(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("page gate bypassed")
		}
	}
	if userCalled {
		t.Fatal("domain handlers ran before platform gate")
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/academy/session", nil))
	if w.Code != 401 {
		t.Fatal("admin resolver was replaced by Courses gate")
	}
	// Unsubscribe remains usable without account or beta access.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/academy/unsubscribe/invalid", nil))
	if w.Code != 404 || w.Body.String() == "closed\n" {
		t.Fatal("unsubscribe was gated")
	}
}
