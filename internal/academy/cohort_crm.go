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

// Assigned staff can enter their Academy workspace even when the public courses beta is closed.
// This exception never grants access to the author's program or landing editor.
func (h *Handler) CoursePage(page http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 5 && parts[0] == "profile" && parts[1] == "courses" && parts[3] == "academy" && oneOf(parts[4], "cohorts", "crm", "reviews", "communications", "automations", "analytics") {
			course, _ := strconv.ParseInt(parts[2], 10, 64)
			user, err := h.user(r)
			if err == nil && user > 0 {
				var allowed bool
				err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id JOIN courses c ON c.id=ch.course_id JOIN users owner ON owner.id=c.owner_id JOIN users u ON u.id=s.user_id WHERE ch.course_id=$1 AND s.user_id=$2 AND NOT owner.is_blocked AND NOT u.is_blocked AND NOT u.is_system)`, course, user).Scan(&allowed)
				if err == nil && allowed {
					page(w, r)
					return
				}
			}
		}
		h.gate(page)(w, r)
	}
}

func (h *Handler) authorOrStaff(w http.ResponseWriter, r *http.Request) {
	user, err := h.user(r)
	if err != nil || user <= 0 {
		fail(w, 401, "Войдите в аккаунт")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/academy/courses/"), "/"), "/")
	course, _ := strconv.ParseInt(parts[0], 10, 64)
	if h.courseOwner(r, course, user) {
		h.gate(h.route)(w, r)
		return
	}
	h.route(w, r)
}
func (h *Handler) cohortWorkspace(w http.ResponseWriter, r *http.Request, course, user int64, owner bool) {
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('id',id,'title',title,'cover_image',cover_image,'status',status,'can_manage',$2::boolean,'can_communicate',($2::boolean OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=$1 AND s.user_id=$3 AND s.can_communicate))) FROM courses WHERE id=$1`, course, owner, user).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"course": raw, "program": nil, "can_manage": owner})
}
func (h *Handler) staffWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, err := h.user(r)
	if err != nil || user <= 0 {
		fail(w, 401, "Войдите в аккаунт")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var raw json.RawMessage
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.title,x.id),'[]') FROM(SELECT DISTINCT c.id,c.title,c.cover_image FROM courses c JOIN users owner ON owner.id=c.owner_id JOIN academy_cohorts ch ON ch.course_id=c.id JOIN academy_cohort_staff s ON s.cohort_id=ch.id JOIN users u ON u.id=s.user_id WHERE s.user_id=$1 AND NOT u.is_blocked AND NOT u.is_system AND NOT owner.is_blocked)x`, user).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
func (h *Handler) crmStudents(r *http.Request, course, user, cohort int64) (json.RawMessage, error) {
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.enrolled_at DESC,x.enrollment_id DESC),'[]') FROM(SELECT m.*,ch.name cohort_name,COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.name) FROM academy_student_tag_assignments a JOIN academy_student_tags t ON t.id=a.tag_id WHERE a.enrollment_id=m.enrollment_id),'[]') tags,(m.completed_at IS NULL AND(COALESCE(m.last_activity_at,m.enrolled_at)<now()-INTERVAL '7 days' OR m.failed_attempts>=2 OR m.pending>0 OR m.overdue_assignments>0 OR (m.expected_lessons>=3 AND m.completed*2<m.expected_lessons AND ch.starts_at<now()-INTERVAL '1 day') OR EXISTS(SELECT 1 FROM academy_cohort_events ev WHERE ev.enrollment_id=m.enrollment_id AND ev.type='deadline_missed' AND ev.delivered_at IS NOT NULL))) needs_attention FROM academy_student_metrics m LEFT JOIN academy_cohorts ch ON ch.id=m.cohort_id WHERE m.course_id=$1 AND($3=0 OR m.cohort_id=$3) AND academy_can_manage_enrollment(m.enrollment_id,$2,false))x`, course, user, cohort).Scan(&raw)
	return raw, err
}
func (h *Handler) cohortDetail(w http.ResponseWriter, r *http.Request, c *Cohort, user int64, owner bool) {
	students, err := h.crmStudents(r, c.CourseID, user, c.ID)
	if err != nil {
		problem(w, err)
		return
	}
	var staff, events json.RawMessage
	// Curators see only their assigned students; no groupwide activity or individual grades leak.
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('user_id',s.user_id,'name',u.full_name,'avatar',COALESCE(u.avatar_url,''),'role',s.role,'can_review',s.can_review,'can_communicate',s.can_communicate) ORDER BY u.full_name),'[]') FROM academy_cohort_staff s JOIN users u ON u.id=s.user_id WHERE s.cohort_id=$1`, c.ID).Scan(&staff)
	if err == nil {
		err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at DESC,x.id DESC),'[]') FROM(SELECT ev.id,ev.type,ev.created_at,ev.data,u.full_name actor_name FROM academy_cohort_events ev LEFT JOIN users u ON u.id=ev.actor_id WHERE ev.cohort_id=$1 AND($3 OR ev.enrollment_id IS NOT NULL AND academy_can_manage_enrollment(ev.enrollment_id,$2,false)) ORDER BY ev.id DESC LIMIT 30)x`, c.ID, user, owner).Scan(&events)
	}
	if err != nil {
		problem(w, err)
		return
	}
	p, err := h.version(r.Context(), c.CourseID, c.VersionID)
	if err != nil {
		problem(w, err)
		return
	}
	for i := range p.Modules {
		for j := range p.Modules[i].Lessons {
			l := &p.Modules[i].Lessons[j]
			for k := range l.Blocks {
				l.Blocks[k].Data = BlockData{Title: l.Blocks[k].Data.Title}
			}
			l.Description = ""
		}
	}
	var metrics struct {
		Total        int      `json:"total"`
		Started      int      `json:"started"`
		Active       int      `json:"active"`
		Completed    int      `json:"completed"`
		Average      float64  `json:"average"`
		Pending      int      `json:"pending"`
		Attention    int      `json:"attention"`
		TestAverage  *float64 `json:"test_average"`
		Distribution [5]int   `json:"distribution"`
	}
	var list []struct {
		Started      *time.Time `json:"started_at"`
		Activity     *time.Time `json:"last_activity_at"`
		Completed    *time.Time `json:"completed_at"`
		Percent      int        `json:"percent"`
		Pending      int        `json:"pending"`
		Attention    bool       `json:"needs_attention"`
		Average      *float64   `json:"test_average"`
		TestAttempts int        `json:"test_attempts"`
	}
	_ = json.Unmarshal(students, &list)
	sumTests, nTests := 0.0, 0
	for _, s := range list {
		metrics.Total++
		if s.Started != nil {
			metrics.Started++
		}
		if s.Activity != nil && time.Since(*s.Activity) < 7*24*time.Hour {
			metrics.Active++
		}
		if s.Completed != nil {
			metrics.Completed++
		}
		metrics.Average += float64(s.Percent)
		metrics.Pending += s.Pending
		if s.Attention {
			metrics.Attention++
		}
		bucket := s.Percent / 25
		if bucket > 4 {
			bucket = 4
		}
		metrics.Distribution[bucket]++
		if s.Average != nil {
			sumTests += *s.Average * float64(s.TestAttempts)
			nTests += s.TestAttempts
		}
	}
	if metrics.Total > 0 {
		metrics.Average /= float64(metrics.Total)
	}
	if nTests > 0 {
		v := sumTests / float64(nTests)
		metrics.TestAverage = &v
	}
	upcoming := []any{}
	for _, m := range p.Modules {
		for _, l := range m.Lessons {
			a := availability(c, m.Key, l.Key, time.Now())
			if a.OpensAt != nil && a.OpensAt.After(time.Now()) {
				upcoming = append(upcoming, map[string]any{"title": l.Title, "at": a.OpensAt, "type": "lesson"})
			}
		}
	}
	for _, d := range c.Deadlines {
		at, er := ruleTime(c, d)
		if er == nil && at.After(time.Now()) {
			upcoming = append(upcoming, map[string]any{"title": d.Key, "at": at, "type": "deadline"})
		}
	}
	respond(w, 200, map[string]any{"cohort": c, "students": students, "staff": staff, "events": events, "metrics": metrics, "program": p, "can_manage": owner, "upcoming": upcoming})
}

func (h *Handler) crmRoute(w http.ResponseWriter, r *http.Request, course, user int64, owner bool, path []string) {
	if len(path) == 0 && r.Method == "GET" {
		raw, err := h.crmStudents(r, course, user, queryInt(r, "cohort"))
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if len(path) > 0 && path[0] == "tags" {
		h.crmTags(w, r, course, user, owner, path[1:])
		return
	}
	id := int64(0)
	if len(path) > 0 {
		id, _ = strconv.ParseInt(path[0], 10, 64)
	}
	e, err := loadEnrollment(r.Context(), h.db, id, user, true, false)
	if err != nil {
		problem(w, err)
		return
	}
	if e.CourseID != course {
		problem(w, sql.ErrNoRows)
		return
	}
	if len(path) == 1 && r.Method == "GET" {
		h.crmCard(w, r, e, user, owner)
		return
	}
	if len(path) > 1 && path[1] == "notes" {
		h.crmNotes(w, r, e, user, owner, path[2:])
		return
	}
	if len(path) == 2 && path[1] == "curator" && r.Method == "PUT" && owner {
		var body struct {
			User *int64 `json:"user_id"`
		}
		if !decode(w, r, &body) {
			return
		}
		if e.CohortID == nil {
			fail(w, 400, "Куратор назначается ученику конкретного потока")
			return
		}
		tx, er := h.db.BeginTx(r.Context(), nil)
		if er != nil {
			problem(w, er)
			return
		}
		defer tx.Rollback()
		_, err = loadCohort(r.Context(), tx, *e.CohortID, course, true)
		if err == nil && body.User != nil {
			var staff int64
			err = tx.QueryRowContext(r.Context(), `SELECT s.user_id FROM academy_cohort_staff s JOIN users u ON u.id=s.user_id WHERE s.cohort_id=$1 AND s.user_id=$2 AND NOT u.is_blocked`, e.CohortID, *body.User).Scan(&staff)
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_student_curators(enrollment_id,user_id,assigned_by) VALUES($1,$2,$3) ON CONFLICT(enrollment_id) DO UPDATE SET user_id=excluded.user_id,assigned_by=excluded.assigned_by,assigned_at=now()`, id, *body.User, user)
			}
		} else if err == nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM academy_student_curators WHERE enrollment_id=$1`, id)
		}
		if err == nil {
			now := time.Now()
			err = cohortEvent(r.Context(), tx, course, e.CohortID, &id, user, "curator_assigned", "", body, &e.UserID, &now)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
		return
	}
	if len(path) == 2 && path[1] == "tags" && r.Method == "PUT" && owner {
		var body struct {
			IDs []int64 `json:"tag_ids"`
		}
		if !decode(w, r, &body) {
			return
		}
		if len(body.IDs) > 30 {
			problem(w, ErrInvalid)
			return
		}
		tx, er := h.db.BeginTx(r.Context(), nil)
		if er != nil {
			problem(w, er)
			return
		}
		defer tx.Rollback()
		var count int
		err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM academy_student_tags WHERE course_id=$1 AND id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, course, jsonValue(body.IDs)).Scan(&count)
		if err == nil && count != len(body.IDs) {
			err = ErrInvalid
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM academy_student_tag_assignments WHERE enrollment_id=$1`, id)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_student_tag_assignments(enrollment_id,tag_id,assigned_by) SELECT $1,value::bigint,$3 FROM jsonb_array_elements_text($2::jsonb)`, id, jsonValue(body.IDs), user)
		}
		if err == nil {
			err = cohortEvent(r.Context(), tx, course, e.CohortID, &id, user, "student_tags_updated", "", body, nil, nil)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
		return
	}
	fail(w, 403, "Действие недоступно")
}
func (h *Handler) crmCard(w http.ResponseWriter, r *http.Request, e *Enrollment, user int64, owner bool) {
	if err := h.progress(r.Context(), e); err != nil {
		problem(w, err)
		return
	}
	var notes, history, tags, tests, assignments json.RawMessage
	queries := []struct {
		dst *json.RawMessage
		sql string
	}{
		{&notes, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT n.id,n.text,n.revision,n.author_id,n.created_at,n.updated_at,u.full_name author_name FROM academy_student_notes n JOIN users u ON u.id=n.author_id WHERE n.enrollment_id=$1 AND n.deleted_at IS NULL)x`},
		{&history, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at DESC,x.id DESC),'[]') FROM(SELECT id,type,data,created_at FROM academy_learning_events WHERE enrollment_id=$1 UNION ALL SELECT -id,type,data,created_at FROM academy_cohort_events WHERE enrollment_id=$1)x`},
		{&tags, `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.name),'[]') FROM academy_student_tags t JOIN academy_student_tag_assignments a ON a.tag_id=t.id WHERE a.enrollment_id=$1`},
		{&tests, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT t.id,a.title,t.number,t.status,t.percent,t.passed,t.started_at,t.finished_at FROM academy_test_attempts t JOIN academy_assessment_versions a ON a.id=t.assessment_id WHERE t.enrollment_id=$1)x`},
		{&assignments, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id DESC),'[]') FROM(SELECT s.id,a.title,s.number,s.status,s.score,s.comment,s.submitted_at,s.reviewed_at FROM academy_assignment_submissions s JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.enrollment_id=$1 AND s.submitted_at IS NOT NULL)x`},
	}
	for _, q := range queries {
		if err := h.db.QueryRowContext(r.Context(), q.sql, e.ID).Scan(q.dst); err != nil {
			problem(w, err)
			return
		}
	}
	lessons := []any{}
	for _, m := range learningProgram(e.Program).Modules {
		for _, l := range m.Lessons {
			lessons = append(lessons, map[string]any{"key": l.Key, "title": l.Title, "module": m.Title, "availability": e.lessonAvailability(l.Key)})
		}
	}
	var profile string
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT '/profiles/view/'||id FROM resumes WHERE user_id=$1 AND status='published' AND visibility='public' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1),'')`, e.UserID).Scan(&profile)
	if err != nil {
		problem(w, err)
		return
	}
	metrics, err := h.studentAssessmentSummary(r.Context(), e)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"enrollment": e, "notes": notes, "history": history, "tags": tags, "tests": tests, "assignments": assignments, "lessons": lessons, "assessment_summary": metrics, "can_manage": owner, "actor_id": user, "profile_url": profile})
}
func (h *Handler) crmNotes(w http.ResponseWriter, r *http.Request, e *Enrollment, user int64, owner bool, path []string) {
	if !oneOf(r.Method, "POST", "PUT", "DELETE") {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var body struct {
		Text     string `json:"text"`
		Revision int64  `json:"revision"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Text = strings.TrimSpace(body.Text)
	if r.Method != "DELETE" && (body.Text == "" || utf8.RuneCountInString(body.Text) > 10000) {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	if e.CohortID != nil {
		if _, err = loadCohort(r.Context(), tx, *e.CohortID, e.CourseID, true); err != nil {
			problem(w, err)
			return
		}
	}
	if _, err = loadEnrollment(r.Context(), tx, e.ID, user, true, true); err != nil {
		problem(w, err)
		return
	}
	id := int64(0)
	typ := "note_added"
	if r.Method == "POST" && len(path) == 0 {
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_student_notes(enrollment_id,author_id,text) VALUES($1,$2,$3) RETURNING id`, e.ID, user, body.Text).Scan(&id)
	} else if len(path) == 1 {
		id, _ = strconv.ParseInt(path[0], 10, 64)
		var author, revision int64
		var previous string
		err = tx.QueryRowContext(r.Context(), `SELECT author_id,revision,text FROM academy_student_notes WHERE id=$1 AND enrollment_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, e.ID).Scan(&author, &revision, &previous)
		if err == nil && author != user && !owner {
			err = ErrAccess
		}
		if err == nil && revision != body.Revision {
			err = ErrConflict
		}
		if err == nil {
			typ = "note_updated"
			if r.Method == "DELETE" {
				typ = "note_deleted"
			}
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_student_notes SET text=CASE WHEN $3 THEN text ELSE $2 END,deleted_at=CASE WHEN $3 THEN now() END,revision=revision+1,updated_at=now() WHERE id=$1`, id, body.Text, r.Method == "DELETE")
		}
		if err == nil {
			err = cohortEvent(r.Context(), tx, e.CourseID, e.CohortID, &e.ID, user, typ, "", map[string]any{"note_id": id, "previous": previous, "text": body.Text}, nil, nil)
		}
	} else {
		err = ErrInvalid
	}
	if err == nil && typ == "note_added" {
		err = cohortEvent(r.Context(), tx, e.CourseID, e.CohortID, &e.ID, user, typ, "", map[string]any{"note_id": id, "text": body.Text}, nil, nil)
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
func (h *Handler) crmTags(w http.ResponseWriter, r *http.Request, course, user int64, owner bool, path []string) {
	if r.Method == "GET" {
		var raw json.RawMessage
		err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.name,t.id),'[]') FROM academy_student_tags t WHERE course_id=$1`, course).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if !owner {
		fail(w, 403, "Только владелец управляет метками")
		return
	}
	var b struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Color == "" {
		b.Color = "blue"
	}
	if r.Method != "DELETE" && (utf8.RuneCountInString(b.Name) < 1 || utf8.RuneCountInString(b.Name) > 60 || !oneOf(b.Color, "blue", "teal", "purple", "amber", "rose")) {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	id := int64(0)
	if len(path) == 0 && r.Method == "POST" {
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_student_tags(course_id,name,color,created_by) VALUES($1,$2,$3,$4) ON CONFLICT(course_id,name) DO UPDATE SET color=excluded.color RETURNING id`, course, b.Name, b.Color, user).Scan(&id)
	} else if len(path) == 1 && oneOf(r.Method, "PUT", "DELETE") {
		id, _ = strconv.ParseInt(path[0], 10, 64)
		if r.Method == "PUT" {
			var got int64
			err = tx.QueryRowContext(r.Context(), `UPDATE academy_student_tags SET name=$3,color=$4 WHERE id=$1 AND course_id=$2 RETURNING id`, id, course, b.Name, b.Color).Scan(&got)
		} else {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM academy_student_tag_assignments a USING academy_student_tags t WHERE a.tag_id=t.id AND t.id=$1 AND t.course_id=$2`, id, course)
			if err == nil {
				var got int64
				err = tx.QueryRowContext(r.Context(), `DELETE FROM academy_student_tags WHERE id=$1 AND course_id=$2 RETURNING id`, id, course).Scan(&got)
			}
		}
	} else {
		err = ErrInvalid
	}
	if err == nil {
		err = cohortEvent(r.Context(), tx, course, nil, nil, user, "tag_updated", "", map[string]any{"tag_id": id, "method": r.Method, "name": b.Name}, nil, nil)
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
