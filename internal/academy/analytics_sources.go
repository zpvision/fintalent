package academy

import (
	"context"
	"database/sql"
	"strconv"
)

func (h *Handler) courseInterests(ctx context.Context, d *analyticsDataset) (map[string]any, error) {
	var entries []analyticsRow
	source := d.Filters.Source
	var sourceID int64
	if source != "" && source != "platform" {
		sourceID, _ = strconv.ParseInt(source, 10, 64)
	}
	where := `i.user_id IS NOT NULL AND i.course_id=$1 AND i.first_clicked_at>=$2 AND i.first_clicked_at<$3 AND($4='' OR $4='platform' AND i.referral_link_id IS NULL OR i.referral_link_id=$5) AND($6='' OR u.full_name ILIKE '%'||$6||'%' OR i.user_id::text=$6)`
	args := []any{d.Course, d.Filters.from, d.Filters.until, source, sourceID, d.Filters.Search}
	var count int
	err := h.db.QueryRowContext(ctx, `SELECT count(*) FROM course_interests i LEFT JOIN users u ON u.id=i.user_id WHERE `+where, args...).Scan(&count)
	if err != nil {
		return nil, err
	}
	args = append(args, d.Filters.Size, (d.Filters.Page-1)*d.Filters.Size)
	err = analyticsJSON(ctx, h.db, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.first_clicked_at DESC,x.id DESC),'[]') FROM(SELECT i.id,i.user_id,COALESCE(u.full_name,'Гость') name,COALESCE(l.name,'FinTalent') source_name,i.first_clicked_at,i.tariff_id FROM course_interests i LEFT JOIN users u ON u.id=i.user_id LEFT JOIN course_referral_links l ON l.id=i.referral_link_id WHERE `+where+` ORDER BY i.first_clicked_at DESC,i.id DESC LIMIT $7 OFFSET $8)x`, &entries, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"rows": entries, "total": count, "page": d.Filters.Page, "size": d.Filters.Size}, nil
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
	rows, e := h.db.QueryContext(ctx, `SELECT l.id,l.name,l.deleted_at IS NOT NULL,COALESCE((SELECT sum(v.visits) FROM course_referral_visitors v WHERE v.referral_link_id=l.id),0),(SELECT count(*) FROM course_referral_visitors v WHERE v.referral_link_id=l.id),(SELECT count(*) FROM course_referral_attributions a WHERE a.referral_link_id=l.id AND a.course_id=l.course_id),(SELECT count(*) FROM course_interests i WHERE i.referral_link_id=l.id AND i.course_id=l.course_id AND i.user_id IS NOT NULL) FROM course_referral_links l WHERE l.course_id=$1 ORDER BY l.id`, d.Course)
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
	if e = h.db.QueryRowContext(ctx, `SELECT count(*) FROM course_interests WHERE course_id=$1 AND referral_link_id IS NULL AND user_id IS NOT NULL`, d.Course).Scan(&interests); e != nil {
		return nil, e
	}
	result = append([]analyticsRow{{"id": 0, "name": "FinTalent", "interests": interests}}, result...)
	return result, nil
}
