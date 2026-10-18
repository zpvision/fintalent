package academy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func analyticsProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		fail(w, 503, "Расчёт занял слишком много времени. Сузьте период или поток и повторите запрос")
		return
	}
	var in inputError
	if !errors.Is(err, ErrAccess) && !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrConflict) && !errors.Is(err, sql.ErrNoRows) && !errors.As(err, &in) {
		log.Printf("academy analytics: %v", err)
		fail(w, 500, "Не удалось подготовить аналитику. Повторите запрос позднее")
		return
	}
	problem(w, err)
}
func (h *Handler) analyticsRoute(w http.ResponseWriter, r *http.Request, course, actor int64, path []string) {
	section := "overview"
	if len(path) > 0 {
		section = path[0]
	}
	if section == "reports" {
		h.analyticsReports(w, r, course, actor, path[1:])
		return
	}
	if section == "settings" {
		h.analyticsSettingsRoute(w, r, course, actor)
		return
	}
	if len(path) > 2 {
		fail(w, 404, "Раздел аналитики не найден")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if !oneOf(section, "overview", "activity", "funnel", "students", "lessons", "modules", "tests", "assignments", "cohorts", "staff", "communications", "context") {
		fail(w, 404, "Раздел аналитики не найден")
		return
	}
	f, err := parseAnalyticsFilters(r.URL.Query(), time.Now())
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if section == "context" {
		if len(path) != 2 {
			fail(w, 404, "Укажите зачисление")
			return
		}
		id, e := strconv.ParseInt(path[1], 10, 64)
		if e != nil || id <= 0 {
			analyticsProblem(w, sql.ErrNoRows)
			return
		}
		f.Enrollment = id
		f.Period = "all"
		f, err = parseAnalyticsFilters(f.values(), time.Now())
		if err != nil {
			analyticsProblem(w, err)
			return
		}
	}
	select {
	case h.analyticsSlots <- struct{}{}:
		defer func() { <-h.analyticsSlots }()
	default:
		fail(w, 429, "Аналитика уже рассчитывается. Повторите запрос через несколько секунд")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	start := time.Now()
	communications := section == "communications" || section == "context" && administrative(ctx)
	if section == "context" && !administrative(ctx) {
		err = h.db.QueryRowContext(ctx, `SELECT academy_can_communicate($1,$2)`, f.Enrollment, actor).Scan(&communications)
		if err != nil {
			analyticsProblem(w, err)
			return
		}
	}
	d, err := h.loadAnalytics(ctx, course, actor, f, communications, nil)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	if section == "context" {
		d.Communications = communications
		value, e := d.studentContext(f.Enrollment)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		respond(w, 200, value)
		return
	}
	detail := ""
	if len(path) == 2 {
		detail = path[1]
	}
	rows := []analyticsRow{}
	data := map[string]any{}
	switch section {
	case "overview":
		data = d.overview()
		lessons := d.lessonRows("")
		sort.SliceStable(lessons, func(i, j int) bool { return lessons[i]["stopped"].(int) > lessons[j]["stopped"].(int) })
		data["stops"] = firstRows(positiveRows(lessons, "stopped"), 5)
		tests := d.testRows(0)
		sort.SliceStable(tests, func(i, j int) bool { return rateValue(tests[i]["success_rate"]) < rateValue(tests[j]["success_rate"]) })
		data["difficult_tests"] = firstRows(positiveRows(tests, "attempts"), 5)
		works := d.assignmentRows(0)
		sort.SliceStable(works, func(i, j int) bool {
			return rateValue(works[i]["revision_rate"]) > rateValue(works[j]["revision_rate"])
		})
		data["revised_assignments"] = firstRows(positiveRows(works, "revision_decisions"), 5)
	case "activity":
		data["activity"] = d.activity()
		if detail != "" {
			at, e := time.ParseInLocation("2006-01-02", detail, mustLocation(f.Timezone))
			if e != nil {
				analyticsProblem(w, ErrInvalid)
				return
			}
			end := at.AddDate(0, 0, 1)
			switch r.URL.Query().Get("unit") {
			case "week":
				end = at.AddDate(0, 0, 7)
			case "month":
				end = at.AddDate(0, 1, 0)
			case "", "day":
			default:
				analyticsProblem(w, ErrInvalid)
				return
			}
			seen := map[int64]bool{}
			for _, ev := range d.Events {
				student := d.ByID[ev.Enrollment]
				if student != nil && ev.User == student.User && f.in(ev.At) && !ev.At.Before(at) && ev.At.Before(end) {
					seen[student.ID] = true
				}
			}
			for _, student := range d.Students {
				if seen[student.ID] {
					rows = append(rows, analyticsRow{"student": student})
				}
			}
		}

	case "funnel":
		data["funnel"] = d.overview()["funnel"]
	case "students":
		for _, s := range d.Students {
			rows = append(rows, analyticsRow{"student": s})
		}
	case "lessons":
		if detail != "" && f.Version == 0 {
			fail(w, 400, "Для детализации выберите опубликованную версию программы")
			return
		}
		rows = d.lessonRows(detail)
		if detail != "" && len(rows) == 1 {
			page := 1
			if value := r.URL.Query().Get("student_page"); value != "" {
				page, err = strconv.Atoi(value)
				if err != nil || page < 1 || page > 100000 {
					analyticsProblem(w, ErrInvalid)
					return
				}
			}
			students := rows[0]["students"].([]analyticsRow)
			total := len(students)
			start := min(total, (page-1)*50)
			rows[0]["students"] = students[start:min(total, start+50)]
			rows[0]["student_total"], rows[0]["student_page"] = total, page
		}
		data["modules"] = d.moduleRows()
	case "modules":
		rows = d.moduleRows()
	case "tests", "assignments":
		id := int64(0)
		if detail != "" {
			id, err = strconv.ParseInt(detail, 10, 64)
			if err != nil || id <= 0 {
				analyticsProblem(w, sql.ErrNoRows)
				return
			}
		}
		if section == "tests" {
			rows = d.testRows(id)
		} else {
			rows = d.assignmentRows(id)
		}
	case "cohorts":
		rows = d.cohortRows()
		versions := map[int64]bool{}
		for _, row := range rows {
			versions[row["version_id"].(int64)] = true
		}
		data["different_versions"] = len(versions) > 1
		data["comparison_basis"] = "Текущий прогресс выбранной аудитории; события за календарный период"
		if f.Compare == "relative" {
			data["comparison_basis"] = fmt.Sprintf("Первое успешное завершение уроков в первые %d дней. Это достигнутый рубеж, а не пересчёт текущего прогресса", f.Days)
		}
	case "staff":
		rows = d.staffRows()
	case "communications":
		data = d.communications()
		rows = data["rows"].([]analyticsRow)
		delete(data, "rows")
	}
	if detail != "" && section != "activity" && len(rows) == 0 {
		analyticsProblem(w, sql.ErrNoRows)
		return
	}
	total := len(rows)
	from := min(total, (f.Page-1)*f.Size)
	until := min(total, from+f.Size)
	rows = rows[from:until]
	versions := []analyticsRow{}
	for id, p := range d.Programs {
		versions = append(versions, analyticsRow{"id": id, "number": d.VersionNumbers[id], "modules": len(p.Modules), "lessons": len(publishedLessons(p, true))})
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i]["id"].(int64) > versions[j]["id"].(int64) })
	cohorts := []analyticsRow{}
	for id, c := range d.Cohorts {
		cohorts = append(cohorts, analyticsRow{"id": id, "name": c.Name, "version_id": c.VersionID})
	}
	sort.Slice(cohorts, func(i, j int) bool { return cohorts[i]["id"].(int64) > cohorts[j]["id"].(int64) })
	curators := []analyticsRow{}
	seen := map[int64]bool{}
	for _, s := range d.Staff {
		if !seen[s.ID] && (d.Owner || s.ID == actor) {
			curators = append(curators, analyticsRow{"id": s.ID, "name": s.Name})
			seen[s.ID] = true
		}
	}
	var courses []analyticsRow
	err = analyticsJSON(ctx, h.db, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',c.id,'title',c.title) ORDER BY c.title),'[]') FROM courses c JOIN users owner ON owner.id=c.owner_id WHERE($2 OR NOT owner.is_blocked) AND($2 OR c.owner_id=$1 OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$1))`, &courses, actor, administrative(ctx))
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	retention, err := academySettings(ctx, h.db)
	if err != nil {
		analyticsProblem(w, err)
		return
	}
	elapsed := time.Since(start).Milliseconds()
	w.Header().Set("Server-Timing", fmt.Sprintf("analytics;dur=%d", elapsed))
	respond(w, 200, map[string]any{"section": section, "data": data, "rows": rows, "total": total, "page": f.Page, "size": f.Size, "filters": f, "settings": d.Settings, "generated_at": d.Now, "elapsed_ms": elapsed, "meta": map[string]any{"versions": versions, "cohorts": cohorts, "curators": curators, "courses": courses, "owner": d.Owner, "communications": d.Communications, "report_days": retention.ReportDays}, "notes": []string{"Период выбирает дату зачисления аудитории. Активность, попытки, проверки и сообщения считаются за этот период. Прогресс, доступ и риск — текущее состояние выбранной аудитории.", "Закрытые по расписанию уроки не считаются остановкой. Проценты без знаменателя не рассчитываются.", "Риск — сигнал для помощи, а не оценка способностей; доступ автоматически не изменяется."}})
}
func firstRows(rows []analyticsRow, n int) []analyticsRow {
	if len(rows) > n {
		return rows[:n]
	}
	return rows
}
func rateValue(v any) float64 {
	if p, ok := v.(*float64); ok && p != nil {
		return *p
	}
	return 101
}
func (h *Handler) analyticsSettingsRoute(w http.ResponseWriter, r *http.Request, course, actor int64) {
	if r.Method == "GET" {
		s, e := analyticsSettings(r.Context(), h.db, course)
		if e != nil {
			analyticsProblem(w, e)
			return
		}
		respond(w, 200, s)
		return
	}
	if r.Method != "PUT" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if !h.courseOwner(r, course, actor) {
		fail(w, 403, "Пороги настраивает владелец курса")
		return
	}
	var s AnalyticsSettings
	if !decode(w, r, &s) {
		return
	}
	if e := s.validate(); e != nil {
		analyticsProblem(w, e)
		return
	}
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(hashtext('academy:analytics-settings'),$1::integer)`, course)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	old, e := analyticsSettings(r.Context(), tx, course)
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	if old.Revision != s.Revision {
		analyticsProblem(w, ErrConflict)
		return
	}
	s.Revision++
	_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_analytics_settings(course_id,settings,revision,updated_by) VALUES($1,$2,$3,$4) ON CONFLICT(course_id) DO UPDATE SET settings=excluded.settings,revision=excluded.revision,updated_by=excluded.updated_by,updated_at=now()`, course, jsonValue(s), s.Revision, actor)
	if e == nil {
		e = communicationAudit(r.Context(), tx, course, actor, "analytics_settings", course, "updated", s)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		analyticsProblem(w, e)
		return
	}
	respond(w, 200, s)
}

func reportCommunicationKind(kind string) bool { return strings.EqualFold(kind, "communications") }

func positiveRows(rows []analyticsRow, key string) []analyticsRow {
	out := []analyticsRow{}
	for _, r := range rows {
		if v, ok := r[key].(int); ok && v > 0 {
			out = append(out, r)
		}
	}
	return out
}
