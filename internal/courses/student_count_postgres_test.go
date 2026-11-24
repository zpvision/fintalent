package courses

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Uses a cloud connection supplied explicitly by the caller. All queries run in
// a read-only transaction; synthetic enrollments exist only in a query CTE.
func TestCourseStudentCountPostgres(t *testing.T) {
	dsn := os.Getenv("COURSE_STUDENT_COUNT_TEST_DSN")
	if dsn == "" {
		t.Skip("cloud COURSE_STUDENT_COUNT_TEST_DSN is not configured")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid cloud connection configuration")
	}
	if config.Host == "localhost" || config.Host == "127.0.0.1" || config.Host == "::1" {
		t.Fatal("cloud PostgreSQL is required")
	}
	db := stdlib.OpenDB(*config)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal("cannot start read-only cloud transaction")
	}
	defer tx.Rollback()
	var courseID int64
	if err := tx.QueryRowContext(ctx, `SELECT c.id FROM courses c JOIN users u ON u.id=c.owner_id ORDER BY c.id LIMIT 1`).Scan(&courseID); err != nil {
		t.Fatal("a course is required for the read-only detail query check")
	}
	for _, tc := range []struct {
		name, rows string
		want       int64
	}{
		{"empty course", `SELECT $1::bigint AS course_id, 1::bigint AS user_id, 1::bigint AS tariff_id WHERE false`, 0},
		{"one student", `SELECT $1::bigint, 1::bigint, 1::bigint`, 1},
		{"all tariffs and unique students", `VALUES ($1::bigint,1::bigint,1::bigint),($1,2,2),($1,1,2),($1,3,NULL),(-1,4,1)`, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			query := `WITH academy_enrollments(course_id,user_id,tariff_id) AS (` + tc.rows + `) ` + detailSQL + `c.id=$1`
			if err := tx.QueryRowContext(ctx, query, courseID).Scan(&raw); err != nil {
				t.Fatal("course detail query failed", err)
			}
			var result struct {
				StudentCount int64 `json:"student_count"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if result.StudentCount != tc.want {
				t.Fatalf("student_count=%d, want %d", result.StudentCount, tc.want)
			}
		})
	}
	// Verify the same production query against the real enrollment aggregate.
	var raw json.RawMessage
	if err := tx.QueryRowContext(ctx, detailSQL+`c.id=$1`, courseID).Scan(&raw); err != nil {
		t.Fatal("live course detail query failed", err)
	}
	var result struct {
		StudentCount int64 `json:"student_count"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	var want int64
	if err := tx.QueryRowContext(ctx, `SELECT count(DISTINCT user_id) FROM academy_enrollments WHERE course_id=$1`, courseID).Scan(&want); err != nil {
		t.Fatal("live enrollment aggregate failed")
	}
	if result.StudentCount != want {
		t.Fatalf("live student_count=%d, want %d", result.StudentCount, want)
	}
}
