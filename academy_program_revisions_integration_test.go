package main

import (
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Never falls back to DATABASE_URL: that may be the live production database.
func newProgramRevisionFixture(t *testing.T, legacy ...bool) academyFocusedFixture {
	t.Helper()
	dsn := os.Getenv("ACADEMY_REVISIONS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated cloud test database is not configured")
	}
	c, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid dedicated test DSN")
	}
	localTest := (c.Host == "127.0.0.1" || c.Host == "localhost") && c.Port == 55493 && c.Database == "fintalent_revisions_test"
	if c.Host == "200.169.191.202" || !localTest && (c.Host == "localhost" || c.Host == "127.0.0.1") || !strings.Contains(strings.ToLower(c.Database), "test") {
		t.Fatal("refusing non-test database")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("RUN_COURSES_DB_TESTS", "1")
	if len(legacy) > 0 && legacy[0] {
		original := academyProgramRevisionsSchema
		academyProgramRevisionsSchema = ""
		defer func() { academyProgramRevisionsSchema = original }()
	}
	return newAcademyFocusedFixture(t)
}

func TestAcademyProgramRevisionMigrationIsolatedPostgres(t *testing.T) {
	f := newProgramRevisionFixture(t, true)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := f.DB.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	scalar := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if e := f.DB.QueryRow(q, args...).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}
	var enrollment int64
	if e := f.DB.QueryRow(`INSERT INTO academy_enrollments(course_id,user_id,program_version_id,request_key,enrolled_by,completed_at)
 VALUES($1,$2,$3,'revision-migration-fixture',$4,'2026-01-01T00:00:00Z') RETURNING id`, f.Course, f.Student, f.Version, f.Owner).Scan(&enrollment); e != nil {
		t.Fatal(e)
	}
	exec(`INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status,completed_at)
 SELECT $1,$2,content->'modules'->0->'lessons'->0->>'key','completed','2026-01-01T00:00:00Z'
 FROM academy_program_versions WHERE id=$2`, enrollment, f.Version)
	if scalar(`SELECT count(*) FROM pg_tables WHERE schemaname=current_schema() AND tablename='academy_program_revisions'`) != 0 {
		t.Fatal("fixture is not the old schema")
	}
	aid := scalar(`INSERT INTO academy_assessment_versions(program_version_id,lesson_key,block_key,kind,config,questions,title)
 SELECT id,content->'modules'->0->'lessons'->0->>'key','historical-test','test','{}','[{"id":1,"question":"Old question","points":10}]','Historical test' FROM academy_program_versions WHERE id=$1 RETURNING id`, f.Version)
	tid := scalar(`INSERT INTO academy_test_attempts(enrollment_id,program_version_id,assessment_id,number,request_key,status,answers,grades,score,max_score,percent,passed,finished_at)
 VALUES($1,$2,$3,1,'migration-old-attempt','finished','[{"question_id":1,"selected_answer_ids":[1]}]','[{"earned_points":5}]',5,10,50,false,now()) RETURNING id`, enrollment, f.Version, aid)
	var originalAttempt string
	if err := f.DB.QueryRow(`SELECT to_jsonb(t)::text FROM academy_test_attempts t WHERE id=$1`, tid).Scan(&originalAttempt); err != nil {
		t.Fatal(err)
	}
	// The fixture creates the old immutable version after prepareDatabase().
	// Re-run only the additive migration to exercise old-data backfill.
	exec(academyProgramRevisionsSchema)
	exec(academyProgramRevisionsSchema)
	if scalar(`SELECT count(*) FROM academy_program_revisions WHERE version_id=$1`, f.Version) != 1 {
		t.Fatal("revision backfill duplicated")
	}
	var migratedAttempt string
	if err := f.DB.QueryRow(`SELECT to_jsonb(t)::text FROM academy_test_attempts t WHERE id=$1`, tid).Scan(&migratedAttempt); err != nil {
		t.Fatal(err)
	}
	if migratedAttempt != originalAttempt {
		t.Fatal("historical attempt changed during migration")
	}
	if scalar(`SELECT count(*) FROM academy_assessment_versions WHERE id=$1 AND publication_revision_id IS NULL`, aid) != 1 {
		t.Fatal("old assessment identity changed")
	}
	if scalar(`SELECT count(*) FROM academy_program_revision_heads h JOIN academy_program_revisions r ON r.id=h.published_revision_id AND r.version_id=h.version_id
 JOIN academy_program_versions v ON v.id=h.version_id WHERE h.version_id=$1 AND r.content=v.content`, f.Version) != 1 {
		t.Fatal("published content mismatch")
	}
	if scalar(`SELECT count(*) FROM academy_enrollments WHERE id=$1 AND program_version_id=$2 AND completed_at='2026-01-01T00:00:00Z'`, enrollment, f.Version) != 1 {
		t.Fatal("enrollment/completion changed")
	}
	if scalar(`SELECT count(*) FROM academy_lesson_progress WHERE enrollment_id=$1 AND status='completed' AND completed_at='2026-01-01T00:00:00Z'`, enrollment) != 1 {
		t.Fatal("progress changed")
	}
	// Draft edits do not modify either old published rows or revision 1.
	exec(`UPDATE academy_program_version_drafts SET content=jsonb_set(content,'{modules,0,title}','"Unpublished"'),revision=revision+1 WHERE version_id=$1`, f.Version)
	if scalar(`SELECT count(*) FROM academy_program_revisions r JOIN academy_program_versions v ON v.id=r.version_id WHERE v.id=$1 AND r.content=v.content`, f.Version) != 1 {
		t.Fatal("draft leaked into publication")
	}
	if _, e := f.DB.Exec(`UPDATE academy_program_revisions SET content='{}' WHERE version_id=$1`, f.Version); e == nil {
		t.Fatal("published revision mutable")
	}
	if _, e := f.DB.Exec(`DELETE FROM academy_program_revisions WHERE version_id=$1`, f.Version); e == nil {
		t.Fatal("published revision deletable")
	}
	exec(academyProgramRevisionsSchema)
	if scalar(`SELECT revision FROM academy_program_version_drafts WHERE version_id=$1`, f.Version) != 2 {
		t.Fatal("migration overwrote draft")
	}
}
