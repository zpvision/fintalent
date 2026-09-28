package accountingcompany

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) adminReviews(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		failure(w, 403, "Недостаточно прав")
		return
	}
	if r.Method != http.MethodGet {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT jsonb_build_object('id',r.id,'company_name',c.name,'author_name',r.author_name,'text',r.text,'rating',r.rating,'created_at',r.created_at) FROM accounting_company_reviews r JOIN accounting_companies c ON c.id=r.company_id WHERE r.status='pending' AND c.deleted_at IS NULL AND ($1='' OR c.name ILIKE '%'||$1||'%' OR r.author_name ILIKE '%'||$1||'%') ORDER BY r.created_at,r.id LIMIT 200`, strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		failure(w, 500, "Не удалось загрузить отзывы")
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			failure(w, 500, "Не удалось загрузить отзывы")
			return
		}
		items = append(items, raw)
	}
	if rows.Err() != nil {
		failure(w, 500, "Не удалось загрузить отзывы")
		return
	}
	response(w, 200, map[string]any{"items": items})
}

func (h *Handler) adminReviewAction(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		failure(w, 403, "Недостаточно прав")
		return
	}
	if r.Method != http.MethodPost {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/community/company-reviews/"), "/")
	if len(parts) != 2 {
		failure(w, 404, "Действие не найдено")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	status := map[string]string{"approve": "published", "reject": "rejected"}[parts[1]]
	if err != nil || id <= 0 || status == "" {
		failure(w, 400, "Некорректное действие")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `UPDATE accounting_company_reviews SET status=$2,updated_at=NOW() WHERE id=$1 AND status='pending'`, id, status)
	if err != nil {
		failure(w, 500, "Не удалось обработать отзыв")
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		failure(w, 409, "Отзыв уже обработан или не найден")
		return
	}
	response(w, 200, map[string]any{"id": id, "status": status})
}
