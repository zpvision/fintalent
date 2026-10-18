package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func loadCohort(ctx context.Context, q queryer, id, course int64, lock bool) (*Cohort, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF c"
	}
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT to_jsonb(c)||jsonb_build_object('version_number',v.number) FROM academy_cohorts c JOIN academy_program_versions v ON v.id=c.program_version_id WHERE c.id=$1 AND c.course_id=$2`+suffix, id, course).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var c Cohort
	err = json.Unmarshal(raw, &c)
	return &c, err
}
func (e *Enrollment) lessonAvailability(key string) LessonAvailability {
	for _, m := range e.Program.Modules {
		for _, l := range m.Lessons {
			if l.Key == key && l.Status == "published" && m.Status == "published" {
				return availability(e.Cohort, m.Key, key, time.Now())
			}
		}
	}
	return LessonAvailability{Reason: "Урок не найден"}
}
func (e *Enrollment) checkLesson(key string) error {
	if _, ok := findLesson(e.Program, key); !ok {
		return sql.ErrNoRows
	}
	a := e.lessonAvailability(key)
	if !a.Available {
		return inputAccess{a.Reason}
	}
	return nil
}

type inputAccess struct{ message string }

func (e inputAccess) Error() string        { return e.message }
func (e inputAccess) Is(target error) bool { return target == ErrAccess }
func normalizeCohortProgram(e *Enrollment) {
	if e.Cohort != nil {
		for i := range e.Program.Modules {
			for j := range e.Program.Modules[i].Lessons {
				e.Program.Modules[i].Lessons[j].SelfPaced = true
			}
		}
	}
}

func cohortEvent(ctx context.Context, tx *sql.Tx, course int64, cohort, enrollment *int64, actor int64, typ, key string, data any, recipient *int64, due *time.Time) error {
	if key == "" {
		key = Key()
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO academy_cohort_events(course_id,cohort_id,enrollment_id,actor_id,type,event_key,data,recipient_id,notify_at) VALUES($1,$2,$3,NULLIF($4,0),$5,$6,$7,$8,$9) ON CONFLICT(event_key) DO UPDATE SET cancelled=false WHERE academy_cohort_events.delivered_at IS NULL`, course, cohort, enrollment, actor, typ, key, jsonValue(data), recipient, due)
	return err
}

// Reconcile future notifications transactionally whenever schedule/enrollments change.
// A rescheduled event has a distinct key; stale pending events are cancelled, never sent.
func scheduleCohortEvents(ctx context.Context, tx *sql.Tx, c *Cohort, eid, student, actor int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE academy_cohort_events SET cancelled=true WHERE enrollment_id=$1 AND delivered_at IS NULL AND type IN('lesson_unlocked','deadline_approaching','deadline_missed')`, eid); err != nil {
		return err
	}
	if !oneOf(c.Status, "active", "completed") {
		return nil
	}
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT content FROM academy_program_versions WHERE id=$1`, c.VersionID).Scan(&raw); err != nil {
		return err
	}
	var p Program
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	for _, m := range p.Modules {
		if m.Status != "published" {
			continue
		}
		for _, l := range m.Lessons {
			if l.Status != "published" {
				continue
			}
			a := availability(c, m.Key, l.Key, time.Now())
			if a.OpensAt != nil && a.OpensAt.After(time.Now()) {
				key := "unlock:" + jsonValue(eid) + ":" + l.Key + ":" + a.OpensAt.Format(time.RFC3339)
				if err := cohortEvent(ctx, tx, c.CourseID, &c.ID, &eid, actor, "lesson_unlocked", key, map[string]any{"title": l.Title, "lesson_key": l.Key}, &student, a.OpensAt); err != nil {
					return err
				}
			}
		}
	}
	for _, r := range c.Deadlines {
		at, err := ruleTime(c, r)
		if err != nil {
			return err
		}
		for _, typ := range []string{"deadline_approaching", "deadline_missed"} {
			due := at
			if typ == "deadline_approaching" {
				due = at.Add(-24 * time.Hour)
			}
			if !due.After(time.Now()) {
				continue
			}
			key := typ + ":" + jsonValue(eid) + ":" + r.Key + ":" + at.Format(time.RFC3339)
			if err := cohortEvent(ctx, tx, c.CourseID, &c.ID, &eid, actor, typ, key, map[string]any{"block_key": r.Key, "deadline_at": at}, &student, &due); err != nil {
				return err
			}
		}
	}
	return nil
}

// Called by the application's existing bounded, cancellable worker lifecycle.
// SKIP LOCKED and a shared transaction prevent duplicate notifications across instances.
func DeliverCohortNotifications(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT x.id,x.recipient_id,x.enrollment_id,x.type,x.data FROM academy_cohort_events x JOIN academy_enrollments e ON e.id=x.enrollment_id JOIN users u ON u.id=x.recipient_id JOIN courses c ON c.id=e.course_id JOIN users owner ON owner.id=c.owner_id WHERE x.recipient_id IS NOT NULL AND x.delivered_at IS NULL AND NOT x.cancelled AND x.notify_at<=now() AND ((e.access_status='active' AND (e.expires_at IS NULL OR e.expires_at>now())) OR x.type IN('access_suspended','access_revoked','student_transferred')) AND NOT u.is_blocked AND NOT owner.is_blocked ORDER BY x.notify_at,x.id LIMIT 100 FOR UPDATE OF x SKIP LOCKED`)
	if err != nil {
		return err
	}
	type pending struct {
		id, user, eid int64
		typ           string
		data          json.RawMessage
	}
	items := []pending{}
	for rows.Next() {
		var v pending
		if err = rows.Scan(&v.id, &v.user, &v.eid, &v.typ, &v.data); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	titles := map[string]string{"student_enrolled": "Вы зачислены в поток", "student_transferred": "Вы переведены в другой поток", "curator_assigned": "Вам назначен куратор", "lesson_unlocked": "Открыт новый урок", "deadline_approaching": "Приближается срок задания", "deadline_missed": "Срок задания наступил", "access_extended": "Доступ к обучению продлён", "access_restored": "Доступ к обучению восстановлен", "access_revoked": "Доступ к обучению отозван", "access_suspended": "Доступ к обучению приостановлен"}
	for _, v := range items {
		skip := false
		if oneOf(v.typ, "deadline_approaching", "deadline_missed") {
			var d struct {
				Key string `json:"block_key"`
			}
			_ = json.Unmarshal(v.data, &d)
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_assignment_submissions s JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.enrollment_id=$1 AND a.block_key=$2 AND s.submitted_at IS NOT NULL AND s.status IN('submitted','in_review','accepted'))`, v.eid, d.Key).Scan(&skip)
			if err != nil {
				return err
			}
		}
		if !skip {
			title := titles[v.typ]
			if title == "" {
				title = "Обновление обучения"
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) VALUES($1,$2,$3,$4,'academy_enrollment',$5)`, v.user, "academy_"+v.typ, title, "Откройте учебную комнату, чтобы посмотреть подробности", v.eid)
			if err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE academy_cohort_events SET delivered_at=now() WHERE id=$1`, v.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
