package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Recipient selectors need identity and grouping, not the full progress report.
func (h *Handler) crmChoices(w http.ResponseWriter, r *http.Request, course, user int64) {
	var raw json.RawMessage
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.user_name,x.enrollment_id),'[]') FROM (
 SELECT e.id enrollment_id,e.user_id,u.full_name user_name,e.cohort_id,COALESCE(ch.name,'') cohort_name,v.number version_number,sc.user_id curator_id,COALESCE(cu.full_name,'') curator_name
 FROM academy_enrollments e JOIN users u ON u.id=e.user_id JOIN academy_program_versions v ON v.id=e.program_version_id
 LEFT JOIN academy_cohorts ch ON ch.id=e.cohort_id LEFT JOIN academy_student_curators sc ON sc.enrollment_id=e.id LEFT JOIN users cu ON cu.id=sc.user_id
 WHERE e.course_id=$1 AND academy_can_manage_enrollment(e.id,$2,false))x`, course, user).Scan(&raw)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}

func (h *Handler) crmPage(w http.ResponseWriter, r *http.Request, course, user int64) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 || page > 100000 {
		problem(w, ErrInvalid)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 300 {
		problem(w, ErrInvalid)
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && !oneOf(status, "not_started", "in_progress", "completed", "inactive", "suspended") {
		problem(w, ErrInvalid)
		return
	}
	order := map[string]string{"": "COALESCE(last_activity_at,enrolled_at) DESC,enrollment_id DESC", "activity": "COALESCE(last_activity_at,enrolled_at) DESC,enrollment_id DESC", "name": "user_name,enrollment_id", "progress": "percent DESC,enrollment_id"}[r.URL.Query().Get("sort")]
	if order == "" {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "SET LOCAL jit=off"); err != nil {
		problem(w, err)
		return
	}
	var raw json.RawMessage
	query := `WITH visible AS MATERIALIZED (` + crmRowsSQL + `), filtered AS MATERIALIZED (SELECT * FROM visible WHERE
 ($4='' OR strpos(lower(user_name),lower($4))>0 OR user_id::text=$4)
 AND($5=0 OR EXISTS(SELECT 1 FROM academy_student_tag_assignments ta WHERE ta.enrollment_id=visible.enrollment_id AND ta.tag_id=$5))
 AND($6='' OR $6='not_started' AND started_at IS NULL OR $6='in_progress' AND started_at IS NOT NULL AND completed_at IS NULL OR $6='completed' AND completed_at IS NOT NULL OR $6='inactive' AND(last_activity_at IS NULL OR last_activity_at<now()-INTERVAL '7 days') OR $6='suspended' AND effective_status='suspended')),
 page_rows AS (SELECT * FROM filtered ORDER BY ` + order + ` LIMIT 50 OFFSET $7)
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY ` + order + `) FROM page_rows p),'[]'),'total',(SELECT count(*) FROM filtered),'page',$8::int,'page_size',50)`
	err = tx.QueryRowContext(r.Context(), query, course, user, queryInt(r, "cohort"), search, queryInt(r, "tag"), status, (page-1)*50, page).Scan(&raw)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, raw)
}
