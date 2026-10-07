package courses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrConflict = errors.New("Черновик изменён в другой вкладке. Скопируйте новые изменения и перезагрузите страницу")
var ErrChild = errors.New("Некорректная структура курса: обновите страницу")
var ErrCategory = errors.New("Выберите активную категорию курса")

// Child IDs survive edits. Ownership is checked in every UPDATE, including nested lessons.
func syncRow(ctx context.Context, tx *sql.Tx, table, parent string, parentID int64, id *int64, columns []string, values []any) error {
	names := append([]string{parent}, columns...)
	args := append([]any{parentID}, values...)
	if *id == 0 {
		marks := make([]string, len(args))
		for i := range marks {
			marks[i] = fmt.Sprintf("$%d", i+1)
		}
		return tx.QueryRowContext(ctx, "INSERT INTO "+table+"("+strings.Join(names, ",")+") VALUES("+strings.Join(marks, ",")+") RETURNING id", args...).Scan(id)
	}
	sets := make([]string, len(columns))
	for i, n := range columns {
		sets[i] = fmt.Sprintf("%s=$%d", n, i+2)
	}
	args = append(args, *id)
	result, e := tx.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+fmt.Sprintf(" WHERE %s=$1 AND id=$%d", parent, len(args)), args...)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrChild
	}
	return nil
}
func prune(ctx context.Context, tx *sql.Tx, table, parent string, parentID int64, ids []int64) error {
	b, _ := json.Marshal(ids)
	_, e := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+parent+"=$1 AND id NOT IN (SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))", parentID, string(b))
	return e
}
func saveChildren(ctx context.Context, tx *sql.Tx, id int64, v *Input) error {
	ids := []int64{}
	seen := map[int64]bool{}
	unique := func(id int64) error {
		if seen[id] {
			return ErrChild
		}
		seen[id] = true
		return nil
	}
	for i := range v.Benefits {
		b := &v.Benefits[i]
		if e := syncRow(ctx, tx, "course_benefits", "course_id", id, &b.ID, []string{"icon", "title", "description", "sort_order"}, []any{b.Icon, b.Title, b.Description, i}); e != nil {
			return e
		}
		if e := unique(b.ID); e != nil {
			return e
		}
		ids = append(ids, b.ID)
	}
	if e := prune(ctx, tx, "course_benefits", "course_id", id, ids); e != nil {
		return e
	}
	ids = []int64{}
	seen = map[int64]bool{}
	for i := range v.Bonuses {
		b := &v.Bonuses[i]
		if e := syncRow(ctx, tx, "course_bonuses", "course_id", id, &b.ID, []string{"title", "sort_order"}, []any{b.Title, i}); e != nil {
			return e
		}
		if e := unique(b.ID); e != nil {
			return e
		}
		ids = append(ids, b.ID)
	}
	if e := prune(ctx, tx, "course_bonuses", "course_id", id, ids); e != nil {
		return e
	}
	ids = []int64{}
	seen = map[int64]bool{}
	lessonSeen := map[int64]bool{}
	for i := range v.Modules {
		m := &v.Modules[i]
		if e := syncRow(ctx, tx, "course_modules", "course_id", id, &m.ID, []string{"title", "description", "sort_order"}, []any{m.Title, m.Description, i}); e != nil {
			return e
		}
		if e := unique(m.ID); e != nil {
			return e
		}
		ids = append(ids, m.ID)
		lessons := []int64{}
		for j := range m.Lessons {
			l := &m.Lessons[j]
			if e := syncRow(ctx, tx, "course_lessons", "module_id", m.ID, &l.ID, []string{"title", "description", "duration_minutes", "type", "sort_order"}, []any{l.Title, l.Description, l.Duration, l.Type, j}); e != nil {
				return e
			}
			if lessonSeen[l.ID] {
				return ErrChild
			}
			lessonSeen[l.ID] = true
			lessons = append(lessons, l.ID)
		}
		if e := prune(ctx, tx, "course_lessons", "module_id", m.ID, lessons); e != nil {
			return e
		}
	}
	if e := prune(ctx, tx, "course_modules", "course_id", id, ids); e != nil {
		return e
	}
	ids = []int64{}
	seen = map[int64]bool{}
	for i := range v.FAQ {
		q := &v.FAQ[i]
		if e := syncRow(ctx, tx, "course_faq", "course_id", id, &q.ID, []string{"question", "answer", "sort_order"}, []any{q.Question, q.Answer, i}); e != nil {
			return e
		}
		if e := unique(q.ID); e != nil {
			return e
		}
		ids = append(ids, q.ID)
	}
	return prune(ctx, tx, "course_faq", "course_id", id, ids)
}

// Aggregate only at the API boundary; all editable entities are relational rows.
const detailSQL = `SELECT to_jsonb(c)||jsonb_build_object(
 'category',COALESCE(d.value,''),
 'benefits',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY sort_order,id) FROM course_benefits b WHERE course_id=c.id),'[]'),
 'bonuses',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY sort_order,id) FROM course_bonuses b WHERE course_id=c.id),'[]'),
 'faq',COALESCE((SELECT jsonb_agg(to_jsonb(f) ORDER BY sort_order,id) FROM course_faq f WHERE course_id=c.id),'[]'),
 'modules',COALESCE((SELECT jsonb_agg(to_jsonb(m)||jsonb_build_object('lessons',COALESCE((SELECT jsonb_agg(to_jsonb(l) ORDER BY sort_order,id) FROM course_lessons l WHERE module_id=m.id),'[]')) ORDER BY m.sort_order,m.id) FROM course_modules m WHERE course_id=c.id),'[]'),
 'instructors',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',u.id,'name',u.full_name,'avatar',COALESCE(u.avatar_url,''),'profile_id',r.id,'description',COALESCE((SELECT LEFT(concat_ws(E'\n',NULLIF(e.achievements,''),NULLIF(e.responsibilities,'')),1200) FROM resume_work_experiences e WHERE e.resume_id=r.id ORDER BY is_current DESC,start_year DESC,id DESC LIMIT 1),''),'headline',COALESCE((SELECT position FROM resume_work_experiences e WHERE e.resume_id=r.id ORDER BY is_current DESC,start_year DESC,id DESC LIMIT 1),'')) ORDER BY ci.sort_order,u.id) FROM course_instructors ci JOIN users u ON u.id=ci.user_id LEFT JOIN LATERAL (SELECT id FROM resumes WHERE user_id=u.id AND status='published' AND visibility='public' AND deleted_at IS NULL ORDER BY id DESC LIMIT 1) r ON TRUE WHERE ci.course_id=c.id),'[]'),
 'reviews',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',rv.id,'rating',rv.rating,'body',rv.body,'created_at',rv.created_at,'name',u.full_name,'avatar',COALESCE(u.avatar_url,'')) ORDER BY rv.created_at DESC,rv.id DESC) FROM course_reviews rv JOIN users u ON u.id=rv.user_id WHERE rv.course_id=c.id AND rv.status='published' AND NOT u.is_blocked),'[]'),
 'student_count',0)
 FROM courses c LEFT JOIN dictionary_items d ON d.id=c.category_id JOIN users owner ON owner.id=c.owner_id
 WHERE `

func (h *Handler) detail(ctx context.Context, where string, args ...any) (json.RawMessage, error) {
	var raw json.RawMessage
	e := h.db.QueryRowContext(ctx, detailSQL+where, args...).Scan(&raw)
	return raw, e
}

func (h *Handler) save(ctx context.Context, id, user int64, v *Input) error {
	if e := Validate(v, false); e != nil {
		return e
	}
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var revision int64
	if e = tx.QueryRowContext(ctx, `SELECT revision FROM courses WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, user).Scan(&revision); e != nil {
		return e
	}
	if revision != v.Revision {
		return ErrConflict
	}
	if v.CategoryID != nil {
		var ok bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE i.id=$1 AND d.alias='course_categories' AND i.active AND i.deleted_at IS NULL)`, *v.CategoryID).Scan(&ok)
		if e != nil {
			return e
		}
		if !ok {
			return ErrCategory
		}
	}
	_, e = tx.ExecContext(ctx, `UPDATE courses SET title=$1,description=$2,category_id=$3,level=$4,cover_image=$5,promo_url=$6,price=$7,old_price=$8,hours=$9,access_months=$10,certificate=$11,chat=$12,updates=$13,any_device=$14,corporate=$15,revision=revision+1,updated_at=NOW(),status=CASE WHEN status IN ('published','moderation') THEN 'draft' ELSE status END WHERE id=$16`, v.Title, v.Description, v.CategoryID, v.Level, v.CoverImage, v.PromoURL, v.Price, v.OldPrice, v.Hours, v.AccessMonths, v.Certificate, v.Chat, v.Updates, v.AnyDevice, v.Corporate, id)
	if e != nil {
		return e
	}
	if e = saveChildren(ctx, tx, id, v); e != nil {
		return e
	}
	return tx.Commit()
}
