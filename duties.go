package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type dutyCategory struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	SortOrder   int       `json:"sort_order"`
	IsActive    bool      `json:"is_active"`
	DutyCount   int       `json:"duty_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Duties      []duty    `json:"duties,omitempty"`
}

type duty struct {
	ID           int64     `json:"id"`
	CategoryID   int64     `json:"category_id"`
	CategoryName string    `json:"category_name,omitempty"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SortOrder    int       `json:"sort_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func registerDutyRoutes() {
	http.HandleFunc("/api/v1/duty-categories", publicDutyCategories)
	http.HandleFunc("/api/v1/duties", publicDuties)
	http.HandleFunc("/api/v1/resumes/duties", resumeDuties)
	http.HandleFunc("/api/admin/duty-categories", adminDutyCategories)
	http.HandleFunc("/api/admin/duty-categories/", adminDutyCategory)
	http.HandleFunc("/api/admin/duties", adminDuties)
	http.HandleFunc("/api/admin/duties/bulk", adminDutiesBulk)
	http.HandleFunc("/api/admin/duties/", adminDuty)
}

func loadDutyCategories(r *http.Request, activeOnly, withDuties bool) ([]dutyCategory, error) {
	query := `SELECT c.id,c.name,c.description,c.icon,c.sort_order,c.is_active,c.created_at,c.updated_at,COUNT(d.id) FROM duty_categories c LEFT JOIN duties d ON d.category_id=c.id`
	if activeOnly {
		query += ` AND d.is_active=TRUE WHERE c.is_active=TRUE`
	}
	query += ` GROUP BY c.id ORDER BY c.sort_order,c.id`
	rows, err := db.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []dutyCategory{}
	for rows.Next() {
		var item dutyCategory
		if err = rows.Scan(&item.ID, &item.Name, &item.Description, &item.Icon, &item.SortOrder, &item.IsActive, &item.CreatedAt, &item.UpdatedAt, &item.DutyCount); err != nil {
			return nil, err
		}
		item.Duties = []duty{}
		items = append(items, item)
	}
	if !withDuties {
		return items, rows.Err()
	}
	duties, err := queryDuties(r, activeOnly)
	if err != nil {
		return nil, err
	}
	indexes := map[int64]int{}
	for index := range items {
		indexes[items[index].ID] = index
	}
	for _, item := range duties {
		if index, ok := indexes[item.CategoryID]; ok {
			items[index].Duties = append(items[index].Duties, item)
		}
	}
	return items, nil
}

func queryDuties(r *http.Request, activeOnly bool) ([]duty, error) {
	query := `SELECT d.id,d.category_id,c.name,d.name,d.description,d.sort_order,d.is_active,d.created_at,d.updated_at FROM duties d JOIN duty_categories c ON c.id=d.category_id WHERE 1=1`
	args := []any{}
	if activeOnly {
		query += ` AND d.is_active=TRUE AND c.is_active=TRUE`
	}
	if category := r.URL.Query().Get("category"); category != "" {
		if id, err := strconv.ParseInt(category, 10, 64); err == nil {
			args = append(args, id)
			query += ` AND d.category_id=$` + strconv.Itoa(len(args))
		}
	}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		args = append(args, "%"+search+"%")
		query += ` AND (d.name ILIKE $` + strconv.Itoa(len(args)) + ` OR d.description ILIKE $` + strconv.Itoa(len(args)) + `)`
	}
	query += ` ORDER BY c.sort_order,c.id,d.sort_order,d.id`
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []duty{}
	for rows.Next() {
		var item duty
		if err = rows.Scan(&item.ID, &item.CategoryID, &item.CategoryName, &item.Name, &item.Description, &item.SortOrder, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func publicDutyCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	items, err := loadDutyCategories(r, true, r.URL.Query().Get("include_duties") == "true")
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить категории обязанностей")
		return
	}
	writeAdminJSON(w, 200, items)
}

func publicDuties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	items, err := queryDuties(r, true)
	if err != nil {
		writeJSON(w, 500, "Не удалось загрузить обязанности")
		return
	}
	writeAdminJSON(w, 200, items)
}

func adminDutyCategories(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method == http.MethodGet {
		items, err := loadDutyCategories(r, false, false)
		if err != nil {
			writeJSON(w, 500, "Не удалось загрузить категории")
			return
		}
		writeAdminJSON(w, 200, items)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	var input dutyCategory
	if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
		writeJSON(w, 400, "Укажите название категории")
		return
	}
	err := db.QueryRowContext(r.Context(), `INSERT INTO duty_categories(name,description,icon,sort_order,is_active) VALUES($1,$2,$3,$4,$5) RETURNING id,created_at,updated_at`, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), strings.TrimSpace(input.Icon), input.SortOrder, input.IsActive).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		writeJSON(w, 409, "Категория с таким названием уже существует")
		return
	}
	writeAdminJSON(w, 201, input)
}

func adminDutyCategory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/duty-categories/"), "/"), 10, 64)
	if err != nil {
		writeJSON(w, 400, "Некорректная категория")
		return
	}
	if r.Method == http.MethodDelete {
		var count int
		_ = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM duties WHERE category_id=$1`, id).Scan(&count)
		if count > 0 {
			writeJSON(w, 409, "В категории есть обязанности. Сначала перенесите или удалите их")
			return
		}
		result, execErr := db.ExecContext(r.Context(), `DELETE FROM duty_categories WHERE id=$1`, id)
		if execErr != nil {
			writeJSON(w, 500, "Не удалось удалить категорию")
			return
		}
		if n, _ := result.RowsAffected(); n == 0 {
			writeJSON(w, 404, "Категория не найдена")
			return
		}
		writeJSON(w, 200, "Категория удалена")
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	var input dutyCategory
	if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
		writeJSON(w, 400, "Укажите название категории")
		return
	}
	result, err := db.ExecContext(r.Context(), `UPDATE duty_categories SET name=$1,description=$2,icon=$3,sort_order=$4,is_active=$5,updated_at=NOW() WHERE id=$6`, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), strings.TrimSpace(input.Icon), input.SortOrder, input.IsActive, id)
	if err != nil {
		writeJSON(w, 409, "Не удалось сохранить категорию")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeJSON(w, 404, "Категория не найдена")
		return
	}
	writeJSON(w, 200, "Категория сохранена")
}

func adminDuties(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method == http.MethodGet {
		items, err := queryDuties(r, false)
		if err != nil {
			writeJSON(w, 500, "Не удалось загрузить обязанности")
			return
		}
		writeAdminJSON(w, 200, items)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	var input duty
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		writeJSON(w, 400, "Укажите категорию и название обязанности")
		return
	}
	err := db.QueryRowContext(r.Context(), `INSERT INTO duties(category_id,name,description,sort_order,is_active) SELECT id,$2,$3,$4,$5 FROM duty_categories WHERE id=$1 RETURNING id,created_at,updated_at`, input.CategoryID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), input.SortOrder, input.IsActive).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		writeJSON(w, 409, "Не удалось создать обязанность")
		return
	}
	writeAdminJSON(w, 201, input)
}

func adminDuty(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/duties/"), "/"), 10, 64)
	if err != nil {
		writeJSON(w, 400, "Некорректная обязанность")
		return
	}
	if r.Method == http.MethodDelete {
		result, execErr := db.ExecContext(r.Context(), `DELETE FROM duties WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM vacancy_duties WHERE duty_id=$1) AND NOT EXISTS(SELECT 1 FROM resume_duties WHERE duty_id=$1)`, id)
		if execErr != nil {
			writeJSON(w, 500, "Не удалось удалить обязанность")
			return
		}
		if n, _ := result.RowsAffected(); n == 0 {
			writeJSON(w, 409, "Обязанность используется. Отключите её вместо удаления")
			return
		}
		writeJSON(w, 200, "Обязанность удалена")
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	var input duty
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.CategoryID <= 0 || strings.TrimSpace(input.Name) == "" {
		writeJSON(w, 400, "Укажите категорию и название обязанности")
		return
	}
	result, err := db.ExecContext(r.Context(), `UPDATE duties SET category_id=$1,name=$2,description=$3,sort_order=$4,is_active=$5,updated_at=NOW() WHERE id=$6`, input.CategoryID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), input.SortOrder, input.IsActive, id)
	if err != nil {
		writeJSON(w, 409, "Не удалось сохранить обязанность")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeJSON(w, 404, "Обязанность не найдена")
		return
	}
	writeJSON(w, 200, "Обязанность сохранена")
}

func adminDutiesBulk(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		IDs      []int64 `json:"ids"`
		IsActive bool    `json:"is_active"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.IDs) == 0 {
		writeJSON(w, 400, "Выберите обязанности")
		return
	}
	seen := map[int64]bool{}
	for _, id := range input.IDs {
		if id <= 0 || seen[id] {
			writeJSON(w, 400, "Некорректный список обязанностей")
			return
		}
		seen[id] = true
	}
	if _, err := db.ExecContext(r.Context(), `UPDATE duties SET is_active=$1,updated_at=NOW() WHERE id=ANY($2)`, input.IsActive, input.IDs); err != nil {
		// pgx stdlib does not encode []int64 directly in every version; use a safe per-id transaction fallback.
		tx, txErr := db.BeginTx(r.Context(), nil)
		if txErr != nil {
			writeJSON(w, 500, "Не удалось изменить обязанности")
			return
		}
		defer tx.Rollback()
		for _, id := range input.IDs {
			if _, txErr = tx.ExecContext(r.Context(), `UPDATE duties SET is_active=$1,updated_at=NOW() WHERE id=$2`, input.IsActive, id); txErr != nil {
				writeJSON(w, 500, "Не удалось изменить обязанности")
				return
			}
		}
		if txErr = tx.Commit(); txErr != nil {
			writeJSON(w, 500, "Не удалось изменить обязанности")
			return
		}
	}
	writeJSON(w, 200, "Обязанности обновлены")
}

func resumeDuties(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Требуется авторизация")
		return
	}
	if r.Method == http.MethodGet {
		rows, queryErr := db.QueryContext(r.Context(), `SELECT d.id,d.category_id,c.name,d.name,d.description,d.sort_order,d.is_active,d.created_at,d.updated_at FROM resumes x JOIN resume_duties rd ON rd.resume_id=x.id JOIN duties d ON d.id=rd.duty_id JOIN duty_categories c ON c.id=d.category_id WHERE x.user_id=$1 AND x.deleted_at IS NULL ORDER BY c.sort_order,d.sort_order,d.id`, u.ID)
		if queryErr != nil {
			writeJSON(w, 500, "Не удалось загрузить обязанности резюме")
			return
		}
		defer rows.Close()
		items := []duty{}
		for rows.Next() {
			var item duty
			if queryErr = rows.Scan(&item.ID, &item.CategoryID, &item.CategoryName, &item.Name, &item.Description, &item.SortOrder, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); queryErr != nil {
				writeJSON(w, 500, "Не удалось загрузить обязанности резюме")
				return
			}
			items = append(items, item)
		}
		writeAdminJSON(w, 200, items)
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	var input struct {
		DutyIDs []int64 `json:"duty_ids"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.DutyIDs) == 0 || duplicateIDs(input.DutyIDs) {
		writeJSON(w, 400, "Выберите хотя бы одну обязанность")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить обязанности")
		return
	}
	defer tx.Rollback()
	var resumeID int64
	if err = tx.QueryRowContext(r.Context(), `INSERT INTO resumes(user_id,current_step) VALUES($1,1) ON CONFLICT(user_id) DO UPDATE SET updated_at=NOW() RETURNING id`, u.ID).Scan(&resumeID); err != nil {
		writeJSON(w, 500, "Не удалось сохранить обязанности")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(704000000000000000::bigint + $1::bigint)`, resumeID); err != nil {
		writeJSON(w, 500, "Не удалось заблокировать сохранение обязанностей резюме")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM resume_duties WHERE resume_id=$1`, resumeID); err != nil {
		writeJSON(w, 500, "Не удалось сохранить обязанности")
		return
	}
	for _, id := range input.DutyIDs {
		result, execErr := tx.ExecContext(r.Context(), `INSERT INTO resume_duties(resume_id,duty_id) SELECT $1,d.id FROM duties d JOIN duty_categories c ON c.id=d.category_id WHERE d.id=$2 AND d.is_active=TRUE AND c.is_active=TRUE`, resumeID, id)
		if execErr != nil {
			writeJSON(w, 500, "Не удалось сохранить обязанности")
			return
		}
		if n, _ := result.RowsAffected(); n != 1 {
			writeJSON(w, 400, "Выбрана недоступная обязанность")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось сохранить обязанности")
		return
	}
	writeJSON(w, 200, "Обязанности резюме сохранены")
}

func duplicateIDs(ids []int64) bool {
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

var _ = sql.ErrNoRows
