package academy

import (
	"FinTalent/internal/courses"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) adminAction(w http.ResponseWriter, r *http.Request, p AdminPrincipal, path []string) {
	if r.Method != "POST" && r.Method != "PUT" && r.Method != "DELETE" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if r.Method == "DELETE" && !(len(path) == 2 && path[0] == "grants" || len(path) == 3 && path[0] == "cohorts" && path[2] == "staff") {
		fail(w, 405, "Удаление этого объекта недоступно")
		return
	}
	var cmd AdminCommand
	if !decode(w, r, &cmd) {
		return
	}
	if err := cmd.validate(); err != nil {
		analyticsProblem(w, err)
		return
	}
	entity, permission, action := "", "", ""
	id := int64(0)
	if len(path) > 1 {
		id, _ = strconv.ParseInt(path[1], 10, 64)
	}
	if len(path) > 2 {
		action = path[2]
	} else if len(path) == 2 && oneOf(path[0], "courses", "cohorts") {
		action = "edit"
	}
	validPath := len(path) == 1 && path[0] == "settings" || len(path) == 2 && oneOf(path[0], "courses", "cohorts", "grants") || len(path) == 3 && (path[0] == "courses" && oneOf(action, "program", "enroll", "restriction", "moderation") || path[0] == "enrollments" && oneOf(action, "access", "recalculate", "transfer", "curator") || path[0] == "cohorts" && action == "staff" || oneOf(path[0], "tests", "submissions") && action == "correct" || path[0] == "campaigns" && action == "cancel" || path[0] == "automations" && action == "pause" || path[0] == "authors" && action == "restriction")
	if !validPath {
		fail(w, 404, "Административное действие не найдено")
		return
	}
	switch path[0] {
	case "courses":
		entity, permission = "course", "edit"
		if action == "enroll" {
			permission = "transfer"
		}
		if action == "restriction" || action == "moderation" {
			permission = "moderate"
		}
		if action == "program" {
			entity = "program"
		}
	case "enrollments":
		entity = "enrollment"
		permission = map[string]string{"access": "access", "recalculate": "recalculate", "transfer": "transfer", "curator": "staff"}[action]
	case "cohorts":
		entity, permission = "cohort", "cohorts"
		if action == "staff" {
			permission = "staff"
		}
	case "tests", "submissions":
		entity, permission = "enrollment", "correct"
		action = "correct"
	case "campaigns":
		entity, permission, action = "campaign", "communications", "cancel"
	case "automations":
		entity, permission, action = "automation", "communications", "pause"
	case "grants":
		entity, permission, action = "grant", "grants", "grant"
	case "settings":
		entity, permission, action, id = "settings", "settings", "settings", 1
	case "authors":
		entity, permission, action = "author", "moderate", "restriction"
	}
	if permission == "" || !p.Can(permission) || id <= 0 {
		fail(w, 403, "Действие недоступно для ваших полномочий")
		return
	}
	m := &adminMutation{Principal: p, Entity: entity, Permission: permission, Action: action, ID: id, Reason: cmd.Reason, Expected: cmd.Expected, Key: cmd.Key, Fingerprint: adminHash(map[string]any{"path": r.URL.Path, "method": r.Method, "command": cmd})}
	var err error
	switch path[0] {
	case "courses":
		m.Course = id
	case "enrollments":
		m.Enrollment = id
		err = h.db.QueryRowContext(r.Context(), "SELECT course_id,user_id FROM academy_enrollments WHERE id=$1", id).Scan(&m.Course, &m.Student)
	case "cohorts":
		err = h.db.QueryRowContext(r.Context(), "SELECT course_id FROM academy_cohorts WHERE id=$1", id).Scan(&m.Course)
	case "tests", "submissions":
		table := "academy_test_attempts"
		if path[0] == "submissions" {
			table = "academy_assignment_submissions"
		}
		err = h.db.QueryRowContext(r.Context(), "SELECT e.id,e.course_id,e.user_id FROM "+table+" x JOIN academy_enrollments e ON e.id=x.enrollment_id WHERE x.id=$1", id).Scan(&m.Enrollment, &m.Course, &m.Student)
		m.ID = m.Enrollment
	case "campaigns", "automations":
		table := "academy_campaigns"
		if path[0] == "automations" {
			table = "academy_automations"
		}
		err = h.db.QueryRowContext(r.Context(), "SELECT course_id FROM "+table+" WHERE id=$1", id).Scan(&m.Course)
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if permission == "correct" && m.Student == p.User && p.User > 0 {
		fail(w, 403, "Нельзя корректировать собственные учебные результаты")
		return
	}
	var previous struct {
		Fingerprint, Actor string
		ID                 int64
	}
	err = h.db.QueryRowContext(r.Context(), "SELECT id,fingerprint,actor_key FROM academy_admin_audit WHERE request_key=$1", cmd.Key).Scan(&previous.ID, &previous.Fingerprint, &previous.Actor)
	if err == nil {
		if previous.Fingerprint != m.Fingerprint || previous.Actor != p.Key {
			fail(w, 409, "Ключ уже использован для другого действия")
			return
		}
		respond(w, 200, map[string]any{"replayed": true, "audit_id": previous.ID})
		return
	}
	if err != sql.ErrNoRows {
		analyticsProblem(w, err)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), adminMutationKey{}, m))
	capture := &adminStatusWriter{ResponseWriter: w, status: 200}
	w = capture
	defer func() {
		if capture.status >= 400 {
			copyM := *m
			copyM.Key = Key()
			copyM.Result = "failed"
			if len(copyM.Before) == 0 {
				copyM.Before = json.RawMessage(`{}`)
			}
			auditContext, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
			defer cancel()
			tx, e := h.db.BeginTx(auditContext, nil)
			if e == nil {
				defer tx.Rollback()
				if _, e = adminAudit(auditContext, tx, &copyM, map[string]any{"http_status": capture.status}); e == nil {
					_ = tx.Commit()
				}
			}
		}
	}()
	if path[0] == "courses" && action == "enroll" {
		var input struct {
			User    int64      `json:"user_id"`
			Cohort  int64      `json:"cohort_id"`
			Version int64      `json:"version_id"`
			Expires *time.Time `json:"expires_at"`
		}
		if json.Unmarshal(cmd.Data, &input) != nil {
			problem(w, ErrInvalid)
			return
		}
		r.Method = "POST"
		if input.Cohort > 0 {
			c, e := loadCohort(r.Context(), h.db, input.Cohort, m.Course, false)
			if e != nil {
				analyticsProblem(w, e)
				return
			}
			r.Body = jsonBody([]byte(jsonValue(map[string]any{"user_ids": []int64{input.User}, "request_key": cmd.Key, "expires_at": input.Expires})))
			h.cohortEnroll(w, r, c, p.User)
		} else {
			r.Body = jsonBody([]byte(jsonValue(map[string]any{"user_id": input.User, "version_id": input.Version, "request_key": cmd.Key, "expires_at": input.Expires})))
			h.students(w, r, m.Course, p.User, nil)
		}
		return
	}
	// Reuse the same domain handlers, validation, locks and transaction commit hook; actor remains the administrator.
	if path[0] == "enrollments" && action == "access" {
		r.Body = jsonBody(cmd.Data)
		h.students(w, r, m.Course, p.User, []string{strconv.FormatInt(id, 10)})
		return
	}
	if path[0] == "cohorts" && action != "staff" {
		c, e := loadCohort(r.Context(), h.db, id, m.Course, false)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		r.Body = jsonBody(cmd.Data)
		h.saveCohort(w, r, m.Course, p.User, c)
		return
	}
	if path[0] == "cohorts" && action == "staff" {
		c, e := loadCohort(r.Context(), h.db, id, m.Course, false)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		r.Body = jsonBody(cmd.Data)
		h.cohortStaff(w, r, c, p.User)
		return
	}
	if path[0] == "enrollments" && action == "curator" {
		r.Body = jsonBody(cmd.Data)
		h.crmRoute(w, r, m.Course, p.User, true, []string{strconv.FormatInt(id, 10), "curator"})
		return
	}
	if path[0] == "enrollments" && action == "transfer" {
		var input struct {
			Cohort  int64      `json:"cohort_id"`
			Expires *time.Time `json:"expires_at"`
		}
		if json.Unmarshal(cmd.Data, &input) != nil {
			problem(w, ErrInvalid)
			return
		}
		c, e := loadCohort(r.Context(), h.db, input.Cohort, m.Course, false)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		body := jsonValue(map[string]any{"user_ids": []int64{m.Student}, "expires_at": input.Expires, "request_key": cmd.Key, "transfer_enrollment_id": id})
		r.Body = jsonBody([]byte(body))
		h.cohortEnroll(w, r, c, p.User)
		return
	}
	tx, err := h.beginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	var result any = map[string]bool{"saved": true}
	switch path[0] {
	case "courses":
		if action == "moderation" {
			var input struct {
				Action string `json:"action"`
			}
			err = json.Unmarshal(cmd.Data, &input)
			if err == nil {
				_, err = courses.TransitionAdministrative(r.Context(), tx, id, input.Action)
			}
		} else if action == "restriction" {
			err = h.adminRestriction(r.Context(), tx, m, cmd.Data, false)
		} else if action == "program" {
			var program Program
			err = json.Unmarshal(cmd.Data, &program)
			if err == nil {
				err = Validate(&program)
			}
			if err == nil {
				err = h.adminSaveProgram(r.Context(), tx, m, &program)
			}
		} else {
			var existing json.RawMessage
			existing, err = courses.AdministrativeCourse(r.Context(), tx, id)
			var values map[string]json.RawMessage
			var patch map[string]json.RawMessage
			if err == nil {
				err = json.Unmarshal(existing, &values)
			}
			if err == nil {
				err = json.Unmarshal(cmd.Data, &patch)
			}
			for key, value := range patch {
				values[key] = value
			}
			merged, _ := json.Marshal(values)
			var input courses.Input
			if err == nil {
				err = json.Unmarshal(merged, &input)
			}
			if err == nil {
				err = courses.SaveAdministrative(r.Context(), tx, id, &input)
			}
			if errors.Is(err, courses.ErrChild) || errors.Is(err, courses.ErrCategory) || errors.Is(err, courses.ErrFeature) || errors.Is(err, courses.ErrLessonType) || errors.Is(err, courses.ErrCustomFeature) {
				err = inputError{err.Error()}
			}
		}
	case "authors":
		err = h.adminRestriction(r.Context(), tx, m, cmd.Data, true)
	case "enrollments":
		if action == "recalculate" {
			result, err = h.adminRecalculate(r.Context(), tx, m)
		} else {
			err = ErrInvalid
		}
	case "tests", "submissions":
		result, err = h.adminCorrection(r.Context(), tx, m, id, path[0], cmd.Data)
	case "campaigns":
		var changed sql.Result
		changed, err = tx.ExecContext(r.Context(), "UPDATE academy_campaigns SET status='cancelled',cancelled_at=now(),revision=revision+1,updated_at=now() WHERE id=$1 AND status IN('draft','scheduled','sending','partial','failed')", id)
		if err == nil {
			if n, _ := changed.RowsAffected(); n != 1 {
				err = ErrConflict
			}
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), "UPDATE academy_message_deliveries SET status='cancelled',error_code='administration_cancelled' WHERE campaign_id=$1 AND status='queued'", id)
		}
	case "automations":
		var changed sql.Result
		changed, err = tx.ExecContext(r.Context(), "UPDATE academy_automations SET status='paused',revision=revision+1,updated_at=now() WHERE id=$1 AND status<>'archived'", id)
		if err == nil {
			if n, _ := changed.RowsAffected(); n != 1 {
				err = ErrConflict
			}
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), "UPDATE academy_message_deliveries d SET status='cancelled',error_code='administration_paused' FROM academy_automation_runs run WHERE run.id=d.run_id AND run.automation_id=$1 AND d.status='queued'", id)
		}
	case "grants":
		var grant struct {
			Role     string `json:"role"`
			Contacts bool   `json:"view_contacts"`
			Answers  bool   `json:"view_answers"`
			Correct  bool   `json:"correct_results"`
		}
		err = json.Unmarshal(cmd.Data, &grant)
		if err == nil && (!oneOf(grant.Role, "academy_admin", "support", "moderator", "") || grant.Role == "" && r.Method != "DELETE") {
			err = ErrInvalid
		}
		if err == nil {
			if r.Method == "DELETE" {
				var result sql.Result
				result, err = tx.ExecContext(r.Context(), "DELETE FROM academy_admin_grants WHERE user_id=$1", id)
				if err == nil {
					if n, _ := result.RowsAffected(); n != 1 {
						err = sql.ErrNoRows
					}
				}
			} else {
				var result sql.Result
				result, err = tx.ExecContext(r.Context(), `INSERT INTO academy_admin_grants(user_id,role,view_contacts,view_answers,correct_results) SELECT id,$2,$3,$4,$5 FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system ON CONFLICT(user_id) DO UPDATE SET role=excluded.role,view_contacts=excluded.view_contacts,view_answers=excluded.view_answers,correct_results=excluded.correct_results,revision=academy_admin_grants.revision+1,updated_at=now()`, id, grant.Role, grant.Contacts, grant.Answers, grant.Correct)
				if err == nil {
					if n, _ := result.RowsAffected(); n != 1 {
						err = sql.ErrNoRows
					}
				}
			}
		}
	case "settings":
		var settings AdminSettings
		err = json.Unmarshal(cmd.Data, &settings)
		if err == nil {
			err = settings.validate()
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_admin_settings(id,settings) VALUES(1,$1) ON CONFLICT(id) DO UPDATE SET settings=excluded.settings,revision=academy_admin_settings.revision+1,updated_at=now()`, jsonValue(settings))
		}
	}
	var invalidType *json.UnmarshalTypeError
	var invalidSyntax *json.SyntaxError
	if errors.As(err, &invalidType) || errors.As(err, &invalidSyntax) {
		err = ErrInvalid
	}
	var transition courses.TransitionError
	if errors.As(err, &transition) {
		err = inputError{transition.Message}
	}
	if errors.Is(err, courses.ErrConflict) {
		err = ErrConflict
	}
	if err == nil {
		err = commitAcademy(r.Context(), tx)
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	respond(w, 200, result)
}
func (h *Handler) adminRecalculate(ctx context.Context, tx *sql.Tx, m *adminMutation) (any, error) {
	e, err := loadEnrollment(ctx, tx, m.Enrollment, m.Principal.User, true, true)
	if err != nil {
		return nil, err
	}
	if err = readProgress(ctx, tx, e); err != nil {
		return nil, err
	}
	before := e.Summary
	for _, module := range learningProgram(e.Program).Modules {
		for _, lesson := range module.Lessons {
			if err = reconcileAssessmentTx(ctx, tx, e, m.Principal.User, lesson.Key); err != nil {
				return nil, err
			}
		}
	}
	if err = syncCourseTx(ctx, tx, e, m.Principal.User); err != nil {
		return nil, err
	}
	if err = readProgress(ctx, tx, e); err != nil {
		return nil, err
	}
	err = event(ctx, tx, e, m.Principal.User, "administrative_recalculation", "admin:"+m.Key, "", map[string]any{"reason": m.Reason, "before": before, "after": e.Summary, "administrator": m.Principal.Name})
	return map[string]any{"before": before, "after": e.Summary}, err
}

type adminStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *adminStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
