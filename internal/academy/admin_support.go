package academy

import (
	"FinTalent/internal/courses"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

func (h *Handler) adminCourse(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64, path []string) {
	raw, err := courses.AdministrativeCourse(r.Context(), h.db, id)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var program *Program
	program, err = load(r.Context(), h.db, id)
	if err != nil && err != sql.ErrNoRows {
		analyticsProblem(w, err)
		return
	}
	var versions json.RawMessage
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',v.id,'number',v.number,'created_at',v.created_at,'published_at',r.published_at) ORDER BY v.number DESC),'[]') FROM academy_program_versions v JOIN academy_course_programs cp ON cp.id=v.program_id LEFT JOIN academy_version_releases r ON r.version_id=v.id WHERE cp.course_id=$1`, id).Scan(&versions)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if len(path) > 0 && path[0] == "program" {
		if version := queryInt(r, "version"); version > 0 {
			v, err := h.version(r.Context(), id, version)
			if err != nil {
				analyticsProblem(w, err)
				return
			}
			program = &v
		}
		var assets json.RawMessage
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',a.id,'name',a.name,'size',a.size,'mime',a.mime,'url','/api/admin/academy/assets/'||a.id)),'[]') FROM academy_media_assets a JOIN academy_course_programs cp ON cp.id=a.program_id WHERE cp.course_id=$1 AND a.submission_id IS NULL AND a.deleted_at IS NULL`, id).Scan(&assets)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		respond(w, 200, map[string]any{"program": program, "assets": assets, "versions": versions, "snapshot": func() any {
			if queryInt(r, "version") == 0 && program != nil {
				return adminSnapshotResponse(r.Context(), h.db, "program", id)
			}
			return nil
		}()})
		return
	}
	respond(w, 200, map[string]any{"course": raw, "program": program, "versions": versions, "snapshot": adminSnapshotResponse(r.Context(), h.db, "course", id)})
}
func (h *Handler) adminCohort(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64) {
	var course int64
	err := h.db.QueryRowContext(r.Context(), "SELECT course_id FROM academy_cohorts WHERE id=$1", id).Scan(&course)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	c, err := loadCohort(r.Context(), h.db, id, course, false)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var staff json.RawMessage
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('user_id',s.user_id,'name',u.full_name,'role',s.role,'can_review',s.can_review,'can_communicate',s.can_communicate) ORDER BY s.user_id),'[]') FROM academy_cohort_staff s JOIN users u ON u.id=s.user_id WHERE s.cohort_id=$1`, id).Scan(&staff)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	respond(w, 200, map[string]any{"cohort": c, "staff": staff, "snapshot": adminSnapshotResponse(r.Context(), h.db, "cohort", id), "program": func() any { v, _ := h.version(r.Context(), course, c.VersionID); return learningProgram(v) }()})
}
func (h *Handler) adminEnrollment(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64) {
	e, err := loadEnrollment(r.Context(), h.db, id, p.User, true, false)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if err = readProgress(r.Context(), h.db, e); err != nil {
		analyticsProblem(w, err)
		return
	}
	acks := map[string]*time.Time{}
	if err = analyticsJSON(r.Context(), h.db, "SELECT COALESCE(jsonb_object_agg(lesson_key,materials_acknowledged_at),'{}') FROM academy_lesson_progress WHERE enrollment_id=$1", &acks, id); err != nil {
		analyticsProblem(w, err)
		return
	}
	lessons := []any{}
	states := map[string]Progress{}
	for _, v := range e.Progress {
		states[v.LessonKey] = v
	}
	for _, m := range learningProgram(e.Program).Modules {
		for _, l := range m.Lessons {
			checks := []any{}
			for _, b := range l.Blocks {
				if oneOf(b.Type, "test", "assignment") {
					checks = append(checks, map[string]any{"key": b.Key, "title": b.Data.Title, "kind": b.Type, "required": requiredBlock(l, b), "satisfied": e.AssessmentStates[b.Key]})
				}
			}
			ack := acks[l.Key]
			reason := ""
			if e.EffectiveStatus != "active" {
				reason = map[string]string{"suspended": "Зачисление приостановлено", "revoked": "Доступ по зачислению отозван", "expired": "Срок доступа истёк"}[e.EffectiveStatus]
				if reason == "" {
					reason = ErrAccess.Error()
				}
				if e.Unavailable {
					reason = "Аккаунт ученика или автора ограничен платформой. Восстановление зачисления не отменяет блокировку аккаунта"
				}
			} else if err := e.checkLesson(l.Key); err != nil {
				reason = err.Error()
			}
			lessons = append(lessons, map[string]any{"key": l.Key, "title": l.Title, "module": m.Title, "availability": e.lessonAvailability(l.Key), "allowed": reason == "", "access_reason": reason, "progress": states[l.Key], "completion_rule": l.CompletionRule, "requirements": checks, "completion_reason": requirementsReason(l, e.AssessmentStates), "materials_acknowledged_at": ack})
		}
	}
	var events json.RawMessage
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at DESC),'[]') FROM(SELECT id,type,data,created_at FROM academy_learning_events WHERE enrollment_id=$1 UNION ALL SELECT -id,type,data,created_at FROM academy_cohort_events WHERE enrollment_id=$1 ORDER BY created_at DESC LIMIT 100)x`, id).Scan(&events)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	respond(w, 200, map[string]any{"enrollment": e, "lessons": lessons, "history": events, "server_time": time.Now(), "timezone": func() string {
		if e.Cohort != nil {
			return e.Cohort.Timezone
		}
		return "UTC"
	}(), "snapshot": adminSnapshotResponse(r.Context(), h.db, "enrollment", id)})
}
func (h *Handler) adminAssessment(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64, kind string) {
	table, base, field := "academy_effective_test_attempts", "academy_test_attempts", "test_attempt_id"
	if kind == "submissions" {
		table, base, field = "academy_effective_assignment_submissions", "academy_assignment_submissions", "submission_id"
	}
	var eid int64
	var row, original, assessment, history json.RawMessage
	err := h.db.QueryRowContext(r.Context(), "SELECT x.enrollment_id,to_jsonb(x)-'request_key'-'answers'-'answer'-'grades' FROM "+table+" x WHERE x.id=$1", id).Scan(&eid, &row)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	projection := "to_jsonb(x)-'request_key'"
	if !p.Can("answers") {
		projection += "-'answers'-'answer'-'grades'"
	}
	err = h.db.QueryRowContext(r.Context(), "SELECT "+projection+" FROM "+base+" x WHERE id=$1", id).Scan(&original)
	if err == nil {
		err = h.db.QueryRowContext(r.Context(), "SELECT jsonb_build_object('id',a.id,'title',a.title,'kind',a.kind,'config',a.config,'lesson_key',a.lesson_key,'version_id',a.program_version_id,'questions',CASE WHEN $2 THEN a.questions ELSE '[]'::jsonb END) FROM academy_assessment_versions a JOIN "+base+" x ON x.assessment_id=a.id WHERE x.id=$1", id, p.Can("answers")).Scan(&assessment)
	}
	if err == nil {
		err = h.db.QueryRowContext(r.Context(), "SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.id DESC),'[]') FROM academy_admin_corrections c WHERE "+field+"=$1", id).Scan(&history)
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var reviews json.RawMessage = json.RawMessage(`[]`)
	if kind == "submissions" {
		err = h.db.QueryRowContext(r.Context(), "SELECT COALESCE(jsonb_agg(to_jsonb(x)-'request_key' ORDER BY x.id),'[]') FROM academy_submission_reviews x WHERE submission_id=$1", id).Scan(&reviews)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
	}
	respond(w, 200, map[string]any{"item": row, "original": original, "assessment": assessment, "corrections": history, "reviews": reviews, "snapshot": adminSnapshotResponse(r.Context(), h.db, "enrollment", eid)})
}
func (h *Handler) adminFile(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64) {
	var storage, name, mime string
	var course int64
	var submission *int64
	err := h.db.QueryRowContext(r.Context(), `SELECT a.storage_key,a.name,a.mime,cp.course_id,a.submission_id FROM academy_media_assets a JOIN academy_course_programs cp ON cp.id=a.program_id WHERE a.id=$1 AND a.deleted_at IS NULL`, id).Scan(&storage, &name, &mime, &course, &submission)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if submission != nil && !p.Can("answers") {
		fail(w, 403, "Нет разрешения на просмотр ответов и файлов работ")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	defer tx.Rollback()
	m := &adminMutation{Principal: p, Action: "file_downloaded", Entity: "asset", ID: id, Course: course, Reason: "Просмотр материала при административной диагностике", Key: Key(), Before: json.RawMessage(`{}`)}
	if _, err = adminAudit(r.Context(), tx, m, map[string]any{"asset_id": id}); err == nil {
		err = tx.Commit()
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	h.sendAssetFile(w, r, storage, name, mime)
}
