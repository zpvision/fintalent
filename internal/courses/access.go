package courses

import (
	"context"
	"database/sql"
	"net/http"
)

func AccessAllowed(ctx context.Context, db *sql.DB, userID int64) (bool, error) {
	var allowed bool
	err := db.QueryRowContext(ctx, `SELECT public_access OR EXISTS(SELECT 1 FROM users WHERE id=$1 AND courses_access AND NOT is_blocked AND NOT is_system) FROM course_access_settings WHERE id=TRUE`, userID).Scan(&allowed)
	return allowed, err
}

func (h *Handler) CanAccess(r *http.Request) (bool, error) {
	if h.admin(r) {
		return true, nil
	}
	userID, _ := h.user(r)
	return AccessAllowed(r.Context(), h.db, userID)
}

func (h *Handler) Gate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed, err := h.CanAccess(r)
		if err != nil {
			dbError(w, err)
			return
		}
		if !allowed {
			fail(w, http.StatusNotFound, "Раздел недоступен")
			return
		}
		next(w, r)
	}
}

func (h *Handler) access(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	allowed, err := h.CanAccess(r)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, map[string]bool{"allowed": allowed})
}
