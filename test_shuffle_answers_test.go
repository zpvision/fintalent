package main

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestTestShuffleMigrationIntegration(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 and DATABASE_URL to a separate cloud test database")
	}
	loadLocalEnv(".env")
	conn, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("cannot configure test database")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, populated := range []bool{false, true} {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("cannot connect to test database")
		}
		func() {
			defer tx.Rollback()
			exec := func(query string) {
				t.Helper()
				if _, err := tx.ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			exec(`CREATE TEMP TABLE test_versions (id BIGINT PRIMARY KEY);
				CREATE TEMP TABLE test_questions (test_version_id BIGINT, settings JSONB);`)
			if populated {
				exec(`INSERT INTO test_versions VALUES (1),(2),(3),(4);
					INSERT INTO test_questions VALUES (1,'{"shuffle_answers":true}'),(1,'{"shuffle_answers":false}'),
					(2,'{"shuffle_answers":false}'),(3,'{}');`)
			}
			exec(testShuffleAnswersMigrationSQL)
			if !populated {
				exec(`INSERT INTO test_versions(id) VALUES (1)`)
			}
			check := func(id int, want bool) {
				t.Helper()
				var got bool
				if err := tx.QueryRowContext(ctx, `SELECT shuffle_answers FROM test_versions WHERE id=$1`, id).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("version %d: shuffle=%v, want %v", id, got, want)
				}
			}
			check(1, populated)
			if populated {
				for _, id := range []int{2, 3, 4} {
					check(id, false)
				}
				exec(`UPDATE test_versions SET shuffle_answers=false WHERE id=1;
					UPDATE test_versions SET shuffle_answers=true WHERE id=2;`)
			}
			exec(testShuffleAnswersMigrationSQL)
			check(1, false)
			if populated {
				check(2, true)
			}
		}()
	}
}
