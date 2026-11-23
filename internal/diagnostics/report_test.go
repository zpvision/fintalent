package diagnostics

import (
	"net/url"
	"testing"
	"time"
)

func TestExplicitReportPeriod(t *testing.T) {
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		query string
		valid bool
	}{
		{"hours=24", true},
		{"since=2026-11-23T12:00:00%2B01:00&until=2026-11-23T17:00:00%2B01:00", true},
		{"since=2026-11-23T12:00", false},
		{"since=2026-11-23T17:00:00Z&until=2026-11-23T12:00:00Z", false},
		{"until=2026-11-24T12:00:00Z", false},
		{"since=2026-10-01T12:00:00Z", false},
		{"hours=721", false},
	} {
		q, _ := url.ParseQuery(tc.query)
		since, until, err := ParsePeriod(q, now)
		if (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.query, err)
		}
		if tc.valid && q.Get("since") != "" && (since.Hour() != 11 || until.Hour() != 16 || since.Location() != time.UTC) {
			t.Fatalf("timezone conversion failed: %s - %s", since, until)
		}
	}
}

func TestReportRangeFiltersBeforeGroupingAndSeparatesExternalResources(t *testing.T) {
	s := testStore(t, 3)
	start := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		s.now = func() time.Time { return start.Add(time.Duration(i) * time.Minute) }
		s.Record(Event{Source: "frontend", Code: "network_error", Route: "/api/courses/42", UserID: int64(i + 1)})
	}
	s.now = func() time.Time { return start.Add(3 * time.Minute) }
	s.Record(Event{Source: "frontend", Code: "resource_error", ResourceURL: "https://mc.yandex.ru/metrika/tag.js"})
	s.Record(Event{Source: "frontend", Code: "resource_error", ResourceURL: "/static/app.js"})
	r, err := s.ReportRange(start.Add(time.Minute), start.Add(3*time.Minute))
	if err != nil || r.Events != 5 || len(r.Groups) != 3 || r.Timezone != "UTC" {
		t.Fatalf("unexpected report: %+v %v", r, err)
	}
	for _, g := range r.Groups {
		if g.Sample.Code == "network_error" && (g.Count != 3 || g.AffectedUsers != 3 || !g.First.Equal(start.Add(time.Minute)) || !g.Last.Equal(r.Until)) {
			t.Fatalf("events outside range affected group: %+v", g)
		}
		if g.Sample.ResourceURL == "https://mc.yandex.ru/metrika/tag.js" && g.Category != "external_resource" {
			t.Fatal("external resource mixed with application errors")
		}
		if g.Sample.ResourceURL == "/static/app.js" && g.Category != "application" {
			t.Fatal("local asset hidden as external")
		}
	}
}

func TestKnownCourseAccessRoute(t *testing.T) {
	for raw, want := range map[string]string{
		"/api/courses/access?token=secret": "/api/courses/access",
		"/api/courses/private-slug":        "/api/courses/:id",
		"/courses/access":                  "/courses/:id",
	} {
		if got := SafePath(raw); got != want {
			t.Fatalf("%s: %s", raw, got)
		}
	}
}
