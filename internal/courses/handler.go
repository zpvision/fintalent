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
	"unicode/utf8"
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
	rows, e := h.db.QueryContext(r.Context(), `SELECT to_jsonb(c)||jsonb_build_object('category',COALESCE(d.value,''),'author_name',u.full_name,'min_price',CASE WHEN c.pricing_mode='tariffs' THEN (SELECT MIN(t.price) FROM course_tariffs t WHERE t.course_id=c.id) ELSE c.price END) FROM courses c JOIN users u ON u.id=c.owner_id LEFT JOIN dictionary_items d ON d.id=c.category_id WHERE `+where+` ORDER BY c.updated_at DESC,c.id DESC LIMIT 200`, args...)
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
		h.transition(w, r, id, uid, action, false, "", 0)
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
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, id, uid int64, action string, admin bool, reason string, expectedRevision int64) {
	tx, e := h.db.BeginTx(r.Context(), nil)
	if e != nil {
		dbError(w, e)
		return
	}
	defer tx.Rollback()
	var before json.RawMessage
	if admin {
		if e = LockPublication(r.Context(), tx); e != nil {
			dbError(w, e)
			return
		}
		var locked int64
		if e = tx.QueryRowContext(r.Context(), "SELECT id FROM courses WHERE id=$1 FOR UPDATE", id).Scan(&locked); e != nil {
			dbError(w, e)
			return
		}
		before, e = AdministrativeCourse(r.Context(), tx, id)
		if e != nil {
			dbError(w, e)
			return
		}
	}
	if admin {
		var current struct{ Revision int64 }
		_ = json.Unmarshal(before, &current)
		if current.Revision != expectedRevision {
			fail(w, 409, ErrConflict.Error())
			return
		}
	}
	target, e := transitionCourse(r.Context(), tx, id, uid, action, admin)
	if e == nil && admin {
		// Preserve the legacy endpoint and identify its existing configured administrator explicitly.
		var hasAudit bool
		e = tx.QueryRowContext(r.Context(), "SELECT to_regclass('academy_admin_audit') IS NOT NULL").Scan(&hasAudit)
		if e == nil && hasAudit {
			var old struct {
				Status   string
				Revision int64
			}
			_ = json.Unmarshal(before, &old)
			_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_admin_audit(actor_key,actor_name,action,entity,entity_id,course_id,reason,result,before_value,after_value,request_key,fingerprint) VALUES('platform_admin','Администратор FinTalent',$1,'course',$2,$2,$6,'succeeded',$3,$4,$5,'legacy-course-moderation')`, action, id, string(mustJSON(map[string]any{"status": old.Status, "revision": old.Revision})), string(mustJSON(map[string]any{"status": target})), rand.Text(), reason)
		}
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		var validation TransitionError
		if errors.As(e, &validation) {
			fail(w, validation.Status, validation.Message)
		} else if errors.Is(e, ErrConflict) {
			fail(w, 409, e.Error())
		} else {
			dbError(w, e)
		}
		return
	}
	respond(w, 200, map[string]string{"status": target})
}
func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
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
	if e := http.NewCrossOriginProtection().Check(r); e != nil {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	var command struct {
		Reason   string `json:"reason"`
		Confirm  bool   `json:"confirm"`
		Revision int64  `json:"revision"`
	}
	if !decode(w, r, &command) {
		return
	}
	command.Reason = strings.TrimSpace(command.Reason)
	if !command.Confirm || utf8.RuneCountInString(command.Reason) < 5 || utf8.RuneCountInString(command.Reason) > 1000 || command.Revision <= 0 {
		fail(w, 400, "Подтвердите действие, укажите причину и текущую ревизию курса")
		return
	}
	h.transition(w, r, id, 0, action, true, command.Reason, command.Revision)
}
