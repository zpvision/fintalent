package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrConflict = errors.New("Программа изменена в другой вкладке. Ваши правки сохранены локально. Обновите страницу для сравнения")
var ErrInvalid = errors.New("Элемент не принадлежит этой программе. Обновите страницу")

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const programSQL = `SELECT to_jsonb(p)||jsonb_build_object('modules',COALESCE((SELECT jsonb_agg(to_jsonb(m)||jsonb_build_object('cover_asset_id',COALESCE(m.cover_asset_id,0),'lessons',COALESCE((SELECT jsonb_agg(to_jsonb(l)||jsonb_build_object('cover_asset_id',COALESCE(l.cover_asset_id,0),'blocks',COALESCE((SELECT jsonb_agg(to_jsonb(b) ORDER BY b.sort_order,b.id) FROM academy_lesson_blocks b WHERE b.lesson_id=l.id),'[]')) ORDER BY l.sort_order,l.id) FROM academy_lessons l WHERE l.module_id=m.id),'[]')) ORDER BY m.sort_order,m.id) FROM academy_modules m WHERE m.program_id=p.id),'[]')) FROM academy_course_programs p WHERE p.course_id=$1`

func load(ctx context.Context, q queryer, course int64) (*Program, error) {
	var raw []byte
	if e := q.QueryRowContext(ctx, programSQL, course).Scan(&raw); e != nil {
		return nil, e
	}
	var p Program
	e := json.Unmarshal(raw, &p)
	return &p, e
}
func lock(ctx context.Context, tx *sql.Tx, course, user int64) (int64, int64, error) {
	var id, rev int64
	e := tx.QueryRowContext(ctx, `SELECT p.id,p.revision FROM academy_course_programs p JOIN courses c ON c.id=p.course_id JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND (c.owner_id=$2 AND NOT u.is_blocked OR $3) FOR UPDATE OF p`, course, user, adminPrincipal(ctx).Can("edit")).Scan(&id, &rev)
	return id, rev, e
}
func sync(ctx context.Context, tx *sql.Tx, table string, program int64, id *int64, key string, columns []string, values []any) error {
	if *id == 0 {
		args := append([]any{program, key}, values...)
		names := append([]string{"program_id", "key"}, columns...)
		marks := []string{}
		for i := range args {
			marks = append(marks, fmt.Sprintf("$%d", i+1))
		}
		return tx.QueryRowContext(ctx, "INSERT INTO "+table+"("+strings.Join(names, ",")+") VALUES("+strings.Join(marks, ",")+") RETURNING id", args...).Scan(id)
	}
	args := append([]any{program, *id, key}, values...)
	sets := []string{}
	for i, c := range columns {
		sets = append(sets, fmt.Sprintf("%s=$%d", c, i+4))
	}
	r, e := tx.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE program_id=$1 AND id=$2 AND key=$3", args...)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n != 1 {
		return ErrInvalid
	}
	return e
}
func prune(ctx context.Context, tx *sql.Tx, table string, program int64, ids []int64) error {
	b, _ := json.Marshal(ids)
	_, e := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE program_id=$1 AND id NOT IN (SELECT value::bigint FROM jsonb_array_elements_text($2::jsonb))", program, string(b))
	return e
}
func nullable(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
func (h *Handler) saveTree(ctx context.Context, tx *sql.Tx, p *Program) error {
	if e := Validate(p); e != nil {
		return inputError{e.Error()}
	}
	mids, lids, bids := []int64{}, []int64{}, []int64{}
	asset := func(id int64, key string, image bool) error {
		if id == 0 {
			return nil
		}
		var ok bool
		e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_media_assets a JOIN academy_asset_links x ON x.asset_id=a.id WHERE a.id=$1 AND a.program_id=$2 AND x.program_id=$2 AND x.entity_key=$3 AND a.deleted_at IS NULL AND (NOT $4 OR a.mime LIKE 'image/%'))`, id, p.ID, key, image).Scan(&ok)
		if e != nil {
			return e
		}
		if !ok {
			return ErrInvalid
		}
		return nil
	}
	for i := range p.Modules {
		m := &p.Modules[i]
		if e := asset(m.CoverAssetID, m.Key, true); e != nil {
			return e
		}
		if e := sync(ctx, tx, "academy_modules", p.ID, &m.ID, m.Key, []string{"title", "description", "status", "cover_asset_id", "sort_order"}, []any{m.Title, m.Description, m.Status, nullable(m.CoverAssetID), i}); e != nil {
			return e
		}
		mids = append(mids, m.ID)
		for j := range m.Lessons {
			l := &m.Lessons[j]
			if e := asset(l.CoverAssetID, l.Key, true); e != nil {
				return e
			}
			if e := sync(ctx, tx, "academy_lessons", p.ID, &l.ID, l.Key, []string{"module_id", "title", "description", "duration_minutes", "status", "required", "free_preview", "self_paced", "completion_rule", "cover_asset_id", "sort_order"}, []any{m.ID, l.Title, l.Description, l.Duration, l.Status, l.Required, l.FreePreview, l.SelfPaced, l.CompletionRule, nullable(l.CoverAssetID), j}); e != nil {
				return e
			}
			lids = append(lids, l.ID)
			for k := range l.Blocks {
				b := &l.Blocks[k]
				for _, ref := range []struct {
					id    int64
					image bool
				}{{b.Data.AssetID, b.Type == "image"}, {b.Data.PosterID, true}} {
					if e := asset(ref.id, l.Key, ref.image); e != nil {
						return e
					}
				}
				for _, id := range b.Data.MaterialIDs {
					if id <= 0 {
						return ErrInvalid
					}
					if e := asset(id, l.Key, false); e != nil {
						return e
					}
				}
				if b.Data.TestVersionID > 0 {
					var ok bool
					e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM test_versions v JOIN tests t ON t.id=v.test_id JOIN courses c ON c.id=$2 WHERE v.id=$1 AND t.author_id=c.owner_id AND t.deleted_at IS NULL AND t.status NOT IN ('deleted','blocked','archived'))`, b.Data.TestVersionID, p.CourseID).Scan(&ok)
					if e != nil {
						return e
					}
					if !ok {
						return inputError{"Можно прикрепить только доступный тест автора курса"}
					}
				}
				data, _ := json.Marshal(b.Data)
				if e := sync(ctx, tx, "academy_lesson_blocks", p.ID, &b.ID, b.Key, []string{"lesson_id", "type", "data", "sort_order"}, []any{l.ID, b.Type, string(data), k}); e != nil {
					return e
				}
				bids = append(bids, b.ID)
			}
		}
	}
	for _, entry := range []struct {
		table string
		ids   []int64
	}{{"academy_lesson_blocks", bids}, {"academy_lessons", lids}, {"academy_modules", mids}} {
		if e := prune(ctx, tx, entry.table, p.ID, entry.ids); e != nil {
			return e
		}
	}
	return nil
}
func (h *Handler) save(ctx context.Context, course, user int64, p *Program) (*Program, error) {
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	id, rev, e := lock(ctx, tx, course, user)
	if e != nil {
		return nil, e
	}
	if rev != p.Revision {
		return nil, ErrConflict
	}
	p.ID = id
	p.CourseID = course
	if e = h.saveTree(ctx, tx, p); e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `UPDATE academy_course_programs SET revision=revision+1,updated_at=NOW() WHERE id=$1`, id)
	if e != nil {
		return nil, e
	}
	p, e = load(ctx, tx, course)
	if e != nil {
		return nil, e
	}
	return p, tx.Commit()
}
func (h *Handler) initialize(ctx context.Context, course, user int64, source string) (*Program, error) {
	if source != "landing" && source != "empty" {
		return nil, inputError{"Выберите способ создания программы"}
	}
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var check int64
	// Lock the parent even before a program exists, making retries/double clicks idempotent.
	if e = tx.QueryRowContext(ctx, `SELECT id FROM courses WHERE id=$1 AND owner_id=$2 FOR UPDATE`, course, user).Scan(&check); e != nil {
		return nil, e
	}
	if p, err := load(ctx, tx, course); err == nil {
		return p, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	p := &Program{CourseID: course, Source: source, Revision: 1, Modules: []Module{}}
	if e = tx.QueryRowContext(ctx, `INSERT INTO academy_course_programs(course_id,source) VALUES($1,$2) RETURNING id`, course, source).Scan(&p.ID); e != nil {
		return nil, e
	}
	if source == "landing" {
		var raw []byte
		e = tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(m)||jsonb_build_object('lessons',COALESCE((SELECT jsonb_agg(to_jsonb(l) ORDER BY l.sort_order,l.id) FROM course_lessons l WHERE l.module_id=m.id),'[]')) ORDER BY m.sort_order,m.id),'[]') FROM course_modules m WHERE m.course_id=$1`, course).Scan(&raw)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &p.Modules); e != nil {
			return nil, e
		}
		if len(p.Modules) == 0 {
			return nil, inputError{"Программа лендинга пуста. Создайте программу с нуля"}
		}
		for i := range p.Modules {
			m := &p.Modules[i]
			defaults := NewModule()
			m.ID = 0
			m.Key = defaults.Key
			m.Status = defaults.Status
			if strings.TrimSpace(m.Title) == "" {
				m.Title = defaults.Title
			}
			for j := range m.Lessons {
				l := &m.Lessons[j]
				defaults := NewLesson()
				l.ID = 0
				l.Key = defaults.Key
				l.Status = defaults.Status
				if strings.TrimSpace(l.Title) == "" {
					l.Title = defaults.Title
				}
				l.Required = true
				l.SelfPaced = true
				l.CompletionRule = "manual"
				l.Blocks = []Block{}
			}
		}
	}
	if e = h.saveTree(ctx, tx, p); e != nil {
		return nil, e
	}
	return p, tx.Commit()
}
func (h *Handler) snapshot(ctx context.Context, course, user, revision int64) (any, error) {
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	id, rev, e := lock(ctx, tx, course, user)
	if e != nil {
		return nil, e
	}
	if rev != revision {
		return nil, ErrConflict
	}
	p, e := load(ctx, tx, course)
	if e != nil {
		return nil, e
	}
	if len(p.Modules) == 0 {
		return nil, inputError{"Добавьте хотя бы один модуль"}
	}
	raw, _ := json.Marshal(p)
	var version int64
	e = tx.QueryRowContext(ctx, `INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,(SELECT COALESCE(MAX(number),0)+1 FROM academy_program_versions WHERE program_id=$1),$2,$3,$4) ON CONFLICT(program_id,source_revision) DO NOTHING RETURNING number`, id, rev, string(raw), user).Scan(&version)
	// Conflict must not UPDATE immutable rows: resolve a retry by reading the existing snapshot.
	if errors.Is(e, sql.ErrNoRows) {
		e = tx.QueryRowContext(ctx, `SELECT number FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, id, rev).Scan(&version)
	}
	if e != nil {
		return nil, e
	}
	var versionID int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM academy_program_versions WHERE program_id=$1 AND source_revision=$2`, id, rev).Scan(&versionID); e != nil {
		return nil, e
	}
	if e = freezeAssessments(ctx, tx, versionID, *p, user); e != nil {
		return nil, e
	}
	return map[string]any{"number": version, "revision": rev}, tx.Commit()
}
