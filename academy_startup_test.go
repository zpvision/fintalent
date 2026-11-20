package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type academyStartupScript struct{ stage, query string }

func academyStartupScripts() []academyStartupScript {
	return []academyStartupScript{
		{"schema", academySchema}, {"learning", academyLearningSchema},
		{"assessment", academyAssessmentSchema}, {"cohorts", academyCohortSchema},
		{"communications", academyCommunicationSchema}, {"analytics", academyAnalyticsSchema},
		{"admin", academyAdminSchema}, {"publication_defaults", academyPublicationDefaultsSchema},
		{"object_storage", academyObjectStorageSchema}, {"storage_owners", academyStorageOwnersSchema},
		{"manual_payments", academyManualPaymentsSchema}, {"program_revisions", academyProgramRevisionsSchema},
		{"default_automations", academyDefaultAutomationsSchema},
	}
}

type academyStartupFixture struct {
	t       *testing.T
	ctx     context.Context
	scripts []academyStartupScript
	logs    fmt.Stringer
	calls   int
	failAt  int
	failure error
}
type academyStartupConnector struct{ f *academyStartupFixture }
type academyStartupDriver struct{}
type academyStartupConn struct{ f *academyStartupFixture }

func (c academyStartupConnector) Connect(context.Context) (driver.Conn, error) {
	return academyStartupConn{c.f}, nil
}
func (academyStartupConnector) Driver() driver.Driver { return academyStartupDriver{} }
func (academyStartupDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("connector required")
}
func (academyStartupConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared query")
}
func (academyStartupConn) Close() error { return nil }
func (academyStartupConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (c academyStartupConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	f := c.f
	if f.calls >= len(f.scripts) {
		f.t.Fatal("additional SQL call")
	}
	script := f.scripts[f.calls]
	if query != script.query || len(args) != 0 || ctx != f.ctx {
		f.t.Fatal("SQL text, arguments or context changed", script.stage)
	}
	if !strings.Contains(f.logs.String(), "stage=database.academy."+script.stage+" event=begin") {
		f.t.Fatal("timer missing before SQL", script.stage)
	}
	if f.calls > 0 {
		previous := f.scripts[f.calls-1].stage
		if !strings.Contains(f.logs.String(), "stage=database.academy."+previous+" event=end duration_ms=") {
			f.t.Fatal("previous timer spans another script", previous)
		}
	}
	f.calls++
	if f.calls == f.failAt {
		return nil, f.failure
	}
	return driver.RowsAffected(0), nil
}

func TestAcademyStartupScriptTimingsPreserveExecution(t *testing.T) {
	scripts := academyStartupScripts()
	for failAt := 0; failAt <= len(scripts); failAt++ {
		t.Run(fmt.Sprintf("failure_at_%d", failAt), func(t *testing.T) {
			logs := captureStartupLogs(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("private-database-error-token")
			f := &academyStartupFixture{t: t, ctx: ctx, scripts: scripts, logs: logs, failAt: failAt, failure: failure}
			fakeDB := sql.OpenDB(academyStartupConnector{f})
			defer fakeDB.Close()
			original := db
			db = fakeDB
			t.Cleanup(func() { db = original })
			err := prepareAcademyDatabase(ctx)
			wantCalls := len(scripts)
			if failAt > 0 {
				wantCalls = failAt
				if err != failure {
					t.Fatal("original SQL error not preserved")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if f.calls != wantCalls {
				t.Fatal("incorrect number of SQL calls", f.calls, wantCalls)
			}
			for i, script := range scripts {
				for _, event := range []string{"begin", "end"} {
					marker := "startup stage=database.academy." + script.stage + " event=" + event
					want := 0
					if i < wantCalls {
						want = 1
					}
					if got := strings.Count(logs.String(), marker); got != want {
						t.Fatal("incorrect script timing records", script.stage, event, got, want)
					}
				}
			}
			if strings.Contains(logs.String(), failure.Error()) || strings.Contains(logs.String(), "CREATE TABLE") {
				t.Fatal("raw error or SQL logged")
			}
		})
	}
}
