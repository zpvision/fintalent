package academy

import (
	"context"
	"database/sql"
	"time"
)

// Callers hold the course lock. Both manual enrollment and paid applications
// use this transaction-scoped operation and the same published release checks.
func ensureIndependentEnrollment(ctx context.Context, tx *sql.Tx, course, student, version, actor int64, expires *time.Time, key string) (id int64, existing bool, err error) {
	var checked int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system`, student).Scan(&checked); err != nil {
		return
	}
	if err = tx.QueryRowContext(ctx, `SELECT version_id FROM academy_current_version_releases WHERE course_id=$1 AND version_id=$2`, course, version).Scan(&checked); err != nil {
		if err == sql.ErrNoRows {
			err = inputError{"Сначала опубликуйте выбранную версию учебной программы"}
		}
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 AND program_version_id=$3 AND cohort_id IS NULL AND access_status IN ('active','suspended') ORDER BY id DESC LIMIT 1`, course, student, version).Scan(&id)
	existing = err == nil
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `INSERT INTO academy_enrollments(course_id,user_id,program_version_id,expires_at,request_key,enrolled_by) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(request_key) DO NOTHING RETURNING id`, course, student, version, expires, key, nullable(actor)).Scan(&id)
		if err == sql.ErrNoRows {
			err = tx.QueryRowContext(ctx, `SELECT id FROM academy_enrollments WHERE request_key=$1 AND course_id=$2 AND user_id=$3 AND program_version_id=$4`, key, course, student, version).Scan(&id)
		}
		if err == nil {
			err = event(ctx, tx, &Enrollment{ID: id}, actor, "enrolled", "enrolled", "", map[string]any{"version_id": version, "expires_at": expires})
		}
	}
	return
}
