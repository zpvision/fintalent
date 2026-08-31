package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/045_help_module.sql
var helpMigrationFS embed.FS

type helpTopic struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Icon             string    `json:"icon"`
	ShortDescription string    `json:"short_description"`
	Active           bool      `json:"active"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type publicHelpStats struct {
	Completed   int     `json:"completed"`
	ReviewCount int     `json:"review_count"`
	Average     float64 `json:"average"`
}

type publicHelpReview struct {
	ID        int64     `json:"id"`
	Author    user      `json:"author"`
	Topic     helpTopic `json:"topic"`
	Rating    int       `json:"rating"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type publicResumeHelp struct {
	Topics  []helpTopic        `json:"topics"`
	Stats   publicHelpStats    `json:"stats"`
	Reviews []publicHelpReview `json:"reviews"`
}

type helpRequestPerson struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type helpRequestView struct {
	ID            int64             `json:"id"`
	Requester     helpRequestPerson `json:"requester"`
	Expert        helpRequestPerson `json:"expert"`
	Topic         helpTopic         `json:"topic"`
	Text          string            `json:"text"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	AcceptedAt    *time.Time        `json:"accepted_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	MessagesCount int               `json:"messages_count"`
	ReviewID      *int64            `json:"review_id,omitempty"`
	CanReview     bool              `json:"can_review"`
}

type helpMessageView struct {
	ID        int64             `json:"id"`
	Author    helpRequestPerson `json:"author"`
	Text      string            `json:"text"`
	CreatedAt time.Time         `json:"created_at"`
}

func prepareHelpDatabase(ctx context.Context) error {
	schema, err := helpMigrationFS.ReadFile("migrations/045_help_module.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func registerHelpRoutes() {
	http.HandleFunc("/api/admin/help-topics", adminHelpTopics)
	http.HandleFunc("/api/admin/help-topics/", adminHelpTopic)
	http.HandleFunc("/api/public/help-topics", publicHelpTopics)
	http.HandleFunc("/api/v1/resumes/help-topics", resumeHelpTopics)
	http.HandleFunc("/api/v1/help/requests", helpRequests)
	http.HandleFunc("/api/v1/help/requests/", helpRequestAction)
	http.HandleFunc("/api/v1/help/notifications", helpNotifications)
}

func adminHelpTopics(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := loadHelpTopics(r.Context(), false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить направления помощи")
			return
		}
		writeAdminJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item helpTopic
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&item) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		normalizeHelpTopic(&item)
		if item.Name == "" {
			writeJSON(w, http.StatusBadRequest, "Укажите название направления")
			return
		}
		err := db.QueryRowContext(r.Context(), `INSERT INTO help_topics(name,category,icon,short_description,is_active,sort_order)
			VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, item.Name, item.Category, item.Icon, item.ShortDescription, item.Active, item.SortOrder).Scan(&item.ID)
		if err != nil {
			writeJSON(w, http.StatusConflict, "Направление помощи с таким названием уже существует")
			return
		}
		writeAdminJSON(w, http.StatusCreated, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func adminHelpTopic(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/help-topics/"), "/"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректное направление помощи")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var item helpTopic
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&item) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		normalizeHelpTopic(&item)
		if item.Name == "" {
			writeJSON(w, http.StatusBadRequest, "Укажите название направления")
			return
		}
		result, err := db.ExecContext(r.Context(), `UPDATE help_topics SET name=$1,category=$2,icon=$3,short_description=$4,is_active=$5,sort_order=$6,updated_at=NOW()
			WHERE id=$7 AND deleted_at IS NULL`, item.Name, item.Category, item.Icon, item.ShortDescription, item.Active, item.SortOrder, id)
		if err != nil {
			writeJSON(w, http.StatusConflict, "Не удалось сохранить направление помощи")
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeJSON(w, http.StatusNotFound, "Направление помощи не найдено")
			return
		}
		writeJSON(w, http.StatusOK, "Направление помощи сохранено")
	case http.MethodDelete:
		result, err := db.ExecContext(r.Context(), `UPDATE help_topics SET is_active=FALSE,deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось удалить направление помощи")
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeJSON(w, http.StatusNotFound, "Направление помощи не найдено")
			return
		}
		writeJSON(w, http.StatusOK, "Направление помощи удалено")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func normalizeHelpTopic(item *helpTopic) {
	item.Name = strings.Join(strings.Fields(item.Name), " ")
	item.Category = strings.Join(strings.Fields(item.Category), " ")
	item.Icon = strings.TrimSpace(item.Icon)
	item.ShortDescription = strings.TrimSpace(item.ShortDescription)
	if len([]rune(item.Name)) > 200 {
		item.Name = string([]rune(item.Name)[:200])
	}
	if len([]rune(item.Category)) > 160 {
		item.Category = string([]rune(item.Category)[:160])
	}
	if len([]rune(item.Icon)) > 500 {
		item.Icon = string([]rune(item.Icon)[:500])
	}
	if len([]rune(item.ShortDescription)) > 2000 {
		item.ShortDescription = string([]rune(item.ShortDescription)[:2000])
	}
}

func loadHelpTopics(ctx context.Context, activeOnly bool) ([]helpTopic, error) {
	where := "deleted_at IS NULL"
	if activeOnly {
		where += " AND is_active=TRUE"
	}
	rows, err := db.QueryContext(ctx, `SELECT id,name,category,icon,short_description,is_active,sort_order,created_at,updated_at
		FROM help_topics WHERE `+where+` ORDER BY sort_order,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []helpTopic{}
	for rows.Next() {
		var item helpTopic
		if err = rows.Scan(&item.ID, &item.Name, &item.Category, &item.Icon, &item.ShortDescription, &item.Active, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func publicHelpTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	items, err := loadHelpTopics(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить направления помощи")
		return
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func resumeHelpTopics(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := loadHelpTopics(r.Context(), true)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить направления помощи")
			return
		}
		selected, err := selectedResumeHelpTopicIDs(r.Context(), u.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить выбранные направления")
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{"items": items, "selected_ids": selected})
	case http.MethodPut:
		var payload struct {
			TopicIDs []int64 `json:"topic_ids"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&payload) != nil || len(payload.TopicIDs) > 30 || duplicateIDs(payload.TopicIDs) {
			writeJSON(w, http.StatusBadRequest, "Проверьте выбранные направления помощи")
			return
		}
		if err := saveResumeHelpTopics(r.Context(), u.ID, payload.TopicIDs); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusBadRequest, "Выбрано недоступное направление помощи")
				return
			}
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить направления помощи")
			return
		}
		writeJSON(w, http.StatusOK, "Направления помощи сохранены")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func selectedResumeHelpTopicIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT rht.topic_id FROM resumes r JOIN resume_help_topics rht ON rht.resume_id=r.id WHERE r.user_id=$1 AND r.deleted_at IS NULL ORDER BY rht.sort_order,rht.topic_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func saveResumeHelpTopics(ctx context.Context, userID int64, ids []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var resumeID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO resumes(user_id,current_step) VALUES($1,1) ON CONFLICT(user_id) DO UPDATE SET updated_at=NOW() RETURNING id`, userID).Scan(&resumeID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resume_help_topics WHERE resume_id=$1`, resumeID); err != nil {
		return err
	}
	for order, id := range ids {
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO resume_help_topics(resume_id,topic_id,sort_order)
			SELECT $1,id,$3 FROM help_topics WHERE id=$2 AND is_active=TRUE AND deleted_at IS NULL`, resumeID, id, order)
		if insertErr != nil {
			return insertErr
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}

func loadPublicResumeHelp(ctx context.Context, view *publicResumeView) error {
	view.Help = publicResumeHelp{Topics: []helpTopic{}, Reviews: []publicHelpReview{}}
	rows, err := db.QueryContext(ctx, `SELECT t.id,t.name,t.category,t.icon,t.short_description,t.is_active,t.sort_order,t.created_at,t.updated_at
		FROM resume_help_topics rht JOIN help_topics t ON t.id=rht.topic_id
		WHERE rht.resume_id=$1 AND t.is_active=TRUE AND t.deleted_at IS NULL
		ORDER BY rht.sort_order,t.sort_order,t.id`, view.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item helpTopic
		if err = rows.Scan(&item.ID, &item.Name, &item.Category, &item.Icon, &item.ShortDescription, &item.Active, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		view.Help.Topics = append(view.Help.Topics, item)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	var avg sql.NullFloat64
	if err = db.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM help_requests WHERE expert_id=$1 AND status='completed'),
			COUNT(hr.id),
			AVG(hr.rating)
		FROM help_reviews hr WHERE hr.recipient_id=$1`, view.OwnerID).Scan(&view.Help.Stats.Completed, &view.Help.Stats.ReviewCount, &avg); err != nil {
		return err
	}
	if avg.Valid {
		view.Help.Stats.Average = float64(int(avg.Float64*10+0.5)) / 10
	}
	reviewRows, err := db.QueryContext(ctx, `SELECT hr.id,u.id,u.full_name,COALESCE(u.avatar_url,''),t.id,t.name,t.category,t.icon,t.short_description,t.is_active,t.sort_order,t.created_at,t.updated_at,hr.rating,hr.text,hr.created_at
		FROM help_reviews hr
		JOIN users u ON u.id=hr.author_id
		JOIN help_requests req ON req.id=hr.help_request_id
		JOIN help_topics t ON t.id=req.topic_id
		WHERE hr.recipient_id=$1
		ORDER BY hr.created_at DESC LIMIT 50`, view.OwnerID)
	if err != nil {
		return err
	}
	defer reviewRows.Close()
	for reviewRows.Next() {
		var item publicHelpReview
		if err = reviewRows.Scan(&item.ID, &item.Author.ID, &item.Author.FullName, &item.Author.Avatar, &item.Topic.ID, &item.Topic.Name, &item.Topic.Category, &item.Topic.Icon, &item.Topic.ShortDescription, &item.Topic.Active, &item.Topic.SortOrder, &item.Topic.CreatedAt, &item.Topic.UpdatedAt, &item.Rating, &item.Text, &item.CreatedAt); err != nil {
			return err
		}
		if item.Author.Avatar == "" {
			item.Author.Avatar = "/static/avatar-placeholder.svg"
		}
		view.Help.Reviews = append(view.Help.Reviews, item)
	}
	return reviewRows.Err()
}

func helpRequests(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	switch r.Method {
	case http.MethodGet:
		scope := strings.TrimSpace(r.URL.Query().Get("scope"))
		if scope == "" {
			scope = "incoming"
		}
		items, err := loadHelpRequests(r.Context(), u.ID, scope)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить обращения")
			return
		}
		writeAdminJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var payload struct {
			ResumeID int64  `json:"resume_id"`
			TopicID  int64  `json:"topic_id"`
			Text     string `json:"text"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&payload) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные обращения")
			return
		}
		payload.Text = strings.TrimSpace(payload.Text)
		if payload.ResumeID <= 0 || payload.TopicID <= 0 || len([]rune(payload.Text)) < 10 || len([]rune(payload.Text)) > 4000 {
			writeJSON(w, http.StatusBadRequest, "Выберите направление и опишите вопрос от 10 до 4000 символов")
			return
		}
		var expertID int64
		err = db.QueryRowContext(r.Context(), `SELECT r.user_id
			FROM resumes r JOIN resume_help_topics rht ON rht.resume_id=r.id
			JOIN help_topics t ON t.id=rht.topic_id
			WHERE r.id=$1 AND rht.topic_id=$2 AND r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL AND t.is_active=TRUE AND t.deleted_at IS NULL`, payload.ResumeID, payload.TopicID).Scan(&expertID)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusBadRequest, "Это направление недоступно у выбранного специалиста")
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось проверить направление помощи")
			return
		}
		if expertID == u.ID {
			writeJSON(w, http.StatusBadRequest, "Нельзя отправить запрос помощи самому себе")
			return
		}
		var id int64
		err = db.QueryRowContext(r.Context(), `INSERT INTO help_requests(requester_id,expert_id,topic_id,request_text) VALUES($1,$2,$3,$4) RETURNING id`, u.ID, expertID, payload.TopicID, payload.Text).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось отправить запрос помощи")
			return
		}
		writeAdminJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "new"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func loadHelpRequests(ctx context.Context, userID int64, scope string) ([]helpRequestView, error) {
	where := "req.expert_id=$1"
	if scope == "outgoing" {
		where = "req.requester_id=$1"
	} else if scope != "incoming" {
		return nil, errors.New("invalid scope")
	}
	rows, err := db.QueryContext(ctx, `SELECT req.id,req.request_text,req.status,req.created_at,req.accepted_at,req.completed_at,
			requester.id,requester.full_name,COALESCE(requester.avatar_url,''),
			expert.id,expert.full_name,COALESCE(expert.avatar_url,''),
			t.id,t.name,t.category,t.icon,t.short_description,t.is_active,t.sort_order,t.created_at,t.updated_at,
			(SELECT COUNT(*) FROM help_request_messages m WHERE m.help_request_id=req.id),
			review.id
		FROM help_requests req
		JOIN users requester ON requester.id=req.requester_id
		JOIN users expert ON expert.id=req.expert_id
		JOIN help_topics t ON t.id=req.topic_id
		LEFT JOIN help_reviews review ON review.help_request_id=req.id
		WHERE `+where+`
		ORDER BY CASE req.status WHEN 'new' THEN 0 WHEN 'accepted' THEN 1 WHEN 'completed' THEN 2 ELSE 3 END, req.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []helpRequestView{}
	for rows.Next() {
		var item helpRequestView
		var accepted, completed sql.NullTime
		var reviewID sql.NullInt64
		if err = rows.Scan(&item.ID, &item.Text, &item.Status, &item.CreatedAt, &accepted, &completed,
			&item.Requester.ID, &item.Requester.Name, &item.Requester.Avatar,
			&item.Expert.ID, &item.Expert.Name, &item.Expert.Avatar,
			&item.Topic.ID, &item.Topic.Name, &item.Topic.Category, &item.Topic.Icon, &item.Topic.ShortDescription, &item.Topic.Active, &item.Topic.SortOrder, &item.Topic.CreatedAt, &item.Topic.UpdatedAt,
			&item.MessagesCount, &reviewID); err != nil {
			return nil, err
		}
		if accepted.Valid {
			item.AcceptedAt = &accepted.Time
		}
		if completed.Valid {
			item.CompletedAt = &completed.Time
		}
		if reviewID.Valid {
			item.ReviewID = &reviewID.Int64
		}
		if item.Requester.Avatar == "" {
			item.Requester.Avatar = "/static/avatar-placeholder.svg"
		}
		if item.Expert.Avatar == "" {
			item.Expert.Avatar = "/static/avatar-placeholder.svg"
		}
		item.CanReview = scope == "outgoing" && item.Status == "completed" && !reviewID.Valid
		items = append(items, item)
	}
	return items, rows.Err()
}

func helpRequestAction(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/help/requests/"), "/"), "/")
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, "Некорректное действие")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, "Некорректное обращение")
		return
	}
	action := parts[1]
	if action == "messages" {
		helpRequestMessages(w, r, u.ID, id)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	switch action {
	case "accept":
		updateHelpRequestStatus(w, r, id, u.ID, "accepted")
	case "decline":
		updateHelpRequestStatus(w, r, id, u.ID, "declined")
	case "complete":
		updateHelpRequestStatus(w, r, id, u.ID, "completed")
	case "cancel":
		cancelHelpRequest(w, r, id, u.ID)
	case "review":
		createHelpReview(w, r, id, u.ID)
	default:
		writeJSON(w, http.StatusBadRequest, "Некорректное действие")
	}
}

func updateHelpRequestStatus(w http.ResponseWriter, r *http.Request, id, expertID int64, next string) {
	var query string
	switch next {
	case "accepted":
		query = `UPDATE help_requests SET status='accepted',accepted_at=NOW(),updated_at=NOW() WHERE id=$1 AND expert_id=$2 AND status='new'`
	case "declined":
		query = `UPDATE help_requests SET status='declined',updated_at=NOW() WHERE id=$1 AND expert_id=$2 AND status='new'`
	case "completed":
		query = `UPDATE help_requests SET status='completed',completed_at=NOW(),updated_at=NOW() WHERE id=$1 AND expert_id=$2 AND status='accepted'`
	default:
		writeJSON(w, http.StatusBadRequest, "Некорректный статус")
		return
	}
	result, err := db.ExecContext(r.Context(), query, id, expertID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось обновить обращение")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeJSON(w, http.StatusBadRequest, "Обращение не найдено или действие недоступно")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]string{"status": next})
}

func cancelHelpRequest(w http.ResponseWriter, r *http.Request, id, requesterID int64) {
	result, err := db.ExecContext(r.Context(), `UPDATE help_requests SET status='cancelled',updated_at=NOW() WHERE id=$1 AND requester_id=$2 AND status IN ('new','accepted')`, id, requesterID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось отменить обращение")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeJSON(w, http.StatusBadRequest, "Обращение не найдено или действие недоступно")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func helpRequestMessages(w http.ResponseWriter, r *http.Request, userID, requestID int64) {
	var status string
	var participant bool
	err := db.QueryRowContext(r.Context(), `SELECT status, requester_id=$2 OR expert_id=$2 FROM help_requests WHERE id=$1`, requestID, userID).Scan(&status, &participant)
	if err == sql.ErrNoRows || !participant {
		writeJSON(w, http.StatusNotFound, "Обращение не найдено")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить переписку")
		return
	}
	if status != "accepted" && status != "completed" {
		writeJSON(w, http.StatusBadRequest, "Переписка доступна после принятия запроса")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.QueryContext(r.Context(), `SELECT m.id,u.id,u.full_name,COALESCE(u.avatar_url,''),m.text,m.created_at
			FROM help_request_messages m JOIN users u ON u.id=m.author_id
			WHERE m.help_request_id=$1 ORDER BY m.created_at,m.id`, requestID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить переписку")
			return
		}
		defer rows.Close()
		items := []helpMessageView{}
		for rows.Next() {
			var item helpMessageView
			if err = rows.Scan(&item.ID, &item.Author.ID, &item.Author.Name, &item.Author.Avatar, &item.Text, &item.CreatedAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить переписку")
				return
			}
			if item.Author.Avatar == "" {
				item.Author.Avatar = "/static/avatar-placeholder.svg"
			}
			items = append(items, item)
		}
		writeAdminJSON(w, http.StatusOK, items)
	case http.MethodPost:
		if status != "accepted" {
			writeJSON(w, http.StatusBadRequest, "Нельзя отправлять сообщения в завершенном обращении")
			return
		}
		var payload struct {
			Text string `json:"text"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректное сообщение")
			return
		}
		payload.Text = strings.TrimSpace(payload.Text)
		if payload.Text == "" || len([]rune(payload.Text)) > 4000 {
			writeJSON(w, http.StatusBadRequest, "Сообщение должно быть от 1 до 4000 символов")
			return
		}
		var id int64
		if err = db.QueryRowContext(r.Context(), `INSERT INTO help_request_messages(help_request_id,author_id,text) VALUES($1,$2,$3) RETURNING id`, requestID, userID, payload.Text).Scan(&id); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось отправить сообщение")
			return
		}
		writeAdminJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func createHelpReview(w http.ResponseWriter, r *http.Request, requestID, authorID int64) {
	var payload struct {
		Rating int    `json:"rating"`
		Text   string `json:"text"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload) != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректный отзыв")
		return
	}
	payload.Text = strings.TrimSpace(payload.Text)
	if payload.Rating < 1 || payload.Rating > 5 || len([]rune(payload.Text)) > 4000 {
		writeJSON(w, http.StatusBadRequest, "Поставьте оценку от 1 до 5 и проверьте текст отзыва")
		return
	}
	var id int64
	err := db.QueryRowContext(r.Context(), `INSERT INTO help_reviews(author_id,recipient_id,help_request_id,rating,text)
		SELECT requester_id,expert_id,id,$3,$4
		FROM help_requests
		WHERE id=$1 AND requester_id=$2 AND status='completed'
		RETURNING id`, requestID, authorID, payload.Rating, payload.Text).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			writeJSON(w, http.StatusConflict, "Отзыв по этому обращению уже оставлен")
			return
		}
		writeJSON(w, http.StatusBadRequest, "Отзыв можно оставить только по завершенному обращению")
		return
	}
	writeAdminJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func helpNotifications(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	var incomingNew int
	if err = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM help_requests WHERE expert_id=$1 AND status='new'`, u.ID).Scan(&incomingNew); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить уведомления")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]int{"incoming_new": incomingNew})
}
