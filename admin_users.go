package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type adminUser struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	IsBlocked bool      `json:"is_blocked"`
	CreatedAt time.Time `json:"created_at"`
}

func adminUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT id,email,full_name,is_blocked,created_at FROM users ORDER BY created_at DESC,id DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить пользователей")
		return
	}
	defer rows.Close()
	users := []adminUser{}
	for rows.Next() {
		var item adminUser
		if err = rows.Scan(&item.ID, &item.Email, &item.FullName, &item.IsBlocked, &item.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить пользователей")
			return
		}
		users = append(users, item)
	}
	if err = rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить пользователей")
		return
	}
	writeAdminJSON(w, http.StatusOK, users)
}

func adminUserAction(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/"), "/")
	if len(parts) != 2 || r.Method != http.MethodPut {
		writeJSON(w, http.StatusNotFound, "Действие не найдено")
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректный пользователь")
		return
	}
	switch parts[1] {
	case "block":
		var payload struct {
			IsBlocked bool `json:"is_blocked"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&payload) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пользователя")
			return
		}
		defer tx.Rollback()
		result, execErr := tx.ExecContext(r.Context(), `UPDATE users SET is_blocked=$1 WHERE id=$2`, payload.IsBlocked, userID)
		if execErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пользователя")
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeJSON(w, http.StatusNotFound, "Пользователь не найден")
			return
		}
		if payload.IsBlocked {
			if _, execErr = tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id=$1`, userID); execErr != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось заблокировать пользователя")
				return
			}
		}
		if tx.Commit() != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пользователя")
			return
		}
		writeJSON(w, http.StatusOK, "Статус пользователя изменён")
	case "password":
		var payload struct {
			Password string `json:"password"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&payload) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		if len(payload.Password) < 8 || len(payload.Password) > 72 {
			writeJSON(w, http.StatusBadRequest, "Пароль должен содержать от 8 до 72 символов")
			return
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось обработать пароль")
			return
		}
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пароль")
			return
		}
		defer tx.Rollback()
		result, execErr := tx.ExecContext(r.Context(), `UPDATE users SET password_hash=$1 WHERE id=$2`, string(hash), userID)
		if execErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пароль")
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeJSON(w, http.StatusNotFound, "Пользователь не найден")
			return
		}
		if _, execErr = tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id=$1`, userID); execErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пароль")
			return
		}
		if tx.Commit() != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пароль")
			return
		}
		writeJSON(w, http.StatusOK, "Пароль изменён")
	default:
		writeJSON(w, http.StatusNotFound, "Действие не найдено")
	}
}
