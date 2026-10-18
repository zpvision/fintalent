package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) courseOwner(r *http.Request, course, user int64) bool {
	var yes bool
	_ = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND c.owner_id=$2 AND NOT u.is_blocked)`, course, user).Scan(&yes)
	return yes
}
func (h *Handler) cohortRoute(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	owner := h.courseOwner(r, course, user)
	if len(path) == 0 {
		if r.Method == "GET" {
			h.cohortList(w, r, course, user, owner)
			return
		}
		if r.Method == "POST" && owner {
			h.saveCohort(w, r, course, user, nil)
			return
		}
		fail(w, 403, "Недостаточно прав")
		return
	}
	id, _ := strconv.ParseInt(path[0], 10, 64)
	c, err := loadCohort(r.Context(), h.db, id, course, false)
	if err != nil {
		problem(w, err)
		return
	}
	if !owner {
		var allowed bool
		err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN users u ON u.id=s.user_id WHERE s.cohort_id=$1 AND s.user_id=$2 AND NOT u.is_blocked)`, id, user).Scan(&allowed)
		if err != nil || !allowed {
			problem(w, sql.ErrNoRows)
			return
		}
	}
	if len(path) >= 2 && oneOf(path[1], "preview", "assets") && oneOf(r.Method, "GET", "HEAD") {
		if !owner {
			var allowed bool
			err = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=$1 AND s.user_id=$2 AND(s.role='teacher' OR EXISTS(SELECT 1 FROM academy_enrollments e WHERE e.cohort_id=s.cohort_id AND academy_can_manage_enrollment(e.id,$2,false))))`, c.ID, user).Scan(&allowed)
			if err != nil || !allowed {
				problem(w, sql.ErrNoRows)
				return
			}
		}
		p, er := h.version(r.Context(), course, c.VersionID)
		if er != nil {
			problem(w, er)
			return
		}
		ep := Enrollment{Program: p, Cohort: c}
		normalizeCohortProgram(&ep)
		p = ep.Program
		if path[1] == "preview" && len(path) == 2 {
			var tests json.RawMessage
			er = h.db.QueryRowContext(r.Context(), `SELECT test_metadata FROM academy_version_releases WHERE version_id=$1`, c.VersionID).Scan(&tests)
			if er != nil {
				problem(w, er)
				return
			}
			h.previewRoom(w, r, course, p, tests)
			return
		}
		if path[1] == "assets" && len(path) == 3 {
			asset, _ := strconv.ParseInt(path[2], 10, 64)
			l, ok := findLesson(p, r.URL.Query().Get("lesson"))
			if !ok || !lessonAssets(l)[asset] {
				problem(w, sql.ErrNoRows)
				return
			}
			h.serveAsset(w, r, p.ID, asset)
			return
		}
	}
	if len(path) == 1 && r.Method == "GET" {
		h.cohortDetail(w, r, c, user, owner)
		return
	}
	if !owner {
		fail(w, 403, "Только владелец курса может изменять поток")
		return
	}
	if len(path) == 1 && r.Method == "PUT" {
		h.saveCohort(w, r, course, user, c)
		return
	}
	if len(path) == 2 && path[1] == "enroll" && r.Method == "POST" {
		h.cohortEnroll(w, r, c, user)
		return
	}
	if len(path) == 2 && path[1] == "staff" && oneOf(r.Method, "PUT", "DELETE") {
		h.cohortStaff(w, r, c, user)
		return
	}
	fail(w, 405, "Метод не поддерживается")
}
func (h *Handler) cohortList(w http.ResponseWriter, r *http.Request, course, user int64, owner bool) {
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.starts_at DESC,x.id DESC),'[]') FROM(SELECT c.*,v.number version_number,(SELECT COALESCE(round(avg(m.percent)),0) FROM academy_student_metrics m WHERE m.cohort_id=c.id AND academy_can_manage_enrollment(m.enrollment_id,$2,false)) average_progress,(SELECT min(ev.notify_at) FROM academy_cohort_events ev WHERE ev.cohort_id=c.id AND ev.type IN('lesson_unlocked','deadline_approaching') AND NOT ev.cancelled AND ev.delivered_at IS NULL AND ev.notify_at>now()) next_event_at,(SELECT count(*) FROM academy_enrollments e WHERE e.cohort_id=c.id AND academy_can_manage_enrollment(e.id,$2,false)) students,(SELECT count(*) FROM academy_student_metrics sm WHERE sm.cohort_id=c.id AND sm.effective_status='active' AND academy_can_manage_enrollment(sm.enrollment_id,$2,false)) active_students,(SELECT COALESCE(jsonb_agg(jsonb_build_object('name',u.full_name,'role',s.role) ORDER BY u.full_name),'[]') FROM academy_cohort_staff s JOIN users u ON u.id=s.user_id WHERE s.cohort_id=c.id) staff FROM academy_cohorts c JOIN academy_program_versions v ON v.id=c.program_version_id WHERE c.course_id=$1 AND ($3 OR EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=c.id AND s.user_id=$2)) ORDER BY c.starts_at DESC,c.id DESC)x`, course, user, owner).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
func (h *Handler) saveCohort(w http.ResponseWriter, r *http.Request, course, user int64, old *Cohort) {
	var input struct {
		Cohort
		Start string `json:"start_local"`
		End   string `json:"end_local"`
	}
	if !decode(w, r, &input) {
		return
	}
	c := input.Cohort
	c.CourseID = course
	var err error
	if input.Start != "" {
		c.StartsAt, err = cohortTime(input.Start, c.Timezone)
	}
	if err == nil && input.End != "" {
		var end time.Time
		end, err = cohortTime(input.End, c.Timezone)
		c.EndsAt = &end
	}
	if err != nil {
		problem(w, err)
		return
	}
	p, err := h.version(r.Context(), course, c.VersionID)
	if err == nil {
		err = validateCohort(&c, p)
	}
	if err != nil {
		problem(w, err)
		return
	}
	if c.Rules == nil {
		c.Rules = []CohortRule{}
	}
	if c.Deadlines == nil {
		c.Deadlines = []CohortRule{}
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	var lock int64
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM courses WHERE id=$1 AND owner_id=$2 FOR UPDATE`, course, user).Scan(&lock)
	var published bool
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_version_releases WHERE course_id=$1 AND version_id=$2)`, course, c.VersionID).Scan(&published)
	}
	if err != nil {
		problem(w, err)
		return
	}
	if !published {
		fail(w, 400, "Выберите опубликованную версию программы")
		return
	}
	for i := range c.Rules {
		if c.Rules[i].At != "" {
			at, _ := ruleTime(&c, c.Rules[i])
			c.Rules[i].At = at.Format(time.RFC3339)
		}
	}
	for i := range c.Deadlines {
		if c.Deadlines[i].At != "" {
			at, _ := ruleTime(&c, c.Deadlines[i])
			c.Deadlines[i].At = at.Format(time.RFC3339)
		}
	}
	typ := "cohort_created"
	if old == nil {
		if !keyPattern.MatchString(c.RequestKey) {
			problem(w, ErrInvalid)
			return
		}
		c.Status = "draft"
		err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_cohorts(course_id,program_version_id,name,description,type,starts_at,ends_at,timezone,capacity,schedule_mode,rules,deadlines,created_by,request_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(request_key) DO NOTHING RETURNING id`, course, c.VersionID, c.Name, c.Description, c.Type, c.StartsAt, c.EndsAt, c.Timezone, c.Capacity, c.Mode, jsonValue(c.Rules), jsonValue(c.Deadlines), user, c.RequestKey).Scan(&c.ID)
		if err == sql.ErrNoRows {
			err = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_cohorts WHERE course_id=$1 AND request_key=$2`, course, c.RequestKey).Scan(&c.ID)
		}
	} else {
		old, err = loadCohort(r.Context(), tx, old.ID, course, true)
		if err != nil {
			problem(w, err)
			return
		}
		c.ID = old.ID
		if old.Revision != c.Revision {
			problem(w, ErrConflict)
			return
		}
		if !cohortTransition(old.Status, c.Status) {
			fail(w, 409, "Недопустимый переход статуса потока")
			return
		}
		var count int
		err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM academy_enrollments WHERE cohort_id=$1`, c.ID).Scan(&count)
		if err != nil {
			problem(w, err)
			return
		}
		if count > 0 && c.VersionID != old.VersionID {
			fail(w, 409, "После первого зачисления версия потока не меняется. Создайте новый поток")
			return
		}
		if c.Capacity != nil && count > *c.Capacity {
			fail(w, 409, "Вместимость меньше числа уже зачисленных учеников")
			return
		}
		if old.Status == "archived" {
			fail(w, 409, "Архивный поток доступен только для просмотра")
			return
		}
		typ = "cohort_updated"
		if c.Status != old.Status {
			typ = map[string]string{"active": "cohort_started", "completed": "cohort_completed", "archived": "cohort_archived", "scheduled": "cohort_scheduled", "draft": "cohort_draft"}[c.Status]
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE academy_cohorts SET program_version_id=$2,name=$3,description=$4,type=$5,status=$6,starts_at=$7,ends_at=$8,timezone=$9,capacity=$10,schedule_mode=$11,rules=$12,deadlines=$13,revision=revision+1,updated_at=now() WHERE id=$1`, c.ID, c.VersionID, c.Name, c.Description, c.Type, c.Status, c.StartsAt, c.EndsAt, c.Timezone, c.Capacity, c.Mode, jsonValue(c.Rules), jsonValue(c.Deadlines))
	}
	if err == nil {
		err = cohortEvent(r.Context(), tx, course, &c.ID, nil, user, typ, "", c, nil, nil)
	}
	if err == nil {
		rows, er := tx.QueryContext(r.Context(), `SELECT id,user_id FROM academy_enrollments WHERE cohort_id=$1`, c.ID)
		if er != nil {
			err = er
		} else {
			pairs := [][2]int64{}
			for rows.Next() {
				var pair [2]int64
				if er = rows.Scan(&pair[0], &pair[1]); er != nil {
					break
				}
				pairs = append(pairs, pair)
			}
			if er == nil {
				er = rows.Err()
			}
			rows.Close()
			err = er
			for _, pair := range pairs {
				if err == nil {
					err = scheduleCohortEvents(r.Context(), tx, &c, pair[0], pair[1], user)
				}
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	saved, err := loadCohort(r.Context(), h.db, c.ID, course, false)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, saved)
}

func (h *Handler) cohortEnroll(w http.ResponseWriter, r *http.Request, c *Cohort, user int64) {
	var body struct {
		Users    []int64    `json:"user_ids"`
		Expires  *time.Time `json:"expires_at"`
		Key      string     `json:"request_key"`
		Transfer int64      `json:"transfer_enrollment_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Users) == 0 || len(body.Users) > 100 || !keyPattern.MatchString(body.Key) || body.Expires != nil && !body.Expires.After(time.Now()) || body.Transfer > 0 && len(body.Users) != 1 {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	// Course first, cohort second: all grants/transfers/settings share this lock order.
	var locked int64
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM courses WHERE id=$1 AND owner_id=$2 FOR UPDATE`, c.CourseID, user).Scan(&locked)
	if err == nil {
		c, err = loadCohort(r.Context(), tx, c.ID, c.CourseID, true)
	}
	if err != nil {
		problem(w, err)
		return
	}
	if oneOf(c.Status, "completed", "archived") {
		fail(w, 409, "Зачисление в завершённый или архивный поток недоступно")
		return
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, student := range body.Users {
		if seen[student] {
			continue
		}
		seen[student] = true
		var valid bool
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system)`, student).Scan(&valid)
		if err != nil {
			problem(w, err)
			return
		}
		if !valid {
			fail(w, 400, "Выберите зарегистрированного пользователя с действующим аккаунтом")
			return
		}
		var id int64
		var previousKey, status string
		var expiration *time.Time
		requestKey := "cohort:" + body.Key + ":" + strconv.FormatInt(student, 10)
		err = tx.QueryRowContext(r.Context(), `SELECT id,request_key,access_status,expires_at FROM academy_enrollments WHERE cohort_id=$1 AND user_id=$2`, c.ID, student).Scan(&id, &previousKey, &status, &expiration)
		if err == nil {
			if body.Transfer > 0 && previousKey != requestKey {
				fail(w, 409, "Ученик уже зачислен в выбранный поток. Управляйте существующим доступом; прежнее обучение не изменено")
				return
			}
			if status != "active" || expiration != nil && !expiration.After(time.Now()) {
				fail(w, 409, "У ученика уже есть зачисление в этот поток. Восстановите или продлите доступ в его карточке")
				return
			}
		}
		if err == sql.ErrNoRows {
			var count int
			err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM academy_enrollments WHERE cohort_id=$1`, c.ID).Scan(&count)
			if err != nil {
				problem(w, err)
				return
			}
			if c.Capacity != nil && count >= *c.Capacity {
				fail(w, 409, "В потоке нет свободных мест. Никто из выбранной группы не был зачислен")
				return
			}
			err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_enrollments(course_id,user_id,program_version_id,cohort_id,expires_at,source,request_key,enrolled_by) VALUES($1,$2,$3,$4,$5,'cohort_manual',$6,$7) RETURNING id`, c.CourseID, student, c.VersionID, c.ID, body.Expires, "cohort:"+body.Key+":"+strconv.FormatInt(student, 10), user).Scan(&id)
			if err == nil {
				now := time.Now()
				err = event(r.Context(), tx, &Enrollment{ID: id}, user, "student_enrolled", "enrolled", "", map[string]any{"cohort_id": c.ID, "version_id": c.VersionID})
				if err == nil {
					err = cohortEvent(r.Context(), tx, c.CourseID, &c.ID, &id, user, "student_enrolled", "enrolled:"+strconv.FormatInt(id, 10), map[string]any{"cohort": c.Name}, &student, &now)
				}
			}
			if err == nil {
				err = scheduleCohortEvents(r.Context(), tx, c, id, student, user)
			}
		}
		if err != nil {
			problem(w, err)
			return
		}
		ids = append(ids, id)
		if body.Transfer > 0 {
			old, er := loadEnrollment(r.Context(), tx, body.Transfer, user, true, true)
			if er != nil {
				problem(w, er)
				return
			}
			if old.CourseID != c.CourseID || old.UserID != student || old.ID == id {
				problem(w, ErrInvalid)
				return
			}
			// Retry is safe: never erase old progress or manufacture results in the new enrollment.
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_enrollments SET access_status='revoked' WHERE id=$1`, old.ID)
			if err == nil {
				err = event(r.Context(), tx, old, user, "student_transferred", "transfer:"+strconv.FormatInt(id, 10), "", map[string]any{"new_enrollment_id": id, "cohort_id": c.ID})
			}
			if err == nil {
				now := time.Now()
				err = cohortEvent(r.Context(), tx, c.CourseID, &c.ID, &id, user, "student_transferred", "transfer:"+strconv.FormatInt(old.ID, 10)+":"+strconv.FormatInt(id, 10), map[string]any{"previous_enrollment_id": old.ID}, &student, &now)
			}
		}
		if err != nil {
			problem(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"ids": ids})
}
func (h *Handler) cohortStaff(w http.ResponseWriter, r *http.Request, c *Cohort, user int64) {
	var body struct {
		User        int64  `json:"user_id"`
		Role        string `json:"role"`
		Review      bool   `json:"can_review"`
		Communicate bool   `json:"can_communicate"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.User <= 0 || r.Method == "PUT" && !oneOf(body.Role, "teacher", "curator") {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	_, err = loadCohort(r.Context(), tx, c.ID, c.CourseID, true)
	if err == nil && r.Method == "PUT" {
		var valid bool
		err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system)`, body.User).Scan(&valid)
		if err == nil && !valid {
			err = ErrInvalid
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_cohort_staff(cohort_id,user_id,role,can_review,assigned_by,can_communicate) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(cohort_id,user_id) DO UPDATE SET role=excluded.role,can_review=excluded.can_review,can_communicate=excluded.can_communicate,assigned_by=excluded.assigned_by,assigned_at=now()`, c.ID, body.User, body.Role, body.Review, user, body.Communicate)
		}
	}
	if err == nil && r.Method == "DELETE" {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM academy_student_curators sc USING academy_enrollments e WHERE e.id=sc.enrollment_id AND e.cohort_id=$1 AND sc.user_id=$2`, c.ID, body.User)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM academy_cohort_staff WHERE cohort_id=$1 AND user_id=$2`, c.ID, body.User)
		}
	}
	if err == nil {
		err = cohortEvent(r.Context(), tx, c.CourseID, &c.ID, nil, user, "staff_updated", "", body, nil, nil)
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
func queryInt(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get(key)), 10, 64)
	return v
}
