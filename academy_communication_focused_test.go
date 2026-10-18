package main

import (
	"FinTalent/internal/academy"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Focused opt-in fixture: no live user data and no SMTP transport.
type academyFocusedFixture struct {
	DB                                                *sql.DB
	Mux                                               *http.ServeMux
	Owner, Student, Teacher, Course, Program, Version int64
	Handler                                           *academy.Handler
	Root                                              string
}

func newAcademyFocusedFixture(t *testing.T) academyFocusedFixture {
	t.Helper()
	if os.Getenv("RUN_COURSES_DB_TESTS") != "1" {
		t.Skip("set RUN_COURSES_DB_TESTS=1 and cloud DATABASE_URL")
	}
	loadLocalEnv(".env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}
	config, e := pgx.ParseConfig(dsn)
	if e != nil {
		t.Fatal("invalid DATABASE_URL")
	}
	admin := stdlib.OpenDB(*config)
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("academy_comm_audit_%d", time.Now().UnixNano())
	t.Logf("isolated schema: %s", schema)
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = admin.Exec("CREATE SCHEMA " + quoted); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "academy_comm_audit_") {
			panic("unsafe test schema")
		}
		if _, e := admin.Exec("DROP SCHEMA " + quoted + " CASCADE"); e != nil {
			t.Error(e)
		}
	})
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	t.Cleanup(func() { isolated.Close() })
	oldDB := db
	db = isolated
	t.Cleanup(func() { db = oldDB })
	var actual string
	if e = isolated.QueryRow("SELECT current_schema()").Scan(&actual); e != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEMO_DATA", "false")
	t.Setenv("SYNC_GEOGRAPHY", "false")
	if e = prepareDatabase(); e != nil {
		t.Fatal(e)
	}
	if e = prepareCoursesDatabase(context.Background()); e != nil {
		t.Fatal("migration repeat", e)
	}

	var owner, student, teacher int64
	for i, p := range []*int64{&owner, &student, &teacher} {
		email := fmt.Sprintf("academy-e-%d@example.invalid", i)
		if i == 2 {
			email = "academy-d-1@example.invalid"
		}
		if e = db.QueryRow("INSERT INTO users(full_name,email,password_hash) VALUES($1,$2,repeat('*',60)) RETURNING id", fmt.Sprintf("Stage E %d", i), email).Scan(p); e != nil {
			t.Fatal(e)
		}
	}
	var course, program, version int64
	if e = db.QueryRow("INSERT INTO courses(owner_id,slug,title) VALUES($1,'stage-e','Stage E course') RETURNING id", owner).Scan(&course); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow("INSERT INTO academy_course_programs(course_id,source) VALUES($1,'empty') RETURNING id", course).Scan(&program); e != nil {
		t.Fatal(e)
	}
	p := academy.Program{ID: program, CourseID: course, Revision: 1, Source: "empty", Modules: []academy.Module{{Key: academy.Key(), Title: "Module", Status: "published", Lessons: []academy.Lesson{{Key: academy.Key(), Title: "Lesson", Status: "published", Required: true, SelfPaced: true, CompletionRule: "manual", Blocks: []academy.Block{{Key: academy.Key(), Type: "text", Data: academy.BlockData{Text: "Text"}}}}}}}}
	raw, _ := json.Marshal(p)
	if e = db.QueryRow("INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,1,1,$2,$3) RETURNING id", program, raw, owner).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO academy_version_releases(course_id,version_id,published_by) VALUES($1,$2,$3)", course, version, owner); e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	mux := http.NewServeMux()
	h := academy.New(db, func(r *http.Request) (int64, error) { return strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64) }, func(h http.HandlerFunc) http.HandlerFunc { return h }, root)
	h.Register(mux)
	return academyFocusedFixture{DB: db, Mux: mux, Owner: owner, Student: student, Teacher: teacher, Course: course, Program: program, Version: version, Root: root, Handler: h}
}

func TestAcademyCommunicationsIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	testAcademyStageE(t, f.DB, f.Mux, f.Owner, f.Student, f.Course)
}
