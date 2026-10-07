package courses

import "net/http"

func (h *Handler) favorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	uid, ok := h.current(w, r)
	if !ok {
		return
	}
	id, action := pathID(r.URL.Path, "/api/courses/favorites/")
	if id <= 0 || action != "" {
		fail(w, 404, "Курс не найден")
		return
	}
	var exists bool
	if e := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND c.status='published' AND NOT u.is_blocked)`, id).Scan(&exists); e != nil {
		dbError(w, e)
		return
	}
	if !exists {
		fail(w, 404, "Курс не найден")
		return
	}
	if r.Method == "POST" {
		var v struct {
			Favorite bool `json:"favorite"`
		}
		if !decode(w, r, &v) {
			return
		}
		query := `DELETE FROM course_favorites WHERE course_id=$1 AND user_id=$2`
		if v.Favorite {
			query = `INSERT INTO course_favorites(course_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`
		}
		if _, e := h.db.ExecContext(r.Context(), query, id, uid); e != nil {
			dbError(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	var favorite bool
	if e := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM course_favorites WHERE course_id=$1 AND user_id=$2)`, id, uid).Scan(&favorite); e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, map[string]bool{"favorite": favorite})
}
