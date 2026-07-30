package main

import (
	"context"
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
	return err
}

func registerDemoContentRoutes() {
	http.HandleFunc("/api/public/home-showcase", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		type card struct {
			ID       int64   `json:"id"`
			Title    string  `json:"title"`
			Name     string  `json:"name"`
			City     string  `json:"city"`
			Salary   float64 `json:"salary"`
			Avatar   string  `json:"avatar,omitempty"`
			Subtitle string  `json:"subtitle,omitempty"`
		}
		result := struct {
			Vacancies []card `json:"vacancies"`
			Resumes   []card `json:"resumes"`
		}{Vacancies: []card{}, Resumes: []card{}}
		rows, _ := db.QueryContext(r.Context(), `SELECT v.id,v.title,u.full_name,v.city,COALESCE(v.salary_from,0) FROM vacancies v JOIN users u ON u.id=v.user_id WHERE v.status='published' AND v.deleted_at IS NULL ORDER BY random() LIMIT 4`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var c card
				_ = rows.Scan(&c.ID, &c.Title, &c.Name, &c.City, &c.Salary)
				result.Vacancies = append(result.Vacancies, c)
			}
		}
		rows, _ = db.QueryContext(r.Context(), `SELECT r.id,u.full_name,COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order LIMIT 1),'Финансовый специалист'),COALESCE(c.name,''),COALESCE(r.desired_salary,0),COALESCE(u.avatar_url,'') FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id WHERE r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL ORDER BY random() LIMIT 4`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var c card
				_ = rows.Scan(&c.ID, &c.Name, &c.Title, &c.City, &c.Salary, &c.Avatar)
				result.Resumes = append(result.Resumes, c)
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(result)
	})
}
