package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const latestSubmission = `NOT EXISTS(SELECT 1 FROM academy_assignment_submissions newer WHERE newer.enrollment_id=s.enrollment_id AND newer.assessment_id=s.assessment_id AND newer.number>s.number AND newer.submitted_at IS NOT NULL)`

func (h *Handler) reviewQueue(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) > 0 {
		h.reviewItem(w, r, course, user, path)
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && !oneOf(status, "pending", "revision_requested", "accepted", "rejected") {
		problem(w, ErrInvalid)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 || offset > 100000 {
		problem(w, ErrInvalid)
		return
	}
	student, _ := strconv.ParseInt(r.URL.Query().Get("student"), 10, 64)
	cohort := queryInt(r, "cohort")
	mine := r.URL.Query().Get("mine") == "1"
	order := "s.submitted_at ASC,s.id ASC"
	if r.URL.Query().Get("sort") == "newest" {
		order = "s.submitted_at DESC,s.id DESC"
	}
	var items, stats, options json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(x.payload ORDER BY x.ord),'[]') FROM(SELECT row_number() OVER(ORDER BY `+order+`) ord,jsonb_build_object('id',s.id,'enrollment_id',e.id,'user_id',e.user_id,'user_name',u.full_name,'avatar',COALESCE(u.avatar_url,''),'title',a.title,'lesson_key',a.lesson_key,'lesson_title',l->>'title','module_key',m->>'key','module_title',m->>'title','course_title',c.title,'submitted_at',s.submitted_at,'deadline_at',s.submitted_deadline_at,'is_late',s.is_late,'number',s.number,'status',s.status,'reviewer_name',COALESCE(reviewer.full_name,''),'score',s.score,'max_score',(a.config->'assessment'->>'max_score')::numeric,'version_number',v.number) payload FROM academy_assignment_submissions s JOIN academy_enrollments e ON e.id=s.enrollment_id JOIN courses c ON c.id=e.course_id JOIN users u ON u.id=e.user_id JOIN academy_assessment_versions a ON a.id=s.assessment_id JOIN academy_program_versions v ON v.id=e.program_version_id JOIN LATERAL jsonb_array_elements(v.content->'modules') m ON true JOIN LATERAL jsonb_array_elements(m->'lessons') l ON l->>'key'=a.lesson_key LEFT JOIN users reviewer ON reviewer.id=COALESCE(s.reviewed_by,s.assigned_to) WHERE c.id=$1 AND academy_can_manage_enrollment(e.id,$2,true) AND s.submitted_at IS NOT NULL AND `+latestSubmission+` AND ($3='' OR ($3='pending' AND s.status IN('submitted','in_review')) OR s.status=$3) AND ($4='' OR a.lesson_key=$4) AND ($5=0 OR e.user_id=$5) AND ($6='' OR m->>'key'=$6) AND ($8=0 OR e.cohort_id=$8) AND (NOT $9 OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=$2)) ORDER BY `+order+` LIMIT 100 OFFSET $7)x`, course, user, status, r.URL.Query().Get("lesson"), student, r.URL.Query().Get("module"), offset, cohort, mine).Scan(&items)
	if err != nil {
		problem(w, err)
		return
	}
	err = h.db.QueryRowContext(r.Context(), `WITH latest AS(SELECT s.* FROM academy_assignment_submissions s JOIN academy_enrollments e ON e.id=s.enrollment_id WHERE e.course_id=$1 AND ($3=0 OR e.cohort_id=$3) AND (NOT $4 OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=$2)) AND academy_can_manage_enrollment(e.id,$2,true) AND s.submitted_at IS NOT NULL AND `+latestSubmission+`) SELECT jsonb_build_object('pending',count(*) FILTER(WHERE status IN('submitted','in_review')),'revision',count(*) FILTER(WHERE status='revision_requested'),'accepted',count(*) FILTER(WHERE status='accepted'),'rejected',count(*) FILTER(WHERE status='rejected'),'reviewed',(SELECT count(*) FROM academy_submission_reviews r JOIN academy_assignment_submissions s ON s.id=r.submission_id JOIN academy_enrollments e ON e.id=s.enrollment_id WHERE e.course_id=$1 AND ($3=0 OR e.cohort_id=$3) AND (NOT $4 OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=$2)) AND academy_can_manage_enrollment(e.id,$2,true) AND r.action<>'in_review'),'success_percent',CASE WHEN count(*)>0 THEN round(100.0*count(*) FILTER(WHERE status='accepted')/count(*)) END,'test_average',(SELECT round(avg(t.percent),2) FROM academy_test_attempts t JOIN academy_enrollments e ON e.id=t.enrollment_id WHERE e.course_id=$1 AND ($3=0 OR e.cohort_id=$3) AND (NOT $4 OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=$2)) AND academy_can_manage_enrollment(e.id,$2,true) AND t.status='finished')) FROM latest`, course, user, cohort, mine).Scan(&stats)
	if err != nil {
		problem(w, err)
		return
	}
	err = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.module_title,x.lesson_title),'[]') FROM(SELECT DISTINCT ON(m->>'key',l->>'key') m->>'key' module_key,m->>'title' module_title,l->>'key' lesson_key,l->>'title' lesson_title FROM academy_enrollments e JOIN academy_program_versions v ON v.id=e.program_version_id JOIN LATERAL jsonb_array_elements(v.content->'modules') m ON true JOIN LATERAL jsonb_array_elements(m->'lessons') l ON true WHERE e.course_id=$1 AND ($3=0 OR e.cohort_id=$3) AND (NOT $4 OR EXISTS(SELECT 1 FROM academy_student_curators sc WHERE sc.enrollment_id=e.id AND sc.user_id=$2)) AND academy_can_manage_enrollment(e.id,$2,true) AND m->>'status'='published' AND l->>'status'='published' ORDER BY m->>'key',l->>'key',v.number DESC)x`, course, user, cohort, mine).Scan(&options)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"items": items, "stats": stats, "options": options})
}
func (h *Handler) reviewItem(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	id, err := strconv.ParseInt(path[0], 10, 64)
	if err != nil {
		problem(w, sql.ErrNoRows)
		return
	}
	var eid int64
	var lesson, block string
	err = h.db.QueryRowContext(r.Context(), `SELECT e.id,a.lesson_key,a.block_key FROM academy_assignment_submissions s JOIN academy_enrollments e ON e.id=s.enrollment_id JOIN courses c ON c.id=e.course_id JOIN academy_assessment_versions a ON a.id=s.assessment_id WHERE s.id=$1 AND c.id=$2 AND academy_can_manage_enrollment(e.id,$3,true) AND s.submitted_at IS NOT NULL`, id, course, user).Scan(&eid, &lesson, &block)
	if err != nil {
		problem(w, err)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	e, err := loadEnrollment(r.Context(), tx, eid, user, true, true)
	if err != nil {
		problem(w, err)
		return
	}
	a, err := assessment(r.Context(), tx, e, lesson, block)
	if err != nil {
		problem(w, err)
		return
	}
	list, err := submissions(r.Context(), tx, e.ID, a.ID)
	if err != nil {
		problem(w, err)
		return
	}
	var s *Submission
	for i := range list {
		if list[i].ID == id {
			s = &list[i]
		}
	}
	if s == nil {
		problem(w, sql.ErrNoRows)
		return
	}
	if len(path) > 1 && path[1] == "files" {
		h.submissionFile(w, r, tx, e, a, user, s, path[2:], true, func() {})
		return
	}
	if r.Method == "GET" {
		var definition BlockData
		var raw []byte
		if err = tx.QueryRowContext(r.Context(), `SELECT config FROM academy_assessment_versions WHERE id=$1`, a.ID).Scan(&raw); err != nil {
			problem(w, err)
			return
		}
		_ = json.Unmarshal(raw, &definition)
		var files, reviews json.RawMessage
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'name',name,'mime',mime,'size',size) ORDER BY id),'[]') FROM academy_media_assets WHERE submission_id=$1 AND deleted_at IS NULL AND id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, id, jsonValue(s.Answer.AssetIDs)).Scan(&files)
		if err == nil {
			err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.created_at,x.id),'[]') FROM(SELECT r.id,r.action,r.score,r.criterion_scores,r.comment,r.source,r.created_at,COALESCE(u.full_name,'Автоматически') reviewer_name FROM academy_submission_reviews r LEFT JOIN users u ON u.id=r.reviewer_id WHERE r.submission_id=$1)x`, id).Scan(&reviews)
		}
		if err != nil {
			problem(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			problem(w, err)
			return
		}
		shared := []Submission{}
		for _, previous := range list {
			if previous.SubmittedAt != nil {
				shared = append(shared, previous)
			}
		}
		respond(w, 200, map[string]any{"enrollment": e, "assessment": a, "definition": definition, "submission": s, "files": files, "history": shared, "reviews": reviews, "self_review": e.UserID == user})
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if e.UserID == user {
		fail(w, 403, "Нельзя проверять собственную работу")
		return
	}
	var body struct {
		Status   string             `json:"status"`
		Revision int64              `json:"revision"`
		Score    *float64           `json:"score"`
		Criteria map[string]float64 `json:"criterion_scores"`
		Comment  string             `json:"comment"`
		Key      string             `json:"request_key"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Comment = strings.TrimSpace(body.Comment)
	if !oneOf(body.Status, "in_review", "accepted", "revision_requested", "rejected") || !keyPattern.MatchString(body.Key) || utf8.RuneCountInString(body.Comment) > 10000 {
		problem(w, ErrInvalid)
		return
	}
	var seen bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_submission_reviews WHERE submission_id=$1 AND request_key=$2)`, id, body.Key).Scan(&seen); err != nil {
		problem(w, err)
		return
	}
	if !seen {
		if !oneOf(s.Status, "submitted", "in_review") || s.Revision != body.Revision {
			fail(w, 409, "Работа уже проверена или изменена. Обновите карточку")
			return
		}
		if body.Status == "revision_requested" && !canRequestRevision(a.Policy, s.Number) {
			fail(w, 400, "Возврат на доработку недоступен: проверьте разрешение и оставшиеся попытки")
			return
		}
		if oneOf(body.Status, "revision_requested", "rejected") && body.Comment == "" {
			fail(w, 400, "Укажите комментарий для ученика")
			return
		}
		if body.Status != "in_review" {
			body.Score, err = reviewScore(a.Policy, body.Score, body.Criteria, body.Status)
			if err != nil {
				problem(w, err)
				return
			}
		} else {
			body.Score = nil
			body.Criteria = map[string]float64{}
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE academy_assignment_submissions SET status=$2,assigned_to=$3::bigint,assigned_at=COALESCE(assigned_at,NOW()),reviewed_by=CASE WHEN $2='in_review' THEN NULL ELSE $3::bigint END,reviewed_at=CASE WHEN $2='in_review' THEN NULL ELSE NOW() END,score=$4,comment=$5,revision=revision+1,updated_at=NOW() WHERE id=$1`, id, body.Status, user, body.Score, body.Comment)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_submission_reviews(submission_id,reviewer_id,source,action,score,criterion_scores,comment,request_key) VALUES($1,$2,'manual',$3,$4,$5,$6,$7)`, id, user, body.Status, body.Score, jsonValue(body.Criteria), body.Comment, body.Key)
		}
		if err == nil {
			err = assessmentEvent(r.Context(), tx, e, user, a, "assignment_"+body.Status, id, map[string]any{"score": body.Score, "comment": body.Comment, "source": "manual"})
		}
		if err == nil && body.Status != "in_review" {
			err = notifyAssessment(r.Context(), tx, e, e.UserID, body.Status, "Работа проверена", a.Title, false)
		}
		if err == nil {
			err = reconcileAssessmentTx(r.Context(), tx, e, user, lesson)
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
	var next *int64
	err = h.db.QueryRowContext(r.Context(), `SELECT s.id FROM academy_assignment_submissions s JOIN academy_enrollments e ON e.id=s.enrollment_id WHERE e.course_id=$1 AND academy_can_manage_enrollment(e.id,$2,true) AND e.user_id<>$2 AND s.status IN('submitted','in_review') AND s.id<>$3 ORDER BY s.submitted_at,s.id LIMIT 1`, course, user, id).Scan(&next)
	if err != nil && err != sql.ErrNoRows {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"saved": true, "next_id": next})
}
