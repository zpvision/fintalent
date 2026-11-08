package academy

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestStudentSourceFilters(t *testing.T) {
	for _, v := range []struct {
		source string
		id     int64
		match  bool
	}{{"", 0, true}, {"", 42, true}, {"platform", 0, true}, {"platform", 42, false}, {"42", 42, true}, {"41", 42, false}} {
		s := &AnalyticsStudent{SourceID: v.id}
		if got := analyticsStudentMatch(s, AnalyticsFilters{Source: v.source}, defaultAnalyticsSettings()); got != v.match {
			t.Fatalf("source=%s id=%d: got %v", v.source, v.id, got)
		}
	}
	now := time.Now()
	for _, source := range []string{"", "platform", "42"} {
		f, e := parseAnalyticsFilters(url.Values{"source": {source}}, now)
		if e != nil || f.Source != source || f.values().Get("source") != source {
			t.Fatal("source filter roundtrip")
		}
	}
	for _, source := range []string{"0", "-1", "x", "1 OR true"} {
		if _, e := parseAnalyticsFilters(url.Values{"source": {source}}, now); e == nil {
			t.Fatal("invalid source accepted")
		}
	}
}

func TestStaffSourcesStayInAuthorizedScope(t *testing.T) {
	h := &Handler{} // a nil DB proves this path cannot query all-course statistics.
	d := &analyticsDataset{Students: []*AnalyticsStudent{{SourceID: 42, SourceName: "Telegram"}, {SourceID: 42, SourceName: "Telegram"}, {SourceName: "FinTalent"}}}
	sources, e := h.studentSources(context.Background(), d)
	if e != nil || len(sources) != 1 || sources[0]["id"] != int64(42) || sources[0]["visits"] != nil {
		t.Fatal("staff acquisition scope leaked")
	}
}
