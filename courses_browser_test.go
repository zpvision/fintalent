package main

import (
	"FinTalent/internal/courses"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// Optional browser harness; this exists only in the test binary. The caller has
// already verified an isolated PostgreSQL schema. No workers/mail are started.
func serveCourseBrowserFixture(t *testing.T, author, course int64) {
	t.Helper()
	ready := os.Getenv("COURSES_BROWSER_READY_FILE")
	if ready == "" {
		return
	}
	mux := http.NewServeMux()
	courses.New(db, func(r *http.Request) (int64, error) {
		u, e := userFromRequest(r)
		if e != nil {
			return 0, e
		}
		return u.ID, nil
	}, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "1" }).Register(mux)
	mux.HandleFunc("/api/courses/upload", profiMarketUploadAPI)
	mux.HandleFunc("/api/me", currentUser)
	mux.HandleFunc("/__test/login", func(w http.ResponseWriter, r *http.Request) {
		if e := createSession(w, author); e != nil {
			t.Error(e)
			http.Error(w, "session", 500)
			return
		}
		http.Redirect(w, r, "/profile/courses", 302)
	})
	// Unrelated profile widgets are outside this module's browser harness.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, reactFrontendIndex) })
	server := httptest.NewServer(mux)
	defer server.Close()
	t.Setenv("APP_ENV", "development")
	t.Setenv("COOKIE_SECURE", "false")
	info, _ := json.Marshal(map[string]any{"url": server.URL, "course_id": course})
	if e := os.WriteFile(ready, info, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("browser fixture ready on loopback; waiting for completion file")
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		if _, e := os.Stat(ready + ".done"); e == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Error("browser fixture timed out")
}
