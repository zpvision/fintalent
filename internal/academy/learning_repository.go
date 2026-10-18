package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrAccess = errors.New("Доступ к обучению приостановлен, отозван или закончился")

type Enrollment struct {
	AssessmentStates map[string]bool `json:"-"`
	ID               int64           `json:"id"`
	CourseID         int64           `json:"course_id"`
	UserID           int64           `json:"user_id"`
	VersionID        int64           `json:"program_version_id"`
	VersionNumber    int             `json:"version_number"`
	AccessStatus     string          `json:"access_status"`
	EffectiveStatus  string          `json:"effective_status"`
	EnrolledAt       time.Time       `json:"enrolled_at"`
	StartedAt        *time.Time      `json:"started_at"`
	ExpiresAt        *time.Time      `json:"expires_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
	LastActivityAt   *time.Time      `json:"last_activity_at"`
	LastLessonKey    string          `json:"last_lesson_key"`
	Title            string          `json:"title"`
	Cover            string          `json:"cover_image"`
	Author           string          `json:"author_name"`
	UserName         string          `json:"user_name"`
	Avatar           string          `json:"avatar"`
	Program          Program         `json:"-"`
	Tests            json.RawMessage `json:"-"`
	ProgramID        int64           `json:"-"`
	Unavailable      bool            `json:"-"`
	Summary          Summary         `json:"summary"`
	Progress         []Progress      `json:"progress,omitempty"`
}

const enrollmentSQL = `SELECT e.id,e.course_id,e.user_id,e.program_version_id,v.number,e.access_status,e.enrolled_at,e.started_at,e.expires_at,e.completed_at,e.last_activity_at,e.last_lesson_key,c.title,c.cover_image,author.full_name,student.full_name,COALESCE(student.avatar_url,''),v.content,r.test_metadata,v.program_id,(author.is_blocked OR student.is_blocked OR student.is_system) FROM academy_enrollments e JOIN courses c ON c.id=e.course_id JOIN users author ON author.id=c.owner_id JOIN users student ON student.id=e.user_id JOIN academy_program_versions v ON v.id=e.program_version_id JOIN academy_version_releases r ON r.version_id=v.id WHERE e.id=$1 AND (($3 AND c.owner_id=$2) OR (NOT $3 AND e.user_id=$2))`

func loadEnrollment(ctx context.Context, q queryer, id, user int64, owner, locked bool) (*Enrollment, error) {
	sqlText := enrollmentSQL
	if locked {
		sqlText += " FOR UPDATE OF e"
	}
	var e Enrollment
	var raw []byte
	err := q.QueryRowContext(ctx, sqlText, id, user, owner).Scan(&e.ID, &e.CourseID, &e.UserID, &e.VersionID, &e.VersionNumber, &e.AccessStatus, &e.EnrolledAt, &e.StartedAt, &e.ExpiresAt, &e.CompletedAt, &e.LastActivityAt, &e.LastLessonKey, &e.Title, &e.Cover, &e.Author, &e.UserName, &e.Avatar, &raw, &e.Tests, &e.ProgramID, &e.Unavailable)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &e.Program); err != nil {
		return nil, err
	}
	e.EffectiveStatus = e.AccessStatus
	if e.Unavailable {
		e.EffectiveStatus = "suspended"
	} else if e.AccessStatus == "active" && e.ExpiresAt != nil && !e.ExpiresAt.After(time.Now()) {
		e.EffectiveStatus = "expired"
	}
	return &e, nil
}
func (h *Handler) progress(ctx context.Context, e *Enrollment) error {
	return readProgress(ctx, h.db, e)
}

func event(ctx context.Context, tx *sql.Tx, e *Enrollment, actor int64, typ, key, lesson string, data any) error {
	if key == "" {
		key = Key()
	}
	raw, _ := json.Marshal(data)
	_, err := tx.ExecContext(ctx, `INSERT INTO academy_learning_events(enrollment_id,actor_id,type,event_key,lesson_key,data) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(enrollment_id,event_key) DO NOTHING`, e.ID, actor, typ, key, lesson, string(raw))
	return err
}
func (h *Handler) recordProgress(ctx context.Context, id, user int64, key, operation, eventKey string, ack bool) (*Enrollment, error) {
	// Client request keys must not collide with server-owned lifecycle events.
	if operation == "open" {
		eventKey = "open:" + eventKey
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	e, err := loadEnrollment(ctx, tx, id, user, false, true)
	if err != nil {
		return nil, err
	}
	if e.EffectiveStatus != "active" {
		return nil, ErrAccess
	}
	lesson, ok := findLesson(e.Program, key)
	if !ok {
		return nil, sql.ErrNoRows
	}
	if operation == "open" {
		var seen bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_learning_events WHERE enrollment_id=$1 AND event_key=$2)`, id, eventKey).Scan(&seen); err != nil {
			return nil, err
		}
		if seen {
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			err = h.progress(ctx, e)
			return e, err
		}
	}
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT status FROM academy_lesson_progress WHERE enrollment_id=$1 AND lesson_key=$2`, id, key).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if operation == "complete" {
		states, err := assessmentOutcomes(ctx, tx, e)
		if err != nil {
			return nil, err
		}
		if reason := requirementsReason(lesson, states); reason != "" {
			return nil, inputError{reason}
		}
		if previous != "completed" && lesson.CompletionRule == "materials" && !ack {
			return nil, inputError{"Подтвердите, что вы изучили материалы урока"}
		}
		if previous == "" {
			return nil, inputError{"Сначала откройте урок"}
		}
		if previous != "completed" {
			_, err = tx.ExecContext(ctx, `UPDATE academy_lesson_progress SET status='completed',completed_at=NOW(),materials_acknowledged_at=CASE WHEN $3 THEN NOW() ELSE materials_acknowledged_at END WHERE enrollment_id=$1 AND lesson_key=$2`, id, key, ack)
			if err != nil {
				return nil, err
			}
			if err = event(ctx, tx, e, user, "lesson_completed", "complete:"+key, key, map[string]any{"rule": lesson.CompletionRule, "materials_acknowledged": ack}); err != nil {
				return nil, err
			}
		}
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO academy_lesson_progress(enrollment_id,program_version_id,lesson_key,status) VALUES($1,$2,$3,'in_progress') ON CONFLICT(enrollment_id,lesson_key) DO UPDATE SET last_opened_at=NOW()`, id, e.VersionID, key)
		if err != nil {
			return nil, err
		}
		typ := "lesson_opened"
		if previous != "" {
			typ = "lesson_reopened"
		}
		if err = event(ctx, tx, e, user, typ, eventKey, key, map[string]any{"from": e.LastLessonKey}); err != nil {
			return nil, err
		}
		if e.LastLessonKey != "" && e.LastLessonKey != key {
			if err = event(ctx, tx, e, user, "lesson_transition", eventKey+":transition", key, map[string]string{"from": e.LastLessonKey, "to": key}); err != nil {
				return nil, err
			}
		}
	}
	if e.StartedAt == nil {
		if err = event(ctx, tx, e, user, "course_started", "course_started", key, map[string]any{}); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE academy_enrollments SET started_at=COALESCE(started_at,NOW()),last_activity_at=NOW(),last_lesson_key=$2 WHERE id=$1`, id, key)
	if err != nil {
		return nil, err
	}
	if err = syncCourseTx(ctx, tx, e, user); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	e, err = loadEnrollment(ctx, h.db, id, user, false, false)
	if err != nil {
		return nil, err
	}
	err = h.progress(ctx, e)
	return e, err
}
func (h *Handler) version(ctx context.Context, course, version int64) (Program, error) {
	var raw []byte
	var p Program
	err := h.db.QueryRowContext(ctx, `SELECT v.content FROM academy_program_versions v JOIN academy_course_programs p ON p.id=v.program_id WHERE v.id=$1 AND p.course_id=$2`, version, course).Scan(&raw)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(raw, &p)
	return p, err
}
