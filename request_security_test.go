package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestOriginProtection(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://fintalent.ru")
	h := requestSecurity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tt := range []struct {
		origin, site string
		status       int
	}{
		{"https://evil.test", "cross-site", 403}, {"https://evil.fintalent.ru", "same-site", 403}, {"http://fintalent.ru", "", 403}, {"https://fintalent.ru", "same-origin", 204}, {"", "", 204},
	} {
		r := httptest.NewRequest("POST", "https://fintalent.ru/api/login", nil)
		r.Header.Set("Origin", tt.origin)
		r.Header.Set("Sec-Fetch-Site", tt.site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.status {
			t.Errorf("origin=%s site=%s: %d", tt.origin, tt.site, w.Code)
		}
	}
}

func TestRequestClientIPDoesNotTrustClientHeaders(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "198.51.100.10:123"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	if got := requestClientIP(r); got != "198.51.100.10" {
		t.Fatal(got)
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	r.RemoteAddr = "127.0.0.1:123"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 198.51.100.10")
	if got := requestClientIP(r); got != "198.51.100.10" {
		t.Fatal(got)
	}
}

func TestLimiterConcurrentAndExpiry(t *testing.T) {
	l := &requestLimiter{}
	now := time.Now()
	var n atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.allow("client", 10, now) {
				n.Add(1)
			}
		}()
	}
	wg.Wait()
	if n.Load() != 10 {
		t.Fatal(n.Load())
	}
	if !l.allow("client", 10, now.Add(time.Minute)) {
		t.Fatal("window did not expire")
	}
	if !l.allow("other", 10, now) {
		t.Fatal("other client blocked")
	}
}

func TestSVGStaticSubset(t *testing.T) {
	for _, s := range []string{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0" stroke="currentColor"/></svg>`, `<svg><defs><linearGradient id="a"><stop offset="0" stop-color="#fff"/></linearGradient></defs><path fill="url(#a)"/></svg>`} {
		if !safeSVG([]byte(s)) {
			t.Errorf("rejected safe svg %s", s)
		}
	}
	for _, s := range []string{`<svg onload = "alert(1)"></svg>`, `<svg><path onmouseover="alert(1)"/></svg>`, `<svg><foreignObject/></svg>`, `<svg><script/></svg>`, `<svg><use href="https://evil.test/x"/></svg>`, `<svg><path style="fill:red"/></svg>`, `<!DOCTYPE svg><svg/>`, `<svg/><svg/>`} {
		if safeSVG([]byte(s)) {
			t.Errorf("accepted unsafe svg %s", s)
		}
	}
}

func TestUploadedSVGDirectNavigationSandbox(t *testing.T) {
	w := httptest.NewRecorder()
	securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, httptest.NewRequest("GET", "/static/uploads/old.SVG", nil))
	if w.Header().Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'; sandbox" {
		t.Fatal("historical SVG is not sandboxed")
	}
}

func TestInvitationTokenResponsePrivacy(t *testing.T) {
	for _, path := range []string{"/employee-test?token=fixture", "/api/employee-test/fixture"} {
		w := httptest.NewRecorder()
		securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("invitation response can propagate/cache token: %s", path)
		}
	}
}

func TestProductionConfigurationRejectsUnsafeExamples(t *testing.T) {
	for key, value := range map[string]string{"APP_ENV": "production", "ADMIN_LOGIN": "operator", "ADMIN_PASSWORD": "unit-test-only-password", "PASSWORD_RESET_SECRET": strings.Repeat("x", 32), "APP_BASE_URL": "https://fintalent.ru", "COOKIE_SECURE": "true", "REACT_FRONTEND": "false", "SEED_DEMO_DATA": "false", "SYNC_GEOGRAPHY": "false", "TRUSTED_PROXY_CIDRS": "127.0.0.1/32"} {
		t.Setenv(key, value)
	}
	if err := validateProductionConfig(); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ key, value string }{{"PASSWORD_RESET_SECRET", "replace-with-at-least-32-random-characters"}, {"ADMIN_PASSWORD", "use-a-long-random-password"}, {"COOKIE_SECURE", "false"}, {"APP_BASE_URL", "http://fintalent.ru"}, {"SEED_DEMO_DATA", "true"}, {"SYNC_GEOGRAPHY", "true"}, {"TRUSTED_PROXY_CIDRS", "not-a-network"}} {
		t.Run(fixture.key, func(t *testing.T) {
			t.Setenv(fixture.key, fixture.value)
			if validateProductionConfig() == nil {
				t.Fatal("unsafe production configuration accepted")
			}
		})
	}
}
