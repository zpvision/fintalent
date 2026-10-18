package academy

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *Handler) assessmentResults(w http.ResponseWriter, r *http.Request, course, user int64) {
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	before := int64(9223372036854775807)
	if s := r.URL.Query().Get("before"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			problem(w, ErrInvalid)
			return
		}
		before = v
	}
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(x.payload ORDER BY x.id DESC),'[]') FROM(SELECT t.id,jsonb_build_object('id',t.id,'enrollment_id',e.id,'user_name',u.full_name,'title',a.title,'number',t.number,'score',t.score,'max_score',t.max_score,'percent',t.percent,'passed',t.passed,'finished_at',t.finished_at,'version_number',v.number) payload FROM academy_test_attempts t JOIN academy_enrollments e ON e.id=t.enrollment_id JOIN courses c ON c.id=e.course_id JOIN users u ON u.id=e.user_id JOIN academy_assessment_versions a ON a.id=t.assessment_id JOIN academy_program_versions v ON v.id=e.program_version_id WHERE c.id=$1 AND c.owner_id=$2 AND t.status='finished' AND t.id<$3 ORDER BY t.id DESC LIMIT 200)x`, course, user, before).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
func (h *Handler) studentAssessmentSummary(ctx context.Context, e *Enrollment) (json.RawMessage, error) {
	passed := 0
	for _, m := range e.Program.Modules {
		for _, l := range m.Lessons {
			for _, b := range l.Blocks {
				if b.Type == "test" && e.AssessmentStates[b.Key] {
					passed++
				}
			}
		}
	}
	var raw json.RawMessage
	err := h.db.QueryRowContext(ctx, `SELECT jsonb_build_object('tests_passed',$2::integer,'test_average',(SELECT round(avg(percent),2) FROM academy_test_attempts WHERE enrollment_id=$1 AND status='finished'),'assignments_accepted',count(*) FILTER(WHERE s.status='accepted'),'pending',count(*) FILTER(WHERE s.status IN('submitted','in_review')),'revision',count(*) FILTER(WHERE s.status='revision_requested')) FROM academy_assignment_submissions s WHERE s.enrollment_id=$1 AND s.submitted_at IS NOT NULL AND `+latestSubmission, e.ID, passed).Scan(&raw)
	return raw, err
}
func (h *Handler) academyNotifications(w http.ResponseWriter, r *http.Request, user int64) {
	if r.Method == "PUT" {
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if !decode(w, r, &body) {
			return
		}
		if len(body.IDs) > 100 {
			problem(w, ErrInvalid)
			return
		}
		_, err := h.db.ExecContext(r.Context(), `UPDATE notifications SET is_read=true WHERE user_id=$1 AND type LIKE 'academy\_%' ESCAPE '\' AND id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, user, jsonValue(body.IDs))
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'title',title,'body',body,'is_read',is_read,'created_at',created_at,'href',CASE WHEN entity_type='academy_course' THEN '/profile/courses/'||entity_id||'/academy/reviews' ELSE '/profile/learning/'||entity_id END) ORDER BY id DESC),'[]') FROM(SELECT * FROM notifications WHERE user_id=$1 AND type LIKE 'academy\_%' ESCAPE '\' ORDER BY id DESC LIMIT 30)x`, user).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
