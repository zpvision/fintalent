package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func publicationActionAPI(w http.ResponseWriter, r *http.Request) {
	if !requirePublicationSameOrigin(w, r) {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/publications/"), "/")
	parts := strings.Split(tail, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, 400, "Некорректная публикация")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			getPublicationJSON(w, r, id)
		case http.MethodPut:
			updatePublication(w, r, id)
		case http.MethodDelete:
			deletePublication(w, r, id)
		default:
			writeJSON(w, 405, "Метод не поддерживается")
		}
		return
	}
	action := parts[1]
	switch action {
	case "publish", "unpublish", "relevance":
		publicationStateAction(w, r, id, action)
	case "reaction":
		togglePublicationReaction(w, r, id)
	case "bookmark":
		togglePublicationBookmark(w, r, id)
	case "comments":
		publicationComments(w, r, id)
	case "recommendations":
		publicationRecommendations(w, r, id)
	case "versions":
		publicationVersions(w, r, id)
	case "analytics":
		publicationAnalytics(w, r, id)
	case "report":
		publicationReport(w, r, id)
	case "progress":
		publicationProgress(w, r, id)
	default:
		writeJSON(w, 404, "Действие не найдено")
	}
}

func updatePublication(w http.ResponseWriter, r *http.Request, id int64) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	in, ok := decodePublicationInput(w, r)
	if !ok {
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить публикацию")
		return
	}
	defer tx.Rollback()
	var oldSlug, status string
	var version int
	err = tx.QueryRowContext(r.Context(), `SELECT slug,status,COALESCE((SELECT MAX(version) FROM publication_versions WHERE publication_id=p.id),0) FROM publications p WHERE id=$1 AND author_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, u.ID).Scan(&oldSlug, &status, &version)
	if err != nil {
		writeJSON(w, 403, "Нельзя редактировать чужую публикацию")
		return
	}
	contentJSON, _ := json.Marshal(in.Content)
	summary, _ := json.Marshal(in.SummaryPoints)
	body := renderPublicationBlocks(in.Content)
	if status == "published" {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO publication_versions(publication_id,version,changed_by,title,excerpt,content_json,content_html,change_summary,change_reason,status) SELECT id,$2,$3,title,excerpt,content_json,content_html,$4,'updated','previous' FROM publications WHERE id=$1`, id, version+1, u.ID, in.ChangeSummary)
		if err != nil {
			writeJSON(w, 500, "Не удалось создать версию")
			return
		}
	}
	if oldSlug != in.Slug {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO publication_slug_history(publication_id,old_slug) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, oldSlug); err != nil {
			writeJSON(w, 409, "Такой URL уже использовался")
			return
		}
	}
	res, err := tx.ExecContext(r.Context(), `UPDATE publications SET category_id=NULLIF($1,0),title=$2,subtitle=$3,excerpt=$4,cover_image=$5,content_json=$6,content_html=$7,summary_points=$8,slug=$9,seo_title=$10,seo_description=$11,visibility=$12,difficulty=$13,language=$14,reading_time=$15,allow_comments=$16,updated_at=NOW() WHERE id=$17 AND author_id=$18`, in.CategoryID, in.Title, in.Subtitle, in.Excerpt, in.CoverImage, contentJSON, body, summary, in.Slug, in.SEOTitle, in.SEODescription, in.Visibility, in.Difficulty, in.Language, in.ReadingTime, in.AllowComments, id, u.ID)
	if err != nil {
		status, msg := dbErrorStatus(err)
		writeJSON(w, status, msg)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, 403, "Нельзя редактировать чужую публикацию")
		return
	}
	if err = savePublicationLinks(r.Context(), tx, id, in); err != nil {
		writeJSON(w, 500, "Не удалось сохранить настройки")
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось сохранить публикацию")
		return
	}
	writeAdminJSON(w, 200, map[string]any{"id": id, "slug": in.Slug, "saved_at": timeNowISO()})
}

func deletePublication(w http.ResponseWriter, r *http.Request, id int64) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	res, err := db.ExecContext(r.Context(), `UPDATE publications SET deleted_at=NOW(),status='archived',updated_at=NOW() WHERE id=$1 AND author_id=$2 AND deleted_at IS NULL`, id, u.ID)
	if err != nil {
		writeJSON(w, 500, "Не удалось удалить публикацию")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeJSON(w, 403, "Нельзя удалить чужую публикацию")
		return
	}
	writeJSON(w, 200, "Публикация удалена")
}

func publicationStateAction(w http.ResponseWriter, r *http.Request, id int64, action string) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	switch action {
	case "publish":
		res, err := db.ExecContext(r.Context(), `UPDATE publications SET status='published',moderation_status='published',visibility=CASE WHEN visibility='draft' THEN 'public' ELSE visibility END,published_at=COALESCE(published_at,NOW()),last_relevance_check_at=NOW(),next_relevance_check_at=NOW()+INTERVAL '6 months',updated_at=NOW() WHERE id=$1 AND author_id=$2 AND deleted_at IS NULL AND title<>'' AND jsonb_array_length(content_json)>0`, id, u.ID)
		if err != nil {
			writeJSON(w, 500, "Не удалось опубликовать материал")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 400, "Добавьте содержание перед публикацией")
			return
		}
		writeJSON(w, 200, "Публикация опубликована")
	case "unpublish":
		res, _ := db.ExecContext(r.Context(), `UPDATE publications SET status='draft',moderation_status='draft',visibility='draft',updated_at=NOW() WHERE id=$1 AND author_id=$2`, id, u.ID)
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 403, "Недостаточно прав")
			return
		}
		writeJSON(w, 200, "Публикация снята с публикации")
	case "relevance":
		res, _ := db.ExecContext(r.Context(), `UPDATE publications SET relevance_status='current',last_relevance_check_at=NOW(),next_relevance_check_at=NOW()+INTERVAL '6 months',updated_at=NOW() WHERE id=$1 AND author_id=$2`, id, u.ID)
		n, _ := res.RowsAffected()
		if n == 0 {
			writeJSON(w, 403, "Недостаточно прав")
			return
		}
		writeJSON(w, 200, "Актуальность подтверждена")
	}
}

func togglePublicationReaction(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	var in struct {
		Type string `json:"type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	allowed := map[string]bool{"useful": true, "used_at_work": true, "solved_problem": true, "helped_audit": true}
	if !allowed[in.Type] {
		writeJSON(w, 400, "Некорректная реакция")
		return
	}
	var active bool
	err = db.QueryRowContext(r.Context(), `WITH removed AS (DELETE FROM publication_reactions WHERE publication_id=$1 AND user_id=$2 AND reaction_type=$3 RETURNING 1),added AS (INSERT INTO publication_reactions(publication_id,user_id,reaction_type) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM removed) ON CONFLICT DO NOTHING RETURNING 1) SELECT EXISTS(SELECT 1 FROM added)`, id, u.ID, in.Type).Scan(&active)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить реакцию")
		return
	}
	writeAdminJSON(w, 200, map[string]any{"active": active})
}

func togglePublicationBookmark(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	var active bool
	err = db.QueryRowContext(r.Context(), `WITH removed AS (DELETE FROM publication_bookmarks WHERE publication_id=$1 AND user_id=$2 RETURNING 1),added AS (INSERT INTO publication_bookmarks(publication_id,user_id) SELECT $1,$2 WHERE NOT EXISTS(SELECT 1 FROM removed) ON CONFLICT DO NOTHING RETURNING 1) SELECT EXISTS(SELECT 1 FROM added)`, id, u.ID).Scan(&active)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить закладку")
		return
	}
	writeAdminJSON(w, 200, map[string]any{"active": active})
}

func publicationAuthorAPI(w http.ResponseWriter, r *http.Request) {
	if !requirePublicationSameOrigin(w, r) {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/publication-authors/"), "/"), "/")
	if len(parts) != 2 || parts[1] != "subscribe" || r.Method != http.MethodPost {
		writeJSON(w, 404, "Действие не найдено")
		return
	}
	authorID, _ := strconv.ParseInt(parts[0], 10, 64)
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	if authorID == u.ID {
		writeJSON(w, 400, "Нельзя подписаться на себя")
		return
	}
	var active bool
	err = db.QueryRowContext(r.Context(), `WITH removed AS (DELETE FROM author_subscriptions WHERE subscriber_id=$1 AND author_id=$2 RETURNING 1),added AS (INSERT INTO author_subscriptions(subscriber_id,author_id) SELECT $1,$2 WHERE NOT EXISTS(SELECT 1 FROM removed) ON CONFLICT DO NOTHING RETURNING 1) SELECT EXISTS(SELECT 1 FROM added)`, u.ID, authorID).Scan(&active)
	if err != nil {
		writeJSON(w, 500, "Не удалось изменить подписку")
		return
	}
	if active {
		_, _ = db.ExecContext(r.Context(), `INSERT INTO notifications(user_id,type,title,body,entity_type,entity_id) SELECT $1,'new_follower','Новый подписчик',$2,'user',$3`, authorID, u.FullName+" подписался на ваши публикации", u.ID)
	}
	writeAdminJSON(w, 200, map[string]any{"active": active})
}

func publicationSeriesAPI(w http.ResponseWriter, r *http.Request) {
	if !requirePublicationSameOrigin(w, r) {
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT s.id,s.title,s.slug,s.description,s.status,COUNT(i.publication_id) FROM publication_series s LEFT JOIN publication_series_items i ON i.series_id=s.id WHERE s.author_id=$1 GROUP BY s.id ORDER BY s.updated_at DESC`, u.ID)
		if err != nil {
			writeJSON(w, 500, "Не удалось загрузить серии")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, count int64
			var title, slug, description, status string
			_ = rows.Scan(&id, &title, &slug, &description, &status, &count)
			items = append(items, map[string]any{"id": id, "title": title, "slug": slug, "description": description, "status": status, "count": count})
		}
		writeAdminJSON(w, 200, map[string]any{"items": items})
		return
	}
	if r.Method == http.MethodPost {
		var in struct{ Title, Description string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.Title = strings.TrimSpace(in.Title)
		if len([]rune(in.Title)) < 3 {
			writeJSON(w, 400, "Укажите название серии")
			return
		}
		slug := slugify(in.Title)
		var id int64
		err = db.QueryRowContext(r.Context(), `INSERT INTO publication_series(author_id,title,slug,description) VALUES($1,$2,$3,$4) RETURNING id`, u.ID, in.Title, slug, strings.TrimSpace(in.Description)).Scan(&id)
		if err != nil {
			writeJSON(w, 409, "Серия с таким названием уже существует")
			return
		}
		writeAdminJSON(w, 201, map[string]any{"id": id, "slug": slug})
		return
	}
	writeJSON(w, 405, "Метод не поддерживается")
}

func publicationUpload(w http.ResponseWriter, r *http.Request) {
	if !requirePublicationSameOrigin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	if _, err := userFromRequest(r); err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeJSON(w, 400, "Изображение слишком большое")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, 400, "Выберите изображение")
		return
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	mime := http.DetectContentType(head[:n])
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}[mime]
	if ext == "" {
		writeJSON(w, 400, "Поддерживаются JPG, PNG, WebP и GIF")
		return
	}
	_ = header
	token := make([]byte, 16)
	_, _ = rand.Read(token)
	dir := filepath.Join("static", "uploads", "publications")
	if err = os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	name := hex.EncodeToString(token) + ext
	out, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	defer out.Close()
	if _, err = out.Write(head[:n]); err == nil {
		_, err = io.Copy(out, io.LimitReader(file, 5<<20))
	}
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить файл")
		return
	}
	writeAdminJSON(w, 201, map[string]string{"url": "/static/uploads/publications/" + name})
}

func timeNowISO() string { return time.Now().UTC().Format(time.RFC3339) }
