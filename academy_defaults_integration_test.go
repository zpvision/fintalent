package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/courses"
	"context"
	"encoding/json"
	"testing"
)

func TestCourseApprovedEditsStayPublishedPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	ctx := context.Background()
	for _, state := range []struct {
		status   string
		approved bool
		expected string
	}{{"published", true, "published"}, {"draft", true, "published"}, {"hidden", true, "hidden"}, {"moderation", false, "draft"}} {
		if _, err := f.DB.Exec(`UPDATE courses SET status=$2,published_at=CASE WHEN $3 THEN now() ELSE NULL END WHERE id=$1`, f.Course, state.status, state.approved); err != nil {
			t.Fatal(err)
		}
		tx, err := f.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := courses.AdministrativeCourse(ctx, tx, f.Course)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		var input courses.Input
		if err = json.Unmarshal(raw, &input); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		input.Title = "Updated course title"
		if err = courses.SaveAdministrative(ctx, tx, f.Course, &input); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		var status string
		if err = tx.QueryRow(`SELECT status FROM courses WHERE id=$1`, f.Course).Scan(&status); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		tx.Rollback()
		if status != state.expected {
			t.Fatalf("%s approved=%v: got %s", state.status, state.approved, status)
		}
	}
}

func TestAcademyDefaultAutomationsPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t)
	var course int64
	if err := f.DB.QueryRow(`INSERT INTO courses(owner_id,slug) VALUES($1,$2) RETURNING id`, f.Owner, academy.Key()).Scan(&course); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		rows, err := f.DB.Query(`SELECT trigger,config,status FROM academy_automations WHERE course_id=$1 ORDER BY trigger`, course)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var event, status string
			var raw []byte
			if err = rows.Scan(&event, &raw, &status); err != nil {
				t.Fatal(err)
			}
			var config academy.AutomationConfig
			if err = json.Unmarshal(raw, &config); err != nil {
				t.Fatal(err)
			}
			if status != "draft" || len(config.Steps) != 1 || config.Steps[0].Target != "student" || config.Steps[0].Channel != "email" {
				t.Fatal("invalid default scenario")
			}
			if err = academy.ValidateMessage(&config.Steps[0].Message, "https://fintalent.ru"); err != nil {
				t.Fatal(err)
			}
			if event != "enrolled" && event != "course_completed" {
				t.Fatal("unexpected event")
			}
			count++
		}
		if rows.Err() != nil || count != 2 {
			t.Fatal("expected exactly two defaults", count, rows.Err())
		}
	}
	check()
	if _, err := f.DB.Exec(academyDefaultAutomationsSchema); err != nil {
		t.Fatal(err)
	}
	check()
	if _, err := f.DB.Exec(`UPDATE academy_automations SET status='archived' WHERE course_id=$1`, course); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(`SELECT academy_seed_course_automations($1,$2)`, course, f.Owner); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.DB.QueryRow(`SELECT count(*) FROM academy_automations WHERE course_id=$1 AND status='archived'`, course).Scan(&count); err != nil || count != 2 {
		t.Fatal("archived defaults recreated", err)
	}
}
