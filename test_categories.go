package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type testCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Active    bool   `json:"active"`
}

var initialTestCategories = []string{"Бухгалтерский учёт", "Налоговый учёт", "ТМЦ и зарплата", "Финансовый анализ", "МСФО", "Право и отчётность", "1С и программы", "Другое"}

func prepareTestCategories(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS test_categories(
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(200) NOT NULL UNIQUE,
		sort_order INTEGER NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE tests ADD COLUMN IF NOT EXISTS category_id BIGINT REFERENCES test_categories(id) ON DELETE SET NULL;
	CREATE INDEX IF NOT EXISTS tests_category_id_idx ON tests(category_id);
	CREATE INDEX IF NOT EXISTS test_categories_order_idx ON test_categories(active,sort_order,id);`)
	if err != nil {
		return err
	}
	var categoryCount int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_categories`).Scan(&categoryCount); err != nil {
		return err
	}
	if categoryCount == 0 {
		for order, name := range initialTestCategories {
			if _, err = db.ExecContext(ctx, `INSERT INTO test_categories(name,sort_order) VALUES($1,$2)`, name, order); err != nil {
				return err
			}
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE tests t SET category_id=c.id FROM test_categories c WHERE t.category_id IS NULL AND LOWER(TRIM(t.category))=LOWER(c.name)`)
	return err
}

func registerTestCategoryRoutes() {
	http.HandleFunc("/api/test-categories", publicTestCategories)
	http.HandleFunc("/api/admin/test-categories", adminTestCategories)
	http.HandleFunc("/api/admin/test-categories/", adminTestCategory)
}

func loadTestCategories(ctx context.Context, activeOnly bool) ([]testCategory, error) {
	query := `SELECT id,name,sort_order,active FROM test_categories`
	if activeOnly {
		query += ` WHERE active=TRUE`
	}
	query += ` ORDER BY sort_order,id`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []testCategory{}
	for rows.Next() {
		var item testCategory
		if err = rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.Active); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func publicTestCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	items, err := loadTestCategories(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить категории")
		return
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func adminTestCategories(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := loadTestCategories(r.Context(), false)
		if err != nil {
			writeJSON(w, 500, "Не удалось загрузить категории")
			return
		}
		writeAdminJSON(w, 200, items)
	case http.MethodPost:
		var input testCategory
		if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
			writeJSON(w, 400, "Укажите название категории")
			return
		}
		if err := db.QueryRowContext(r.Context(), `INSERT INTO test_categories(name,sort_order,active) VALUES($1,$2,$3) RETURNING id`, strings.TrimSpace(input.Name), input.SortOrder, input.Active).Scan(&input.ID); err != nil {
			writeJSON(w, 409, "Категория с таким названием уже существует")
			return
		}
		writeAdminJSON(w, 201, input)
	default:
		writeJSON(w, 405, "Метод не поддерживается")
	}
}

func adminTestCategory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/test-categories/"), "/"), 10, 64)
	if err != nil {
		writeJSON(w, 400, "Некорректная категория")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input testCategory
		if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" {
			writeJSON(w, 400, "Укажите название категории")
			return
		}
		result, execErr := db.ExecContext(r.Context(), `UPDATE test_categories SET name=$1,sort_order=$2,active=$3,updated_at=NOW() WHERE id=$4`, strings.TrimSpace(input.Name), input.SortOrder, input.Active, id)
		if execErr != nil {
			writeJSON(w, 409, "Не удалось сохранить категорию")
			return
		}
		if count, _ := result.RowsAffected(); count == 0 {
			writeJSON(w, 404, "Категория не найдена")
			return
		}
		_, _ = db.ExecContext(r.Context(), `UPDATE tests SET category=$1 WHERE category_id=$2`, strings.TrimSpace(input.Name), id)
		writeJSON(w, 200, "Категория сохранена")
	case http.MethodDelete:
		var used int
		_ = db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tests WHERE category_id=$1 AND deleted_at IS NULL`, id).Scan(&used)
		if used > 0 {
			writeJSON(w, 409, "Категория используется в тестах. Отключите её вместо удаления")
			return
		}
		if _, err = db.ExecContext(r.Context(), `DELETE FROM test_categories WHERE id=$1`, id); err != nil && err != sql.ErrNoRows {
			writeJSON(w, 500, "Не удалось удалить категорию")
			return
		}
		writeJSON(w, 200, "Категория удалена")
	default:
		writeJSON(w, 405, "Метод не поддерживается")
	}
}
