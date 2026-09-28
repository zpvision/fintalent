package main

import (
	"database/sql"
	"net/http"
)

// Keep publication visibility and author blocking stable until the interaction
// is committed. The caller must use this transaction for all related writes.
func beginPublicationInteraction(w http.ResponseWriter, r *http.Request, id int64) (*sql.Tx, bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить действие")
		return nil, false
	}
	var found int64
	err = tx.QueryRowContext(r.Context(), `SELECT p.id FROM publications p JOIN users u ON u.id=p.author_id WHERE p.id=$1 AND p.deleted_at IS NULL AND p.status='published' AND p.visibility IN('public','unlisted') AND (NOT u.is_blocked OR u.is_system) FOR SHARE OF p,u`, id).Scan(&found)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			writeJSON(w, 404, "Публикация не найдена")
		} else {
			writeJSON(w, 500, "Не удалось проверить доступ")
		}
		return nil, false
	}
	return tx, true
}

func commitPublicationInteraction(w http.ResponseWriter, tx *sql.Tx) bool {
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось сохранить действие")
		return false
	}
	return true
}

func requireProfiAccess(w http.ResponseWriter, r *http.Request, id int64, allowOwner bool) bool {
	uid := int64(0)
	if allowOwner {
		if u := profiCurrentUser(r); u != nil {
			uid = u.ID
		}
	}
	var allowed bool
	err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM profimarket_solutions s JOIN users u ON u.id=s.author_user_id WHERE s.id=$1 AND s.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) AND (s.status='PUBLISHED' OR s.author_user_id=$2))`, id, uid).Scan(&allowed)
	if err != nil {
		writeJSON(w, 500, "Не удалось проверить доступ")
		return false
	}
	if !allowed {
		writeJSON(w, 404, "Решение не найдено")
		return false
	}
	return true
}

func requirePublicationAccess(w http.ResponseWriter, r *http.Request, id int64, allowOwner bool) bool {
	uid := int64(0)
	if allowOwner {
		if u, err := userFromRequest(r); err == nil {
			uid = u.ID
		}
	}
	var allowed bool
	err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM publications p JOIN users u ON u.id=p.author_id WHERE p.id=$1 AND p.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) AND (p.author_id=$2 OR (p.status='published' AND p.visibility IN('public','unlisted'))))`, id, uid).Scan(&allowed)
	if err != nil {
		writeJSON(w, 500, "Не удалось проверить доступ")
		return false
	}
	if !allowed {
		writeJSON(w, 404, "Публикация не найдена")
		return false
	}
	return true
}
