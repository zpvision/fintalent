package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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
