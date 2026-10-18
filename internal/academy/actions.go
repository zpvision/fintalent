package academy

import (
	"context"
	"encoding/json"
)

// The same atomic tree writer serves autosave and explicit structural commands.
type Action struct {
	Revision int64   `json:"revision"`
	Type     string  `json:"type"`
	Key      string  `json:"key"`
	Target   string  `json:"target"`
	Index    int     `json:"index"`
	Module   *Module `json:"module,omitempty"`
	Lesson   *Lesson `json:"lesson,omitempty"`
	Block    *Block  `json:"block,omitempty"`
}

func NewModule() Module {
	return Module{Key: Key(), Title: "Новый модуль", Status: "draft", Lessons: []Lesson{}}
}
func NewLesson() Lesson {
	return Lesson{Key: Key(), Title: "Новый урок", Status: "draft", Required: true, SelfPaced: true, CompletionRule: "manual", Blocks: []Block{}}
}
func copyTitle(title string) string {
	runes := []rune(title)
	if len(runes) > 170 {
		runes = runes[:170]
	}
	return string(runes) + " (копия)"
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var c T; _ = json.Unmarshal(b, &c); return c }
func insert[T any](items []T, v T, i int) []T {
	if i < 0 {
		i = 0
	}
	if i > len(items) {
		i = len(items)
	}
	items = append(items, v)
	copy(items[i+1:], items[i:len(items)-1])
	items[i] = v
	return items
}
func (h *Handler) action(ctx context.Context, course, user int64, a Action) (*Program, error) {
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	id, rev, e := lock(ctx, tx, course, user)
	if e != nil {
		return nil, e
	}
	if a.Revision != rev {
		return nil, ErrConflict
	}
	p, e := load(ctx, tx, course)
	if e != nil {
		return nil, e
	}
	mi, li, bi := -1, -1, -1
	target := -1
	for i, m := range p.Modules {
		if m.Key == a.Target {
			target = i
		}
		if m.Key == a.Key {
			mi = i
		}
		for j, l := range m.Lessons {
			if l.Key == a.Key {
				mi = i
				li = j
			}
			for k, b := range l.Blocks {
				if b.Key == a.Key {
					mi = i
					li = j
					bi = k
				}
			}
		}
	}
	copyAssets := func(from, to string) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO academy_asset_links(asset_id,program_id,entity_key) SELECT asset_id,program_id,$3 FROM academy_asset_links WHERE program_id=$1 AND entity_key=$2 ON CONFLICT DO NOTHING`, id, from, to)
		return err
	}
	duplicateLesson := func(l Lesson) (Lesson, error) {
		l = clone(l)
		old := l.Key
		l.ID = 0
		l.Key = Key()
		l.Status = "draft"
		l.Title = copyTitle(l.Title)
		for i := range l.Blocks {
			l.Blocks[i].ID = 0
			l.Blocks[i].Key = Key()
		}
		return l, copyAssets(old, l.Key)
	}
	switch a.Type {
	case "module.create":
		m := NewModule()
		if a.Module != nil {
			m.Title = a.Module.Title
			m.Description = a.Module.Description
		}
		p.Modules = append(p.Modules, m)
	case "module.update":
		if mi < 0 || li >= 0 || a.Module == nil {
			return nil, ErrInvalid
		}
		m := a.Module
		m.ID = p.Modules[mi].ID
		m.Key = p.Modules[mi].Key
		m.Lessons = p.Modules[mi].Lessons
		p.Modules[mi] = *m
	case "module.delete":
		if mi < 0 || li >= 0 {
			return nil, ErrInvalid
		}
		p.Modules = append(p.Modules[:mi], p.Modules[mi+1:]...)
	case "module.move":
		if mi < 0 || li >= 0 {
			return nil, ErrInvalid
		}
		m := p.Modules[mi]
		p.Modules = append(p.Modules[:mi], p.Modules[mi+1:]...)
		p.Modules = insert(p.Modules, m, a.Index)
	case "module.duplicate":
		if mi < 0 || li >= 0 {
			return nil, ErrInvalid
		}
		m := clone(p.Modules[mi])
		old := m.Key
		m.ID = 0
		m.Key = Key()
		m.Title = copyTitle(m.Title)
		m.Status = "draft"
		if e = copyAssets(old, m.Key); e != nil {
			return nil, e
		}
		for i := range m.Lessons {
			m.Lessons[i], e = duplicateLesson(m.Lessons[i])
			if e != nil {
				return nil, e
			}
		}
		p.Modules = insert(p.Modules, m, mi+1)
	case "lesson.create":
		if target < 0 {
			return nil, ErrInvalid
		}
		l := NewLesson()
		if a.Lesson != nil {
			l.Title = a.Lesson.Title
		}
		p.Modules[target].Lessons = append(p.Modules[target].Lessons, l)
	case "lesson.update":
		if mi < 0 || li < 0 || bi >= 0 || a.Lesson == nil {
			return nil, ErrInvalid
		}
		l := a.Lesson
		l.ID = p.Modules[mi].Lessons[li].ID
		l.Key = p.Modules[mi].Lessons[li].Key
		l.Blocks = p.Modules[mi].Lessons[li].Blocks
		p.Modules[mi].Lessons[li] = *l
	case "lesson.delete":
		if mi < 0 || li < 0 || bi >= 0 {
			return nil, ErrInvalid
		}
		p.Modules[mi].Lessons = append(p.Modules[mi].Lessons[:li], p.Modules[mi].Lessons[li+1:]...)
	case "lesson.move":
		if mi < 0 || li < 0 || bi >= 0 || target < 0 {
			return nil, ErrInvalid
		}
		l := p.Modules[mi].Lessons[li]
		p.Modules[mi].Lessons = append(p.Modules[mi].Lessons[:li], p.Modules[mi].Lessons[li+1:]...)
		p.Modules[target].Lessons = insert(p.Modules[target].Lessons, l, a.Index)
	case "lesson.duplicate":
		if mi < 0 || li < 0 || bi >= 0 {
			return nil, ErrInvalid
		}
		l, err := duplicateLesson(p.Modules[mi].Lessons[li])
		if err != nil {
			return nil, err
		}
		p.Modules[mi].Lessons = insert(p.Modules[mi].Lessons, l, li+1)
	case "block.create":
		if mi < 0 || li < 0 || bi >= 0 || a.Block == nil {
			return nil, ErrInvalid
		}
		b := *a.Block
		b.ID = 0
		b.Key = Key()
		p.Modules[mi].Lessons[li].Blocks = insert(p.Modules[mi].Lessons[li].Blocks, b, a.Index)
	case "block.update":
		if bi < 0 || a.Block == nil {
			return nil, ErrInvalid
		}
		b := *a.Block
		b.ID = p.Modules[mi].Lessons[li].Blocks[bi].ID
		b.Key = a.Key
		p.Modules[mi].Lessons[li].Blocks[bi] = b
	case "block.delete":
		if bi < 0 {
			return nil, ErrInvalid
		}
		bs := p.Modules[mi].Lessons[li].Blocks
		p.Modules[mi].Lessons[li].Blocks = append(bs[:bi], bs[bi+1:]...)
	case "block.move":
		if bi < 0 {
			return nil, ErrInvalid
		}
		bs := p.Modules[mi].Lessons[li].Blocks
		b := bs[bi]
		bs = append(bs[:bi], bs[bi+1:]...)
		p.Modules[mi].Lessons[li].Blocks = insert(bs, b, a.Index)
	case "block.duplicate":
		if bi < 0 {
			return nil, ErrInvalid
		}
		bs := p.Modules[mi].Lessons[li].Blocks
		b := clone(bs[bi])
		b.ID = 0
		b.Key = Key()
		p.Modules[mi].Lessons[li].Blocks = insert(bs, b, bi+1)
	default:
		return nil, inputError{"Неизвестное действие редактора"}
	}
	if e = h.saveTree(ctx, tx, p); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE academy_course_programs SET revision=revision+1,updated_at=NOW() WHERE id=$1`, id); e != nil {
		return nil, e
	}
	p, e = load(ctx, tx, course)
	if e != nil {
		return nil, e
	}
	return p, tx.Commit()
}
