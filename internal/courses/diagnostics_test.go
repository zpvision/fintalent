package courses

import (
	"FinTalent/internal/diagnostics"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

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
		dbError(w, fmt.Errorf("query failed: %w", &pgconn.PgError{Code: "08006", Message: "PRIVATE"}), "courses.access")
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
