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

type adminProfiMarketOneCConfiguration struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Logo      string `json:"logo"`
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

type adminProfiMarketSolution struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	ProductType string    `json:"product_type"`
	CoverImage  string    `json:"cover_image"`
	OwnerName   string    `json:"owner_name"`
	OwnerEmail  string    `json:"owner_email"`
	Status      string    `json:"status"`
	Purchases   int       `json:"purchases"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func adminProfiMarketSolutions(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT s.id,s.title,s.slug,s.type,COALESCE(s.cover_image,''),u.full_name,u.email,s.status,
		(SELECT COUNT(*) FROM profimarket_purchases p WHERE p.solution_id=s.id),s.updated_at
		FROM profimarket_solutions s JOIN users u ON u.id=s.author_user_id
		WHERE s.deleted_at IS NULL ORDER BY s.updated_at DESC,s.id DESC`)
	if err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить карточки ПрофиМаркета"})
		return
	}
	defer rows.Close()
	items := make([]adminProfiMarketSolution, 0)
	for rows.Next() {
		var item adminProfiMarketSolution
		if err = rows.Scan(&item.ID, &item.Title, &item.Slug, &item.ProductType, &item.CoverImage, &item.OwnerName, &item.OwnerEmail, &item.Status, &item.Purchases, &item.UpdatedAt); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить карточки ПрофиМаркета"})
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить карточки ПрофиМаркета"})
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{"items": items})
}

func adminProfiMarketSolutionAction(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/profimarket/solutions/"), "/"), "/")
	if len(parts) == 0 || len(parts) > 2 {
		writeAdminJSON(w, http.StatusNotFound, map[string]string{"error": "Действие не найдено"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректная карточка"})
		return
	}
	if len(parts) == 2 && parts[1] == "unpublish" && r.Method == http.MethodPost {
		result, execErr := db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET status='ARCHIVED',updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL AND status='PUBLISHED'`, id)
		if execErr != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось снять карточку с публикации"})
			return
		}
		if count, _ := result.RowsAffected(); count == 0 {
			writeAdminJSON(w, http.StatusConflict, map[string]string{"error": "Карточка уже снята с публикации или удалена"})
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]string{"message": "Карточка снята с публикации"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		result, execErr := db.ExecContext(r.Context(), `UPDATE profimarket_solutions SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
		if execErr != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось удалить карточку"})
			return
		}
		if count, _ := result.RowsAffected(); count == 0 {
			writeAdminJSON(w, http.StatusNotFound, map[string]string{"error": "Карточка не найдена"})
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]string{"message": "Карточка удалена"})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
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

func adminProfiMarketOneCConfigurations(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.QueryContext(r.Context(), `SELECT c.id,c.code,c.name,c.logo,c.sort_order,c.active,EXISTS(
			SELECT 1 FROM profimarket_solutions s WHERE s.type='ONEC_INTEGRATION' AND s.deleted_at IS NULL AND COALESCE(s.product_data->'configurations','[]'::jsonb) ? c.name
		) FROM profimarket_onec_configurations c ORDER BY c.sort_order,c.id`)
		if err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить конфигурации 1С"})
			return
		}
		defer rows.Close()
		items := make([]adminProfiMarketOneCConfiguration, 0)
		for rows.Next() {
			var item adminProfiMarketOneCConfiguration
			if err = rows.Scan(&item.ID, &item.Code, &item.Name, &item.Logo, &item.SortOrder, &item.Active, &item.Used); err != nil {
				writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось загрузить конфигурации 1С"})
				return
			}
			items = append(items, item)
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var item adminProfiMarketOneCConfiguration
		if json.NewDecoder(r.Body).Decode(&item) != nil || !validProfiMarketOneCConfiguration(&item) {
			writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Укажите название и корректный code"})
			return
		}
		if err := db.QueryRowContext(r.Context(), `INSERT INTO profimarket_onec_configurations(code,name,logo,sort_order,active) VALUES($1,$2,$3,$4,$5) RETURNING id`, item.Code, item.Name, item.Logo, item.SortOrder, item.Active).Scan(&item.ID); err != nil {
			writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Конфигурация с таким code или названием уже существует"})
			return
		}
		writeAdminJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func adminProfiMarketOneCConfigurationItem(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/profimarket/onec-configurations/"), "/"), 10, 64)
	if err != nil || id <= 0 {
		writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректная конфигурация 1С"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var item adminProfiMarketOneCConfiguration
		if json.NewDecoder(r.Body).Decode(&item) != nil || !validProfiMarketOneCConfiguration(&item) {
			writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Укажите название и корректный code"})
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось сохранить конфигурацию"})
			return
		}
		defer tx.Rollback()
		var oldName string
		if err = tx.QueryRowContext(r.Context(), `SELECT name FROM profimarket_onec_configurations WHERE id=$1 FOR UPDATE`, id).Scan(&oldName); err != nil {
			writeAdminJSON(w, http.StatusNotFound, map[string]string{"error": "Конфигурация не найдена"})
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE profimarket_onec_configurations SET code=$1,name=$2,logo=$3,sort_order=$4,active=$5,updated_at=NOW() WHERE id=$6`, item.Code, item.Name, item.Logo, item.SortOrder, item.Active, id); err != nil {
			writeAdminJSON(w, http.StatusBadRequest, map[string]string{"error": "Конфигурация с таким code или названием уже существует"})
			return
		}
		if oldName != item.Name {
			_, err = tx.ExecContext(r.Context(), `UPDATE profimarket_solutions s SET product_data=jsonb_set(s.product_data,'{configurations}',COALESCE((SELECT jsonb_agg(CASE WHEN value=$1 THEN to_jsonb($2::text) ELSE to_jsonb(value) END) FROM jsonb_array_elements_text(COALESCE(s.product_data->'configurations','[]'::jsonb)) value),'[]'::jsonb)),updated_at=NOW() WHERE s.type='ONEC_INTEGRATION' AND s.deleted_at IS NULL AND COALESCE(s.product_data->'configurations','[]'::jsonb) ? $1`, oldName, item.Name)
			if err != nil {
				writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось обновить заполненные карточки"})
				return
			}
		}
		if err = tx.Commit(); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось сохранить конфигурацию"})
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]string{"message": "Конфигурация сохранена"})
	case http.MethodDelete:
		var used bool
		if err = db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM profimarket_solutions s JOIN profimarket_onec_configurations c ON c.id=$1 WHERE s.type='ONEC_INTEGRATION' AND s.deleted_at IS NULL AND COALESCE(s.product_data->'configurations','[]'::jsonb) ? c.name)`, id).Scan(&used); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось удалить конфигурацию"})
			return
		}
		if used {
			_, err = db.ExecContext(r.Context(), `UPDATE profimarket_onec_configurations SET active=FALSE,updated_at=NOW() WHERE id=$1`, id)
		} else {
			_, err = db.ExecContext(r.Context(), `DELETE FROM profimarket_onec_configurations WHERE id=$1`, id)
		}
		if err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось удалить конфигурацию"})
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]string{"message": map[bool]string{true: "Конфигурация отключена", false: "Конфигурация удалена"}[used]})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func validProfiMarketOneCConfiguration(item *adminProfiMarketOneCConfiguration) bool {
	item.Code = strings.ToLower(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Logo = strings.TrimSpace(item.Logo)
	return item.Name != "" && profiMarketPlatformCodePattern.MatchString(item.Code) && len(item.Name) <= 160 && len(item.Code) <= 80 && len(item.Logo) <= 1000
}
