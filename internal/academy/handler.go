package academy

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	db   *sql.DB
	user func(*http.Request) (int64, error)
	gate func(http.HandlerFunc) http.HandlerFunc
	root string
}

func New(db *sql.DB, user func(*http.Request) (int64, error), gate func(http.HandlerFunc) http.HandlerFunc, root string) *Handler {
	return &Handler{db, user, gate, root}
}

type inputError struct{ message string }

func (e inputError) Error() string { return e.message }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func problem(w http.ResponseWriter, e error) {
	var in inputError
	switch {
	case errors.Is(e, ErrConflict):
		fail(w, 409, e.Error())
	case errors.Is(e, sql.ErrNoRows):
		fail(w, 404, "Курс или материал не найден")
	case errors.Is(e, ErrInvalid), errors.As(e, &in):
		fail(w, 400, e.Error())
	default:
		log.Printf("academy: %v", e)
		fail(w, 500, "Не удалось сохранить учебную программу. Ваши правки остаются в редакторе")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	d := json.NewDecoder(r.Body)
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Некорректные данные программы")
		return false
	}
	return true
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/academy/courses/", h.gate(h.route))
}
func (h *Handler) route(w http.ResponseWriter, r *http.Request) {
	user, e := h.user(r)
	if e != nil || user <= 0 {
		fail(w, 401, "Войдите в аккаунт")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/academy/courses/"), "/"), "/")
	course, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || course <= 0 {
		fail(w, 404, "Курс не найден")
		return
	}
	var owner int64
	e = h.db.QueryRowContext(r.Context(), `SELECT c.owner_id FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND c.owner_id=$2 AND NOT u.is_blocked`, course, user).Scan(&owner)
	if e != nil {
		problem(w, e)
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if action == "assets" {
		h.assets(w, r, course, user, parts[2:])
		return
	}
	if oneOf(action, "modules", "lessons", "blocks") {
		h.entities(w, r, course, user, parts[1:])
		return
	}
	switch {
	case action == "" && r.Method == "GET":
		h.workspace(w, r, course, user)
	case action == "program" && r.Method == "GET":
		p, e := load(r.Context(), h.db, course)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, p)
	case action == "program" && r.Method == "POST":
		var v struct {
			Source string `json:"source"`
		}
		if !decode(w, r, &v) {
			return
		}
		p, e := h.initialize(r.Context(), course, user, v.Source)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, p)
	case action == "program" && r.Method == "PUT":
		var v Program
		if !decode(w, r, &v) {
			return
		}
		p, e := h.save(r.Context(), course, user, &v)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, p)
	case action == "actions" && r.Method == "POST":
		var v Action
		if !decode(w, r, &v) {
			return
		}
		p, e := h.action(r.Context(), course, user, v)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, p)
	case action == "versions" && r.Method == "POST":
		var v struct {
			Revision int64 `json:"revision"`
		}
		if !decode(w, r, &v) {
			return
		}
		result, e := h.snapshot(r.Context(), course, user, v.Revision)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, result)
	case action == "preview" && r.Method == "GET":
		p, e := load(r.Context(), h.db, course)
		if e != nil {
			problem(w, e)
			return
		}
		for _, m := range p.Modules {
			for _, l := range m.Lessons {
				if len(parts) > 2 && l.Key == parts[2] {
					respond(w, 200, l)
					return
				}
			}
		}
		fail(w, 404, "Урок не найден")
	default:
		fail(w, 405, "Метод или действие не поддерживается")
	}
}

// Resource routes are thin aliases of the same revision-checked command handler.
func (h *Handler) entities(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	kind := path[0]
	key := ""
	suffix := ""
	if len(path) > 1 {
		key = path[1]
	}
	if len(path) > 2 {
		suffix = path[2]
	}
	if r.Method == "GET" {
		p, e := load(r.Context(), h.db, course)
		if e != nil {
			problem(w, e)
			return
		}
		if kind == "modules" && key == "" {
			respond(w, 200, p.Modules)
			return
		}
		for _, m := range p.Modules {
			if kind == "modules" && m.Key == key {
				if suffix == "lessons" {
					respond(w, 200, m.Lessons)
				} else {
					respond(w, 200, m)
				}
				return
			}
			for _, l := range m.Lessons {
				if kind == "lessons" && l.Key == key {
					if suffix == "blocks" {
						respond(w, 200, l.Blocks)
					} else {
						respond(w, 200, l)
					}
					return
				}
				for _, b := range l.Blocks {
					if kind == "blocks" && b.Key == key {
						respond(w, 200, b)
						return
					}
				}
			}
		}
		fail(w, 404, "Элемент программы не найден")
		return
	}
	var a Action
	if !decode(w, r, &a) {
		return
	}
	a.Key = key
	singular := map[string]string{"modules": "module", "lessons": "lesson", "blocks": "block"}[kind]
	switch {
	case r.Method == "POST" && kind == "modules" && key == "":
		a.Type = "module.create"
	case r.Method == "POST" && kind == "modules" && suffix == "lessons":
		a.Type = "lesson.create"
		a.Target = key
	case r.Method == "POST" && kind == "lessons" && suffix == "blocks":
		a.Type = "block.create"
	case r.Method == "POST" && oneOf(suffix, "move", "duplicate"):
		a.Type = singular + "." + suffix
	case r.Method == "PUT" && key != "" && suffix == "":
		a.Type = singular + ".update"
	case r.Method == "DELETE" && key != "" && suffix == "":
		a.Type = singular + ".delete"
	default:
		fail(w, 405, "Метод не поддерживается")
		return
	}
	p, e := h.action(r.Context(), course, user, a)
	if e != nil {
		problem(w, e)
		return
	}
	respond(w, 200, p)
}
func (h *Handler) workspace(w http.ResponseWriter, r *http.Request, course, user int64) {
	var raw []byte
	e := h.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('id',id,'owner_id',owner_id,'title',title,'cover_image',cover_image,'status',status,'slug',slug,'landing_modules',(SELECT count(*) FROM course_modules WHERE course_id=c.id),'landing_lessons',(SELECT count(*) FROM course_lessons l JOIN course_modules m ON m.id=l.module_id WHERE m.course_id=c.id)) FROM courses c WHERE id=$1`, course).Scan(&raw)
	if e != nil {
		problem(w, e)
		return
	}
	p, e := load(r.Context(), h.db, course)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		problem(w, e)
		return
	}
	var tests, versions []byte
	e = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',v.id,'test_id',t.id,'title',v.title,'version',v.version) ORDER BY v.id DESC),'[]') FROM test_versions v JOIN tests t ON t.id=v.test_id WHERE t.author_id=$1 AND t.deleted_at IS NULL AND t.status NOT IN ('deleted','blocked','archived')`, user).Scan(&tests)
	if e != nil {
		problem(w, e)
		return
	}
	e = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',v.id,'number',v.number,'revision',v.source_revision,'created_at',v.created_at) ORDER BY number DESC),'[]') FROM academy_program_versions v JOIN academy_course_programs p ON p.id=v.program_id WHERE p.course_id=$1`, course).Scan(&versions)
	if e != nil {
		problem(w, e)
		return
	}
	respond(w, 200, map[string]any{"course": json.RawMessage(raw), "program": p, "tests": json.RawMessage(tests), "versions": json.RawMessage(versions)})
}
