package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// This test uses an isolated PostgreSQL schema and never touches application
// rows in public. Run it explicitly with RUN_STARTUP_DB_TESTS=1.
func TestProductionStartupIsEmptySafeIdempotentAndConcurrent(t *testing.T) {
	if os.Getenv("RUN_STARTUP_DB_TESTS") != "1" {
		t.Skip("set RUN_STARTUP_DB_TESTS=1 and DATABASE_URL to run the isolated startup test")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}
	adminDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	schema := fmt.Sprintf("startup_audit_%d", time.Now().UnixNano())
	if _, err = adminDB.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer adminDB.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`)

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = schema
	isolatedDB := stdlib.OpenDB(*config)
	defer isolatedDB.Close()
	originalDB := db
	db = isolatedDB
	t.Cleanup(func() { db = originalDB })
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEMO_DATA", "false")
	t.Setenv("SYNC_GEOGRAPHY", "false")

	if err = prepareDatabase(); err != nil {
		t.Fatalf("first empty production startup: %v", err)
	}
	assertCount(t, `SELECT COUNT(*) FROM users WHERE NOT is_system`, 0)
	assertCount(t, `SELECT COUNT(*) FROM users WHERE email='system@fintalent.local' AND is_system AND is_blocked`, 1)
	assertCount(t, `SELECT COUNT(*) FROM users WHERE email LIKE '%@fintalent.local' AND email<>'system@fintalent.local'`, 0)
	assertCount(t, `SELECT COUNT(*) FROM test_attempts`, 0)
	assertCount(t, `SELECT COUNT(*) FROM test_reviews`, 0)
	assertCount(t, `SELECT COUNT(*) FROM publications`, 0)
	assertCount(t, `SELECT COUNT(*) FROM accounting_companies`, 0)
	assertCount(t, `SELECT COUNT(*) FROM client_exchange_listings`, 0)

	var systemTests, questions int
	if err = db.QueryRow(`SELECT COUNT(*) FROM tests WHERE slug LIKE 'position-skill-%' OR slug LIKE 'accounting-topic-%'`).Scan(&systemTests); err != nil || systemTests == 0 {
		t.Fatalf("system tests: count=%d err=%v", systemTests, err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM test_questions`).Scan(&questions); err != nil || questions == 0 {
		t.Fatalf("system questions: count=%d err=%v", questions, err)
	}

	var userID, testID, versionID int64
	err = db.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES('Production User','changed-email@example.com',repeat('*',60)) RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO tests(author_id,slug,status,visibility) VALUES($1,'user-owned-audit-test','published','marketplace') RETURNING id`, userID).Scan(&testID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(`INSERT INTO test_versions(test_id,version,title,created_by,published_at) VALUES($1,1,'User test',$2,NOW()) RETURNING id`, testID, userID).Scan(&versionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO test_questions(test_version_id,question,question_type) VALUES($1,'Keep me','text'); INSERT INTO test_statistics(test_id,attempts_count) VALUES($2,17)`, versionID, testID); err != nil {
		t.Fatal(err)
	}

	if err = prepareDatabase(); err != nil {
		t.Fatalf("second startup: %v", err)
	}
	if err = prepareDatabase(); err != nil {
		t.Fatalf("third startup: %v", err)
	}

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- prepareDatabase() }()
	}
	wg.Wait()
	close(errs)
	for startupErr := range errs {
		if startupErr != nil {
			t.Fatalf("parallel startup: %v", startupErr)
		}
	}

	assertCount(t, `SELECT COUNT(*) FROM tests WHERE slug LIKE 'position-skill-%' OR slug LIKE 'accounting-topic-%'`, systemTests)
	assertCount(t, `SELECT COUNT(*) FROM test_questions`, questions+1)
	assertCount(t, `SELECT COUNT(*) FROM tests WHERE slug='user-owned-audit-test' AND author_id=$1`, 1, userID)
	assertCount(t, `SELECT attempts_count FROM test_statistics WHERE test_id=$1`, 17, testID)
	assertCount(t, `SELECT COUNT(*) FROM test_attempts`, 0)
	assertCount(t, `SELECT COUNT(*) FROM users WHERE NOT is_system`, 1)
}

func assertCount(t *testing.T, query string, want int, args ...any) {
	t.Helper()
	var got int
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.QueryRowContext(ctx, query, args...).Scan(&got); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("query %q: got %d, want %d", query, got, want)
	}
}
