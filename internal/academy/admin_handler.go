package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) administration(w http.ResponseWriter, r *http.Request) {
	if h.admin == nil {
		fail(w, 401, "Требуется административный доступ")
		return
	}
	p, err := h.admin(r)
	if err != nil || p.Key == "" {
		fail(w, 401, "Войдите в админку или в аккаунт сотрудника с выданными полномочиями")
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if err = http.NewCrossOriginProtection().Check(r); err != nil {
			fail(w, 403, "Недопустимый источник запроса")
			return
		}
	}
	ctx, cancel := context.WithTimeout(adminScope(r.Context(), p), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	path := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/academy"), "/"), "/")
	section := path[0]
	if section == "session" && r.Method == "GET" {
		caps := map[string]bool{}
		for _, key := range []string{"read_courses", "program", "read", "analytics", "files", "access", "recalculate", "edit", "cohorts", "staff", "transfer", "communications", "moderate", "correct", "contacts", "answers", "settings", "grants"} {
			caps[key] = p.Can(key)
		}
		respond(w, 200, map[string]any{"principal": p, "capabilities": caps})
		return
	}
	permission := "read"
	if oneOf(section, "courses", "authors", "program") {
		permission = "read_courses"
	}
	if section == "users" {
		permission = "staff"
	}
	if oneOf(section, "settings", "grants") {
		permission = section
	}
	if section == "analytics" || section == "overview" {
		permission = "analytics"
	}
	if section == "assets" {
		permission = "files"
	}
	if !p.Can(permission) {
		fail(w, 403, "Недостаточно административных полномочий")
		return
	}
	if section == "courses" && len(path) >= 3 && path[2] == "analytics" {
		if !p.Can("analytics") {
			fail(w, 403, "Аналитика недоступна")
			return
		}
		id, _ := strconv.ParseInt(path[1], 10, 64)
		if id <= 0 {
			problem(w, sql.ErrNoRows)
			return
		}
		if r.Method != "GET" && !(len(path) >= 4 && path[3] == "reports" && (r.Method == "POST" || r.Method == "DELETE")) {
			fail(w, 405, "Изменение авторских настроек здесь недоступно")
			return
		}
		h.analyticsRoute(w, r, id, p.User, path[3:])
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		h.adminAction(w, r, p, path)
		return
	}
	if section == "overview" || section == "analytics" {
		h.adminOverview(w, r, p)
		return
	}
	if section == "assets" && len(path) == 2 {
		id, _ := strconv.ParseInt(path[1], 10, 64)
		h.adminFile(w, r, p, id)
		return
	}
	if section == "settings" {
		v, err := academySettings(ctx, h.db)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		snap, err := adminSnapshot(ctx, h.db, "settings", 1)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		respond(w, 200, map[string]any{"settings": v, "expected": adminHash(snap)})
		return
	}
	if section == "users" && len(path) == 1 {
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		if len(search) < 3 {
			respond(w, 200, []any{})
			return
		}
		var users json.RawMessage
		err = h.db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT id,full_name name,CASE WHEN $2 THEN email END email FROM users WHERE NOT is_blocked AND NOT is_system AND(full_name ILIKE '%'||$1||'%' OR id::text=$1 OR $2 AND lower(email)=lower($1)) ORDER BY full_name,id LIMIT 10)x`, search, p.Can("contacts")).Scan(&users)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		respond(w, 200, users)
		return
	}
	if len(path) == 1 {
		h.adminList(w, r, p, section)
		return
	}
	id, err := strconv.ParseInt(path[1], 10, 64)
	if err != nil || id <= 0 {
		problem(w, sql.ErrNoRows)
		return
	}
	switch section {
	case "grants":
		var name string
		err = h.db.QueryRowContext(ctx, "SELECT full_name FROM users WHERE id=$1 AND NOT is_system", id).Scan(&name)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
		respond(w, 200, map[string]any{"name": name, "snapshot": adminSnapshotResponse(ctx, h.db, "grant", id)})
	case "courses":
		h.adminCourse(w, r, p, id, path[2:])
	case "students", "authors":
		h.adminPerson(w, r, p, id, section)
	case "enrollments":
		h.adminEnrollment(w, r, p, id)
	case "cohorts":
		h.adminCohort(w, r, p, id)
	case "tests", "submissions":
		h.adminAssessment(w, r, p, id, section)
	case "campaigns", "automations":
		h.adminCommunication(w, r, p, id, section)
	case "audit":
		var row json.RawMessage
		err = h.db.QueryRowContext(ctx, "SELECT to_jsonb(a) FROM academy_admin_audit a WHERE id=$1", id).Scan(&row)
		if err == nil {
			respond(w, 200, row)
		} else {
			analyticsProblem(w, err)
		}
	default:
		fail(w, 404, "Раздел не найден")
	}
}
func adminID(r *http.Request, key string) (int64, error) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id < 0 {
		return 0, ErrInvalid
	}
	return id, nil
}
func (h *Handler) adminList(w http.ResponseWriter, r *http.Request, p AdminPrincipal, section string) {
	f, err := parseAnalyticsFilters(r.URL.Query(), time.Now())
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	course, err := adminID(r, "course")
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	author, err := adminID(r, "author")
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	user, err := adminID(r, "user")
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	enrollment, err := adminID(r, "enrollment")
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	campaign, err := adminID(r, "campaign")
	if err != nil {
		problem(w, err)
		return
	}
	automation, err := adminID(r, "automation")
	if err != nil {
		problem(w, err)
		return
	}
	minimum, err := adminID(r, "min_students")
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	published := r.URL.Query().Get("has_program")
	if published != "" && published != "yes" && published != "no" {
		problem(w, ErrInvalid)
		return
	}
	// Each fixed SELECT projects only UI fields. Filtering never constructs identifiers from user strings.
	query := ""
	status := r.URL.Query().Get("state")
	switch section {
	case "courses":
		query = `SELECT c.id,c.title,c.slug,c.owner_id author_id,u.full_name author_name,c.status,c.created_at,c.updated_at,c.revision,(SELECT count(*) FROM academy_enrollments e WHERE e.course_id=c.id) enrollments,(SELECT count(*) FROM academy_cohorts ch WHERE ch.course_id=c.id) cohorts,(SELECT max(v.number) FROM academy_version_releases r JOIN academy_program_versions v ON v.id=r.version_id WHERE r.course_id=c.id) published_version FROM courses c JOIN users u ON u.id=c.owner_id WHERE($1='' OR c.title ILIKE '%'||$1||'%' OR u.full_name ILIKE '%'||$1||'%') AND($2=0 OR c.id=$2) AND($3=0 OR c.owner_id=$3) AND($6='' OR c.status=$6) AND c.created_at>=$7 AND c.created_at<$8 AND(SELECT count(*) FROM academy_enrollments e WHERE e.course_id=c.id)>=$13 AND($14='' OR EXISTS(SELECT 1 FROM academy_version_releases rel WHERE rel.course_id=c.id)=($14='yes'))`
	case "authors":
		query = `SELECT u.id,u.full_name name,count(DISTINCT c.id) courses,(SELECT count(*) FROM academy_enrollments e JOIN courses own ON own.id=e.course_id WHERE own.owner_id=u.id) enrollments,(SELECT count(*) FROM academy_cohorts ch JOIN courses own ON own.id=ch.course_id WHERE own.owner_id=u.id AND ch.status='active') active_cohorts,max(c.updated_at) last_activity_at FROM users u JOIN courses c ON c.owner_id=u.id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.id::text=$1) AND($2=0 OR c.id=$2) AND($3=0 OR u.id=$3) AND c.created_at>=$7 AND c.created_at<$8 GROUP BY u.id`
	case "students":
		query = `SELECT u.id,u.full_name name,CASE WHEN $9 THEN u.email END email,u.is_blocked,count(DISTINCT e.course_id) courses,count(*) enrollments,count(*) FILTER(WHERE NOT u.is_blocked AND NOT u.is_system AND NOT EXISTS(SELECT 1 FROM users owner WHERE owner.id=c.owner_id AND owner.is_blocked) AND e.access_status='active' AND(e.expires_at IS NULL OR e.expires_at>now())) active_enrollments,max(e.last_activity_at) last_activity_at,avg((SELECT m.percent FROM academy_student_metrics m WHERE m.enrollment_id=e.id AND m.required>0)) average_progress FROM users u JOIN academy_enrollments e ON e.user_id=u.id JOIN courses c ON c.id=e.course_id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.id::text=$1 OR $9 AND u.email ILIKE '%'||$1||'%') AND($2=0 OR e.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($4=0 OR e.cohort_id=$4) AND($5=0 OR u.id=$5) AND($6='' OR (CASE WHEN u.is_blocked OR u.is_system OR EXISTS(SELECT 1 FROM users owner WHERE owner.id=c.owner_id AND owner.is_blocked) THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END)=$6 OR $6='completed' AND e.completed_at IS NOT NULL) AND e.enrolled_at>=$7 AND e.enrolled_at<$8 GROUP BY u.id`
	case "enrollments":
		query = `SELECT e.id,e.course_id,e.user_id,u.full_name name,c.title course_title,ch.name cohort_name,e.program_version_id,v.number version_number,CASE WHEN u.is_blocked OR u.is_system OR EXISTS(SELECT 1 FROM users owner WHERE owner.id=c.owner_id AND owner.is_blocked) THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END access_status,e.expires_at,e.started_at,e.completed_at,e.enrolled_at,e.last_activity_at FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN courses c ON c.id=e.course_id JOIN academy_program_versions v ON v.id=e.program_version_id LEFT JOIN academy_cohorts ch ON ch.id=e.cohort_id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.id::text=$1 OR $9 AND u.email ILIKE '%'||$1||'%') AND($2=0 OR e.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($4=0 OR e.cohort_id=$4) AND($5=0 OR e.user_id=$5) AND($6='' OR (CASE WHEN u.is_blocked OR u.is_system OR EXISTS(SELECT 1 FROM users owner WHERE owner.id=c.owner_id AND owner.is_blocked) THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END)=$6 OR $6='completed' AND e.completed_at IS NOT NULL) AND e.enrolled_at>=$7 AND e.enrolled_at<$8`
	case "cohorts":
		query = `SELECT ch.id,ch.name,ch.course_id,c.title course_title,u.full_name author_name,ch.status,ch.starts_at,ch.ends_at,ch.timezone,ch.revision,v.number version_number,(SELECT count(*) FROM academy_enrollments e WHERE e.cohort_id=ch.id) enrollments FROM academy_cohorts ch JOIN courses c ON c.id=ch.course_id JOIN users u ON u.id=c.owner_id JOIN academy_program_versions v ON v.id=ch.program_version_id WHERE($1='' OR ch.name ILIKE '%'||$1||'%' OR c.title ILIKE '%'||$1||'%') AND($2=0 OR ch.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($4=0 OR ch.id=$4) AND($6='' OR ch.status=$6) AND ch.created_at>=$7 AND ch.created_at<$8`
	case "tests":
		query = `SELECT t.id,t.enrollment_id,t.number,t.status,t.percent,t.passed,t.correction_id,t.finished_at,a.title,e.course_id,c.title course_title,e.user_id,u.full_name name FROM academy_effective_test_attempts t JOIN academy_enrollments e ON e.id=t.enrollment_id JOIN users u ON u.id=e.user_id JOIN courses c ON c.id=e.course_id JOIN academy_assessment_versions a ON a.id=t.assessment_id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR a.title ILIKE '%'||$1||'%') AND($2=0 OR e.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($4=0 OR e.cohort_id=$4) AND($5=0 OR e.user_id=$5) AND($6='' OR t.status=$6) AND($12=0 OR e.id=$12) AND t.started_at>=$7 AND t.started_at<$8`
	case "submissions":
		query = `SELECT s.id,s.enrollment_id,s.number,s.status,s.score,s.correction_id,s.submitted_at,s.reviewed_at,a.title,e.course_id,c.title course_title,e.user_id,u.full_name name FROM academy_effective_assignment_submissions s JOIN academy_enrollments e ON e.id=s.enrollment_id JOIN users u ON u.id=e.user_id JOIN courses c ON c.id=e.course_id JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR a.title ILIKE '%'||$1||'%') AND($2=0 OR e.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($4=0 OR e.cohort_id=$4) AND($5=0 OR e.user_id=$5) AND($6='' OR s.status=$6) AND($12=0 OR e.id=$12) AND s.created_at>=$7 AND s.created_at<$8`
	case "campaigns":
		query = `SELECT x.id,x.course_id,c.title course_title,u.full_name author_name,x.name,x.status,x.channel,x.send_at,x.created_at,x.revision,(SELECT count(*) FROM academy_message_deliveries d WHERE d.campaign_id=x.id AND NOT d.test) recipients,(SELECT count(*) FROM academy_message_deliveries d WHERE d.campaign_id=x.id AND d.status='failed') errors FROM academy_campaigns x JOIN courses c ON c.id=x.course_id JOIN users u ON u.id=c.owner_id WHERE($1='' OR x.name ILIKE '%'||$1||'%') AND($2=0 OR x.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($6='' OR x.status=$6) AND x.created_at>=$7 AND x.created_at<$8`
	case "automations":
		query = `SELECT x.id,x.course_id,c.title course_title,u.full_name author_name,x.name,x.status,x.trigger,x.created_at,x.revision,(SELECT count(*) FROM academy_automation_runs run WHERE run.automation_id=x.id) runs FROM academy_automations x JOIN courses c ON c.id=x.course_id JOIN users u ON u.id=c.owner_id WHERE($1='' OR x.name ILIKE '%'||$1||'%') AND($2=0 OR x.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($6='' OR x.status=$6) AND x.created_at>=$7 AND x.created_at<$8`
	case "deliveries":
		query = `SELECT d.id,d.course_id,d.enrollment_id,c.title course_title,d.campaign_id,d.channel,d.status,d.attempts,d.error_code,d.queued_at,d.sent_at,d.uncertain FROM academy_message_deliveries d JOIN courses c ON c.id=d.course_id WHERE($1='' OR d.error_code ILIKE '%'||$1||'%') AND($2=0 OR d.course_id=$2) AND($3=0 OR c.owner_id=$3) AND($5=0 OR d.recipient_id=$5) AND($6='' OR d.status=$6) AND($15=0 OR d.campaign_id=$15) AND($16=0 OR EXISTS(SELECT 1 FROM academy_automation_runs run WHERE run.id=d.run_id AND run.automation_id=$16)) AND($12=0 OR d.enrollment_id=$12) AND d.queued_at>=$7 AND d.queued_at<$8`
	case "audit":
		query = `SELECT a.id,a.actor_key,a.actor_name,a.action,a.entity,a.entity_id,a.course_id,a.enrollment_id,a.student_id,a.reason,a.result,a.created_at FROM academy_admin_audit a WHERE($1='' OR a.reason ILIKE '%'||$1||'%' OR a.actor_name ILIKE '%'||$1||'%' OR a.action ILIKE '%'||$1||'%') AND($2=0 OR a.course_id=$2) AND($5=0 OR a.student_id=$5) AND($6='' OR a.action=$6) AND($12=0 OR a.enrollment_id=$12) AND($3=0 OR EXISTS(SELECT 1 FROM courses c WHERE c.id=a.course_id AND c.owner_id=$3)) AND a.created_at>=$7 AND a.created_at<$8`
	case "grants":
		query = `SELECT g.user_id id,u.full_name name,g.role,g.view_contacts,g.view_answers,g.correct_results,g.revision FROM academy_admin_grants g JOIN users u ON u.id=g.user_id WHERE($1='' OR u.full_name ILIKE '%'||$1||'%' OR u.id::text=$1)`
	default:
		fail(w, 404, "Раздел не найден")
		return
	}
	// Declare every placeholder even when a particular projection doesn't need a filter.
	query = "WITH parameters AS(SELECT $1::text search,$2::bigint course,$3::bigint author,$4::bigint cohort,$5::bigint student,$6::text state,$7::timestamptz since,$8::timestamptz until,$9::boolean contacts,$12::bigint enrollment,$13::bigint minimum,$14::text published,$15::bigint campaign,$16::bigint automation),selected AS(" + query + "),page AS(SELECT * FROM selected ORDER BY id DESC LIMIT $10 OFFSET $11) SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY id DESC) FROM page),'[]'),'total',(SELECT count(*) FROM selected))"
	var raw json.RawMessage
	err = h.db.QueryRowContext(r.Context(), query, f.Search, course, author, f.Cohort, user, status, f.from, f.until, p.Can("contacts"), f.Size, (f.Page-1)*f.Size, enrollment, minimum, published, campaign, automation).Scan(&raw)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	respond(w, 200, raw)
}
func adminSnapshotResponse(ctx context.Context, q queryer, entity string, id int64) map[string]any {
	raw, err := adminSnapshot(ctx, q, entity, id)
	if err != nil {
		return nil
	}
	return map[string]any{"value": raw, "expected": adminHash(raw)}
}

// Audited read helpers intentionally return no storage keys, subscriber tokens or SMTP credentials.
func (h *Handler) adminCommunication(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64, section string) {
	table, entity := "academy_campaigns", "campaign"
	if section == "automations" {
		table, entity = "academy_automations", "automation"
	}
	var row map[string]any
	err := analyticsJSON(r.Context(), h.db, "SELECT to_jsonb(x)-'request_key' FROM "+table+" x WHERE id=$1", &row, id)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var runs json.RawMessage = json.RawMessage(`[]`)
	preview, previewError := "", ""
	if section == "automations" {
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT run.id,run.enrollment_id,run.event_type,run.event_at,run.skipped_reason,(SELECT count(*) FROM academy_message_deliveries d WHERE d.run_id=run.id AND d.status='failed') errors FROM academy_automation_runs run WHERE automation_id=$1 ORDER BY id DESC LIMIT 100)x`, id).Scan(&runs)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
	} else {
		raw, _ := json.Marshal(row["message"])
		var message Message
		if err = json.Unmarshal(raw, &message); err == nil {
			_, preview, err = RenderMessage(message, sampleVariables(h.base, "Сотрудник FinTalent"), h.base, "")
		}
		if err != nil {
			previewError = "Письмо не прошло проверку предпросмотра: проверьте блоки, переменные и ссылки"
		}
	}
	respond(w, 200, map[string]any{"item": row, "snapshot": adminSnapshotResponse(r.Context(), h.db, entity, id), "runs": runs, "preview_html": preview, "preview_error": previewError})
}
func (h *Handler) adminPerson(w http.ResponseWriter, r *http.Request, p AdminPrincipal, id int64, section string) {
	var person json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('id',id,'name',full_name,'email',CASE WHEN $2 THEN email END,'registered_at',created_at,'blocked',is_blocked,'profile_url',COALESCE((SELECT '/profiles/view/'||r.id FROM resumes r WHERE r.user_id=u.id AND r.deleted_at IS NULL ORDER BY r.id DESC LIMIT 1),'')) FROM users u WHERE id=$1 AND(CASE WHEN $3 THEN EXISTS(SELECT 1 FROM courses WHERE owner_id=u.id) ELSE EXISTS(SELECT 1 FROM academy_enrollments WHERE user_id=u.id) END)`, id, p.Can("contacts"), section == "authors").Scan(&person)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	respond(w, 200, map[string]any{"person": person, "snapshot": func() any {
		if section == "authors" {
			return adminSnapshotResponse(r.Context(), h.db, "author", id)
		}
		return nil
	}()})
}
