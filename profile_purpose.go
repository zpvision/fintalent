package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	profileModeJobSearch    = "job_search"
	profileModeProfessional = "professional"
)

//go:embed migrations/058_profile_purpose.sql
var profilePurposeMigrationFS embed.FS

type profilePurposePayload struct {
	Mode string `json:"mode"`
}

func prepareProfilePurposeDatabase(ctx context.Context) error {
	schema, err := profilePurposeMigrationFS.ReadFile("migrations/058_profile_purpose.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func registerProfilePurposeRoutes() {
	http.HandleFunc("/api/profile-purpose", profilePurposeHandler)
}

func validProfileMode(value string) bool {
	return value == profileModeJobSearch || value == profileModeProfessional
}

func profilePurposeHandler(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}

	switch r.Method {
	case http.MethodGet:
		var mode sql.NullString
		if err := db.QueryRowContext(r.Context(), `SELECT profile_mode FROM users WHERE id=$1`, u.ID).Scan(&mode); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить назначение профиля")
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{
			"mode":     mode.String,
			"selected": mode.Valid && validProfileMode(mode.String),
		})
	case http.MethodPut:
		var payload profilePurposePayload
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		payload.Mode = strings.TrimSpace(payload.Mode)
		if !validProfileMode(payload.Mode) {
			writeJSON(w, http.StatusBadRequest, "Выберите назначение профиля")
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
			return
		}
		defer tx.Rollback()
		var previousMode sql.NullString
		if err = tx.QueryRowContext(r.Context(), `SELECT profile_mode FROM users WHERE id=$1 FOR UPDATE`, u.ID).Scan(&previousMode); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
			return
		}
		modeChanged := !previousMode.Valid || previousMode.String != payload.Mode
		if _, err = tx.ExecContext(r.Context(), `UPDATE users SET profile_mode=$1 WHERE id=$2`, payload.Mode, u.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
			return
		}
		if modeChanged {
			if _, err = tx.ExecContext(r.Context(), `UPDATE resumes SET current_step=1,updated_at=NOW() WHERE user_id=$1`, u.ID); err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
				return
			}
		}
		if payload.Mode == profileModeProfessional {
			if _, err = tx.ExecContext(r.Context(), `
				UPDATE resumes
				SET search_status_code='not_active',
					visibility=CASE WHEN status='published' THEN 'public' ELSE visibility END,
					updated_at=NOW()
				WHERE user_id=$1`, u.ID); err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось изменить назначение профиля")
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{
			"mode":                        payload.Mode,
			"selected":                    true,
			"requires_profile_completion": modeChanged && payload.Mode == profileModeJobSearch,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
