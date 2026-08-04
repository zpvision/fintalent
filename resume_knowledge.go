package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type resumeKnowledgeConfirmer struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type resumeKnowledgeResult struct {
	TestID        int64                      `json:"test_id"`
	Title         string                     `json:"title"`
	Category      string                     `json:"category"`
	Difficulty    string                     `json:"difficulty"`
	Percent       float64                    `json:"percent"`
	Passed        bool                       `json:"passed"`
	FinishedAt    time.Time                  `json:"finished_at"`
	Duration      int                        `json:"duration_seconds"`
	Confirmations int                        `json:"confirmations"`
	ConfirmedByMe bool                       `json:"confirmed_by_me"`
	Confirmers    []resumeKnowledgeConfirmer `json:"confirmers"`
}

type resumeKnowledgeResponse struct {
	ResumeID   int64                   `json:"resume_id"`
	IsOwner    bool                    `json:"is_owner"`
	CanConfirm bool                    `json:"can_confirm"`
	Results    []resumeKnowledgeResult `json:"results"`
}

func resumeKnowledgeActionHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/resumes/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "test-knowledge" {
		http.NotFound(w, r)
		return
	}
	resumeID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || resumeID <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректное резюме")
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		serveResumeKnowledge(w, r, resumeID)
		return
	}
	if len(parts) == 3 && parts[2] == "confirmations" && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		changeResumeKnowledgeConfirmation(w, r, resumeID)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func serveResumeKnowledge(w http.ResponseWriter, r *http.Request, resumeID int64) {
	var ownerID int64
	if err := db.QueryRowContext(r.Context(), `SELECT user_id FROM resumes WHERE id=$1 AND status='published' AND deleted_at IS NULL`, resumeID).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, "Резюме не найдено")
			return
		}
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить знания")
		return
	}
	viewerID := int64(0)
	if viewer, err := userFromRequest(r); err == nil {
		viewerID = viewer.ID
	}
	response := resumeKnowledgeResponse{ResumeID: resumeID, IsOwner: viewerID == ownerID, CanConfirm: viewerID > 0 && viewerID != ownerID, Results: []resumeKnowledgeResult{}}
	rows, err := db.QueryContext(r.Context(), `
		SELECT DISTINCT ON (a.test_id) a.test_id,v.title,COALESCE(t.category,''),t.difficulty,
			a.percent,COALESCE(a.passed,FALSE),a.finished_at,a.duration_seconds
		FROM test_attempts a
		JOIN tests t ON t.id=a.test_id
		JOIN test_versions v ON v.id=a.test_version_id
		WHERE a.user_id=$1 AND a.status='finished' AND a.finished_at IS NOT NULL
		ORDER BY a.test_id,a.percent DESC,a.finished_at DESC`, ownerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить результаты тестов")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item resumeKnowledgeResult
		item.Confirmers = []resumeKnowledgeConfirmer{}
		if err = rows.Scan(&item.TestID, &item.Title, &item.Category, &item.Difficulty, &item.Percent, &item.Passed, &item.FinishedAt, &item.Duration); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить результаты тестов")
			return
		}
		confirmers, queryErr := db.QueryContext(r.Context(), `SELECT u.id,u.full_name,COALESCE(u.avatar_url,''),(u.id=$3) FROM resume_test_confirmations c JOIN users u ON u.id=c.confirmer_id WHERE c.resume_id=$1 AND c.test_id=$2 ORDER BY c.created_at LIMIT 12`, resumeID, item.TestID, viewerID)
		if queryErr != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить подтверждения")
			return
		}
		for confirmers.Next() {
			var c resumeKnowledgeConfirmer
			var mine bool
			if queryErr = confirmers.Scan(&c.ID, &c.Name, &c.Avatar, &mine); queryErr != nil {
				confirmers.Close()
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить подтверждения")
				return
			}
			item.Confirmers = append(item.Confirmers, c)
			item.ConfirmedByMe = item.ConfirmedByMe || mine
		}
		confirmers.Close()
		item.Confirmations = len(item.Confirmers)
		response.Results = append(response.Results, item)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func changeResumeKnowledgeConfirmation(w http.ResponseWriter, r *http.Request, resumeID int64) {
	viewer, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Войдите, чтобы подтвердить знания")
		return
	}
	var input struct {
		TestID int64 `json:"test_id"`
	}
	if err = json.NewDecoder(r.Body).Decode(&input); err != nil || input.TestID <= 0 {
		writeJSON(w, http.StatusBadRequest, "Выберите тест")
		return
	}
	var ownerID int64
	if err = db.QueryRowContext(r.Context(), `SELECT user_id FROM resumes WHERE id=$1 AND status='published' AND deleted_at IS NULL`, resumeID).Scan(&ownerID); err != nil {
		writeJSON(w, http.StatusNotFound, "Резюме не найдено")
		return
	}
	if ownerID == viewer.ID {
		writeJSON(w, http.StatusForbidden, "Нельзя подтверждать собственные знания")
		return
	}
	if r.Method == http.MethodDelete {
		_, err = db.ExecContext(r.Context(), `DELETE FROM resume_test_confirmations WHERE resume_id=$1 AND test_id=$2 AND confirmer_id=$3`, resumeID, input.TestID, viewer.ID)
	} else {
		_, err = db.ExecContext(r.Context(), `INSERT INTO resume_test_confirmations(resume_id,test_id,confirmer_id) SELECT $1,$2,$3 WHERE EXISTS(SELECT 1 FROM test_attempts WHERE user_id=$4 AND test_id=$2 AND status='finished') ON CONFLICT DO NOTHING`, resumeID, input.TestID, viewer.ID, ownerID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить подтверждение")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]bool{"confirmed": r.Method == http.MethodPost})
}
