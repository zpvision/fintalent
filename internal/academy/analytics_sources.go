package academy

import (
	"context"
	"database/sql"
	"strconv"
)

const applicationSourceSQL = `CASE WHEN i.paid_at IS NOT NULL THEN i.paid_referral_link_id ELSE attribution.referral_link_id END`
const applicationJoinsSQL = ` FROM course_interests i JOIN users u ON u.id=i.user_id LEFT JOIN course_referral_attributions attribution ON attribution.user_id=i.user_id AND attribution.course_id=i.course_id `

func (h *Handler) courseInterests(ctx context.Context, d *analyticsDataset) (map[string]any, error) {
	var entries []analyticsRow
	source := d.Filters.Source
	var sourceID int64
	if source != "" && source != "platform" {
		sourceID, _ = strconv.ParseInt(source, 10, 64)
	}
	where := `i.user_id IS NOT NULL AND i.course_id=$1 AND i.first_clicked_at>=$2 AND i.first_clicked_at<$3 AND($4='' OR $4='platform' AND (` + applicationSourceSQL + `) IS NULL OR (` + applicationSourceSQL + `)=$5) AND($6='' OR u.full_name ILIKE '%'||$6||'%' OR i.user_id::text=$6) AND($7='' OR $7='paid' AND i.paid_at IS NOT NULL OR $7='unpaid' AND i.paid_at IS NULL)`
	args := []any{d.Course, d.Filters.from, d.Filters.until, source, sourceID, d.Filters.Search, d.Filters.InterestPayment}
	var count int
	if err := h.db.QueryRowContext(ctx, `SELECT count(*)`+applicationJoinsSQL+` WHERE `+where, args...).Scan(&count); err != nil {
		return nil, err
	}
	args = append(args, d.Filters.Size, (d.Filters.Page-1)*d.Filters.Size)
	query := `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.first_clicked_at DESC,x.id DESC),'[]') FROM(SELECT i.id,i.user_id,u.full_name name,COALESCE(l.name,'FinTalent') source_name,i.first_clicked_at,i.tariff_id,i.paid_at,i.paid_enrollment_id,i.commission_override_bps,CASE WHEN i.paid_at IS NOT NULL THEN i.paid_commission_bps WHEN i.commission_override_bps IS NOT NULL THEN i.commission_override_bps WHEN (` + applicationSourceSQL + `) IS NULL THEN ct.standard_bps ELSE ct.partner_bps END commission_bps,outbox.status invitation_status,COALESCE((SELECT '/profiles/view/'||r.id FROM resumes r WHERE r.user_id=i.user_id AND r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL ORDER BY r.id DESC LIMIT 1),'') profile_url` + applicationJoinsSQL + `LEFT JOIN course_referral_links l ON l.id=(` + applicationSourceSQL + `) JOIN course_commission_terms ct ON ct.course_id=i.course_id LEFT JOIN notification_outbox outbox ON outbox.id=i.invitation_outbox_id WHERE ` + where + ` ORDER BY i.first_clicked_at DESC,i.id DESC LIMIT $8 OFFSET $9)x`
	if err := analyticsJSON(ctx, h.db, query, &entries, args...); err != nil {
		return nil, err
	}
	return map[string]any{"rows": entries, "total": count, "page": d.Filters.Page, "size": d.Filters.Size}, nil
}

func (h *Handler) interestPaymentSummary(ctx context.Context, d *analyticsDataset) (map[string]any, error) {
	var standard, partner, months int
	if err := h.db.QueryRowContext(ctx, `SELECT t.standard_bps,t.partner_bps,c.access_months FROM course_commission_terms t JOIN courses c ON c.id=t.course_id WHERE t.course_id=$1`, d.Course).Scan(&standard, &partner, &months); err != nil {
		return nil, err
	}
	var unpaid int
	if err := h.db.QueryRowContext(ctx, `SELECT count(*) FROM course_interests WHERE course_id=$1 AND user_id IS NOT NULL AND paid_at IS NULL`, d.Course).Scan(&unpaid); err != nil {
		return nil, err
	}
	return map[string]any{"unpaid": unpaid, "standard_bps": standard, "partner_bps": partner, "access_months": months}, nil
}

func (h *Handler) loadStudentSources(ctx context.Context, tx *sql.Tx, d *analyticsDataset) error {
	ids := []int64{}
	for _, s := range d.Students {
		ids = append(ids, s.User)
		s.SourceName = "FinTalent"
	}
	var sources []struct {
		User int64  `json:"user_id"`
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	err := analyticsJSON(ctx, tx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('user_id',a.user_id,'id',l.id,'name',l.name)),'[]') FROM course_referral_attributions a JOIN course_referral_links l ON l.id=a.referral_link_id WHERE a.course_id=$1 AND a.user_id IN(SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))`, &sources, d.Course, jsonValue(ids))
	if err != nil {
		return err
	}
	byUser := map[int64]int{}
	for i, s := range sources {
		byUser[s.User] = i
	}
	for _, s := range d.Students {
		if i, ok := byUser[s.User]; ok {
			s.SourceID = sources[i].ID
			s.SourceName = sources[i].Name
		}
	}
	return nil
}

// The owner sees all-time acquisition counts; staff only receive source names
// from their already-authorized enrollment scope, never whole-course counts.
func (h *Handler) studentSources(ctx context.Context, d *analyticsDataset) ([]analyticsRow, error) {
	result := []analyticsRow{}
	if !d.Owner {
		seen := map[int64]bool{}
		for _, s := range d.Students {
			if s.SourceID > 0 && !seen[s.SourceID] {
				seen[s.SourceID] = true
				result = append(result, analyticsRow{"id": s.SourceID, "name": s.SourceName})
			}
		}
		return result, nil
	}
	rows, e := h.db.QueryContext(ctx, `SELECT l.id,l.name,l.deleted_at IS NOT NULL,COALESCE((SELECT sum(v.visits) FROM course_referral_visitors v WHERE v.referral_link_id=l.id),0),(SELECT count(*) FROM course_referral_visitors v WHERE v.referral_link_id=l.id),(SELECT count(*) FROM course_referral_attributions a WHERE a.referral_link_id=l.id AND a.course_id=l.course_id),(SELECT count(*)`+applicationJoinsSQL+` WHERE (`+applicationSourceSQL+`)=l.id AND i.course_id=l.course_id AND i.user_id IS NOT NULL) FROM course_referral_links l WHERE l.course_id=$1 ORDER BY l.id`, d.Course)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var id, visits, visitors, accounts, interests int64
		var name string
		var deleted bool
		if e = rows.Scan(&id, &name, &deleted, &visits, &visitors, &accounts, &interests); e != nil {
			return nil, e
		}
		result = append(result, analyticsRow{"id": id, "name": name, "deleted": deleted, "visits": visits, "visitors": visitors, "accounts": accounts, "interests": interests, "purchases": nil, "revenue": nil, "conversion": nil})
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	var interests int64
	if e = h.db.QueryRowContext(ctx, `SELECT count(*)`+applicationJoinsSQL+` WHERE i.course_id=$1 AND (`+applicationSourceSQL+`) IS NULL AND i.user_id IS NOT NULL`, d.Course).Scan(&interests); e != nil {
		return nil, e
	}
	result = append([]analyticsRow{{"id": 0, "name": "FinTalent", "interests": interests}}, result...)
	return result, nil
}
