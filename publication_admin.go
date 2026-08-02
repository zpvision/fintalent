package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func adminPublicationsAPI(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.title,p.slug,u.full_name,p.status,p.visibility,p.moderation_status,p.relevance_status,p.is_recommended,p.editor_mark,p.updated_at,(SELECT COUNT(*) FROM publication_reports x WHERE x.publication_id=p.id AND x.status='new') FROM publications p JOIN users u ON u.id=p.author_id WHERE p.deleted_at IS NULL AND ($1='' OR p.title ILIKE '%'||$1||'%' OR u.full_name ILIKE '%'||$1||'%') AND ($2='' OR p.status=$2) ORDER BY p.updated_at DESC LIMIT 100`, q, status)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить публикации")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, reports int64
		var title, slug, author, state, visibility, moderation, relevance string
		var recommended, mark bool
		var updated any
		_ = rows.Scan(&id, &title, &slug, &author, &state, &visibility, &moderation, &relevance, &recommended, &mark, &updated, &reports)
		items = append(items, map[string]any{"id": id, "title": title, "slug": slug, "author": author, "status": state, "visibility": visibility, "moderation_status": moderation, "relevance_status": relevance, "is_recommended": recommended, "editor_mark": mark, "updated_at": updated, "reports": reports})
	}
	writeAdminJSON(w, 200, map[string]any{"items": items})
}

func adminPublicationActionAPI(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/publications/"), "/"), "/")
	if len(parts) != 2 {
		writeJSON(w, 404, "Действие не найдено")
		return
	}
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	var in struct {
		Status, Reason, Relevance string
		Recommended, EditorMark   *bool
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	action := parts[1]
	var err error
	switch action {
	case "moderate":
		allowed := map[string]bool{"published": true, "rejected": true, "hidden": true, "blocked": true, "archived": true, "draft": true}
		if !allowed[in.Status] || (in.Status == "rejected" || in.Status == "blocked") && strings.TrimSpace(in.Reason) == "" {
			writeJSON(w, 400, "Укажите корректный статус и причину")
			return
		}
		_, err = db.ExecContext(r.Context(), `UPDATE publications SET status=$1,moderation_status=$1,moderation_reason=$2,visibility=CASE WHEN $1='published' THEN 'public' WHEN $1 IN('hidden','blocked','archived') THEN 'draft' ELSE visibility END,published_at=CASE WHEN $1='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$3`, in.Status, strings.TrimSpace(in.Reason), id)
	case "flags":
		_, err = db.ExecContext(r.Context(), `UPDATE publications SET is_recommended=COALESCE($1,is_recommended),editor_mark=COALESCE($2,editor_mark),updated_at=NOW() WHERE id=$3`, in.Recommended, in.EditorMark, id)
	case "relevance":
		_, err = db.ExecContext(r.Context(), `UPDATE publications SET relevance_status=$1,updated_at=NOW() WHERE id=$2`, in.Relevance, id)
	default:
		writeJSON(w, 404, "Действие не найдено")
		return
	}
	if err != nil {
		writeJSON(w, 500, "Не удалось изменить публикацию")
		return
	}
	_, _ = db.ExecContext(r.Context(), `INSERT INTO publication_moderation_audit(publication_id,admin_label,action,reason) VALUES($1,'admin',$2,$3)`, id, action, in.Reason)
	writeJSON(w, 200, "Публикация обновлена")
}
