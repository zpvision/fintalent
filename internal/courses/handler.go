package courses

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	db    *sql.DB
	user  func(*http.Request) (int64, error)
	admin func(*http.Request) bool
}

func New(db *sql.DB, user func(*http.Request) (int64, error), admin func(*http.Request) bool) *Handler {
	return &Handler{db, user, admin}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	if d.Decode(v) != nil {
		fail(w, 400, "Некорректные данные формы")
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Некорректный JSON")
		return false
	}
	return true
}
func (h *Handler) current(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := h.user(r)
	if e != nil || id <= 0 {
		fail(w, 401, "Войдите в аккаунт")
		return 0, false
	}
	return id, true
}
func dbError(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "Курс не найден")
		return
	}
	log.Printf("courses: %v", e)
	fail(w, 500, "Не удалось обработать курс. Попробуйте ещё раз")
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/courses/access", h.access)
	mux.HandleFunc("/api/courses/meta", h.Gate(h.meta))
	mux.HandleFunc("/api/courses", h.Gate(h.catalog))
	mux.HandleFunc("/api/courses/my", h.Gate(h.mine))
	mux.HandleFunc("/api/courses/my/", h.Gate(h.owned))
	mux.HandleFunc("/api/courses/view/", h.Gate(h.public))
	mux.HandleFunc("/api/courses/favorites/", h.Gate(h.favorite))
	mux.HandleFunc("/api/admin/courses", h.adminList)
	mux.HandleFunc("/api/admin/courses/", h.adminAction)
}
func (h *Handler) meta(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var raw json.RawMessage
	e := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',i.id,'name',i.value) ORDER BY i.sort_order,i.id),'[]') FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE d.alias='course_categories' AND i.active AND i.deleted_at IS NULL`).Scan(&raw)
	if e != nil {
		dbError(w, e)
		return
	}
	var features json.RawMessage
	e = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',i.id,'name',i.value,'key',COALESCE(b.feature_key,'')) ORDER BY i.sort_order,i.id),'[]') FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id LEFT JOIN course_feature_bindings b ON b.dictionary_item_id=i.id WHERE d.alias='course_features' AND i.active AND i.deleted_at IS NULL`).Scan(&features)
	if e != nil {
		dbError(w, e)
		return
	}
	var lessonTypes json.RawMessage
	e = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'key',key,'name',name,'icon',icon) ORDER BY sort_order,id),'[]') FROM course_lesson_types WHERE active AND deleted_at IS NULL`).Scan(&lessonTypes)
	if e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, map[string]any{"categories": raw, "features": features, "lesson_types": lessonTypes})
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request, where string, args ...any) {
	rows, e := h.db.QueryContext(r.Context(), `SELECT to_jsonb(c)||jsonb_build_object('category',COALESCE(d.value,''),'author_name',u.full_name) FROM courses c JOIN users u ON u.id=c.owner_id LEFT JOIN dictionary_items d ON d.id=c.category_id WHERE `+where+` ORDER BY c.updated_at DESC,c.id DESC LIMIT 200`, args...)
	if e != nil {
		dbError(w, e)
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			dbError(w, e)
			return
		}
		items = append(items, b)
	}
	if e = rows.Err(); e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, items)
}
func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.list(w, r, "c.status='published' AND NOT u.is_blocked")
}
func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	id, ok := h.current(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case "GET":
		h.list(w, r, "c.owner_id=$1", id)
	case "POST":
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			dbError(w, e)
			return
		}
		defer tx.Rollback()
		var course int64
		slug := fmt.Sprintf("kurs-%x", rand.Text())
		e = tx.QueryRowContext(r.Context(), `INSERT INTO courses(owner_id,slug) VALUES($1,$2) RETURNING id`, id, slug).Scan(&course)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `INSERT INTO course_instructors(course_id,user_id) VALUES($1,$2)`, course, id)
		}
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `INSERT INTO course_feature_selections(course_id,dictionary_item_id) SELECT $1,b.dictionary_item_id FROM course_feature_bindings b JOIN dictionary_items i ON i.id=b.dictionary_item_id WHERE b.feature_key IN ('updates','any_device') AND i.active AND i.deleted_at IS NULL`, course)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			dbError(w, e)
			return
		}
		respond(w, 201, map[string]int64{"id": course})
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}
func pathID(path, prefix string) (int64, string) {
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	if len(parts) > 2 {
		id = 0
	}
	return id, action
}
func (h *Handler) owned(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.current(w, r)
	if !ok {
		return
	}
	id, action := pathID(r.URL.Path, "/api/courses/my/")
	if id <= 0 {
		fail(w, 404, "Курс не найден")
		return
	}
	if action != "" {
		if r.Method != "POST" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		h.transition(w, r, id, uid, action, false)
		return
	}
	switch r.Method {
	case "GET":
		b, e := h.detail(r.Context(), "c.id=$1 AND c.owner_id=$2", id, uid)
		if e != nil {
			dbError(w, e)
			return
		}
		respond(w, 200, b)
	case "PUT":
		var v Input
		if !decode(w, r, &v) {
			return
		}
		if e := Validate(&v, false); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if e := h.save(r.Context(), id, uid, &v); e != nil {
			if errors.Is(e, ErrConflict) {
				fail(w, 409, e.Error())
			} else if errors.Is(e, ErrChild) || errors.Is(e, ErrCategory) || errors.Is(e, ErrFeature) || errors.Is(e, ErrLessonType) || errors.Is(e, ErrCustomFeature) {
				fail(w, 400, e.Error())
			} else {
				dbError(w, e)
			}
			return
		}
		b, e := h.detail(r.Context(), "c.id=$1 AND c.owner_id=$2", id, uid)
		if e != nil {
			dbError(w, e)
			return
		}
		respond(w, 200, b)
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}
func (h *Handler) public(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/courses/view/")
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	b, e := h.detail(r.Context(), "c.slug=$1 AND c.status='published' AND NOT owner.is_blocked", slug)
	if e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, b)
}
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, id, uid int64, action string, admin bool) {
	where := "c.id=$1 AND c.owner_id=$2"
	args := []any{id, uid}
	if admin {
		where = "c.id=$1"
		args = []any{id}
	}
	raw, e := h.detail(r.Context(), where, args...)
	if e != nil {
		dbError(w, e)
		return
	}
	var v Input
	_ = json.Unmarshal(raw, &v)
	var state struct {
		Status      string  `json:"status"`
		Slug        string  `json:"slug"`
		PublishedAt *string `json:"published_at"`
	}
	_ = json.Unmarshal(raw, &state)
	target := ""
	switch {
	case action == "submit" && !admin && (state.Status == "draft" || state.Status == "hidden"):
		target = "moderation"
	case action == "hide" && (state.Status == "published" || state.Status == "moderation"):
		target = "hidden"
	case action == "approve" && admin && state.Status == "moderation":
		target = "published"
	case action == "reject" && admin && state.Status == "moderation":
		target = "draft"
	default:
		fail(w, 409, "Это действие недоступно для текущего статуса курса")
		return
	}
	if target == "moderation" || target == "published" {
		if e = Validate(&v, true); e != nil {
			fail(w, 400, e.Error())
			return
		}
		var active bool
		if e = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE i.id=$1 AND d.alias='course_categories' AND i.active AND i.deleted_at IS NULL)`, v.CategoryID).Scan(&active); e != nil {
			dbError(w, e)
			return
		}
		if !active {
			fail(w, 400, ErrCategory.Error())
			return
		}
	}
	slug := state.Slug
	if state.PublishedAt == nil {
		slug = fmt.Sprintf("%s-%d", Slug(v.Title), id)
	}
	result, e := h.db.ExecContext(r.Context(), `UPDATE courses SET status=$1::text,slug=$2,revision=revision+1,updated_at=NOW(),published_at=CASE WHEN $1::text='published' THEN COALESCE(published_at,NOW()) ELSE published_at END WHERE id=$3 AND revision=$4`, target, slug, id, v.Revision)
	if e != nil {
		dbError(w, e)
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		fail(w, 409, ErrConflict.Error())
		return
	}
	respond(w, 200, map[string]string{"status": target})
}
func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход администратора")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.list(w, r, "TRUE")
}
func (h *Handler) adminAction(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход администратора")
		return
	}
	id, action := pathID(r.URL.Path, "/api/admin/courses/")
	if id <= 0 {
		fail(w, 404, "Курс не найден")
		return
	}
	if r.Method == "GET" && action == "" {
		b, e := h.detail(r.Context(), "c.id=$1", id)
		if e != nil {
			dbError(w, e)
			return
		}
		respond(w, 200, b)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.transition(w, r, id, 0, action, true)
}
