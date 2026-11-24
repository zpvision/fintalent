package academy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"FinTalent/internal/diagnostics"
)

func TestProblemCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
		err    error
		abort  bool
	}{
		{"abandoned request", true, context.Canceled, true},
		{"wrapped cancellation", true, fmt.Errorf("query: %w", context.Canceled), true},
		{"internal cancellation", false, context.Canceled, false},
		{"timeout", true, context.DeadlineExceeded, false},
		{"database failure after disconnect", true, errors.New("database unavailable"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			diagnostics.SetDefault(store)
			defer diagnostics.SetDefault(nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			w := httptest.NewRecorder()
			w.Code = 0
			var recovered any
			cleaned := false
			func() {
				defer func() { recovered = recover() }()
				diagnostics.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer func() { cleaned = true }()
					problem(w, tc.err)
				})).ServeHTTP(w, httptest.NewRequest("POST", "/api/academy/enrollments/1/lessons/lesson/complete", nil).WithContext(ctx))
			}()
			store.Close()
			report, err := store.Report(time.Now().Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if !cleaned {
				t.Fatal("request cleanup skipped")
			}
			if tc.abort {
				if recovered != http.ErrAbortHandler || w.Code != 0 || w.Body.Len() != 0 || report.Events != 0 {
					t.Fatalf("abandoned request produced an error: panic=%v status=%d events=%d", recovered, w.Code, report.Events)
				}
			} else if recovered != nil || w.Code != 500 || report.Events != 1 {
				t.Fatalf("real failure lost: panic=%v status=%d events=%d", recovered, w.Code, report.Events)
			}
		})
	}
}
