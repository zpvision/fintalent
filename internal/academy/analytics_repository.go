package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// All source reads share one repeatable-read snapshot. No row is loaded before its enrollment scope is checked.
func analyticsJSON(ctx context.Context, q queryer, query string, target any, args ...any) error {
	var raw []byte
	if err := q.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
func analyticsSettings(ctx context.Context, q queryer, course int64) (AnalyticsSettings, error) {
	s := defaultAnalyticsSettings()
	var raw []byte
	var rev int64
	err := q.QueryRowContext(ctx, `SELECT settings,revision FROM academy_analytics_settings WHERE course_id=$1`, course).Scan(&raw, &rev)
	if err == sql.ErrNoRows {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s)
	s.Revision = rev
	return s, err
}

func (h *Handler) loadAnalytics(ctx context.Context, course, actor int64, f AnalyticsFilters, communications bool, settings *AnalyticsSettings) (*analyticsDataset, error) {
	tx, err := h.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &analyticsDataset{Course: course, Actor: actor, Filters: f, Now: time.Now(), Programs: map[int64]Program{}, VersionNumbers: map[int64]int{}, Cohorts: map[int64]*Cohort{}}
	var allowed bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=$2 JOIN users owner ON owner.id=c.owner_id WHERE c.id=$1 AND NOT owner.is_blocked AND NOT u.is_blocked AND NOT u.is_system AND(c.owner_id=$2 OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$2))),EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$2),EXISTS(SELECT 1 FROM courses c WHERE c.id=$1 AND(c.owner_id=$2 OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$2 AND s.can_communicate)))`, course, actor).Scan(&allowed, &d.Owner, &d.Communications)
	if err != nil {
		return nil, err
	}
	if adminPrincipal(ctx).Can("analytics") {
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM courses WHERE id=$1)", course).Scan(&allowed); err != nil {
			return nil, err
		}
		d.Owner = true
		d.Communications = true
	}
	if !allowed {
		return nil, sql.ErrNoRows
	}
	if communications && !d.Communications {
		return nil, ErrAccess
	}
	d.Settings, err = analyticsSettings(ctx, tx, course)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		d.Settings = *settings
	}
	if f.Cohort > 0 {
		var yes bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_cohorts ch WHERE ch.id=$1 AND ch.course_id=$2 AND($3 OR EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=ch.id AND s.user_id=$4)))`, f.Cohort, course, d.Owner, actor).Scan(&yes)
		if err != nil {
			return nil, err
		}
		if !yes {
			return nil, sql.ErrNoRows
		}
	}
	var versions []struct {
		ID      int64   `json:"id"`
		Number  int     `json:"number"`
		Program Program `json:"content"`
	}
	err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id),'[]') FROM(SELECT v.id,v.number,v.content FROM academy_program_versions v JOIN academy_version_releases r ON r.version_id=v.id WHERE r.course_id=$1 AND($3 OR EXISTS(SELECT 1 FROM academy_cohorts ch JOIN academy_cohort_staff s ON s.cohort_id=ch.id WHERE ch.course_id=$1 AND ch.program_version_id=v.id AND s.user_id=$2)) AND($4=0 OR v.id=$4))x`, &versions, course, actor, d.Owner, f.Version)
	if err != nil {
		return nil, err
	}
	if f.Version > 0 && len(versions) == 0 {
		return nil, sql.ErrNoRows
	}
	for _, v := range versions {
		d.Programs[v.ID] = v.Program
		d.VersionNumbers[v.ID] = v.Number
	}
	err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.enrollment_id),'[]') FROM(SELECT e.id enrollment_id,e.user_id,u.full_name name,COALESCE(u.avatar_url,'') avatar,e.program_version_id version_id,v.number version_number,COALESCE(e.cohort_id,0) cohort_id,COALESCE(ch.name,'') cohort_name,COALESCE(sc.user_id,0) curator_id,COALESCE(curator.full_name,'') curator_name,e.enrolled_at,e.started_at,e.completed_at,e.last_activity_at,e.last_lesson_key,CASE WHEN u.is_blocked OR u.is_system THEN 'suspended' WHEN e.access_status='active' AND e.expires_at<=now() THEN 'expired' ELSE e.access_status END access_status FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN academy_program_versions v ON v.id=e.program_version_id LEFT JOIN academy_cohorts ch ON ch.id=e.cohort_id LEFT JOIN academy_student_curators sc ON sc.enrollment_id=e.id LEFT JOIN users curator ON curator.id=sc.user_id WHERE e.course_id=$1 AND($9 OR academy_can_manage_enrollment(e.id,$2,false)) AND($3=0 OR e.cohort_id=$3 OR $3=-1 AND e.cohort_id IS NULL) AND($4=0 OR e.program_version_id=$4) AND($5=0 OR sc.user_id=$5) AND e.enrolled_at>=$6 AND e.enrolled_at<$7 AND($8=0 OR e.id=$8) ORDER BY e.id LIMIT 20001)x`, &d.Students, course, actor, f.Cohort, f.Version, f.Curator, f.from, f.until, f.Enrollment, adminPrincipal(ctx).Can("analytics"))
	if err != nil {
		return nil, err
	}
	if len(d.Students) > 20000 {
		return nil, inputError{"Более 20 000 зачислений: сузьте период, поток или версию программы"}
	}
	if d.Students == nil {
		d.Students = []*AnalyticsStudent{}
	}
	ids := []int64{}
	for _, s := range d.Students {
		ids = append(ids, s.ID)
	}
	idJSON := jsonValue(ids)
	var cohorts []*Cohort
	err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(ch)||jsonb_build_object('version_number',v.number) ORDER BY ch.id),'[]') FROM academy_cohorts ch JOIN academy_program_versions v ON v.id=ch.program_version_id WHERE ch.course_id=$1 AND($3 OR EXISTS(SELECT 1 FROM academy_cohort_staff s WHERE s.cohort_id=ch.id AND s.user_id=$2)) AND($4=0 OR ch.id=$4) AND($5=0 OR ch.program_version_id=$5)`, &cohorts, course, actor, d.Owner, f.Cohort, f.Version)
	if err != nil {
		return nil, err
	}
	for _, c := range cohorts {
		d.Cohorts[c.ID] = c
	}
	err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('user_id',s.user_id,'name',u.full_name,'role',s.role,'cohort_id',s.cohort_id,'can_review',s.can_review) ORDER BY s.cohort_id,s.user_id),'[]') FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id JOIN users u ON u.id=s.user_id WHERE ch.course_id=$1 AND($3 OR s.user_id=$2 OR EXISTS(SELECT 1 FROM academy_cohort_staff own WHERE own.cohort_id=ch.id AND own.user_id=$2 AND own.role='teacher')) AND($4=0 OR ch.id=$4)`, &d.Staff, course, actor, d.Owner, f.Cohort)
	if err != nil {
		return nil, err
	}
	if d.Owner {
		var name string
		ownerID := actor
		query := "SELECT full_name FROM users WHERE id=$1"
		if administrative(ctx) {
			if err = tx.QueryRowContext(ctx, "SELECT owner_id FROM courses WHERE id=$1", course).Scan(&ownerID); err != nil {
				return nil, err
			}
		}
		if err = tx.QueryRowContext(ctx, query, ownerID).Scan(&name); err != nil {
			return nil, err
		}
		d.Staff = append(d.Staff, analyticsStaff{ID: ownerID, Name: name, Role: "owner", CanReview: true})
	}
	if len(ids) > 0 {
		queries := []struct {
			query  string
			target any
			limit  int
		}{
			{`SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT p.* FROM academy_lesson_progress p WHERE p.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) LIMIT 500001)x`, &d.Progress, 500000},
			{`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at),'[]') FROM(SELECT enrollment_id,COALESCE(actor_id,0) user_id,type,lesson_key,created_at FROM academy_learning_events WHERE enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) AND type IN('lesson_opened','lesson_completed','course_started','course_completed','test_started','test_finished','assignment_submitted','materials_acknowledged') ORDER BY id LIMIT 500001)x`, &d.Events, 500000},
			{`SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT t.id,t.enrollment_id,t.assessment_id,t.number,t.status,t.percent,t.score,t.passed,t.started_at,t.finished_at,t.grades,COALESCE((SELECT jsonb_agg(DISTINCT(a->>'question_id')::bigint) FROM jsonb_array_elements(t.answers) a),'[]') answered FROM academy_effective_test_attempts t WHERE t.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) ORDER BY t.id LIMIT 200001)x`, &d.Tests, 200000},
			{`SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT id,enrollment_id,assessment_id,number,status,submitted_at,score,COALESCE(reviewed_by,0) reviewed_by,reviewed_at,COALESCE(assigned_to,0) assigned_to FROM academy_effective_assignment_submissions WHERE submitted_at IS NOT NULL AND enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) ORDER BY id LIMIT 200001)x`, &d.Submissions, 200000},
			{`SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT r.id,r.submission_id,COALESCE(r.reviewer_id,0) reviewer_id,COALESCE(u.full_name,'Автоматическая проверка') name,r.action,r.created_at,r.score,r.criterion_scores FROM academy_submission_reviews r JOIN academy_assignment_submissions sub ON sub.id=r.submission_id LEFT JOIN users u ON u.id=r.reviewer_id WHERE sub.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) ORDER BY r.id LIMIT 300001)x`, &d.Reviews, 300000},
		}
		for _, q := range queries {
			if err = analyticsJSON(ctx, tx, q.query, q.target, idJSON); err != nil {
				return nil, err
			}
		}
		if len(d.Progress) > 500000 || len(d.Events) > 500000 || len(d.Tests) > 200000 || len(d.Submissions) > 200000 || len(d.Reviews) > 300000 {
			return nil, inputError{"Слишком большой объём событий: выберите меньшую аудиторию"}
		}
	}
	vids := []int64{}
	for id := range d.Programs {
		vids = append(vids, id)
	}
	err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id),'[]') FROM(SELECT a.id,a.program_version_id,a.lesson_key,a.block_key,a.kind,a.title,a.config,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q->'id','question',q->'question')) FROM jsonb_array_elements(a.questions) q),'[]') questions FROM academy_assessment_versions a WHERE a.program_version_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)))x`, &d.Assessments, jsonValue(vids))
	if err != nil {
		return nil, err
	}
	var runs []struct {
		Enrollment int64 `json:"enrollment_id"`
		Count      int   `json:"count"`
	}
	if communications && len(ids) > 0 {
		err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT d.id,d.enrollment_id,d.recipient_id,COALESCE(d.campaign_id,0) campaign_id,COALESCE(r.automation_id,0) automation_id,COALESCE(c.name,a.name,'Сообщение') name,d.channel,d.status,d.queued_at,d.sent_at,d.error_code,COALESCE(d.run_id,0) run_id FROM academy_message_deliveries d JOIN academy_enrollments e ON e.id=d.enrollment_id LEFT JOIN academy_campaigns c ON c.id=d.campaign_id LEFT JOIN academy_automation_runs r ON r.id=d.run_id LEFT JOIN academy_automations a ON a.id=r.automation_id WHERE d.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) AND NOT d.test AND($3 OR academy_can_communicate(e.id,$2)) ORDER BY d.id LIMIT 200001)x`, &d.Deliveries, idJSON, actor, administrative(ctx))
		if err != nil {
			return nil, err
		}
		if len(d.Deliveries) > 200000 {
			return nil, inputError{"Слишком много сообщений: выберите меньшую аудиторию"}
		}
		err = analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(to_jsonb(x)),'[]') FROM(SELECT r.enrollment_id,count(*) count FROM academy_automation_runs r WHERE r.enrollment_id IN(SELECT value::bigint FROM jsonb_array_elements_text($1::jsonb)) AND($5 OR academy_can_communicate(r.enrollment_id,$2)) AND r.created_at>=$3 AND r.created_at<$4 AND r.skipped_reason='' GROUP BY r.enrollment_id)x`, &runs, idJSON, actor, f.from, f.until, administrative(ctx))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// Bound the population/program cross product in addition to individual source row limits.
	lessonCounts, assessmentCounts := map[int64]int{}, map[int64]int{}
	for id, program := range d.Programs {
		lessonCounts[id] = len(publishedLessons(program, true))
	}
	for _, assessment := range d.Assessments {
		assessmentCounts[assessment.Version]++
	}
	work := 0
	for _, student := range d.Students {
		work += lessonCounts[student.Version] + 2*assessmentCounts[student.Version]
	}
	if work > 5000000 {
		return nil, inputError{"Слишком большая аудитория для этой программы: выберите поток, версию или меньший период"}
	}
	d.prepare()
	filtered := []*AnalyticsStudent{}
	for _, s := range d.Students {
		if analyticsStudentMatch(s, f, d.Settings) {
			filtered = append(filtered, s)
		} else {
			delete(d.ByID, s.ID)
		}
	}
	d.Students = filtered
	for _, run := range runs {
		if d.ByID[run.Enrollment] != nil {
			d.Runs += run.Count
		}
	}
	return d, nil
}
