package academy

import (
	"testing"
	"time"
)

func TestCohortTimeAndDST(t *testing.T) {
	for _, v := range []string{"2026-03-29T02:30", "2026-10-25T02:30"} {
		if _, err := cohortTime(v, "Europe/Prague"); err == nil {
			t.Fatalf("unsafe DST wall time accepted: %s", v)
		}
	}
	a, err := cohortTime("2026-10-25T02:30:00+02:00", "Europe/Prague")
	if err != nil || a.Format(time.RFC3339) != "2026-10-25T00:30:00Z" {
		t.Fatal(a, err)
	}
	start, _ := cohortTime("2026-03-28T10:00", "Europe/Prague")
	c := &Cohort{StartsAt: start, Timezone: "Europe/Prague"}
	next, err := ruleTime(c, CohortRule{Day: 1, Clock: "10:00"})
	if err != nil || next.Sub(start) != 23*time.Hour {
		t.Fatal("relative rule must use calendar days across DST", next, err)
	}
}
func TestCohortRulesPrecedenceAndLifecycle(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	c := &Cohort{Status: "active", StartsAt: start, Timezone: "UTC", Mode: "calendar", Rules: []CohortRule{{Kind: "module", Key: "module", At: time.Now().Add(time.Hour).Format(time.RFC3339)}, {Kind: "lesson", Key: "lesson", At: start.Format(time.RFC3339)}}}
	if !availability(c, "module", "lesson", time.Now()).Available {
		t.Fatal("lesson override lost")
	}
	if availability(c, "module", "other", time.Now()).Available {
		t.Fatal("module rule ignored")
	}
	c.Status = "draft"
	if availability(c, "module", "lesson", time.Now()).Available {
		t.Fatal("draft cohort exposed")
	}
	c.Status = "completed"
	if !availability(c, "module", "lesson", time.Now()).Available {
		t.Fatal("completion revoked access")
	}
	if cohortTransition("archived", "active") || cohortTransition("active", "draft") || !cohortTransition("active", "completed") || !cohortTransition("completed", "archived") {
		t.Fatal("invalid lifecycle")
	}
}
func TestCohortRejectsForeignAndDuplicateRules(t *testing.T) {
	c := &Cohort{Name: "Test", Type: "group", Mode: "calendar", StartsAt: time.Now(), Timezone: "UTC", Rules: []CohortRule{{Kind: "lesson", Key: "foreign", At: time.Now().Format(time.RFC3339)}}}
	if validateCohort(c, Program{}) == nil {
		t.Fatal("foreign lesson accepted")
	}
	c.Rules = nil
	c.Deadlines = []CohortRule{{Kind: "assignment", Key: "foreign"}}
	if validateCohort(c, Program{}) == nil {
		t.Fatal("foreign assignment accepted")
	}
}
