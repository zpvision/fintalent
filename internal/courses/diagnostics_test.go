package courses

import (
	"FinTalent/internal/diagnostics"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCourseDatabaseCancellationHandling(t *testing.T) {
	for _, tc := range []struct {
		name, contextState string
		err                error
		abort              bool
		cause              string
	}{
		{"client cancellation", "cancelled", context.Canceled, true, ""},
		{"wrapped client cancellation", "cancelled", fmt.Errorf("query: %w", context.Canceled), true, ""},
		{"internal cancellation", "active", context.Canceled, false, "cancelled"},
		{"request deadline", "deadline", context.DeadlineExceeded, false, "deadline_exceeded"},
		{"database timeout", "active", fmt.Errorf("query: %w", context.DeadlineExceeded), false, "deadline_exceeded"},
		{"database failure despite cancellation", "cancelled", &pgconn.PgError{Code: "08006"}, false, "postgres_08006"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			diagnostics.SetDefault(s)
			defer diagnostics.SetDefault(nil)
			ctx := context.Background()
			switch tc.contextState {
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
			}
			w := httptest.NewRecorder()
			w.Code = 0 // Detect even an attempted header write to the abandoned response.
			cleanedUp := false
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				diagnostics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer func() { cleanedUp = true }()
					dbError(w, r, tc.err, "courses.access")
				})).ServeHTTP(w, httptest.NewRequest("GET", "/api/courses/access", nil).WithContext(ctx))
			}()
			if !cleanedUp {
				t.Fatal("deferred cleanup did not run")
			}
			s.Close()
			report, err := s.Report(time.Now().Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if tc.abort {
				if recovered != http.ErrAbortHandler || w.Code != 0 || w.Body.Len() != 0 || report.Events != 0 {
					t.Fatalf("cancelled request produced a response or error event: panic=%v status=%d report=%+v", recovered, w.Code, report)
				}
			} else if recovered != nil || w.Code != 500 || report.Events != 1 || report.Groups[0].Sample.Cause != tc.cause {
				t.Fatalf("real failure lost: panic=%v status=%d report=%+v", recovered, w.Code, report)
			}
		})
	}
}

func TestCourseDatabaseFailureDiagnosticContext(t *testing.T) {
	s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	diagnostics.SetDefault(s)
	defer diagnostics.SetDefault(nil)
	w := httptest.NewRecorder()
	diagnostics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dbError(w, r, fmt.Errorf("query failed: %w", &pgconn.PgError{Code: "08006", Message: "PRIVATE"}), "courses.access")
	})).ServeHTTP(w, httptest.NewRequest("GET", "/api/courses/access", nil))
	s.Close()
	report, err := s.Report(time.Now().Add(-time.Hour))
	if err != nil || len(report.Groups) != 1 {
		t.Fatalf("missing event: %+v %v", report, err)
	}
	e := report.Groups[0].Sample
	if w.Code != 500 || e.Operation != "courses.access" || e.Cause != "postgres_08006" || e.Route != "/api/courses/access" || e.RequestID != w.Header().Get("X-Request-ID") || e.RequestID == "" {
		t.Fatalf("missing safe context: %+v", e)
	}
}
