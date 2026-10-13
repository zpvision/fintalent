package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
)

func decodeCourseAccess(w http.ResponseWriter, r *http.Request, field string) (bool, bool) {
	var payload map[string]*bool
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	if decoder.Decode(&payload) != nil || len(payload) != 1 || payload[field] == nil || decoder.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, "Укажите настройку доступа")
		return false, false
	}
	return *payload[field], true
}

func adminCourseAccess(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		var public bool
		if err := db.QueryRowContext(r.Context(), `SELECT public_access FROM course_access_settings WHERE id=TRUE`).Scan(&public); err != nil {
			writeJSON(w, 500, "Не удалось загрузить настройку курсов")
			return
		}
		writeAdminJSON(w, 200, map[string]bool{"public_access": public})
	case http.MethodPut:
		public, ok := decodeCourseAccess(w, r, "public_access")
		if !ok {
			return
		}
		result, err := db.ExecContext(r.Context(), `UPDATE course_access_settings SET public_access=$1 WHERE id=TRUE`, public)
		if err != nil {
			writeJSON(w, 500, "Не удалось изменить настройку курсов")
			return
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			writeJSON(w, 500, "Не удалось изменить настройку курсов")
			return
		}
		writeAdminJSON(w, 200, map[string]bool{"public_access": public})
	default:
		writeJSON(w, 405, "Метод не поддерживается")
	}
}

func adminUserCourseAccess(w http.ResponseWriter, r *http.Request, userID int64) {
	allowed, ok := decodeCourseAccess(w, r, "courses_access")
	if !ok {
		return
	}
	var saved bool
	err := db.QueryRowContext(r.Context(), `UPDATE users SET courses_access=$1 WHERE id=$2 AND NOT is_system RETURNING courses_access`, allowed, userID).Scan(&saved)
	if err == sql.ErrNoRows {
		writeJSON(w, 404, "Пользователь не найден")
		return
	}
	if err != nil {
		writeJSON(w, 500, "Не удалось изменить доступ к курсам")
		return
	}
	writeAdminJSON(w, 200, map[string]bool{"courses_access": saved})
}
