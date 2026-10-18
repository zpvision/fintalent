package academy

import (
	"FinTalent/internal/courses"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

type AdminPrincipal struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	User     int64  `json:"user_id"`
	Role     string `json:"role"`
	Contacts bool   `json:"view_contacts"`
	Answers  bool   `json:"view_answers"`
	Correct  bool   `json:"correct_results"`
}

func (p AdminPrincipal) Can(action string) bool {
	if p.Key == "" {
		return false
	}
	if p.Role == "superadmin" {
		return true
	}
	switch action {
	case "read_courses", "program":
		return oneOf(p.Role, "academy_admin", "support", "moderator")
	case "files":
		return oneOf(p.Role, "academy_admin", "support", "moderator")
	case "read", "analytics":
		return oneOf(p.Role, "academy_admin", "support")
	case "access", "recalculate":
		return oneOf(p.Role, "academy_admin", "support")
	case "edit", "cohorts", "staff", "transfer", "communications":
		return p.Role == "academy_admin"
	case "moderate":
		return oneOf(p.Role, "academy_admin", "moderator")
	case "correct":
		return p.Role == "academy_admin" && p.Correct
	case "contacts":
		return p.Contacts && oneOf(p.Role, "academy_admin", "support")
	case "answers":
		return p.Answers && oneOf(p.Role, "academy_admin", "support")
	}
	return false
}

type adminScopeKey struct{}
type adminMutationKey struct{}

func adminPrincipal(ctx context.Context) AdminPrincipal {
	p, _ := ctx.Value(adminScopeKey{}).(AdminPrincipal)
	return p
}
func administrative(ctx context.Context) bool { return adminPrincipal(ctx).Can("read") }
func adminKey(ctx context.Context) string     { return adminPrincipal(ctx).Key }
func adminScope(ctx context.Context, p AdminPrincipal) context.Context {
	return context.WithValue(ctx, adminScopeKey{}, p)
}
func ResolveAdminGrant(ctx context.Context, db queryer, user int64) (AdminPrincipal, error) {
	var p AdminPrincipal
	err := db.QueryRowContext(ctx, `SELECT g.user_id,u.full_name,g.role,g.view_contacts,g.view_answers,g.correct_results FROM academy_admin_grants g JOIN users u ON u.id=g.user_id WHERE g.user_id=$1 AND NOT u.is_blocked AND NOT u.is_system`, user).Scan(&p.User, &p.Name, &p.Role, &p.Contacts, &p.Answers, &p.Correct)
	if err == nil {
		p.Key = fmt.Sprintf("user:%d", p.User)
	}
	return p, err
}
func adminHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var errAdminReplay = errors.New("administrative action already applied")

type adminMutation struct {
	Principal                                          AdminPrincipal
	Action, Entity, Reason, Key, Expected, Fingerprint string
	ID, Course, Enrollment, Student                    int64
	Before                                             json.RawMessage
	AuditWritten                                       bool
	Permission, Result                                 string
}
type AdminCommand struct {
	Reason   string          `json:"reason"`
	Confirm  bool            `json:"confirm"`
	Key      string          `json:"request_key"`
	Expected string          `json:"expected"`
	Data     json.RawMessage `json:"data"`
}

func (c *AdminCommand) validate() error {
	c.Reason = strings.TrimSpace(c.Reason)
	if !c.Confirm || utf8.RuneCountInString(c.Reason) < 5 || utf8.RuneCountInString(c.Reason) > 1000 || !keyPattern.MatchString(c.Key) || len(c.Expected) != 64 {
		return inputError{"Подтвердите действие, укажите причину от 5 символов и обновите карточку перед изменением"}
	}
	return nil
}

// Only fixed projections are audited. Answers, contact details and transport secrets are never snapshots.
func adminSnapshot(ctx context.Context, q queryer, entity string, id int64) (json.RawMessage, error) {
	table, cols := "", ""
	switch entity {
	case "program":
		return func() (json.RawMessage, error) {
			var raw json.RawMessage
			err := q.QueryRowContext(ctx, programSQL, id).Scan(&raw)
			return raw, err
		}()
	case "author":
		return func() (json.RawMessage, error) {
			var raw json.RawMessage
			err := q.QueryRowContext(ctx, `SELECT jsonb_build_object('id',u.id,'name',u.full_name,'restriction',COALESCE((SELECT to_jsonb(l) FROM academy_publication_limits l WHERE l.author_id=u.id),'{}')) FROM users u WHERE u.id=$1`, id).Scan(&raw)
			return raw, err
		}()
	case "course":
		table = "courses"
		cols = "'landing',to_jsonb(x),'marketing_modules',COALESCE((SELECT jsonb_agg(to_jsonb(m)||jsonb_build_object('lessons',COALESCE((SELECT jsonb_agg(to_jsonb(l) ORDER BY l.sort_order,l.id) FROM course_lessons l WHERE l.module_id=m.id),'[]')) ORDER BY m.sort_order,m.id) FROM course_modules m WHERE m.course_id=x.id),'[]'),'tariffs',COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM course_tariffs t WHERE t.course_id=x.id),'[]'),'id',x.id,'revision',x.revision,'title',x.title,'description',x.description,'category_id',x.category_id,'cover_image',x.cover_image,'status',x.status,'access_months',x.access_months,'enrollments',(SELECT count(*) FROM academy_enrollments e WHERE e.course_id=x.id),'restriction',COALESCE((SELECT to_jsonb(l) FROM academy_publication_limits l WHERE l.course_id=x.id),'{}')"
	case "cohort":
		table = "academy_cohorts"
		cols = "'settings',to_jsonb(x),'id',x.id,'revision',x.revision,'name',x.name,'starts_at',x.starts_at,'ends_at',x.ends_at,'timezone',x.timezone,'capacity',x.capacity,'status',x.status,'program_version_id',x.program_version_id,'schedule_mode',x.schedule_mode,'rules',x.rules,'deadlines',x.deadlines,'staff',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.user_id) FROM academy_cohort_staff s WHERE s.cohort_id=x.id),'[]')"
	case "enrollment":
		table = "academy_enrollments"
		cols = "'id',x.id,'access_status',x.access_status,'expires_at',x.expires_at,'cohort_id',x.cohort_id,'program_version_id',x.program_version_id,'completed_at',x.completed_at,'curator',COALESCE((SELECT sc.user_id FROM academy_student_curators sc WHERE sc.enrollment_id=x.id),0),'correction',COALESCE((SELECT max(c.id) FROM academy_admin_corrections c WHERE c.enrollment_id=x.id),0),'progress',COALESCE((SELECT jsonb_agg(jsonb_build_object('key',p.lesson_key,'status',p.status,'completed_at',p.completed_at) ORDER BY p.lesson_key) FROM academy_lesson_progress p WHERE p.enrollment_id=x.id),'[]')"
	case "campaign":
		table = "academy_campaigns"
		cols = "'id',x.id,'revision',x.revision,'status',x.status"
	case "automation":
		table = "academy_automations"
		cols = "'id',x.id,'revision',x.revision,'status',x.status,'generation',x.generation"
	case "grant":
		table = "academy_admin_grants"
		cols = "'user_id',x.user_id,'role',x.role,'revision',x.revision,'view_contacts',x.view_contacts,'view_answers',x.view_answers,'correct_results',x.correct_results"
	case "settings":
		table = "academy_admin_settings"
		cols = "'id',x.id,'settings',x.settings,'revision',x.revision"
	case "restriction":
		table = "academy_publication_limits"
		cols = "'id',x.id,'course_id',x.course_id,'author_id',x.author_id,'blocked',x.blocked,'reason',x.reason,'revision',x.revision"
	default:
		return nil, ErrInvalid
	}
	key := "id"
	if entity == "grant" {
		key = "user_id"
	}
	var raw json.RawMessage
	err := q.QueryRowContext(ctx, "SELECT jsonb_build_object("+cols+") FROM "+table+" x WHERE x."+key+"=$1", id).Scan(&raw)
	if err == sql.ErrNoRows && oneOf(entity, "grant", "settings", "restriction") {
		return json.RawMessage(`{}`), nil
	}
	return raw, err
}
func (h *Handler) beginTx(ctx context.Context, options *sql.TxOptions) (*sql.Tx, error) {
	tx, err := h.db.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	m, _ := ctx.Value(adminMutationKey{}).(*adminMutation)
	if m == nil {
		return tx, nil
	}
	abort := func(e error) (*sql.Tx, error) { tx.Rollback(); return nil, e }
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "academy-admin:"+m.Key); err != nil {
		return abort(err)
	}
	var actor, fingerprint string
	err = tx.QueryRowContext(ctx, "SELECT actor_key,fingerprint FROM academy_admin_audit WHERE request_key=$1", m.Key).Scan(&actor, &fingerprint)
	if err == nil {
		if actor != m.Principal.Key || fingerprint != m.Fingerprint {
			return abort(ErrConflict)
		}
		return abort(errAdminReplay)
	}
	if err != sql.ErrNoRows {
		return abort(err)
	}
	if m.Permission == "moderate" {
		if err = courses.LockPublication(ctx, tx); err != nil {
			return abort(err)
		}
	}
	// Shared lock order: course, cohort, enrollment. All corrections serialize with learner/reviewer operations.
	if m.Course > 0 {
		var id int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM courses WHERE id=$1 FOR UPDATE", m.Course).Scan(&id); err != nil {
			return abort(err)
		}
	}
	if m.Principal.User > 0 {
		var uid int64
		if err = tx.QueryRowContext(ctx, "SELECT user_id FROM academy_admin_grants WHERE user_id=$1 FOR SHARE", m.Principal.User).Scan(&uid); err != nil {
			return abort(ErrAccess)
		}
		principal, e := ResolveAdminGrant(ctx, tx, m.Principal.User)
		if e != nil || !principal.Can(m.Permission) {
			return abort(ErrAccess)
		}
	}
	if m.Entity == "cohort" {
		if _, err = loadCohort(ctx, tx, m.ID, m.Course, true); err != nil {
			return abort(err)
		}
	}
	if m.Enrollment > 0 {
		var cohort *int64
		if err = tx.QueryRowContext(ctx, "SELECT cohort_id FROM academy_enrollments WHERE id=$1", m.Enrollment).Scan(&cohort); err != nil {
			return abort(err)
		}
		if cohort != nil {
			if _, err = loadCohort(ctx, tx, *cohort, m.Course, true); err != nil {
				return abort(err)
			}
		}
		var id int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM academy_enrollments WHERE id=$1 FOR UPDATE", m.Enrollment).Scan(&id); err != nil {
			return abort(err)
		}
	}
	if oneOf(m.Entity, "settings", "grant", "restriction") {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "academy-admin-object:"+m.Entity+fmt.Sprint(m.ID)); err != nil {
			return abort(err)
		}
	}
	if oneOf(m.Entity, "campaign", "automation") {
		table := "academy_campaigns"
		if m.Entity == "automation" {
			table = "academy_automations"
		}
		var id int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM "+table+" WHERE id=$1 FOR UPDATE", m.ID).Scan(&id); err != nil {
			return abort(err)
		}
	}
	before, err := adminSnapshot(ctx, tx, m.Entity, m.ID)
	if err != nil {
		return abort(err)
	}
	if adminHash(before) != m.Expected {
		return abort(ErrConflict)
	}
	m.Before = before
	return tx, nil
}
func adminAudit(ctx context.Context, tx *sql.Tx, m *adminMutation, after any) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO academy_admin_audit(actor_key,actor_name,actor_user_id,action,entity,entity_id,course_id,enrollment_id,student_id,reason,before_value,after_value,request_key,fingerprint,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,COALESCE(NULLIF($15,''),'succeeded')) RETURNING id`, m.Principal.Key, m.Principal.Name, nullable(m.Principal.User), m.Action, m.Entity, m.ID, nullable(m.Course), nullable(m.Enrollment), nullable(m.Student), m.Reason, string(m.Before), jsonValue(after), m.Key, m.Fingerprint, m.Result).Scan(&id)
	return id, err
}
func commitAcademy(ctx context.Context, tx *sql.Tx) error {
	m, _ := ctx.Value(adminMutationKey{}).(*adminMutation)
	if m != nil && !m.AuditWritten {
		after, err := adminSnapshot(ctx, tx, m.Entity, m.ID)
		if err != nil {
			return err
		}
		if _, err = adminAudit(ctx, tx, m, after); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (h *Handler) SetAdminResolver(resolver func(*http.Request) (AdminPrincipal, error)) {
	h.admin = resolver
}

type AdminSettings struct {
	UploadMB             int  `json:"upload_mb"`
	ReportDays           int  `json:"report_days"`
	EmailPerMinute       int  `json:"email_per_minute"`
	AutomationsPerCourse int  `json:"automations_per_course"`
	EmailEnabled         bool `json:"email_enabled"`
	AutomationsEnabled   bool `json:"automations_enabled"`
	ModerationRequired   bool `json:"moderation_required"`
}

func defaultAdminSettings() AdminSettings { return AdminSettings{20, 7, 30, 100, true, true, true} }
func academySettings(ctx context.Context, q queryer) (AdminSettings, error) {
	v := defaultAdminSettings()
	var raw []byte
	err := q.QueryRowContext(ctx, "SELECT settings FROM academy_admin_settings WHERE id=1").Scan(&raw)
	if err == sql.ErrNoRows {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (s AdminSettings) validate() error {
	if s.UploadMB < 1 || s.UploadMB > 20 || s.ReportDays < 1 || s.ReportDays > 30 || s.EmailPerMinute < 1 || s.EmailPerMinute > 30 || s.AutomationsPerCourse < 1 || s.AutomationsPerCourse > 100 || !s.ModerationRequired {
		return inputError{"Проверьте лимиты: файлы 1–20 МБ, отчёты 1–30 дней, email 1–30/мин., сценарии 1–100. Модерация остаётся обязательной"}
	}
	return nil
}
