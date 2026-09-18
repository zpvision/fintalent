package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type adminProfiMarketPlatformValue struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Active    bool   `json:"active"`
	Used      bool   `json:"used"`
}

var profiMarketPlatformCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type adminProfiMarketPurchase struct {
	ID              int64     `json:"id"`
	SolutionID      int64     `json:"solution_id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	ProductType     string    `json:"product_type"`
	CoverImage      string    `json:"cover_image"`
	BuyerName       string    `json:"buyer_name"`
	BuyerEmail      string    `json:"buyer_email"`
	SellerName      string    `json:"seller_name"`
	SellerEmail     string    `json:"seller_email"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	PricingType     string    `json:"pricing_type"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	SolutionVisible bool      `json:"solution_visible"`
}

func adminProfiMarketPurchases(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	productType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if status != "" && status != "PENDING" && status != "COMPLETED" && status != "CANCELLED" && status != "REFUNDED" {
		writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректный статус покупки"})
		return
	}
	validTypes := map[string]bool{"": true, "AI_ASSISTANT": true, "REGULATION": true, "AUTOMATION": true, "INSTRUCTION": true, "ONEC_INTEGRATION": true, "TEMPLATE": true, "CHECKLIST": true}
	if !validTypes[productType] {
		writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректное направление"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const limit = 50

	where := `($1='' OR buyer.full_name ILIKE '%'||$1||'%' OR buyer.email ILIKE '%'||$1||'%' OR seller.full_name ILIKE '%'||$1||'%' OR seller.email ILIKE '%'||$1||'%')
		AND ($2='' OR p.status=$2)
		AND ($3='' OR COALESCE(NULLIF(p.product_type_snapshot,''),s.type,'')=$3)`
	var total int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM profimarket_purchases p
		JOIN users buyer ON buyer.id=p.buyer_user_id
		JOIN users seller ON seller.id=p.seller_user_id
		LEFT JOIN profimarket_solutions s ON s.id=p.solution_id
		WHERE `+where, search, status, productType).Scan(&total); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить покупки ПрофиМаркета"})
		return
	}

	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.solution_id,
		COALESCE(NULLIF(p.product_title_snapshot,''),s.title,'Удалённое решение'),
		COALESCE(NULLIF(p.product_slug_snapshot,''),s.slug,''),
		COALESCE(NULLIF(p.product_type_snapshot,''),s.type,''),
		COALESCE(NULLIF(p.product_cover_snapshot,''),s.cover_image,''),
		buyer.full_name,buyer.email,seller.full_name,seller.email,
		p.amount,p.currency,p.pricing_type,p.status,p.created_at,
		(s.id IS NOT NULL AND s.deleted_at IS NULL)
		FROM profimarket_purchases p
		JOIN users buyer ON buyer.id=p.buyer_user_id
		JOIN users seller ON seller.id=p.seller_user_id
		LEFT JOIN profimarket_solutions s ON s.id=p.solution_id
		WHERE `+where+`
		ORDER BY p.created_at DESC,p.id DESC LIMIT $4 OFFSET $5`, search, status, productType, limit, (page-1)*limit)
	if err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить покупки ПрофиМаркета"})
		return
	}
	defer rows.Close()
	items := make([]adminProfiMarketPurchase, 0)
	for rows.Next() {
		var item adminProfiMarketPurchase
		if err = rows.Scan(&item.ID, &item.SolutionID, &item.Title, &item.Slug, &item.ProductType, &item.CoverImage, &item.BuyerName, &item.BuyerEmail, &item.SellerName, &item.SellerEmail, &item.Amount, &item.Currency, &item.PricingType, &item.Status, &item.CreatedAt, &item.SolutionVisible); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить покупки ПрофиМаркета"})
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить покупки ПрофиМаркета"})
		return
	}

	statusRows, err := db.QueryContext(r.Context(), `SELECT status,COUNT(*) FROM profimarket_purchases GROUP BY status`)
	if err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить статистику покупок"})
		return
	}
	defer statusRows.Close()
	statusCounts := map[string]int{"PENDING": 0, "COMPLETED": 0, "CANCELLED": 0, "REFUNDED": 0}
	for statusRows.Next() {
		var key string
		var count int
		if err = statusRows.Scan(&key, &count); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить статистику покупок"})
			return
		}
		statusCounts[key] = count
	}
	if err = statusRows.Err(); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить статистику покупок"})
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "limit": limit, "status_counts": statusCounts})
}

func adminProfiMarketPlatforms(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.code,p.name,p.icon,p.sort_order,p.active,EXISTS(SELECT 1 FROM profimarket_solution_platforms x WHERE x.platform_id=p.id) FROM profimarket_platforms p ORDER BY p.sort_order,p.id`)
		if err != nil {
			writeAdminJSON(w, 500, map[string]string{"error": "Не удалось загрузить платформы"})
			return
		}
		defer rows.Close()
		items := []adminProfiMarketPlatformValue{}
		for rows.Next() {
			var item adminProfiMarketPlatformValue
			if err = rows.Scan(&item.ID, &item.Code, &item.Name, &item.Icon, &item.SortOrder, &item.Active, &item.Used); err != nil {
				writeAdminJSON(w, 500, map[string]string{"error": "Не удалось загрузить платформы"})
				return
			}
			items = append(items, item)
		}
		writeAdminJSON(w, 200, map[string]any{"items": items})
	case http.MethodPost:
		var item adminProfiMarketPlatformValue
		if json.NewDecoder(r.Body).Decode(&item) != nil || !validProfiMarketPlatform(&item) {
			writeAdminJSON(w, 400, map[string]string{"error": "Укажите название и корректный code"})
			return
		}
		err := db.QueryRowContext(r.Context(), `INSERT INTO profimarket_platforms(code,name,icon,sort_order,active) VALUES($1,$2,$3,$4,$5) RETURNING id`, item.Code, item.Name, item.Icon, item.SortOrder, item.Active).Scan(&item.ID)
		if err != nil {
			writeAdminJSON(w, 400, map[string]string{"error": "Платформа с таким code уже существует"})
			return
		}
		writeAdminJSON(w, 201, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func adminProfiMarketPlatform(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/profimarket/platforms/"), "/"), 10, 64)
	if err != nil || id <= 0 {
		writeAdminJSON(w, 400, map[string]string{"error": "Некорректная платформа"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var item adminProfiMarketPlatformValue
		if json.NewDecoder(r.Body).Decode(&item) != nil || !validProfiMarketPlatform(&item) {
			writeAdminJSON(w, 400, map[string]string{"error": "Укажите название и корректный code"})
			return
		}
		result, err := db.ExecContext(r.Context(), `UPDATE profimarket_platforms SET code=$1,name=$2,icon=$3,sort_order=$4,active=$5 WHERE id=$6`, item.Code, item.Name, item.Icon, item.SortOrder, item.Active, id)
		if err != nil {
			writeAdminJSON(w, 400, map[string]string{"error": "Платформа с таким code уже существует"})
			return
		}
		if count, _ := result.RowsAffected(); count == 0 {
			writeAdminJSON(w, 404, map[string]string{"error": "Платформа не найдена"})
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Платформа сохранена"})
	case http.MethodDelete:
		var used bool
		err = db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM profimarket_solution_platforms WHERE platform_id=$1)`, id).Scan(&used)
		if err != nil && err != sql.ErrNoRows {
			writeAdminJSON(w, 500, map[string]string{"error": "Не удалось удалить платформу"})
			return
		}
		if used {
			_, err = db.ExecContext(r.Context(), `UPDATE profimarket_platforms SET active=FALSE WHERE id=$1`, id)
		} else {
			_, err = db.ExecContext(r.Context(), `DELETE FROM profimarket_platforms WHERE id=$1`, id)
		}
		if err != nil {
			writeAdminJSON(w, 500, map[string]string{"error": "Не удалось удалить платформу"})
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": map[bool]string{true: "Платформа отключена", false: "Платформа удалена"}[used]})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func validProfiMarketPlatform(item *adminProfiMarketPlatformValue) bool {
	item.Code = strings.ToLower(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Icon = strings.TrimSpace(item.Icon)
	return item.Name != "" && profiMarketPlatformCodePattern.MatchString(item.Code) && len(item.Name) <= 160 && len(item.Code) <= 80 && len(item.Icon) <= 1000
}
