package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

func (h *Handler) communicationPreferences(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/academy/unsubscribe/") {
		token := strings.TrimPrefix(r.URL.Path, "/api/academy/unsubscribe/")
		if !keyPattern.MatchString(token) {
			problem(w, sql.ErrNoRows)
			return
		}
		var course, user int64
		var title string
		err := h.db.QueryRowContext(r.Context(), `SELECT p.course_id,p.user_id,c.title FROM academy_communication_preferences p JOIN courses c ON c.id=p.course_id WHERE unsubscribe_token=$1`, token).Scan(&course, &user, &title)
		if err != nil {
			problem(w, err)
			return
		}
		if r.Method == "GET" {
			respond(w, 200, map[string]any{"course_title": title, "message": "Отключить необязательные email этого курса? Системные письма входа и безопасности продолжат приходить."})
			return
		}
		if r.Method != "POST" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			problem(w, err)
			return
		}
		defer tx.Rollback()
		_, err = tx.ExecContext(r.Context(), `UPDATE academy_communication_preferences SET email_service=false,email_marketing=false,updated_at=now() WHERE user_id=$1 AND course_id=$2`, user, course)
		if err == nil {
			err = communicationAudit(r.Context(), tx, course, user, "preferences", 0, "unsubscribed", nil)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"unsubscribed": true})
		return
	}
	user, err := h.user(r)
	if err != nil || user <= 0 {
		fail(w, 401, "Войдите в аккаунт")
		return
	}
	var active bool
	err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system)`, user).Scan(&active)
	if err != nil {
		problem(w, err)
		return
	}
	if !active {
		fail(w, 403, "Аккаунт недоступен")
		return
	}
	if r.Method == "GET" {
		var raw json.RawMessage
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('course_id',c.id,'title',c.title,'email_service',COALESCE(p.email_service,true),'email_marketing',COALESCE(p.email_marketing,false),'in_app',COALESCE(p.in_app,true),'consented_at',p.consented_at) ORDER BY c.title,c.id),'[]') FROM courses c LEFT JOIN academy_communication_preferences p ON p.course_id=c.id AND p.user_id=$1 WHERE c.owner_id=$1 OR EXISTS(SELECT 1 FROM academy_enrollments e WHERE e.course_id=c.id AND e.user_id=$1) OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$1)`, user).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if r.Method != "PUT" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var b struct {
		Course    int64 `json:"course_id"`
		Service   bool  `json:"email_service"`
		Marketing bool  `json:"email_marketing"`
		InApp     bool  `json:"in_app"`
		Consent   bool  `json:"confirm_marketing_consent"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Marketing && !b.Consent {
		fail(w, 400, "Подтвердите согласие на необязательные рекламные письма этого курса")
		return
	}
	var allowed bool
	err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses c WHERE c.id=$1 AND(c.owner_id=$2 OR EXISTS(SELECT 1 FROM academy_enrollments e WHERE e.course_id=c.id AND e.user_id=$2) OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$2)))`, b.Course, user).Scan(&allowed)
	if err != nil {
		problem(w, err)
		return
	}
	if !allowed {
		problem(w, sql.ErrNoRows)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_communication_preferences(user_id,course_id,email_service,email_marketing,in_app,consented_at,unsubscribe_token) VALUES($1,$2,$3,$4,$5,CASE WHEN $4 THEN now() END,$6) ON CONFLICT(user_id,course_id) DO UPDATE SET email_service=excluded.email_service,email_marketing=excluded.email_marketing,in_app=excluded.in_app,consented_at=CASE WHEN excluded.email_marketing THEN COALESCE(academy_communication_preferences.consented_at,now()) ELSE academy_communication_preferences.consented_at END,updated_at=now()`, user, b.Course, b.Service, b.Marketing, b.InApp, Key())
	if err == nil {
		err = communicationAudit(r.Context(), tx, b.Course, user, "preferences", 0, "updated", map[string]bool{"email_service": b.Service, "email_marketing": b.Marketing, "in_app": b.InApp, "explicit_consent": b.Consent})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]bool{"saved": true})
}
