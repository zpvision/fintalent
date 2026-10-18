package academy

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type CohortRule struct {
	Kind      string `json:"kind"`
	Key       string `json:"key"`
	At        string `json:"at,omitempty"`
	Day       int    `json:"day"`
	Clock     string `json:"clock,omitempty"`
	AllowLate bool   `json:"allow_late"`
}
type Cohort struct {
	ID            int64        `json:"id"`
	CourseID      int64        `json:"course_id"`
	VersionID     int64        `json:"program_version_id"`
	VersionNumber int          `json:"version_number"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	Type          string       `json:"type"`
	Status        string       `json:"status"`
	StartsAt      time.Time    `json:"starts_at"`
	EndsAt        *time.Time   `json:"ends_at"`
	Timezone      string       `json:"timezone"`
	Capacity      *int         `json:"capacity"`
	Mode          string       `json:"schedule_mode"`
	Rules         []CohortRule `json:"rules"`
	Deadlines     []CohortRule `json:"deadlines"`
	Revision      int64        `json:"revision"`
	CreatedAt     time.Time    `json:"created_at"`
	RequestKey    string       `json:"request_key"`
}

// Wall-clock input is interpreted in the cohort zone, never in the server/browser zone.
// Ambiguous or nonexistent DST wall times require an explicit RFC3339 offset.
func cohortTime(value, zone string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, inputError{"Укажите временную зону IANA, например Europe/Moscow"}
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", value, loc)
	if err != nil || t.In(loc).Format("2006-01-02T15:04") != value {
		return time.Time{}, inputError{"Такого местного времени нет. Проверьте дату и переход на летнее время"}
	}
	for _, delta := range []time.Duration{-2 * time.Hour, -time.Hour, time.Hour, 2 * time.Hour} {
		if t.Add(delta).In(loc).Format("2006-01-02T15:04") == value {
			return time.Time{}, inputError{"Это время встречается дважды при переводе часов. Укажите ISO-дату с часовым смещением или другое время"}
		}
	}
	return t.UTC(), nil
}
func ruleTime(c *Cohort, r CohortRule) (time.Time, error) {
	if r.At != "" {
		return cohortTime(r.At, c.Timezone)
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	date := c.StartsAt.In(loc).AddDate(0, 0, r.Day).Format("2006-01-02")
	clock := r.Clock
	if clock == "" {
		clock = c.StartsAt.In(loc).Format("15:04")
	}
	return cohortTime(date+"T"+clock, c.Timezone)
}
func validateCohort(c *Cohort, p Program) error {
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	if utf8.RuneCountInString(c.Name) < 1 || utf8.RuneCountInString(c.Name) > 180 || utf8.RuneCountInString(c.Description) > 4000 || !oneOf(c.Type, "group", "corporate", "individual") || !oneOf(c.Mode, "all", "calendar", "relative") || c.StartsAt.IsZero() || c.EndsAt != nil && !c.EndsAt.After(c.StartsAt) || c.Capacity != nil && (*c.Capacity < 1 || *c.Capacity > 100000) {
		return inputError{"Проверьте название, даты, тип и вместимость потока"}
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil || c.Timezone == "" || c.Timezone == "Local" {
		return inputError{"Неизвестная временная зона IANA"}
	}
	modules, lessons, assignments := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		modules[m.Key] = true
		for _, l := range m.Lessons {
			if l.Status != "published" {
				continue
			}
			lessons[l.Key] = true
			for _, b := range l.Blocks {
				if b.Type == "assignment" {
					assignments[b.Key] = true
				}
			}
		}
	}
	if len(c.Rules) > 6000 || len(c.Deadlines) > 6000 {
		return ErrInvalid
	}
	for i, rules := range [][]CohortRule{c.Rules, c.Deadlines} {
		seen := map[string]bool{}
		for _, r := range rules {
			valid := i == 1 && r.Kind == "assignment" && assignments[r.Key] || i == 0 && (r.Kind == "module" && modules[r.Key] || r.Kind == "lesson" && lessons[r.Key])
			if !valid || seen[r.Kind+":"+r.Key] || r.Day < 0 || r.Day > 3650 {
				return inputError{"Повтор правила или материал не принадлежит опубликованной версии потока"}
			}
			seen[r.Kind+":"+r.Key] = true
			if i == 0 && c.Mode == "calendar" && r.At == "" || i == 0 && c.Mode == "relative" && r.At != "" {
				return inputError{"Правила должны соответствовать режиму расписания"}
			}
			if _, err := ruleTime(c, r); err != nil {
				return err
			}
		}
	}
	return nil
}
func cohortTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "draft":
		return oneOf(to, "scheduled", "active", "archived")
	case "scheduled":
		return oneOf(to, "draft", "active", "archived")
	case "active":
		return oneOf(to, "completed")
	case "completed":
		return to == "archived"
	}
	return false
}

type LessonAvailability struct {
	Available bool       `json:"available"`
	OpensAt   *time.Time `json:"opens_at,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}

func availability(c *Cohort, module, lesson string, now time.Time) LessonAvailability {
	if c == nil {
		return LessonAvailability{Available: true}
	}
	if oneOf(c.Status, "draft", "scheduled") {
		return LessonAvailability{Reason: "Поток ещё не запущен", OpensAt: &c.StartsAt}
	}
	at := c.StartsAt
	if c.Mode != "all" {
		var inherited, own *CohortRule
		for i := range c.Rules {
			r := &c.Rules[i]
			if r.Kind == "module" && r.Key == module {
				inherited = r
			}
			if r.Kind == "lesson" && r.Key == lesson {
				own = r
			}
		}
		if own != nil {
			inherited = own
		}
		if inherited != nil {
			if t, err := ruleTime(c, *inherited); err == nil && t.After(at) {
				at = t
			}
		}
	}
	if now.Before(at) {
		return LessonAvailability{OpensAt: &at, Reason: fmt.Sprintf("Урок откроется %s", at.In(mustLocation(c.Timezone)).Format("02.01.2006 в 15:04 MST"))}
	}
	return LessonAvailability{Available: true, OpensAt: &at}
}
func mustLocation(zone string) *time.Location {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.UTC
	}
	return loc
}
func assignmentDeadline(c *Cohort, key string) (*time.Time, bool) {
	if c != nil {
		for _, r := range c.Deadlines {
			if r.Key == key {
				if t, err := ruleTime(c, r); err == nil {
					return &t, r.AllowLate
				}
			}
		}
	}
	return nil, true
}

func countBlocks(p Program, kind string) int {
	count := 0
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" {
				continue
			}
			for _, b := range l.Blocks {
				if b.Type == kind {
					count++
				}
			}
		}
	}
	return count
}
