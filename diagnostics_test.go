package main

import (
	"FinTalent/internal/diagnostics"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFrontendDiagnosticValidationAndOrigin(t *testing.T) {
	s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	old := errorJournal
	errorJournal = s
	t.Cleanup(func() { errorJournal = old; s.Close() })
	for _, tc := range []struct {
		body, origin string
		status       int
	}{
		{`{"code":"javascript_error","route":"/profile?token=secret","stack":"Error: secret-password\n/static/react/assets/index.js:12:3","release":"test"}`, "", 204},
		{`{"code":"javascript_error","user_id":42}`, "", 400},
		{`{"code":"arbitrary-private-message"}`, "", 400},
		{`{"code":"network_error","duration_ms":-1}`, "", 400},
		{`{"code":"network_error","visibility":"private text"}`, "", 400},
		{`{"code":"network_error","online":"yes"}`, "", 400},
		{`{"code":"javascript_error"}`, "https://evil.invalid", 403},
		{strings.Repeat("x", 20000), "", 400},
	} {
		r := httptest.NewRequest("POST", "/api/diagnostics/events", strings.NewReader(tc.body))
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		requestSecurity(http.HandlerFunc(frontendErrorEvent)).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
		}
	}
	s.Close()
	report, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.Events != 1 || report.Groups[0].Sample.UserID != 0 {
		t.Fatalf("unexpected accepted events: %+v", report)
	}
	if strings.Contains(report.Groups[0].Sample.Stack, "secret") || strings.Contains(report.Groups[0].Sample.Route, "secret") {
		t.Fatal("sensitive data retained")
	}
	if report.MaxBytes != 50<<20 || report.RetentionDays != 14 {
		t.Fatal("unexpected production storage limits")
	}
}

func TestFrontendNetworkDiagnosticContext(t *testing.T) {
	s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir(), Release: "server-release"})
	if err != nil {
		t.Fatal(err)
	}
	old := errorJournal
	errorJournal = s
	t.Cleanup(func() { errorJournal = old; s.Close() })
	w := httptest.NewRecorder()
	frontendErrorEvent(w, httptest.NewRequest("POST", "/api/diagnostics/events", strings.NewReader(`{"code":"network_error","route":"/api/courses/access","duration_ms":2345,"online":false,"visibility":"hidden","release":"older-client"}`)))
	if w.Code != 204 {
		t.Fatalf("rejected context: %d", w.Code)
	}
	s.Close()
	r, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil || len(r.Groups) != 1 {
		t.Fatalf("missing event: %+v %v", r, err)
	}
	e := r.Groups[0].Sample
	if e.DurationMS != 2345 || e.Online == nil || *e.Online || e.Visibility != "hidden" || e.ClientRelease != "older-client" || e.Release != "server-release" {
		t.Fatalf("context lost: %+v", e)
	}
}

func TestDiagnosticEndpointRateLimit(t *testing.T) {
	h := requestSecurity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for i := 0; i < 11; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/api/diagnostics/events", nil))
		want := 204
		if i == 10 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("event %d: %d", i, w.Code)
		}
	}
}

func TestDiagnosticReportRequiresAdmin(t *testing.T) {
	w := httptest.NewRecorder()
	adminErrorReport(w, httptest.NewRequest("GET", "/api/admin/diagnostics?download=1", nil))
	if w.Code != 401 {
		t.Fatal("report accessible without admin session")
	}
}

func TestCollectorInjectionAndPrivacy(t *testing.T) {
	t.Setenv("APP_RELEASE", `release-123`)
	for _, html := range []string{`<html><head></head></html>`, `<html><head><script src="/static/error-telemetry.js?v=1"></script></head></html>`} {
		out := injectErrorCollector([]byte(html))
		if bytes.Count(out, []byte("/static/error-telemetry.js")) != 1 || !bytes.Contains(out, []byte(`content="release-123"`)) {
			t.Fatal(string(out))
		}
	}
}

func TestFrontendResourceDiagnosticRoundTrip(t *testing.T) {
	s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	old := errorJournal
	errorJournal = s
	t.Cleanup(func() { errorJournal = old; s.Close() })
	for _, body := range []string{
		`{"code":"resource_error","route":"/profile","page_route":"/profile?token=PRIVATE","resource_url":"https://fonts.googleapis.com/css2?family=PRIVATE","resource_type":"stylesheet"}`,
		`{"code":"resource_error","route":"/profile","page_route":"/profile?token=PRIVATE","resource_url":"https://mc.yandex.ru/metrika/tag.js?id=PRIVATE#PRIVATE","resource_type":"script"}`,
	} {
		w := httptest.NewRecorder()
		frontendErrorEvent(w, httptest.NewRequest("POST", "/api/diagnostics/events", strings.NewReader(body)))
		if w.Code != 204 {
			t.Fatalf("resource event rejected: %d %s", w.Code, w.Body.String())
		}
	}
	s.Close()
	r, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil || r.Events != 2 || len(r.Groups) != 2 {
		t.Fatalf("missing resource event: %+v %v", r, err)
	}
	wantTypes := map[string]string{"https://fonts.googleapis.com/css2": "stylesheet", "https://mc.yandex.ru/metrika/tag.js": "script"}
	for _, group := range r.Groups {
		e := group.Sample
		if e.PageRoute != "/profile" || wantTypes[e.ResourceURL] == "" || e.ResourceType != wantTypes[e.ResourceURL] || e.Status != 0 {
			t.Fatalf("incorrect resource context: %+v", e)
		}
	}
}
