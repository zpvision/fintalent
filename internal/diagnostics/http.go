package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
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
		if p != "" && !routeWords[p] {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
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
	status int
	cause  string
	stack  string
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
func (w *response) DiagnosticError(err error) { w.cause = Cause(err) }
func HTTPError(w http.ResponseWriter, err error) {
	for {
		if a, ok := w.(interface{ DiagnosticError(error) }); ok {
			a.DiagnosticError(err)
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
		out := &response{ResponseWriter: w}
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
				Emit(Event{Source: "backend", Code: code, Operation: "http", RequestID: id, UserID: s.user.Load(), Route: r.URL.Path, Method: r.Method, Status: out.status, DurationMS: time.Since(start).Milliseconds(), Cause: out.cause, Stack: stack})
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
