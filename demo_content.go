package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"net/http"
	"strconv"
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
			ID          int64    `json:"id"`
			Title       string   `json:"title"`
			Name        string   `json:"name"`
			City        string   `json:"city"`
			Salary      float64  `json:"salary"`
			Avatar      string   `json:"avatar,omitempty"`
			Subtitle    string   `json:"subtitle,omitempty"`
			Tags        []string `json:"tags,omitempty"`
			ProfileMode string   `json:"profile_mode,omitempty"`
		}
		result := struct {
			Vacancies []card `json:"vacancies"`
			Resumes   []card `json:"resumes"`
		}{Vacancies: []card{}, Resumes: []card{}}
		rows, _ := db.QueryContext(r.Context(), `SELECT v.id,v.title,u.full_name,v.city,COALESCE(v.salary_from,0) FROM vacancies v JOIN users u ON u.id=v.user_id WHERE v.status='published' AND v.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) ORDER BY random() LIMIT 4`)
		if rows != nil {
			for rows.Next() {
				var c card
				_ = rows.Scan(&c.ID, &c.Title, &c.Name, &c.City, &c.Salary)
				result.Vacancies = append(result.Vacancies, c)
			}
			_ = rows.Close()
		}
		rows, _ = db.QueryContext(r.Context(), `SELECT r.id,u.full_name,COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order LIMIT 1),'Финансовый специалист'),COALESCE(c.name,''),COALESCE(r.desired_salary,0),COALESCE(u.avatar_url,''),COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='accounting_areas' ORDER BY rc.sort_order LIMIT 4) areas),''),COALESCE(u.profile_mode,'job_search') FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id WHERE r.status='published' AND r.visibility='public' AND r.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) ORDER BY random() LIMIT 4`)
		if rows != nil {
			for rows.Next() {
				var c card
				var tags string
				_ = rows.Scan(&c.ID, &c.Name, &c.Title, &c.City, &c.Salary, &c.Avatar, &tags, &c.ProfileMode)
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
	limit := 30
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && value > 0 && value <= 60 {
		limit = value
	}
	offset := 0
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("offset")); parseErr == nil && value > 0 {
		offset = value
	}
	helpTopicID := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("help_topic")); raw != "" {
		var parseErr error
		helpTopicID, parseErr = strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || helpTopicID <= 0 {
			writeJSON(w, http.StatusBadRequest, "Некорректное направление консультации")
			return
		}
	}
	parseFilterID := func(name, label string) (int64, bool) {
		raw := strings.TrimSpace(r.URL.Query().Get(name))
		if raw == "" {
			return 0, true
		}
		value, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || value <= 0 {
			writeJSON(w, http.StatusBadRequest, "Некорректный фильтр: "+label)
			return 0, false
		}
		return value, true
	}
	positionID, ok := parseFilterID("position", "специализация")
	if !ok {
		return
	}
	workFormatID, ok := parseFilterID("work_format", "формат работы")
	if !ok {
		return
	}
	accountingAreaID, ok := parseFilterID("accounting_area", "участок")
	if !ok {
		return
	}
	experience := strings.TrimSpace(r.URL.Query().Get("experience"))
	if experience != "" && experience != "none" && experience != "under_1" && experience != "1_3" && experience != "3_5" && experience != "5_10" && experience != "10_plus" {
		writeJSON(w, http.StatusBadRequest, "Некорректный фильтр: опыт работы")
		return
	}
	profileMode := strings.TrimSpace(r.URL.Query().Get("profile_mode"))
	if profileMode != "" && profileMode != "job_search" && profileMode != "professional" {
		writeJSON(w, http.StatusBadRequest, "Некорректный фильтр: статус")
		return
	}
	salaryFrom := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("salary_from")); raw != "" {
		value, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || value < 0 || value > 100000000 {
			writeJSON(w, http.StatusBadRequest, "Некорректная сумма зарплаты")
			return
		}
		salaryFrom = value
	}
	type item struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		Name        string   `json:"name"`
		City        string   `json:"city"`
		Salary      float64  `json:"salary"`
		Avatar      string   `json:"avatar,omitempty"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		ProfileMode string   `json:"profile_mode,omitempty"`
		Experience  int      `json:"experience_months,omitempty"`
		TestsCount  int      `json:"tests_count,omitempty"`
	}
	items := []item{}
	total := 0
	var rows *sql.Rows
	var err error
	if kind == "resumes" {
		rows, err = db.QueryContext(r.Context(), `SELECT r.id,u.full_name,COALESCE((SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias='position' ORDER BY rc.sort_order LIMIT 1),'Финансовый специалист'),COALESCE(c.name,''),COALESCE(r.desired_salary,0),COALESCE(u.avatar_url,''),COALESCE(r.work_preferences,''),COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND d.alias IN ('accounting_areas','software','crm') ORDER BY rc.sort_order LIMIT 6) x),''),COALESCE(u.profile_mode,'job_search'),COALESCE(exp.months,0),COALESCE((SELECT COUNT(DISTINCT ta.test_id) FROM test_attempts ta WHERE ta.user_id=u.id AND ta.status='finished' AND ta.show_in_resume=TRUE),0),COUNT(*) OVER() FROM resumes r JOIN users u ON u.id=r.user_id LEFT JOIN cities c ON c.id=r.preferred_city_id LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,(COALESCE(e.end_year,EXTRACT(YEAR FROM CURRENT_DATE)::int)*12+COALESCE(e.end_month,EXTRACT(MONTH FROM CURRENT_DATE)::int))-(e.start_year*12+e.start_month)+1))::int months FROM resume_work_experiences e WHERE e.resume_id=r.id) exp ON TRUE WHERE r.status='published' AND r.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) AND (r.visibility='public' OR $3::bigint>0) AND ($1='' OR u.full_name ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id WHERE rc.resume_id=r.id AND i.value ILIKE '%'||$1||'%')) AND ($2='' OR c.name ILIKE '%'||$2||'%') AND ($3::bigint=0 OR EXISTS(SELECT 1 FROM resume_help_topics rht JOIN help_topics ht ON ht.id=rht.topic_id WHERE rht.resume_id=r.id AND rht.topic_id=$3 AND ht.is_active=TRUE AND ht.deleted_at IS NULL)) AND ($4::bigint=0 OR EXISTS(SELECT 1 FROM resume_categories rc JOIN dictionary_items i ON i.id=rc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE rc.resume_id=r.id AND rc.category_id=$4 AND d.alias='accounting_areas')) AND ($5='' OR COALESCE(u.profile_mode,'job_search')=$5) AND ($6='' OR ($6='none' AND COALESCE(exp.months,0)=0) OR ($6='under_1' AND COALESCE(exp.months,0) BETWEEN 1 AND 11) OR ($6='1_3' AND COALESCE(exp.months,0) BETWEEN 12 AND 35) OR ($6='3_5' AND COALESCE(exp.months,0) BETWEEN 36 AND 59) OR ($6='5_10' AND COALESCE(exp.months,0) BETWEEN 60 AND 119) OR ($6='10_plus' AND COALESCE(exp.months,0)>=120)) ORDER BY r.published_at DESC NULLS LAST,r.id DESC LIMIT $7 OFFSET $8`, query, city, helpTopicID, accountingAreaID, profileMode, experience, limit, offset)
	} else {
		rows, err = db.QueryContext(r.Context(), `SELECT v.id,v.title,u.full_name,v.city,COALESCE(v.salary_from,0),v.description,COALESCE((SELECT string_agg(value,'|||') FROM (SELECT i.value FROM vacancy_categories vc JOIN dictionary_items i ON i.id=vc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE vc.vacancy_id=v.id AND d.alias IN ('accounting_areas','software','crm') ORDER BY vc.sort_order LIMIT 6) x),''),COUNT(*) OVER() FROM vacancies v JOIN users u ON u.id=v.user_id WHERE v.status='published' AND v.deleted_at IS NULL AND (NOT u.is_blocked OR u.is_system) AND ($1='' OR v.title ILIKE '%'||$1||'%' OR v.description ILIKE '%'||$1||'%') AND ($2='' OR v.city ILIKE '%'||$2||'%') AND ($3::bigint=0 OR EXISTS(SELECT 1 FROM vacancy_categories vc JOIN dictionary_items i ON i.id=vc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE vc.vacancy_id=v.id AND vc.category_id=$3 AND d.alias='position')) AND ($4::bigint=0 OR EXISTS(SELECT 1 FROM vacancy_categories vc JOIN dictionary_items i ON i.id=vc.category_id JOIN dictionaries d ON d.id=i.dictionary_id WHERE vc.vacancy_id=v.id AND vc.category_id=$4 AND d.alias='work_format')) AND ($5::bigint=0 OR GREATEST(COALESCE(v.salary_from,0),COALESCE(v.salary_to,0)) >= $5) ORDER BY v.published_at DESC NULLS LAST,v.id DESC LIMIT $6 OFFSET $7`, query, city, positionID, workFormatID, salaryFrom, limit, offset)
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
			err = rows.Scan(&x.ID, &x.Name, &x.Title, &x.City, &x.Salary, &x.Avatar, &x.Description, &tags, &x.ProfileMode, &x.Experience, &x.TestsCount, &total)
		} else {
			err = rows.Scan(&x.ID, &x.Title, &x.Name, &x.City, &x.Salary, &x.Description, &tags, &total)
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
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total, "limit": limit, "offset": offset, "has_more": offset+len(items) < total})
}
