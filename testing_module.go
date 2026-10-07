package main

import (
	"context"
	_ "embed"
)

//go:embed migrations/003_testing.sql
var testingMigrationSQL string

//go:embed migrations/004_attempt_timing.sql
var attemptTimingMigrationSQL string

//go:embed migrations/026_resume_test_knowledge.sql
var resumeTestKnowledgeMigrationSQL string

//go:embed migrations/054_resume_test_confirmations.sql
var resumeTestConfirmationsMigrationSQL string

//go:embed migrations/047_test_shuffle_answers.sql
var testShuffleAnswersMigrationSQL string

//go:embed migrations/066_attempt_revision.sql
var attemptRevisionMigrationSQL string

//go:embed migrations/044_test_result_resume_visibility.sql
var attemptResumeVisibilityMigrationSQL string

func prepareTestingDatabase(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, testingMigrationSQL); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, attemptResumeVisibilityMigrationSQL); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, attemptTimingMigrationSQL)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, resumeTestConfirmationsMigrationSQL)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, testShuffleAnswersMigrationSQL)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, attemptRevisionMigrationSQL)
	return err
}
