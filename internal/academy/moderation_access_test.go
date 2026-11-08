package academy

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
)

type moderationFixture struct {
	owner, staff, approved, unavailable bool
	queries                             []string
}
type moderationConnector struct{ f *moderationFixture }
type moderationDriver struct{}
type moderationConn struct{ f *moderationFixture }
type moderationRows struct {
	value bool
	done  bool
}

func (c moderationConnector) Connect(context.Context) (driver.Conn, error) {
	return moderationConn{c.f}, nil
}
func (c moderationConnector) Driver() driver.Driver        { return moderationDriver{} }
func (moderationDriver) Open(string) (driver.Conn, error)  { return nil, errors.New("connector only") }
func (moderationConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (moderationConn) Close() error                        { return nil }
func (moderationConn) Begin() (driver.Tx, error)           { return nil, errors.New("writes forbidden") }
func (c moderationConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.f.queries = append(c.f.queries, query)
	if c.f.unavailable {
		return nil, errors.New("fixture unavailable")
	}
	if strings.Contains(query, "published_at IS NOT NULL OR status='published'") {
		return &moderationRows{value: c.f.approved}, nil
	}
	if strings.Contains(query, "academy_cohort_staff") {
		return &moderationRows{value: c.f.staff}, nil
	}
	if strings.Contains(query, "SELECT EXISTS(SELECT 1 FROM courses") {
		return &moderationRows{value: c.f.owner}, nil
	}
	return nil, errors.New("unexpected query after moderation denial")
}
func (*moderationRows) Columns() []string { return []string{"allowed"} }
func (*moderationRows) Close() error      { return nil }
func (r *moderationRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}
func moderationHandler(t *testing.T, f *moderationFixture) *Handler {
	t.Helper()
	db := sql.OpenDB(moderationConnector{f})
	t.Cleanup(func() { db.Close() })
	return New(db, func(*http.Request) (int64, error) { return 7, nil }, func(next http.HandlerFunc) http.HandlerFunc { return next }, t.TempDir())
}

func TestModerationBlocksAuthorAndStaffAPIs(t *testing.T) {
	for _, owner := range []bool{true, false} {
		for _, path := range []string{"program", "assets/uploads", "cohorts", "communications", "students"} {
			f := &moderationFixture{owner: owner, staff: !owner}
			h := moderationHandler(t, f)
			w := httptest.NewRecorder()
			h.route(w, httptest.NewRequest("POST", "/api/academy/courses/42/"+path, strings.NewReader(`{}`)))
			if w.Code != 403 || !strings.Contains(w.Body.String(), "модератором") {
				t.Fatalf("owner=%v path=%s: moderation gate did not deny request", owner, path)
			}
		}
	}
	f := &moderationFixture{}
	w := httptest.NewRecorder()
	moderationHandler(t, f).route(w, httptest.NewRequest("GET", "/api/academy/courses/42/program", nil))
	if w.Code != 404 {
		t.Fatal("unrelated user must retain the existing 404 response")
	}
}

func TestModerationCoursePageAndFailureHandling(t *testing.T) {
	for _, test := range []struct {
		path                  string
		approved, unavailable bool
		status                int
	}{
		{"/profile/courses/42/academy", false, false, 303},
		{"/profile/courses/42/academy/cohorts", false, false, 303},
		{"/profile/courses/42/academy", true, false, 204},
		{"/profile/courses/42/edit", false, false, 204},
		{"/profile/courses/42/preview", false, false, 204},
		{"/profile/courses/42/academy", false, true, 500},
	} {
		h := moderationHandler(t, &moderationFixture{owner: true, approved: test.approved, unavailable: test.unavailable})
		w := httptest.NewRecorder()
		h.CoursePage(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(w, httptest.NewRequest("GET", test.path, nil))
		if w.Code != test.status {
			t.Fatalf("%s: got %d, want %d", test.path, w.Code, test.status)
		}
		if test.status == 303 && w.Header().Get("Location") != "/profile/courses?academy=moderation" {
			t.Fatal("unexpected redirect destination")
		}
	}
}
