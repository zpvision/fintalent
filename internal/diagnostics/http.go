package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var tokenPattern = regexp.MustCompile(`[^a-zA-Z0-9_.:-]`)
var framePattern = regexp.MustCompile(`/(?:static|src)/[a-zA-Z0-9_./-]+\.(?:js|jsx|mjs|tsx|ts):[0-9]+(?::[0-9]+)?`)
var goFrame = regexp.MustCompile(`[a-zA-Z0-9_./-]+\.go:[0-9]+`)
var routeWords = map[string]bool{}

func init() {
	for _, v := range strings.Fields("api v1 public admin academy courses profile users session login logout me assets uploads parts complete learning enrollments versions draft program actions publication publish revisions reports jobs tests take results resume resumes vacancies client-exchange accounting-companies profiles help contact-threads notifications settings password avatar files static react src health ready live diagnostics errors events report reset register feedback challenge dictionaries items teaching workspaces communications cohorts students assignments submissions lessons modules blocks download progress current status invitations employee-test analytics filters surveys marketplace profimarket companies messages home saved") {
		routeWords[v] = true
	}
}

func Token(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	return tokenPattern.ReplaceAllString(s, "")
}

// Drop query strings and arbitrary path values (emails, invite tokens, slugs).
func SafePath(path string) string {
	path = strings.SplitN(strings.SplitN(path, "?", 2)[0], "#", 2)[0]
	if len(path) > 1000 {
		return "/:redacted"
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p != "" && !routeWords[p] && !(i == 3 && path == "/api/courses/access") {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

var publicAssetPath = regexp.MustCompile(`^/[a-zA-Z0-9_./-]+\.(?:js|mjs|css|ico|woff|woff2|ttf|otf)$`)

// Keep public asset locations only, never credentials, queries or private uploads.
// Unknown external hosts may contain tenant identifiers and are not retained.
func SafeResourceURL(raw string) string {
	if raw == "" {
		return ""
	}
	if raw == "/external/:redacted" {
		return raw
	}
	if len(raw) > 2048 {
		return "/:redacted"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http" {
		return "/:redacted"
	}
	if u.Host != "" {
		host := strings.ToLower(u.Hostname())
		scheme := u.Scheme
		if scheme == "" {
			scheme = "https"
		}
		origin := scheme + "://" + host
		if u.Port() != "" {
			origin += ":" + u.Port()
		}
		switch host {
		case "mc.yandex.ru", "mc.yandex.com":
			if u.Path == "/metrika/tag.js" {
				return origin + u.Path
			}
		case "fonts.googleapis.com":
			if u.Path == "/css" || u.Path == "/css2" {
				return origin + u.Path
			}
		case "fonts.gstatic.com", "cdn.jsdelivr.net", "unpkg.com":
			if publicAssetPath.MatchString(u.Path) && !strings.Contains(u.Path, "..") {
				return origin + u.Path
			}
		default:
			return "/external/:redacted"
		}
		return origin + "/:redacted"
	}
	if (strings.HasPrefix(u.Path, "/static/") && !strings.HasPrefix(u.Path, "/static/uploads/") || u.Path == "/favicon.ico") && publicAssetPath.MatchString(u.Path) && !strings.Contains(u.Path, "..") {
		return u.Path
	}
	return "/:redacted"
}

func SafeResourceType(value string) string {
	switch value {
	case "script", "stylesheet", "icon", "preload", "modulepreload", "preconnect", "dns-prefetch", "link":
		return value
	}
	return ""
}

// Keep locations only. Exception text, SQL and argument values are not stored.
func SafeStack(raw string) string {
	if len(raw) > 12000 {
		raw = raw[:12000]
	}
	frames := framePattern.FindAllString(raw, 12)
	for _, f := range goFrame.FindAllString(raw, 24) {
		parts := strings.Split(f, "/")
		frames = append(frames, parts[len(parts)-1])
	}
	return strings.Join(frames, "\n")
}

func Cause(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		return "postgres_" + Token(state.SQLState(), 5)
	}
	return Token(reflect.TypeOf(err).String(), 100)
}

type requestKey struct{}
type requestState struct{ user atomic.Int64 }

func SetUser(ctx context.Context, id int64) {
	if s, ok := ctx.Value(requestKey{}).(*requestState); ok {
		s.user.Store(id)
	}
}

type response struct {
	http.ResponseWriter
	ctx       context.Context
	status    int
	cause     string
	stack     string
	operation string
}

func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *response) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status == 0 {
		if status >= 500 {
			w.stack = SafeStack(string(debug.Stack()))
		}
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *response) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *response) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *response) DiagnosticError(err error) {
	// Only abandon client-cancelled requests. Internal cancellations, timeouts
	// and unrelated database failures must still be reported.
	if errors.Is(err, context.Canceled) && errors.Is(w.ctx.Err(), context.Canceled) {
		panic(http.ErrAbortHandler)
	}
	w.cause = Cause(err)
}
func (w *response) DiagnosticOperation(operation string) { w.operation = Token(operation, 100) }
func HTTPError(w http.ResponseWriter, err error, operation ...string) {
	for {
		if a, ok := w.(interface{ DiagnosticError(error) }); ok {
			a.DiagnosticError(err)
			if len(operation) > 0 {
				if target, ok := w.(interface{ DiagnosticOperation(string) }); ok {
					target.DiagnosticOperation(operation[0])
				}
			}
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate locally: never trust client supplied request IDs.
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			http.Error(w, "Service unavailable", 503)
			return
		}
		id := hex.EncodeToString(b[:])
		w.Header().Set("X-Request-ID", id)
		s := &requestState{}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, s))
		out := &response{ResponseWriter: w, ctx: r.Context(), operation: "http"}
		start := time.Now()
		defer func() {
			code, stack := "http_5xx", out.stack
			abort := false
			if recovered := recover(); recovered != nil {
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}
				code = "panic"
				stack = string(debug.Stack())
				abort = out.status != 0
				if out.status == 0 {
					out.Header().Set("Content-Type", "application/json; charset=utf-8")
					out.WriteHeader(500)
					fmt.Fprintf(out, `{"error":"Не удалось выполнить действие","request_id":%q}`, id)
				}
			}
			if code == "panic" || out.status >= 500 {
				Emit(Event{Source: "backend", Code: code, Operation: out.operation, RequestID: id, UserID: s.user.Load(), Route: r.URL.Path, Method: r.Method, Status: out.status, DurationMS: time.Since(start).Milliseconds(), Cause: out.cause, Stack: stack})
			}
			if abort {
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(out, r)
	})
}

func ParseHours(value string) (time.Duration, error) {
	if value == "" {
		return 24 * time.Hour, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 24*30 {
		return 0, errors.New("hours out of range")
	}
	return time.Duration(n) * time.Hour, nil
}

// ParsePeriod accepts explicit timestamps only with a timezone (RFC3339).
func ParsePeriod(query url.Values, now time.Time) (time.Time, time.Time, error) {
	period, err := ParseHours(query.Get("hours"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	until := now
	if value := query.Get("until"); value != "" {
		until, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	since := until.Add(-period)
	if value := query.Get("since"); value != "" {
		since, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !since.Before(until) || until.Sub(since) > 30*24*time.Hour || until.After(now) {
		return time.Time{}, time.Time{}, errors.New("invalid report interval")
	}
	return since.UTC(), until.UTC(), nil
}
