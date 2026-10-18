package academy

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (h *Handler) teaching(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	switch path[0] {
	case "user-search":
		if r.Method != "GET" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len([]rune(q)) < 3 || len(q) > 160 {
			respond(w, 200, []any{})
			return
		}
		var raw []byte
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',u.id,'name',u.full_name,'avatar',COALESCE(u.avatar_url,'')) ORDER BY u.full_name,u.id),'[]') FROM (SELECT u.id,u.full_name,u.avatar_url FROM users u WHERE NOT u.is_blocked AND NOT u.is_system AND ((position('@' in $1)>0 AND lower(u.email)=lower($1)) OR (position('@' in $1)=0 AND position(lower($1) in lower(u.full_name))>0 AND EXISTS(SELECT 1 FROM resumes p WHERE p.user_id=u.id AND p.status='published' AND p.visibility='public' AND p.deleted_at IS NULL))) ORDER BY u.full_name,u.id LIMIT 8) u`, q).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, json.RawMessage(raw))
	case "releases":
		h.releases(w, r, course, user, path[1:])
	case "students":
		h.students(w, r, course, user, path[1:])
	}
}
func (h *Handler) releases(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 2 && path[1] == "assessments" && r.Method == "POST" {
		version, _ := strconv.ParseInt(path[0], 10, 64)
		p, err := h.version(r.Context(), course, version)
		if err != nil {
			problem(w, err)
			return
		}
		tx, err := h.beginTx(r.Context(), nil)
		if err != nil {
			problem(w, err)
			return
		}
		defer tx.Rollback()
		var locked int64
		if err = tx.QueryRowContext(r.Context(), `SELECT id FROM courses WHERE id=$1 AND(owner_id=$2 OR $3) FOR UPDATE`, course, user, adminPrincipal(r.Context()).Can("cohorts")).Scan(&locked); err == nil {
			err = freezeAssessments(r.Context(), tx, version, p, user)
		}
		if err == nil {
			err = commitAcademy(r.Context(), tx)
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"ready": true})
		return
	}
	if len(path) > 1 && path[1] == "preview" && r.Method == "GET" {
		version, _ := strconv.ParseInt(path[0], 10, 64)
		p, err := h.version(r.Context(), course, version)
		if err != nil {
			problem(w, err)
			return
		}
		var tests json.RawMessage
		err = h.db.QueryRowContext(r.Context(), `SELECT test_metadata FROM academy_version_releases WHERE version_id=$1 AND course_id=$2`, version, course).Scan(&tests)
		if errors.Is(err, sql.ErrNoRows) {
			tests, err = h.testMetadata(r, course, p)
		}
		if err != nil {
			problem(w, err)
			return
		}
		h.previewRoom(w, r, course, p, tests)
		return
	}
	if r.Method == "GET" {
		rows, err := h.db.QueryContext(r.Context(), `SELECT v.id,v.number,v.source_revision,v.content,r.published_at FROM academy_program_versions v JOIN academy_course_programs p ON p.id=v.program_id LEFT JOIN academy_version_releases r ON r.version_id=v.id WHERE p.course_id=$1 ORDER BY v.number DESC`, course)
		if err != nil {
			problem(w, err)
			return
		}
		defer rows.Close()
		items := []any{}
		for rows.Next() {
			var id, rev int64
			var number int
			var raw []byte
			var date *time.Time
			if err = rows.Scan(&id, &number, &rev, &raw, &date); err != nil {
				problem(w, err)
				return
			}
			var p Program
			if err = json.Unmarshal(raw, &p); err != nil {
				problem(w, err)
				return
			}
			items = append(items, map[string]any{"id": id, "number": number, "revision": rev, "published_at": date, "readiness": CheckReadiness(p), "modules": len(learningProgram(p).Modules), "tests": countBlocks(p, "test"), "assignments": countBlocks(p, "assignment")})
		}
		if err = rows.Err(); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, items)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var v struct {
		VersionID   int64 `json:"version_id"`
		Acknowledge bool  `json:"acknowledge_warnings"`
	}
	if !decode(w, r, &v) {
		return
	}
	p, err := h.version(r.Context(), course, v.VersionID)
	if err != nil {
		problem(w, err)
		return
	}
	ready := CheckReadiness(p)
	if len(ready.Blockers) > 0 {
		fail(w, 400, strings.Join(ready.Blockers, ". "))
		return
	}
	if len(ready.Warnings) > 0 && !v.Acknowledge {
		fail(w, 400, "Подтвердите предупреждения о готовности курса перед публикацией")
		return
	}
	tests, err := h.testMetadata(r, course, p)
	if err != nil {
		problem(w, err)
		return
	}
	tx, err := h.beginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	err = freezeAssessments(r.Context(), tx, v.VersionID, p, user)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_version_releases(version_id,course_id,published_by,test_metadata) VALUES($1,$2,$3,$4) ON CONFLICT(version_id) DO NOTHING`, v.VersionID, course, user, string(tests))
	}
	if err == nil {
		err = commitAcademy(r.Context(), tx)
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"published": true, "version_id": v.VersionID})
}
func (h *Handler) testMetadata(r *http.Request, course int64, p Program) (json.RawMessage, error) {
	ids := []int64{}
	for _, m := range p.Modules {
		for _, l := range m.Lessons {
			if m.Status != "published" || l.Status != "published" || !l.SelfPaced {
				continue
			}
			for _, b := range l.Blocks {
				if b.Type == "test" && b.Data.TestVersionID > 0 {
					ids = append(ids, b.Data.TestVersionID)
				}
			}
		}
	}
	raw, _ := json.Marshal(ids)
	var tests json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',v.id,'title',v.title,'description',v.description,'version',v.version) ORDER BY v.id),'[]') FROM test_versions v JOIN tests t ON t.id=v.test_id JOIN courses c ON c.owner_id=t.author_id WHERE c.id=$1 AND v.id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, course, string(raw)).Scan(&tests)
	return tests, err
}
func (h *Handler) students(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 0 {
		if r.Method == "GET" {
			h.enrollmentList(w, r, user, true, course)
			return
		}
		if r.Method != "POST" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		var v struct {
			UserID     int64      `json:"user_id"`
			VersionID  int64      `json:"version_id"`
			ExpiresAt  *time.Time `json:"expires_at"`
			RequestKey string     `json:"request_key"`
		}
		if !decode(w, r, &v) {
			return
		}
		if !keyPattern.MatchString(v.RequestKey) || v.ExpiresAt != nil && !v.ExpiresAt.After(time.Now()) {
			fail(w, 400, "Проверьте срок доступа и ключ запроса")
			return
		}
		tx, err := h.beginTx(r.Context(), nil)
		if err != nil {
			problem(w, err)
			return
		}
		defer tx.Rollback()
		var lock int64
		if err = tx.QueryRowContext(r.Context(), `SELECT id FROM courses WHERE id=$1 AND(owner_id=$2 OR $3) FOR UPDATE`, course, user, adminPrincipal(r.Context()).Can("cohorts")).Scan(&lock); err != nil {
			problem(w, err)
			return
		}
		var student int64
		if err = tx.QueryRowContext(r.Context(), `SELECT id FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system`, v.UserID).Scan(&student); err != nil {
			problem(w, err)
			return
		}
		var version int64
		if err = tx.QueryRowContext(r.Context(), `SELECT version_id FROM academy_version_releases WHERE course_id=$1 AND version_id=$2`, course, v.VersionID).Scan(&version); err != nil {
			fail(w, 400, "Сначала опубликуйте выбранную версию учебной программы")
			return
		}
		var id int64
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 AND program_version_id=$3 AND cohort_id IS NULL AND access_status IN ('active','suspended') ORDER BY id DESC LIMIT 1`, course, student, version).Scan(&id)
		existing := err == nil
		if errorsNoRows(err) {
			err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_enrollments(course_id,user_id,program_version_id,expires_at,request_key,enrolled_by) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(request_key) DO NOTHING RETURNING id`, course, student, version, v.ExpiresAt, v.RequestKey, nullable(user)).Scan(&id)
			if errorsNoRows(err) {
				err = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_enrollments WHERE request_key=$1 AND course_id=$2 AND user_id=$3 AND program_version_id=$4`, v.RequestKey, course, student, version).Scan(&id)
			}
			if err == nil {
				err = event(r.Context(), tx, &Enrollment{ID: id}, user, "enrolled", "enrolled", "", map[string]any{"version_id": version, "expires_at": v.ExpiresAt})
			}
		}
		if err != nil {
			problem(w, err)
			return
		}
		if err = commitAcademy(r.Context(), tx); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]any{"id": id, "existing": existing})
		return
	}
	id, err := strconv.ParseInt(path[0], 10, 64)
	if err != nil {
		fail(w, 404, "Ученик не найден")
		return
	}
	e, err := loadEnrollment(r.Context(), h.db, id, user, true, false)
	if err != nil || e.CourseID != course {
		if err == nil {
			err = sql.ErrNoRows
		}
		problem(w, err)
		return
	}
	if r.Method == "GET" {
		if err = h.progress(r.Context(), e); err != nil {
			problem(w, err)
			return
		}
		var history []byte
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('type',type,'created_at',created_at,'data',data) ORDER BY created_at DESC,id DESC),'[]') FROM academy_learning_events WHERE enrollment_id=$1 AND type IN ('enrolled','access_suspended','access_restored','access_revoked','access_updated','student_enrolled','student_transferred','curator_assigned','course_completed')`, id).Scan(&history)
		if err != nil {
			problem(w, err)
			return
		}
		states := map[string]string{}
		for _, p := range e.Progress {
			states[p.LessonKey] = p.Status
		}
		lessons := []any{}
		for _, m := range learningProgram(e.Program).Modules {
			for _, l := range m.Lessons {
				lessons = append(lessons, map[string]any{"key": l.Key, "title": l.Title, "required": l.Required, "status": states[l.Key]})
			}
		}
		metrics, err := h.studentAssessmentSummary(r.Context(), e)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]any{"enrollment": e, "history": json.RawMessage(history), "lessons": lessons, "assessment_summary": metrics})
		return
	}
	if r.Method != "PUT" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var v struct {
		Status        string     `json:"status"`
		SetExpiration bool       `json:"set_expiration"`
		ExpiresAt     *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !h.courseOwner(r, course, user) && !adminPrincipal(r.Context()).Can("access") {
		fail(w, 403, "Только владелец управляет доступом")
		return
	}
	if !oneOf(v.Status, "active", "suspended", "revoked") || v.SetExpiration && v.ExpiresAt != nil && !v.ExpiresAt.After(time.Now()) {
		fail(w, 400, "Проверьте статус и срок доступа")
		return
	}
	tx, err := h.beginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	if e.CohortID != nil {
		if _, err = loadCohort(r.Context(), tx, *e.CohortID, course, true); err != nil {
			problem(w, err)
			return
		}
	}
	e, err = loadEnrollment(r.Context(), tx, id, user, true, true)
	if err != nil {
		problem(w, err)
		return
	}
	if e.CourseID != course {
		problem(w, sql.ErrNoRows)
		return
	}
	if v.Status == "active" {
		expiry := e.ExpiresAt
		if v.SetExpiration {
			expiry = v.ExpiresAt
		}
		if expiry != nil && !expiry.After(time.Now()) {
			fail(w, 400, "Укажите новый срок или снимите ограничение срока доступа")
			return
		}
		var duplicate bool
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 AND program_version_id=$3 AND id<>$4 AND cohort_id IS NOT DISTINCT FROM $5 AND access_status IN ('active','suspended'))`, course, e.UserID, e.VersionID, e.ID, e.CohortID).Scan(&duplicate)
		if err != nil {
			problem(w, err)
			return
		}
		if duplicate {
			fail(w, 409, "Для этой версии уже есть другое действующее зачисление")
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE academy_enrollments SET access_status=$2,expires_at=CASE WHEN $3 THEN $4 ELSE expires_at END WHERE id=$1`, id, v.Status, v.SetExpiration, v.ExpiresAt)
	var conflict *pgconn.PgError
	if errors.As(err, &conflict) && conflict.Code == "23505" {
		fail(w, 409, "Для этой версии уже есть другое действующее зачисление")
		return
	}
	expirationChanged := v.SetExpiration && ((v.ExpiresAt == nil) != (e.ExpiresAt == nil) || v.ExpiresAt != nil && e.ExpiresAt != nil && !v.ExpiresAt.Equal(*e.ExpiresAt))
	if err == nil && (v.Status != e.AccessStatus || expirationChanged) {
		typ := map[string]string{"active": "access_restored", "suspended": "access_suspended", "revoked": "access_revoked"}[v.Status]
		if v.Status == e.AccessStatus {
			typ = "access_updated"
		}
		err = event(r.Context(), tx, e, user, typ, "", "", map[string]any{"previous_status": e.AccessStatus, "expires_at": v.ExpiresAt, "set_expiration": v.SetExpiration})
		if err == nil {
			if expirationChanged && v.Status == "active" {
				typ = "access_extended"
			}
			now := time.Now()
			err = cohortEvent(r.Context(), tx, course, e.CohortID, &e.ID, user, typ, "", map[string]any{"expires_at": v.ExpiresAt}, &e.UserID, &now)
		}
	}
	if err != nil {
		problem(w, err)
		return
	}
	if err = commitAcademy(r.Context(), tx); err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]bool{"updated": true})
}
func errorsNoRows(err error) bool { return err == sql.ErrNoRows }
