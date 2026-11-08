package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// React uses the real Academy HTTP handlers and an isolated PostgreSQL schema.
// Only the auth boundary is a fixture; no credentials or production are used.
func TestAcademyProgramRevisionBrowser(t *testing.T) {
	if os.Getenv("RUN_ACADEMY_REVISIONS_BROWSER") != "1" {
		t.Skip("opt in to headless browser integration")
	}
	f := newProgramRevisionFixture(t)
	enrollment := int64(0)
	if err := f.DB.QueryRow(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,enrolled_by,request_key) VALUES($1,$2,$3,$4,'browser-learner') RETURNING id`, f.Course, f.Student, f.Version, f.Owner).Scan(&enrollment); err != nil {
		t.Fatal(err)
	}
	static := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/me" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": f.Owner, "full_name": "Synthetic Academy author"})
			return
		}
		if r.URL.Path == "/api/courses/access" || r.URL.Path == "/api/academy/learning/access" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"allowed":true}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/academy/") {
			actor := f.Owner
			if strings.HasPrefix(r.URL.Path, "/api/academy/enrollments/") {
				actor = f.Student
			}
			r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
			f.Mux.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[]`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			static.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "static/react/index.html")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "scripts/academy-revisions-browser-check.cjs", server.URL, strconv.FormatInt(f.Course, 10), strconv.FormatInt(f.Version, 10), strconv.FormatInt(enrollment, 10))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser check: %v\n%s", err, out)
	}
	t.Log(string(out))
	var count int
	if err = f.DB.QueryRow(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version).Scan(&count); err != nil || count < 1 {
		t.Fatal("UI did not publish a revision", err)
	}
	var title string
	if err = f.DB.QueryRow(`SELECT content->'modules'->0->'lessons'->0->>'title' FROM academy_current_program_versions WHERE id=$1`, f.Version).Scan(&title); err != nil || title != "Browser published lesson" {
		t.Fatal(fmt.Sprintf("UI data not persisted: %s", title), err)
	}
}
