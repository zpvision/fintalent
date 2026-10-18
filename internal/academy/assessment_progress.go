package academy

import (
	"context"
	"database/sql"
	"time"
)

type sqlReader interface {
	queryer
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func assessmentOutcomes(ctx context.Context, q sqlReader, e *Enrollment) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.block_key,CASE WHEN a.kind='test' THEN COALESCE((SELECT t.passed FROM academy_effective_test_attempts t WHERE t.enrollment_id=$1 AND t.assessment_id=a.id AND t.status='finished' ORDER BY CASE WHEN COALESCE((a.config->'assessment'->>'use_best')::boolean,true) THEN t.percent END DESC,t.number DESC LIMIT 1),false) ELSE EXISTS(SELECT 1 FROM academy_effective_assignment_submissions s WHERE s.enrollment_id=$1 AND s.assessment_id=a.id AND s.status='accepted') END FROM academy_assessment_versions a WHERE a.program_version_id=$2`, e.ID, e.VersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		var passed bool
		if err = rows.Scan(&key, &passed); err != nil {
			return nil, err
		}
		out[key] = passed
	}
	return out, rows.Err()
}
func requiredBlock(l Lesson, b Block) bool {
	return oneOf(b.Type, "test", "assignment") && (yes(policy(b).Required) || l.CompletionRule == b.Type)
}
func requirementsReason(l Lesson, states map[string]bool) string {
	if reason := CompletionBlock(l); reason != "" {
		return reason
	}
	for _, b := range l.Blocks {
		if requiredBlock(l, b) && !states[b.Key] {
			if b.Type == "test" {
				return "Сначала успешно пройдите обязательный тест"
			}
			return "Дождитесь принятия обязательного задания"
		}
	}
	return ""
}
func readProgress(ctx context.Context, q sqlReader, e *Enrollment) error {
	rows, err := q.QueryContext(ctx, `SELECT lesson_key,status,completed_at FROM academy_lesson_progress WHERE enrollment_id=$1 AND program_version_id=$2 ORDER BY lesson_key`, e.ID, e.VersionID)
	if err != nil {
		return err
	}
	e.Progress = []Progress{}
	for rows.Next() {
		var p Progress
		if err = rows.Scan(&p.LessonKey, &p.Status, &p.CompletedAt); err != nil {
			rows.Close()
			return err
		}
		e.Progress = append(e.Progress, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	e.AssessmentStates, err = assessmentOutcomes(ctx, q, e)
	e.Summary = SummaryFor(e.Program, e.Progress, e.StartedAt != nil)
	return err
}
func syncCourseTx(ctx context.Context, tx *sql.Tx, e *Enrollment, actor int64) error {
	if err := readProgress(ctx, tx, e); err != nil {
		return err
	}
	if e.Summary.Status == "completed" && e.CompletedAt == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE academy_enrollments SET completed_at=NOW() WHERE id=$1`, e.ID); err != nil {
			return err
		}
		return event(ctx, tx, e, actor, "course_completed", "course_completed", "", map[string]int{"required": e.Summary.Required})
	}
	if e.Summary.Status != "completed" && e.CompletedAt != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE academy_enrollments SET completed_at=NULL WHERE id=$1`, e.ID); err != nil {
			return err
		}
		return event(ctx, tx, e, actor, "course_requirements_pending", "", "", map[string]any{})
	}
	return nil
}
func reconcileAssessmentTx(ctx context.Context, tx *sql.Tx, e *Enrollment, actor int64, key string) error {
	l, ok := findLesson(e.Program, key)
	if !ok {
		return ErrInvalid
	}
	states, err := assessmentOutcomes(ctx, tx, e)
	if err != nil {
		return err
	}
	has := false
	for _, b := range l.Blocks {
		has = has || requiredBlock(l, b)
	}
	if !has {
		return nil
	}
	var previous string
	var ack *time.Time
	err = tx.QueryRowContext(ctx, `SELECT status,materials_acknowledged_at FROM academy_lesson_progress WHERE enrollment_id=$1 AND lesson_key=$2`, e.ID, key).Scan(&previous, &ack)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	ready := requirementsReason(l, states) == "" && (l.CompletionRule != "materials" || ack != nil)
	if ready && previous != "completed" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status,completed_at) VALUES($1,$2,$3,'completed',NOW()) ON CONFLICT(enrollment_id,lesson_key) DO UPDATE SET status='completed',completed_at=NOW()`, e.ID, e.VersionID, key); err != nil {
			return err
		}
		if err = event(ctx, tx, e, actor, "lesson_completed", "complete:"+key, key, map[string]any{"source": "assessments"}); err != nil {
			return err
		}
	}
	if !ready && previous == "completed" {
		if _, err = tx.ExecContext(ctx, `UPDATE academy_lesson_progress SET status='in_progress',completed_at=NULL WHERE enrollment_id=$1 AND lesson_key=$2`, e.ID, key); err != nil {
			return err
		}
		if err = event(ctx, tx, e, actor, "lesson_requirements_pending", "", key, map[string]any{}); err != nil {
			return err
		}
	}
	return syncCourseTx(ctx, tx, e, actor)
}
func touchLearningTx(ctx context.Context, tx *sql.Tx, e *Enrollment, actor int64, key string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status) VALUES($1,$2,$3,'in_progress') ON CONFLICT DO NOTHING`, e.ID, e.VersionID, key); err != nil {
		return err
	}
	if e.StartedAt == nil {
		if err := event(ctx, tx, e, actor, "course_started", "course_started", key, map[string]any{}); err != nil {
			return err
		}
		now := time.Now()
		e.StartedAt = &now
	}
	_, err := tx.ExecContext(ctx, `UPDATE academy_enrollments SET started_at=COALESCE(started_at,NOW()),last_activity_at=NOW(),last_lesson_key=$2 WHERE id=$1`, e.ID, key)
	return err
}
