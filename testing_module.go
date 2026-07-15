package main

import (
	"context"
	_ "embed"
)

//go:embed migrations/003_testing.sql
var testingMigrationSQL string

//go:embed migrations/004_attempt_timing.sql
var attemptTimingMigrationSQL string

func prepareTestingDatabase(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, testingMigrationSQL); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, attemptTimingMigrationSQL)
	return err
}
