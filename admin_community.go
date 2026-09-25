package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func adminCommunityVacancies(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeAdminJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется вход администратора"})
		return
	}
	if r.Method != http.MethodGet {
		writeAdminJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}
	query := `SELECT jsonb_build_object(
		'id',v.id,'title',v.title,'city',v.city,'work_format',v.work_format,
		'employment_type',v.employment_type,'salary_from',v.salary_from,'salary_to',v.salary_to,
		'currency',v.currency,'owner_name',u.full_name,'owner_email',u.email,
		'published_at',v.published_at,'updated_at',v.updated_at
	) FROM vacancies v JOIN users u ON u.id=v.user_id
	WHERE v.status='published' AND v.deleted_at IS NULL`
	args := []any{}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		args = append(args, search)
		query += ` AND (v.title ILIKE '%'||$1||'%' OR v.city ILIKE '%'||$1||'%' OR u.full_name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%')`
	}
	query += ` ORDER BY v.published_at DESC NULLS LAST,v.updated_at DESC,v.id DESC`
	adminCommunityRows(w, r, query, args, "вакансии")
}

func adminCommunityProfiles(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeAdminJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется вход администратора"})
		return
	}
	if r.Method != http.MethodGet {
		writeAdminJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}
	query := `SELECT jsonb_build_object(
		'id',r.id,'name',u.full_name,'email',u.email,'avatar',COALESCE(u.avatar_url,''),
		'position',COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order,rc.id LIMIT 1),'Финансовый специалист'),
		'city',COALESCE(c.name,''),'desired_salary',r.desired_salary,'search_status',COALESCE(r.search_status_code,''),
		'published_at',r.published_at,'updated_at',r.updated_at
	) FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id
	WHERE r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL`
	args := []any{}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		args = append(args, search)
		query += ` AND (u.full_name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR c.name ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id WHERE rc.resume_id=r.id AND i.value ILIKE '%'||$1||'%'))`
	}
	query += ` ORDER BY r.published_at DESC NULLS LAST,r.updated_at DESC,r.id DESC`
	adminCommunityRows(w, r, query, args, "профили")
}

func adminCommunityRows(w http.ResponseWriter, r *http.Request, query string, args []any, label string) {
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить " + label})
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить " + label})
			return
		}
		items = append(items, raw)
	}
	if err = rows.Err(); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить " + label})
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"items": items})
}

func adminCommunityVacancyAction(w http.ResponseWriter, r *http.Request) {
	adminCommunityUnpublish(w, r, "/api/admin/community/vacancies/", "vacancies", "вакансия")
}

func adminCommunityProfileAction(w http.ResponseWriter, r *http.Request) {
	adminCommunityUnpublish(w, r, "/api/admin/community/profiles/", "resumes", "профиль")
}

func adminCommunityUnpublish(w http.ResponseWriter, r *http.Request, prefix, table, label string) {
	if !isAdmin(r) {
		writeAdminJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется вход администратора"})
		return
	}
	if r.Method != http.MethodPost {
		writeAdminJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
	if len(parts) != 2 || parts[1] != "unpublish" {
		writeAdminJSON(w, http.StatusNotFound, map[string]string{"error": "Действие не найдено"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		writeAdminJSON(w, http.StatusNotFound, map[string]string{"error": "Запись не найдена"})
		return
	}
	result, err := db.ExecContext(r.Context(), `UPDATE `+table+` SET status='draft',updated_at=NOW() WHERE id=$1 AND status='published' AND deleted_at IS NULL`, id)
	if err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось снять с публикации: " + label})
		return
	}
	updated, _ := result.RowsAffected()
	if updated == 0 {
		writeAdminJSON(w, http.StatusConflict, map[string]string{"error": "Запись уже снята с публикации или не найдена"})
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"id": id, "status": "draft"})
}
