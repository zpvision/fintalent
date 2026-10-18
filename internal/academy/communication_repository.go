package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func segmentSQL(s Segment, course, actor int64) (string, []any, error) {
	if len(s.Conditions) > 30 || len(s.Cohorts) > 100 || len(s.Enrollments) > 1000 || s.Join != "" && !oneOf(s.Join, "and", "or") {
		return "", nil, ErrInvalid
	}
	if s.Mode != "" && !oneOf(s.Mode, "all", "cohorts", "students", "segment") {
		return "", nil, ErrInvalid
	}
	args := []any{course, actor}
	where := `m.course_id=$1 AND academy_can_communicate(m.enrollment_id,$2)`
	if s.Mode == "students" && len(s.Enrollments) == 0 || s.Mode == "cohorts" && len(s.Cohorts) == 0 {
		where += " AND false"
	}
	bind := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if len(s.Cohorts) > 0 {
		for _, id := range s.Cohorts {
			if id <= 0 {
				return "", nil, ErrInvalid
			}
		}
		where += ` AND m.cohort_id IN(SELECT value::bigint FROM jsonb_array_elements_text(` + bind(jsonValue(s.Cohorts)) + `::jsonb))`
	}
	if len(s.Enrollments) > 0 {
		for _, id := range s.Enrollments {
			if id <= 0 {
				return "", nil, ErrInvalid
			}
		}
		where += ` AND m.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text(` + bind(jsonValue(s.Enrollments)) + `::jsonb))`
	}
	conditions := []string{}
	numeric := map[string]string{"progress": "m.percent", "activity_days": "EXTRACT(EPOCH FROM(now()-COALESCE(m.last_activity_at,m.enrolled_at)))/86400", "test_average": "m.test_average", "curator": "m.curator_id", "course": "m.course_id", "cohort": "m.cohort_id"}
	for _, c := range s.Conditions {
		if field, ok := numeric[c.Field]; ok {
			v, err := strconv.ParseFloat(c.Value, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1000000000000 {
				return "", nil, ErrInvalid
			}
			if oneOf(c.Field, "progress", "test_average") && v > 100 {
				return "", nil, ErrInvalid
			}
			op := map[string]string{"eq": "=", "ne": "<>", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}[c.Op]
			if op == "" {
				return "", nil, ErrInvalid
			}
			conditions = append(conditions, "("+field+")::double precision"+op+bind(v))
			continue
		}
		if c.Field == "tag" {
			v, err := strconv.ParseInt(c.Value, 10, 64)
			if err != nil || v <= 0 || !oneOf(c.Op, "eq", "ne") {
				return "", nil, ErrInvalid
			}
			prefix := ""
			if c.Op == "ne" {
				prefix = "NOT "
			}
			conditions = append(conditions, prefix+`EXISTS(SELECT 1 FROM academy_student_tag_assignments ta JOIN academy_student_tags t ON t.id=ta.tag_id WHERE ta.enrollment_id=m.enrollment_id AND t.course_id=m.course_id AND ta.tag_id=`+bind(v)+`)`)
			continue
		}
		if oneOf(c.Field, "pending_assignment", "revision", "pending_review") {
			if !oneOf(c.Op, "eq", "ne") || !oneOf(c.Value, "true", "false") {
				return "", nil, ErrInvalid
			}
			expr := "m.revision>0"
			if c.Field == "pending_review" {
				expr = "m.pending>0"
			}
			if c.Field == "pending_assignment" {
				expr = `EXISTS(SELECT 1 FROM academy_assessment_versions a WHERE a.program_version_id=m.program_version_id AND a.kind='assignment' AND NOT EXISTS(SELECT 1 FROM academy_effective_assignment_submissions sub WHERE sub.enrollment_id=m.enrollment_id AND sub.assessment_id=a.id AND sub.status='accepted'))`
			}
			negate := c.Value == "false"
			if c.Op == "ne" {
				negate = !negate
			}
			if negate {
				expr = "NOT (" + expr + ")"
			}
			conditions = append(conditions, "("+expr+")")
			continue
		}
		if oneOf(c.Field, "learning_status", "access_status") {
			if !oneOf(c.Op, "eq", "ne") {
				return "", nil, ErrInvalid
			}
			expr := "m.effective_status"
			if c.Field == "learning_status" {
				if !oneOf(c.Value, "not_started", "in_progress", "completed") {
					return "", nil, ErrInvalid
				}
				expr = `CASE WHEN m.completed_at IS NOT NULL THEN 'completed' WHEN m.started_at IS NOT NULL THEN 'in_progress' ELSE 'not_started' END`
			} else if !oneOf(c.Value, "active", "suspended", "revoked", "expired") {
				return "", nil, ErrInvalid
			}
			op := "="
			if c.Op == "ne" {
				op = "<>"
			}
			conditions = append(conditions, "("+expr+")"+op+bind(c.Value))
			continue
		}
		return "", nil, inputError{"Неподдерживаемое условие сегмента"}
	}
	if len(conditions) > 0 {
		join := " AND "
		if s.Join == "or" {
			join = " OR "
		}
		where += " AND (" + strings.Join(conditions, join) + ")"
	}
	return where, args, nil
}

type CommunicationRecipient struct {
	EmailService   bool   `json:"-"`
	EmailMarketing bool   `json:"-"`
	InApp          bool   `json:"-"`
	EnrollmentID   int64  `json:"enrollment_id"`
	UserID         int64  `json:"user_id"`
	Name           string `json:"name"`
	CohortID       *int64 `json:"cohort_id"`
	Percent        int    `json:"percent"`
	Status         string `json:"access_status"`
}

type communicationQuery interface {
	queryer
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func audience(ctx context.Context, q communicationQuery, s Segment, course, actor int64) ([]CommunicationRecipient, error) {
	where, args, err := segmentSQL(s, course, actor)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT ON(m.user_id) m.enrollment_id,m.user_id,m.user_name,m.cohort_id,m.percent,m.effective_status,COALESCE(pref.email_service,true),COALESCE(pref.email_marketing AND pref.consented_at IS NOT NULL,false),COALESCE(pref.in_app,true) FROM academy_student_metrics m LEFT JOIN academy_communication_preferences pref ON pref.user_id=m.user_id AND pref.course_id=m.course_id WHERE `+where+` ORDER BY m.user_id,m.enrolled_at DESC,m.enrollment_id DESC LIMIT 10001`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CommunicationRecipient{}
	for rows.Next() {
		var v CommunicationRecipient
		if err = rows.Scan(&v.EnrollmentID, &v.UserID, &v.Name, &v.CohortID, &v.Percent, &v.Status, &v.EmailService, &v.EmailMarketing, &v.InApp); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	if len(items) > 10000 {
		return nil, inputError{"Выберите сегмент не более 10 000 получателей"}
	}
	return items, rows.Err()
}
func matchesSegment(ctx context.Context, q queryer, s Segment, course, actor, eid int64) (bool, error) {
	where, args, err := segmentSQL(s, course, actor)
	if err != nil {
		return false, err
	}
	args = append(args, eid)
	var yes bool
	err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_student_metrics m WHERE `+where+fmt.Sprintf(` AND m.enrollment_id=$%d)`, len(args)), args...).Scan(&yes)
	return yes, err
}
func communicationAudit(ctx context.Context, tx *sql.Tx, course, actor int64, entity string, id int64, action string, data any) error {
	if entity == "report" && administrative(ctx) {
		m := &adminMutation{Principal: adminPrincipal(ctx), Entity: entity, ID: id, Course: course, Action: action, Reason: "Административный отчёт: " + action, Key: Key(), Fingerprint: adminHash(data), Before: json.RawMessage(`{}`)}
		if _, err := adminAudit(ctx, tx, m, data); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO academy_communication_audit(course_id,actor_id,entity,entity_id,action,data) VALUES(NULLIF($1,0),NULLIF($2,0),$3,NULLIF($4,0),$5,$6)`, course, actor, entity, id, action, jsonValue(data))
	return err
}
func loadCampaign(ctx context.Context, q queryer, course, id, actor int64, lock bool) (*Campaign, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT to_jsonb(c) FROM academy_campaigns c WHERE course_id=$1 AND id=$2 AND deleted_at IS NULL AND(created_by=$3 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$3))`+suffix, course, id, actor).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var c Campaign
	err = json.Unmarshal(raw, &c)
	return &c, err
}
func loadAutomation(ctx context.Context, q queryer, course, id, actor int64, lock bool) (*Automation, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT to_jsonb(a) FROM academy_automations a WHERE course_id=$1 AND id=$2 AND(created_by=$3 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$3))`+suffix, course, id, actor).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var a Automation
	err = json.Unmarshal(raw, &a)
	return &a, err
}
func preferenceAllowed(ctx context.Context, q queryer, user, course int64, channel, category string) (bool, error) {
	var yes bool
	err := q.QueryRowContext(ctx, `SELECT CASE WHEN $3='in_app' THEN COALESCE((SELECT in_app FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2),true) AND ($4<>'marketing' OR COALESCE((SELECT email_marketing AND consented_at IS NOT NULL FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2),false)) WHEN $4='marketing' THEN COALESCE((SELECT email_marketing AND consented_at IS NOT NULL FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2),false) ELSE COALESCE((SELECT email_service FROM academy_communication_preferences WHERE user_id=$1 AND course_id=$2),true) END`, user, course, channel, category).Scan(&yes)
	return yes, err
}
func communicationVariablesFor(ctx context.Context, q queryer, e *Enrollment, base string, eventData map[string]any) (map[string]string, error) {
	v := sampleVariables(base, e.UserName)
	v["course.title"] = e.Title
	v["teacher.name"] = e.Author
	v["curator.name"] = e.CuratorName
	if e.CuratorName == "" {
		v["curator.name"] = "преподаватель курса"
	}
	v["progress.percent"] = strconv.Itoa(e.Summary.Percent)
	v["course.url"] = fmt.Sprintf("%s/profile/learning/%d", base, e.ID)
	v["lesson.url"] = v["course.url"]
	if e.LastLessonKey != "" {
		v["lesson.url"] += "?lesson=" + urlQuery(e.LastLessonKey)
	}
	if e.Cohort != nil {
		v["cohort.name"] = e.Cohort.Name
		v["cohort.starts_at"] = e.Cohort.StartsAt.In(mustLocation(e.Cohort.Timezone)).Format("02.01.2006 в 15:04 MST")
		var nearest *time.Time
		targetKey, _ := eventData["block_key"].(string)
		for _, r := range e.Cohort.Deadlines {
			if targetKey != "" && r.Key != targetKey {
				continue
			}
			at, err := ruleTime(e.Cohort, r)
			if err == nil && (targetKey != "" || at.After(time.Now())) && (nearest == nil || at.Before(*nearest)) {
				nearest = &at
				v["deadline"] = at.In(mustLocation(e.Cohort.Timezone)).Format("02.01.2006 в 15:04 MST")
				for _, m := range e.Program.Modules {
					for _, l := range m.Lessons {
						for _, b := range l.Blocks {
							if b.Key == r.Key {
								v["assignment.title"] = b.Data.Title
							}
						}
					}
				}
			}
		}
	} else {
		v["cohort.name"] = "Самостоятельное обучение"
	}
	if id, ok := eventData["assessment_id"].(float64); ok {
		var title string
		if err := q.QueryRowContext(ctx, `SELECT title FROM academy_assessment_versions WHERE id=$1 AND program_version_id=$2`, int64(id), e.VersionID).Scan(&title); err == nil {
			v["assignment.title"] = title
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}
	return v, nil
}
func urlQuery(s string) string { return strings.ReplaceAll(s, " ", "%20") }

func recipientEligible(p CommunicationRecipient, channel, category string) bool {
	if p.Status != "active" {
		return false
	}
	if channel == "in_app" {
		return p.InApp && (category != "marketing" || p.EmailMarketing)
	}
	if category == "marketing" {
		return p.EmailMarketing
	}
	return p.EmailService
}
