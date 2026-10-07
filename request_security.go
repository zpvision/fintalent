package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func trustedProxy(ip net.IP) bool {
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "unknown"
	}
	if !trustedProxy(ip) {
		return ip.String()
	}
	// Walk from the socket peer towards the client; never trust the leftmost
	// client-supplied value when an untrusted hop appears earlier in the chain.
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		next := net.ParseIP(strings.TrimSpace(parts[i]))
		if next == nil {
			return ip.String()
		}
		ip = next
		if !trustedProxy(ip) {
			break
		}
	}
	return ip.String()
}

type rateEntry struct {
	until time.Time
	count int
}
type requestLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

func (l *requestLimiter) allow(key string, max int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]rateEntry)
	}
	entry := l.entries[key]
	if !now.Before(entry.until) {
		if len(l.entries) >= 10000 {
			for k, v := range l.entries {
				if !now.Before(v.until) {
					delete(l.entries, k)
				}
			}
			if len(l.entries) >= 10000 {
				return false
			}
		}
		entry = rateEntry{until: now.Add(time.Minute)}
	}
	if entry.count >= max {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func requestSecurity(next http.Handler) http.Handler {
	csrf := http.NewCrossOriginProtection()
	limiter := &requestLimiter{}
	// Bound expensive password hashing and authenticated mutations, including
	// requests from many distinct addresses. Excess requests retry, never queue.
	expensive := make(chan struct{}, 16)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if unsafe {
			if err := csrf.Check(r); err != nil {
				writeJSON(w, 403, "Недопустимый источник запроса")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				expected := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
				if expected == "" {
					scheme := "http"
					if r.TLS != nil || secureCookies() {
						scheme = "https"
					}
					expected = scheme + "://" + r.Host
				}
				if origin != expected {
					writeJSON(w, 403, "Недопустимый источник запроса")
					return
				}
			}
		}
		if unsafe && strings.HasPrefix(r.URL.Path, "/api/") {
			limit, bucket := 120, "write:"
			if r.URL.Path == "/api/login" || r.URL.Path == "/api/register" || r.URL.Path == "/api/admin/login" || strings.HasPrefix(r.URL.Path, "/api/password-reset/") {
				limit, bucket = 10, "auth:"
				select {
				case expensive <- struct{}{}:
					defer func() { <-expensive }()
				default:
					w.Header().Set("Retry-After", "5")
					writeJSON(w, 429, "Повторите запрос позже")
					return
				}
			}
			if !limiter.allow(bucket+requestClientIP(r), limit, time.Now()) {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, 429, "Слишком много запросов. Повторите позже")
				return
			}
			if r.ContentLength > 8<<20 {
				writeJSON(w, 413, "Запрос слишком большой")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func validateProductionConfig() error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return nil
	}
	for _, key := range []string{"ADMIN_LOGIN", "ADMIN_PASSWORD", "PASSWORD_RESET_SECRET"} {
		value := strings.TrimSpace(os.Getenv(key))
		lower := strings.ToLower(value)
		placeholder := strings.Contains(lower, "change-me") || strings.Contains(lower, "replace-with") || strings.Contains(lower, "changeme") || strings.Contains(lower, "placeholder") || lower == "use-a-long-random-password" || lower == "your-admin-login"
		if value == "" || placeholder || (key != "ADMIN_LOGIN" && (value == "admin" || len(value) < 16)) {
			return errors.New("небезопасная или отсутствующая настройка " + key)
		}
	}
	base, err := url.Parse(os.Getenv("APP_BASE_URL"))
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return errors.New("APP_BASE_URL должен содержать HTTPS origin")
	}
	if !secureCookies() {
		return errors.New("в production требуется COOKIE_SECURE=true")
	}
	for _, key := range []string{"SEED_DEMO_DATA", "SYNC_GEOGRAPHY"} {
		if strings.EqualFold(strings.TrimSpace(os.Getenv(key)), "true") {
			return errors.New("в production требуется " + key + "=false")
		}
	}
	if !reactFrontendDisabled() {
		if _, err := os.Stat("static/react/index.html"); err != nil {
			return errors.New("не найдена React-сборка static/react/index.html")
		}
	}
	if len(os.Getenv("PASSWORD_RESET_SECRET")) < 32 {
		return errors.New("PASSWORD_RESET_SECRET должен содержать не менее 32 символов")
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(raw) != "" {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(raw)); err != nil {
				return errors.New("некорректный TRUSTED_PROXY_CIDRS")
			}
		}
	}
	return nil
}
