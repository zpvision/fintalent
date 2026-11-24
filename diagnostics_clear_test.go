package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"FinTalent/internal/diagnostics"
)

// Session lookup only: no application database or production credentials.
type diagnosticAdminConnector struct{ valid bool }

func (c diagnosticAdminConnector) Connect(context.Context) (driver.Conn, error) {
	return diagnosticAdminConn{valid: c.valid}, nil
}
func (c diagnosticAdminConnector) Driver() driver.Driver { return nil }

type diagnosticAdminConn struct {
	driver.Conn
	valid bool
}

func (c diagnosticAdminConn) Close() error { return nil }
func (c diagnosticAdminConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "FROM admin_sessions") {
		return nil, errors.New("unexpected query")
	}
	return &diagnosticAdminRows{valid: c.valid}, nil
}

type diagnosticAdminRows struct{ valid, read bool }

func (r *diagnosticAdminRows) Columns() []string { return []string{"valid"} }
func (r *diagnosticAdminRows) Close() error      { return nil }
func (r *diagnosticAdminRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	values[0] = r.valid
	return nil
}

func TestAdminDiagnosticClear(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin       string
		cookie, valid, unavailable bool
		want                       int
	}{
		{"clear all periods", "DELETE", "", true, true, false, 204},
		{"no session", "DELETE", "", false, false, false, 401},
		{"expired session", "DELETE", "", true, false, false, 401},
		{"foreign origin", "DELETE", "https://evil.invalid", true, true, false, 403},
		{"unsupported method", "POST", "", true, true, false, 405},
		{"unavailable journal", "DELETE", "", true, true, true, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := diagnostics.Open(diagnostics.Config{Dir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.Record(diagnostics.Event{Source: "backend", Code: "old"})
			oldDB, oldJournal := db, errorJournal
			db = sql.OpenDB(diagnosticAdminConnector{valid: tc.valid})
			defer db.Close()
			errorJournal = s
			if tc.unavailable {
				errorJournal = nil
			}
			defer func() { db, errorJournal = oldDB, oldJournal }()
			r := httptest.NewRequest(tc.method, "/api/admin/diagnostics?hours=invalid", nil)
			if tc.cookie {
				r.AddCookie(&http.Cookie{Name: adminCookie, Value: strings.Repeat("a", 64)})
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			requestSecurity(http.HandlerFunc(adminErrorReport)).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			report, err := s.Report(time.Now().Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			wantEvents := 1
			if tc.want == 204 {
				wantEvents = 0
			}
			if report.Events != wantEvents {
				t.Fatalf("events=%d, want %d", report.Events, wantEvents)
			}
		})
	}
}
