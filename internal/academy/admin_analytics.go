package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

func (h *Handler) adminOverview(w http.ResponseWriter, r *http.Request, p AdminPrincipal) {
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
	select {
	case h.analyticsSlots <- struct{}{}:
		defer func() { <-h.analyticsSlots }()
	default:
		fail(w, 429, "Аналитика уже рассчитывается. Повторите запрос позже")
		return
	}
	d := &analyticsDataset{Actor: p.User, Owner: true, Now: time.Now(), Filters: f, Settings: defaultAnalyticsSettings(), Programs: map[int64]Program{}, VersionNumbers: map[int64]int{}, Cohorts: map[int64]*Cohort{}}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	defer tx.Rollback()
	err = analyticsJSON(r.Context(), tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT e.id enrollment_id,e.user_id,u.full_name name,e.program_version_id version_id,v.number version_number,COALESCE(e.cohort_id,0) cohort_id,e.enrolled_at,e.started_at,e.completed_at,e.last_activity_at,e.last_lesson_key,CASE WHEN u.is_blocked OR u.is_system OR EXISTS(SELECT 1 FROM users owner WHERE owner.id=c.owner_id AND owner.is_blocked) THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END access_status FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN courses c ON c.id=e.course_id JOIN academy_program_versions v ON v.id=e.program_version_id WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) AND($3=0 OR e.cohort_id=$3) AND($6='' OR e.access_status=$6 OR $6='completed' AND e.completed_at IS NOT NULL) AND e.enrolled_at>=$4 AND e.enrolled_at<$5 ORDER BY e.id LIMIT 20001)x`, &d.Students, course, author, f.Cohort, f.from, f.until, r.URL.Query().Get("state"))
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if len(d.Students) > 20000 {
		fail(w, 400, "Более 20 000 зачислений: выберите автора, курс или меньший период")
		return
	}
	ids := []int64{}
	for _, s := range d.Students {
		ids = append(ids, s.ID)
	}
	rawIDs := jsonValue(ids)
	var versions []struct {
		ID      int64   `json:"id"`
		Number  int     `json:"number"`
		Program Program `json:"content"`
	}
	err = analyticsJSON(r.Context(), tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT DISTINCT v.id,v.number,v.content FROM academy_enrollments e JOIN academy_program_versions v ON v.id=e.program_version_id WHERE e.id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)))x`, &versions, rawIDs)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	for _, v := range versions {
		d.Programs[v.ID] = v.Program
		d.VersionNumbers[v.ID] = v.Number
	}
	err = analyticsJSON(r.Context(), tx, `SELECT COALESCE(jsonb_agg(to_jsonb(p)),'[]') FROM(SELECT * FROM academy_lesson_progress WHERE enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) LIMIT 500001)p`, &d.Progress, rawIDs)
	if err == nil {
		err = analyticsJSON(r.Context(), tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT enrollment_id,COALESCE(actor_id,0) user_id,type,lesson_key,created_at FROM academy_learning_events WHERE enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) AND type IN('course_started','lesson_opened','lesson_reopened','lesson_completed','test_started','test_finished','assignment_submitted','materials_acknowledged') AND created_at>=$2 AND created_at<$3 ORDER BY id LIMIT 500001)x`, &d.Events, rawIDs, f.from, f.until)
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if len(d.Events) > 500000 || len(d.Progress) > 500000 {
		fail(w, 400, "Сузьте выборку для глобального расчёта")
		return
	}
	var stats map[string]any
	err = analyticsJSON(r.Context(), tx, `SELECT jsonb_build_object('courses',count(*),'published_courses',count(*) FILTER(WHERE status='published'),'draft_courses',count(*) FILTER(WHERE status='draft'),'authors',count(DISTINCT owner_id),'active_authors',count(DISTINCT owner_id) FILTER(WHERE updated_at>=$4 AND updated_at<$5),'new_courses',count(*) FILTER(WHERE created_at>=$4 AND created_at<$5),'active_cohorts',(SELECT count(*) FROM academy_cohorts ch JOIN courses cc ON cc.id=ch.course_id WHERE ch.status='active' AND($1=0 OR cc.id=$1) AND($2=0 OR cc.owner_id=$2) AND($3=0 OR ch.id=$3)),'pending',(SELECT count(*) FROM academy_effective_assignment_submissions WHERE status IN('submitted','in_review') AND enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($6::jsonb))),'test_attempts',(SELECT count(*) FROM academy_effective_test_attempts WHERE enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($6::jsonb)) AND finished_at>=$4 AND finished_at<$5),'reviewed_assignments',(SELECT count(*) FROM academy_submission_reviews rv JOIN academy_assignment_submissions sub ON sub.id=rv.submission_id WHERE sub.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($6::jsonb)) AND rv.action<>'in_review' AND rv.created_at>=$4 AND rv.created_at<$5)) FROM courses WHERE($1=0 OR id=$1) AND($2=0 OR owner_id=$2)`, &stats, course, author, f.Cohort, f.from, f.until, rawIDs)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var recent, attention json.RawMessage
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.at DESC),'[]') FROM(SELECT * FROM(SELECT c.id::text||':course' id,'course_created' type,c.title title,c.created_at at FROM courses c WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) UNION ALL SELECT ev.id::text||':cohort',ev.type,c.title,ev.created_at FROM academy_cohort_events ev JOIN courses c ON c.id=ev.course_id WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) UNION ALL SELECT r.version_id::text||':version','version_published',c.title,r.published_at FROM academy_version_releases r JOIN courses c ON c.id=r.course_id WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) UNION ALL SELECT a.id::text||':administrative','administrative_'||a.action,c.title,a.created_at FROM academy_admin_audit a LEFT JOIN courses c ON c.id=a.course_id WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) ) all_events WHERE at>=$3 AND at<$4 ORDER BY at DESC LIMIT 30)x`, course, author, f.from, f.until).Scan(&recent)
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT c.id,c.title,(SELECT count(*) FROM academy_message_deliveries d WHERE d.course_id=c.id AND d.status='failed') notification_errors,EXISTS(SELECT 1 FROM academy_course_programs cp WHERE cp.course_id=c.id) AND NOT EXISTS(SELECT 1 FROM academy_version_releases rel WHERE rel.course_id=c.id) unpublished_program FROM courses c WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) AND(EXISTS(SELECT 1 FROM academy_message_deliveries d WHERE d.course_id=c.id AND d.status='failed') OR c.status='published' AND EXISTS(SELECT 1 FROM academy_course_programs cp WHERE cp.course_id=c.id) AND NOT EXISTS(SELECT 1 FROM academy_version_releases rel WHERE rel.course_id=c.id)) ORDER BY c.id DESC LIMIT 20)x`, course, author).Scan(&attention)
	}
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	var courseDays []struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	err = analyticsJSON(r.Context(), tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT (created_at AT TIME ZONE $5)::date::text date,count(*) count FROM courses c WHERE($1=0 OR c.id=$1) AND($2=0 OR c.owner_id=$2) AND created_at>=$3 AND created_at<$4 GROUP BY date)x`, &courseDays, course, author, f.from, f.until, f.Timezone)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		analyticsProblem(w, err)
		return
	}
	d.prepare()
	metrics := d.overview()
	active := 0
	for _, s := range d.Students {
		if s.Access == "active" {
			active++
		}
	}
	stats["active_enrollments"] = active
	for key, value := range metrics {
		if oneOf(key, "risk_students", "pending", "activity") {
			continue
		}
		stats[key] = value
	}
	chart := d.activity()
	unit := "day"
	if len(chart) > 0 {
		unit, _ = chart[0]["unit"].(string)
	}
	bucket := func(value string) string {
		t, _ := time.ParseInLocation("2006-01-02", value, mustLocation(f.Timezone))
		if unit == "week" {
			t = t.AddDate(0, 0, -(int(t.Weekday())+6)%7)
		}
		if unit == "month" {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		}
		return t.Format("2006-01-02")
	}
	series := map[string]analyticsRow{}
	for _, row := range chart {
		series[row["date"].(string)] = row
		row["courses"] = 0
		row["enrollments"] = 0
	}
	for _, v := range courseDays {
		v.Date = bucket(v.Date)
		if series[v.Date] == nil {
			series[v.Date] = analyticsRow{"date": v.Date, "active": 0, "courses": 0, "enrollments": 0}
		}
		series[v.Date]["courses"] = series[v.Date]["courses"].(int) + v.Count
	}
	for _, s := range d.Students {
		day := bucket(s.Enrolled.In(mustLocation(f.Timezone)).Format("2006-01-02"))
		if series[day] == nil {
			series[day] = analyticsRow{"date": day, "active": 0, "courses": 0, "enrollments": 0}
		}
		series[day]["enrollments"] = series[day]["enrollments"].(int) + 1
	}
	chart = []analyticsRow{}
	for _, row := range series {
		chart = append(chart, row)
	}
	sort.Slice(chart, func(i, j int) bool { return chart[i]["date"].(string) < chart[j]["date"].(string) })
	respond(w, 200, map[string]any{"metrics": stats, "activity": chart, "recent": recent, "attention": attention, "generated_at": d.Now, "notes": []string{"Учебные показатели используют SummaryFor и события Этапа F. Период определяет даты зачисления аудитории и событий; прогресс текущий.", "Курсы и авторы — весь доступный каталог; новые курсы и учебные события — выбранный период. Отсутствующие средние значения не заменяются нулём."}})
}
