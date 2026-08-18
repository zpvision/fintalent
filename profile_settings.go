package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type profileNameInput struct {
	FullName string `json:"full_name"`
}

type profilePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func registerProfileSettingsRoutes() {
	http.HandleFunc("/api/profile/name", updateProfileName)
	http.HandleFunc("/api/profile/password", updateProfilePassword)
}

func decodeProfileSettings(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректные данные")
		return false
	}
	return true
}

func updateProfileName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	var input profileNameInput
	if !decodeProfileSettings(w, r, &input) {
		return
	}
	input.FullName = strings.Join(strings.Fields(input.FullName), " ")
	if length := len([]rune(input.FullName)); length < 3 || length > 200 {
		writeJSON(w, http.StatusBadRequest, "ФИО должно содержать от 3 до 200 символов")
		return
	}
	if _, err = db.ExecContext(r.Context(), `UPDATE users SET full_name=$1 WHERE id=$2`, input.FullName, u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить ФИО")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"full_name": input.FullName})
}

func updateProfilePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	var input profilePasswordInput
	if !decodeProfileSettings(w, r, &input) {
		return
	}
	if len(input.CurrentPassword) == 0 {
		writeJSON(w, http.StatusBadRequest, "Введите текущий пароль")
		return
	}
	if length := len(input.NewPassword); length < 8 || length > 72 {
		writeJSON(w, http.StatusBadRequest, "Новый пароль должен содержать от 8 до 72 символов")
		return
	}
	if input.CurrentPassword == input.NewPassword {
		writeJSON(w, http.StatusBadRequest, "Новый пароль должен отличаться от текущего")
		return
	}
	var currentHash string
	if err = db.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&currentHash); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось проверить текущий пароль")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.CurrentPassword)) != nil {
		writeJSON(w, http.StatusUnauthorized, "Текущий пароль указан неверно")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось обработать новый пароль")
		return
	}
	if _, err = db.ExecContext(r.Context(), `UPDATE users SET password_hash=$1 WHERE id=$2`, string(newHash), u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить новый пароль")
		return
	}
	writeJSON(w, http.StatusOK, "Пароль успешно изменён")
}
