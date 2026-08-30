package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/022_demo_content.sql
var demoContentFS embed.FS

func prepareDemoContent(ctx context.Context) error {
	schema, err := demoContentFS.ReadFile("migrations/022_demo_content.sql")
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("11111111"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, strings.ReplaceAll(string(schema), "__DEMO_PASSWORD_HASH__", string(hash)))
	if err != nil {
		return err
	}
	// The knowledge seed depends on demo users and resumes, which are created
	// above. It is idempotent and also keeps existing real attempts untouched.
	_, err = db.ExecContext(ctx, resumeTestKnowledgeMigrationSQL)
	return err
}

func registerDemoContentRoutes() {
	http.HandleFunc("/api/public/catalog", publicCatalogHandler)
	http.HandleFunc("/api/public/home-showcase", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		type card struct {
			ID       int64    `json:"id"`
			Title    string   `json:"title"`
			Name     string   `json:"name"`
			City     string   `json:"city"`
			Salary   float64  `json:"salary"`
			Avatar   string   `json:"avatar,omitempty"`
			Subtitle string   `json:"subtitle,omitempty"`
			Tags     []string `json:"tags,omitempty"`
		}
		result := struct {
			Vacancies []card `json:"vacancies"`
			Resumes   []card `json:"resumes"`
		}{Vacancies: []card{}, Resumes: []card{}}
		rows, _ := db.QueryContext(r.Context(), `SELECT v.id,v.title,u.full_name,v.city,COALESCE(v.salary_from,0) FROM vacancies v JOIN users u ON u.id=v.user_id WHERE v.status='published' AND v.deleted_at IS NULL ORDER BY random() LIMIT 4`)
		if rows != nil {
			for rows.Next() {
				var c card
				_ = rows.Scan(&c.ID, &c.Title, &c.Name, &c.City, &c.Salary)
				result.Vacancies = append(result.Vacancies, c)
			}
			_ = rows.Close()
		}
		rows, _ = db.QueryContext(r.Context(), `SELECT r.id,u.full_name,COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order LIMIT 1),'Финансовый специалист'),COALESCE(c.name,''),COALESCE(r.desired_salary,0),COALESCE(u.avatar_url,''),COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='accounting_areas' ORDER BY rc.sort_order LIMIT 4) areas),'') FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id WHERE r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL ORDER BY random() LIMIT 4`)
		if rows != nil {
			for rows.Next() {
				var c card
				var tags string
				_ = rows.Scan(&c.ID, &c.Name, &c.Title, &c.City, &c.Salary, &c.Avatar, &tags)
				if tags != "" {
					c.Tags = strings.Split(tags, "|||")
				}
				result.Resumes = append(result.Resumes, c)
			}
			_ = rows.Close()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(result)
	})
}

func publicCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	kind, query, city := r.URL.Query().Get("kind"), strings.TrimSpace(r.URL.Query().Get("q")), strings.TrimSpace(r.URL.Query().Get("city"))
	type item struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		Name        string   `json:"name"`
		City        string   `json:"city"`
		Salary      float64  `json:"salary"`
		Avatar      string   `json:"avatar,omitempty"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	items := []item{}
	var rows *sql.Rows
	var err error
	if kind == "resumes" {
		rows, err = db.QueryContext(r.Context(), `SELECT r.id,u.full_name,COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order LIMIT 1),'Финансовый специалист'),COALESCE(c.name,''),COALESCE(r.desired_salary,0),COALESCE(u.avatar_url,''),COALESCE(r.work_preferences,''),COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias IN ('accounting_areas','software','crm') ORDER BY rc.sort_order LIMIT 6) x),'') FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id WHERE r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL AND ($1='' OR u.full_name ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id WHERE rc.resume_id=r.id AND i.value ILIKE '%'||$1||'%')) AND ($2='' OR c.name ILIKE '%'||$2||'%') ORDER BY r.published_at DESC NULLS LAST LIMIT 60`, query, city)
	} else {
		rows, err = db.QueryContext(r.Context(), `SELECT v.id,v.title,u.full_name,v.city,COALESCE(v.salary_from,0),v.description,COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM vacancy_categories vc JOIN dictionary_items i ON i.id=vc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE vc.vacancy_id=v.id AND d.alias IN ('accounting_areas','software','crm') ORDER BY vc.sort_order LIMIT 6) x),'') FROM vacancies v JOIN users u ON u.id=v.user_id WHERE v.status='published' AND v.deleted_at IS NULL AND ($1='' OR v.title ILIKE '%'||$1||'%' OR v.description ILIKE '%'||$1||'%') AND ($2='' OR v.city ILIKE '%'||$2||'%') ORDER BY v.published_at DESC NULLS LAST LIMIT 60`, query, city)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить каталог")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var x item
		var tags string
		if kind == "resumes" {
			err = rows.Scan(&x.ID, &x.Name, &x.Title, &x.City, &x.Salary, &x.Avatar, &x.Description, &tags)
		} else {
			err = rows.Scan(&x.ID, &x.Title, &x.Name, &x.City, &x.Salary, &x.Description, &tags)
		}
		if err != nil {
			continue
		}
		if tags != "" {
			x.Tags = strings.Split(tags, "|||")
		}
		items = append(items, x)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": len(items)})
}
