package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (h *Handler) emailTemplates(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 0 && r.Method == "GET" {
		var raw json.RawMessage
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.updated_at DESC,t.id DESC),'[]') FROM academy_email_templates t WHERE course_id=$1 AND deleted_at IS NULL AND(created_by=$2 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$2))`, course, user).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]any{"builtin": builtinTemplates(), "custom": raw, "variables": communicationVariables})
		return
	}
	var b struct {
		Name     string  `json:"name"`
		Message  Message `json:"message"`
		Revision int64   `json:"revision"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if r.Method != "DELETE" {
		if b.Name == "" || utf8.RuneCountInString(b.Name) > 180 {
			problem(w, ErrInvalid)
			return
		}
		if err := ValidateMessage(&b.Message, h.base); err != nil {
			problem(w, err)
			return
		}
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	id := int64(0)
	action := "created"
	if len(path) == 0 && r.Method == "POST" {
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_email_templates(course_id,created_by,name,message) VALUES($1,$2,$3,$4) RETURNING id`, course, user, b.Name, jsonValue(b.Message)).Scan(&id)
	} else if len(path) == 1 && oneOf(r.Method, "PUT", "DELETE") {
		id, _ = strconv.ParseInt(path[0], 10, 64)
		var rev int64
		err = tx.QueryRowContext(r.Context(), `SELECT revision FROM academy_email_templates WHERE id=$1 AND course_id=$2 AND deleted_at IS NULL AND(created_by=$3 OR EXISTS(SELECT 1 FROM courses WHERE id=$2 AND owner_id=$3)) FOR UPDATE`, id, course, user).Scan(&rev)
		if err == nil && rev != b.Revision {
			err = ErrConflict
		}
		action = "updated"
		if r.Method == "DELETE" {
			action = "deleted"
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_email_templates SET name=CASE WHEN $2 THEN name ELSE $3 END,message=CASE WHEN $2 THEN message ELSE $4::jsonb END,deleted_at=CASE WHEN $2 THEN now() END,revision=revision+1,updated_at=now() WHERE id=$1`, id, r.Method == "DELETE", b.Name, jsonValue(b.Message))
		}
	} else {
		err = ErrInvalid
	}
	if err == nil {
		err = communicationAudit(r.Context(), tx, course, user, "template", id, action, nil)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"id": id})
}

var automationTriggers = []string{"enrolled", "cohort_started", "module_unlocked", "lesson_unlocked", "lesson_opened", "lesson_completed", "course_completed", "test_passed", "test_failed", "assignment_submitted", "assignment_reviewed", "assignment_revision_requested", "deadline_approaching", "deadline_missed", "inactive", "curator_assigned"}

func validateAutomation(a *Automation, base string) error {
	a.Name = strings.TrimSpace(a.Name)
	valid := false
	for _, trigger := range automationTriggers {
		if a.Trigger == trigger {
			valid = true
		}
	}
	if !valid || utf8.RuneCountInString(a.Name) < 1 || utf8.RuneCountInString(a.Name) > 180 || len(a.Config.Steps) < 1 || len(a.Config.Steps) > 8 || a.Config.InactiveDays < 1 || a.Config.InactiveDays > 365 || a.Config.LeadDays < 0 || a.Config.LeadDays > 365 {
		return inputError{"Проверьте название, событие и от 1 до 8 действий сценария"}
	}
	if _, _, err := segmentSQL(a.Config.Audience, a.CourseID, a.Creator); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i := range a.Config.Steps {
		s := &a.Config.Steps[i]
		if seen[s.Key] {
			return ErrInvalid
		}
		seen[s.Key] = true
		if s.Timing == "before_deadline" && !oneOf(a.Trigger, "deadline_approaching", "deadline_missed") {
			return inputError{"Отсчёт до дедлайна применяется к событиям сроков заданий"}
		}
		if err := validateCommunicationStep(s, base); err != nil {
			return err
		}
	}
	return nil
}
func (h *Handler) automations(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 0 && r.Method == "GET" {
		before := queryInt(r, "before")
		if before == 0 {
			before = 9223372036854775807
		}
		var raw json.RawMessage
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT a.*,(SELECT count(*) FROM academy_automation_runs r WHERE r.automation_id=a.id AND r.skipped_reason='') runs,(SELECT count(*) FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=a.id AND d.status IN('sent','delivered')) successful,(SELECT count(*) FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=a.id AND d.status='failed') failed,(SELECT count(*) FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=a.id AND d.status='queued') queued,(SELECT max(COALESCE(d.sent_at,d.claimed_at)) FROM academy_message_deliveries d JOIN academy_automation_runs r ON r.id=d.run_id WHERE r.automation_id=a.id) last_execution FROM academy_automations a WHERE a.course_id=$1 AND(a.created_by=$2 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$2)) AND a.id<$3 AND($4='' OR a.status=$4) ORDER BY a.id DESC LIMIT 100)x`, course, user, before, r.URL.Query().Get("status")).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]any{"items": raw, "triggers": automationTriggers})
		return
	}
	id := int64(0)
	if len(path) > 0 {
		id, _ = strconv.ParseInt(path[0], 10, 64)
	}
	if len(path) == 2 && path[1] == "runs" && r.Method == "GET" {
		if _, err := loadAutomation(r.Context(), h.db, course, id, user, false); err != nil {
			problem(w, err)
			return
		}
		before := queryInt(r, "before")
		if before == 0 {
			before = 9223372036854775807
		}
		var raw json.RawMessage
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT r.*,u.full_name user_name FROM academy_automation_runs r JOIN academy_enrollments e ON e.id=r.enrollment_id JOIN users u ON u.id=e.user_id WHERE r.automation_id=$1 AND academy_can_communicate(e.id,$2) AND r.id<$3 ORDER BY r.id DESC LIMIT 100)x`, id, user, before).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if len(path) == 2 && path[1] == "history" && r.Method == "GET" {
		if _, err := loadAutomation(r.Context(), h.db, course, id, user, false); err != nil {
			problem(w, err)
			return
		}
		h.communicationHistory(w, r, course, user, nil)
		return
	}
	if len(path) == 1 && r.Method == "GET" {
		a, err := loadAutomation(r.Context(), h.db, course, id, user, false)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, a)
		return
	}
	if len(path) > 1 || !oneOf(r.Method, "PUT", "POST") {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var a Automation
	if !decode(w, r, &a) {
		return
	}
	a.CourseID = course
	a.Creator = user
	if err := validateAutomation(&a, h.base); err != nil {
		problem(w, err)
		return
	}
	if !oneOf(a.Status, "draft", "active", "paused", "archived") {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	action := "created"
	if len(path) == 0 && r.Method == "POST" {
		if !keyPattern.MatchString(a.RequestKey) {
			problem(w, ErrInvalid)
			return
		}
		a.Status = "draft"
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_automations(course_id,created_by,name,trigger,config,request_key) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(request_key) DO NOTHING RETURNING id`, course, user, a.Name, a.Trigger, jsonValue(a.Config), a.RequestKey).Scan(&a.ID)
		if err == sql.ErrNoRows {
			err = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_automations WHERE course_id=$1 AND created_by=$2 AND request_key=$3`, course, user, a.RequestKey).Scan(&a.ID)
		}
	} else if len(path) == 1 && r.Method == "PUT" {
		old, er := loadAutomation(r.Context(), tx, course, id, user, true)
		if er != nil {
			problem(w, er)
			return
		}
		if old.Revision != a.Revision {
			problem(w, ErrConflict)
			return
		}
		if old.Status == "archived" {
			fail(w, 409, "Архивный сценарий не изменяется")
			return
		}
		changed := old.Trigger != a.Trigger || jsonValue(old.Config) != jsonValue(a.Config)
		if changed && old.Status == "active" {
			fail(w, 409, "Сначала приостановите сценарий, затем измените условия или действия")
			return
		}
		if (old.Status == "draft" && a.Status == "paused") || (oneOf(old.Status, "active", "paused") && a.Status == "draft") {
			problem(w, ErrInvalid)
			return
		}
		generation := old.Generation
		if changed {
			generation++
		}
		var activeSince *time.Time = old.ActiveSince
		if changed {
			activeSince = nil
		}
		if a.Status == "active" && (activeSince == nil || changed) {
			now := time.Now()
			activeSince = &now
		}
		a.ID = id
		_, err = tx.ExecContext(r.Context(), `UPDATE academy_automations SET name=$2,trigger=$3,config=$4,status=$5,revision=revision+1,generation=$6,active_since=$7,updated_at=now() WHERE id=$1`, id, a.Name, a.Trigger, jsonValue(a.Config), a.Status, generation, activeSince)
		if err == nil && (changed || a.Status == "archived") {
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_message_deliveries d SET status='cancelled',error_code='automation_changed' FROM academy_automation_runs r WHERE r.id=d.run_id AND r.automation_id=$1 AND d.status='queued'`, id)
		}
		action = "updated_" + a.Status
	} else {
		err = ErrInvalid
	}
	if err == nil {
		err = communicationAudit(r.Context(), tx, course, user, "automation", a.ID, action, map[string]any{"status": a.Status})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	saved, err := loadAutomation(r.Context(), h.db, course, a.ID, user, false)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, saved)
}
func (h *Handler) communicationHistory(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 2 && path[1] == "retry" && r.Method == "POST" {
		id, _ := strconv.ParseInt(path[0], 10, 64)
		var b struct {
			ConfirmUncertain bool `json:"confirm_uncertain"`
		}
		if !decode(w, r, &b) {
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			problem(w, err)
			return
		}
		defer tx.Rollback()
		var uncertain bool
		var recipient int64
		err = tx.QueryRowContext(r.Context(), `SELECT uncertain,recipient_id FROM academy_message_deliveries d WHERE id=$1 AND course_id=$2 AND status='failed' AND(actor_id=$3 OR EXISTS(SELECT 1 FROM courses WHERE id=$2 AND owner_id=$3)) FOR UPDATE`, id, course, user).Scan(&uncertain, &recipient)
		if err != nil {
			problem(w, err)
			return
		}
		if uncertain && !b.ConfirmUncertain {
			fail(w, 409, "Результат SMTP неизвестен: письмо могло быть принято. Подтвердите риск повторной отправки")
			return
		}
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `UPDATE academy_message_deliveries SET status='queued',next_attempt_at=now(),uncertain=false,error_code='',lease_key=NULL WHERE id=$1 AND attempts<10`, id)
		if err == nil {
			if n, _ := result.RowsAffected(); n == 0 {
				fail(w, 409, "Достигнут предел повторных отправок")
				return
			}
			err = communicationAudit(r.Context(), tx, course, user, "delivery", id, "retry", map[string]bool{"confirmed_uncertain": b.ConfirmUncertain})
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"queued": true})
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	before := queryInt(r, "before")
	if before == 0 {
		before = 9223372036854775807
	}
	campaign := queryInt(r, "campaign")
	automation := queryInt(r, "automation")
	if parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/"); len(parts) > 2 && parts[len(parts)-1] == "history" {
		automation, _ = strconv.ParseInt(parts[len(parts)-2], 10, 64)
	}
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT d.id,d.campaign_id,d.run_id,d.enrollment_id,d.recipient_id,u.full_name recipient_name,d.channel,d.category,d.status,d.attempts,d.queued_at,d.sent_at,d.next_attempt_at,d.uncertain,d.error_code,d.message_id,d.provider_id,d.test,d.message->>'subject' subject,r.event_type,r.event_at,r.conditions,r.source_key,a.name automation_name,c.name campaign_name,(SELECT COALESCE(jsonb_agg(jsonb_build_object('number',t.number,'started_at',t.started_at,'finished_at',t.finished_at,'outcome',t.outcome,'error_code',t.error_code) ORDER BY t.number),'[]') FROM academy_delivery_attempts t WHERE t.delivery_id=d.id) history FROM academy_message_deliveries d JOIN users u ON u.id=d.recipient_id LEFT JOIN academy_campaigns c ON c.id=d.campaign_id LEFT JOIN academy_automation_runs r ON r.id=d.run_id LEFT JOIN academy_automations a ON a.id=r.automation_id WHERE d.course_id=$1 AND(d.actor_id=$2 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$2)) AND(d.enrollment_id IS NULL OR academy_can_communicate(d.enrollment_id,$2)) AND d.id<$3 AND($4=0 OR d.campaign_id=$4) AND($5=0 OR r.automation_id=$5) ORDER BY d.id DESC LIMIT 100)x`, course, user, before, campaign, automation).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
