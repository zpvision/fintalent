package diagnostics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T, files int) *Store {
	t.Helper()
	s, err := Open(Config{Dir: t.TempDir(), FileBytes: MaxEventBytes, Files: files, Retention: 24 * time.Hour, Release: "test-release"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestJournalHardSizeCapAndRestart(t *testing.T) {
	s := testStore(t, 3)
	for i := 0; i < 300; i++ {
		s.Record(Event{Source: "worker", Code: "failed", Stack: strings.Repeat("/static/react/assets/test.js:42:8\n", 12)})
	}
	entries, err := os.ReadDir(s.cfg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	if len(entries) != 3 {
		t.Fatalf("rotation did not exercise all slots: %d", len(entries))
	}
	for _, e := range entries {
		info, _ := e.Info()
		if info.Size() > s.cfg.FileBytes {
			t.Fatal("oversized journal segment")
		}
		total += info.Size()
	}
	if total > s.cfg.FileBytes*int64(s.cfg.Files) {
		t.Fatal("hard cap exceeded")
	}
	s.Close()
	reopened, err := Open(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	report, err := reopened.Report(time.Now().Add(-time.Hour))
	if err != nil || report.Events == 0 {
		t.Fatalf("restart lost retained events: %+v %v", report, err)
	}
}

func TestJournalRetentionAndOwnedFilesOnly(t *testing.T) {
	s := testStore(t, 3)
	s.Record(Event{Source: "backend", Code: "old"})
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(s.path(0), old, old); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(s.cfg.Dir, "keep.jsonl")
	if err := os.WriteFile(other, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path(0)); !os.IsNotExist(err) {
		t.Fatal("expired segment retained")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("cleanup removed unrelated file")
	}
	// Check the same behavior on startup, without requiring a new event.
	if err := os.WriteFile(s.path(2), []byte("expired"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(s.path(2), old, old); err != nil {
		t.Fatal(err)
	}
	s.Close()
	next, err := Open(s.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if _, err := os.Stat(s.path(2)); !os.IsNotExist(err) {
		t.Fatal("startup retained expired segment")
	}
}

func TestJournalFloodLimitAndWriteFailure(t *testing.T) {
	s := testStore(t, 1)
	for i := 0; i < 1000; i++ {
		s.Record(Event{Source: "backend", Code: "storm"})
	}
	if s.dropped.Load() != 700 {
		t.Fatalf("rate limit did not drop excess events: %d", s.dropped.Load())
	}
	s.Close()
	broken := testStore(t, 1)
	if err := os.Remove(broken.path(0)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(broken.path(0), 0700); err != nil {
		t.Fatal(err)
	}
	broken.Record(Event{Code: "failed"})
	broken.Record(Event{Code: "failed"})
	if broken.writeErrors.Load() != 1 || broken.dropped.Load() != 1 {
		t.Fatal("write failures should open a one-minute backoff")
	}
}

func TestReportGroupingPrivacyAndPeriod(t *testing.T) {
	s := testStore(t, 3)
	for i, path := range []string{"/api/academy/courses/10/assets?token=private-secret", "/api/academy/courses/11/assets?email=private@example.com"} {
		s.Record(Event{Source: "backend", Code: "http_5xx", UserID: int64(i + 1), Route: path, Stack: "Error: private-password\n at f (https://example.com/static/react/assets/index.js:42:2)"})
	}
	r, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 2 || len(r.Groups) != 1 || r.Groups[0].Count != 2 {
		t.Fatalf("expected one grouped issue: %+v", r)
	}
	if r.Groups[0].AffectedUsers != 2 {
		t.Fatal("affected user count missing")
	}
	if r.Groups[0].Sample.Route != "/api/academy/courses/:id/assets" || strings.Contains(r.Groups[0].Sample.Stack, "private") {
		t.Fatal("private data retained")
	}
	r, err = s.Report(time.Now().Add(time.Hour))
	if err != nil || r.Events != 0 {
		t.Fatal("period filter ignored")
	}
	if got := SafePath("/api/password-reset/private-token?secret=abc"); strings.Contains(got, "private") || strings.Contains(got, "abc") {
		t.Fatal(got)
	}
}

func TestHTTPFailuresAndPanicAreLinkedWithoutSecrets(t *testing.T) {
	s := testStore(t, 3)
	SetDefault(s)
	defer SetDefault(nil)
	for _, mode := range []string{"500", "panic", "validation"} {
		h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SetUser(r.Context(), 7)
			switch mode {
			case "panic":
				panic([]byte("secret panic data"))
			case "500":
				HTTPError(w, errors.New("password=secret"))
				w.WriteHeader(503)
			default:
				w.WriteHeader(400)
			}
		}))
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/academy/courses/10/assets?token=secret", nil)
		r.Header.Set("X-Request-ID", "forged")
		h.ServeHTTP(w, r)
		if id := w.Header().Get("X-Request-ID"); len(id) != 32 || id == "forged" {
			t.Fatal("request ID not generated locally")
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("panic leaked to client")
		}
	}
	s.Close() // Flush the bounded async queue before checking disk.
	r, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if r.Events != 2 {
		t.Fatalf("expected only the two failures: %+v", r)
	}
	for _, g := range r.Groups {
		if g.Sample.UserID != 7 || g.Sample.RequestID == "" || strings.Contains(g.Sample.Cause, "secret") || g.Sample.Stack == "" {
			t.Fatalf("missing context or leaked error: %+v", g)
		}
	}
}

func TestHTTPPanicAfterWriteAbortsResponse(t *testing.T) {
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("partial response was not aborted")
		}
	}()
	Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("partial")); panic("hidden") })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}
