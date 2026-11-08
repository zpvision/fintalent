package academy

import (
	"database/sql"
	"net/http"
)

// Publication is the result of landing moderation. Keep access after later
// landing edits/hiding; those operations preserve the first published_at.
func (h *Handler) requireCourseModeration(w http.ResponseWriter, r *http.Request, course int64) bool {
	if adminPrincipal(r.Context()).Can("edit") {
		return true
	}
	var approved bool
	err := h.db.QueryRowContext(r.Context(), `SELECT published_at IS NOT NULL OR status='published' FROM courses WHERE id=$1`, course).Scan(&approved)
	if err != nil {
		problem(w, err)
		return false
	}
	if !approved {
		fail(w, http.StatusForbidden, "Академия станет доступна после одобрения лендинга курса модератором.")
		return false
	}
	return true
}

// A missing owner match is not permission to view a workspace: the existing
// course/staff API checks remain authoritative for those requests.
func (h *Handler) ownerCourseModerated(r *http.Request, course, user int64) (bool, error) {
	var approved bool
	err := h.db.QueryRowContext(r.Context(), `SELECT published_at IS NOT NULL OR status='published' FROM courses WHERE id=$1 AND owner_id=$2`, course, user).Scan(&approved)
	if err == sql.ErrNoRows {
		return true, nil
	}
	return approved, err
}
