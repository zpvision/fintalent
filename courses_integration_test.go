package main

import (
	"FinTalent/internal/courses"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in. Fixtures live only in a new, verified schema on DATABASE_URL.
func TestCoursesIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_COURSES_DB_TESTS") != "1" {
		t.Skip("set RUN_COURSES_DB_TESTS=1 and cloud DATABASE_URL")
	}
	loadLocalEnv(".env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}
	config, e := pgx.ParseConfig(dsn)
	if e != nil {
		t.Fatal("invalid DATABASE_URL")
	}
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := fmt.Sprintf("courses_audit_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = admin.Exec("CREATE SCHEMA " + quoted); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if !strings.HasPrefix(schema, "courses_audit_") {
			panic("unsafe test schema")
		}
		if _, e := admin.Exec("DROP SCHEMA " + quoted + " CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	defer isolated.Close()
	oldDB := db
	db = isolated
	defer func() { db = oldDB }()
	var actual string
	if e = isolated.QueryRow("SELECT current_schema()").Scan(&actual); e != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SEED_DEMO_DATA", "false")
	t.Setenv("SYNC_GEOGRAPHY", "false")
	if e = prepareDatabase(); e != nil {
		t.Fatal(e)
	}
	if e = prepareCoursesDatabase(context.Background()); e != nil {
		t.Fatal("migration repeat", e)
	}
	var author, other, category int64
	for i, p := range []*int64{&author, &other} {
		if e = db.QueryRow(`INSERT INTO users(full_name,email,password_hash) VALUES($1,$2,repeat('*',60)) RETURNING id`, fmt.Sprintf("Автор курса %d", i), fmt.Sprintf("courses-%d@example.invalid", i)).Scan(p); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.QueryRow(`SELECT i.id FROM dictionary_items i JOIN dictionaries d ON d.id=i.dictionary_id WHERE d.alias='course_categories' ORDER BY i.sort_order LIMIT 1`).Scan(&category); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	h := courses.New(db, func(r *http.Request) (int64, error) {
		id, e := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		if e != nil {
			return 0, errors.New("unauthenticated")
		}
		return id, nil
	}, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "1" })
	h.Register(mux)
	request := func(method, path string, uid int64, isAdmin bool, body any, status int) []byte {
		t.Helper()
		var b bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		if uid > 0 {
			r.Header.Set("X-Test-User", strconv.FormatInt(uid, 10))
		}
		if isAdmin {
			r.Header.Set("X-Test-Admin", "1")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	request("POST", "/api/courses/my", 0, false, nil, 401)
	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(request("POST", "/api/courses/my", author, false, nil, 201), &created)
	id := created.ID
	path := fmt.Sprintf("/api/courses/my/%d", id)
	var v courses.Input
	_ = json.Unmarshal(request("GET", path, author, false, nil, 200), &v)
	request("GET", path, other, false, nil, 404)
	request("GET", path, 0, false, nil, 401)
	request("POST", path+"/submit", author, false, nil, 400)
	v.Title = "ИИ для бухгалтера: от первых шагов до реальных кейсов"
	v.Description = "Практический курс, который поможет экономить время и применять ИИ в повседневной работе бухгалтера."
	v.CategoryID = &category
	v.Hours = 6
	v.Price = 590000
	p := int64(890000)
	v.OldPrice = &p
	v.CoverImage = "/static/uploads/profimarket/" + strings.Repeat("a", 32) + ".webp"
	v.Certificate = true
	v.Chat = true
	v.Corporate = true
	v.Benefits = []courses.Benefit{{Icon: "check", Title: "Практические навыки", Description: "Применяйте ИИ в бухгалтерских задачах."}, {Icon: "clock", Title: "Экономия времени", Description: "Автоматизируйте повторяющиеся процессы."}, {Icon: "folder", Title: "Готовые шаблоны", Description: "Используйте материалы в работе."}, {Icon: "sparkles", Title: "Уверенность в технологиях", Description: "Пошаговое обучение без сложных терминов."}}
	v.Modules = []courses.Module{{Title: "Введение в ИИ для бухгалтера", Lessons: []courses.Lesson{{Title: "Как работает ИИ и где он помогает бухгалтеру", Type: "video", Duration: 15}, {Title: "Обзор инструментов", Type: "text", Duration: 25}}}, {Title: "Практические задачи", Lessons: []courses.Lesson{{Title: "Создаём рабочие шаблоны", Type: "practice", Duration: 30}}}}
	v.Benefits[0].Color = "mint"
	v.Benefits[3] = courses.Benefit{Icon: "sparkles", Color: "rose"}
	v.Bonuses = []courses.Bonus{{Title: "Библиотека промптов"}, {Title: "Готовые шаблоны документов"}}
	v.FAQ = []courses.FAQ{{Question: "Подойдёт ли курс новичкам?", Answer: "Да. Начинаем с основ и постепенно переходим к практическим задачам."}}
	request("PUT", path, other, false, v, 404)
	raw := request("PUT", path, author, false, v, 200)
	var saved courses.Input
	_ = json.Unmarshal(raw, &saved)
	if len(saved.Benefits) != 4 || saved.Benefits[0].Color != "mint" || saved.Benefits[3].Color != "rose" || saved.Benefits[3].Title != "" {
		t.Fatal("benefit colors or empty slots were not persisted")
	}
	request("PUT", path, author, false, v, 409)
	if saved.Modules[0].Lessons[0].ID == 0 {
		t.Fatal("missing persistent lesson id")
	}
	lessonID := saved.Modules[0].Lessons[0].ID
	// Invalid child ownership rolls back the entire update.
	invalid := saved
	invalid.Title = "Не должно сохраниться"
	invalid.Bonuses = []courses.Bonus{{ID: 999999, Title: "Чужой бонус"}}
	request("PUT", path, author, false, invalid, 400)
	var unchanged courses.Input
	_ = json.Unmarshal(request("GET", path, author, false, nil, 200), &unchanged)
	if unchanged.Title != saved.Title || unchanged.Revision != saved.Revision {
		t.Fatal("failed child update was not atomic")
	}
	saved.Modules[0].Lessons[0], saved.Modules[0].Lessons[1] = saved.Modules[0].Lessons[1], saved.Modules[0].Lessons[0]
	_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
	if saved.Modules[0].Lessons[1].ID != lessonID {
		t.Fatal("reordering replaced lesson identity")
	}
	var doc struct {
		Slug string `json:"slug"`
	}
	// Instructor data comes from the existing public profile, never career preferences.
	var profileID int64
	if e = db.QueryRow(`INSERT INTO resumes(user_id,status,visibility,work_preferences) VALUES($1,'published','public','Private career preference') RETURNING id`, author).Scan(&profileID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO resume_work_experiences(resume_id,company_name,position,start_month,start_year,is_current,responsibilities,achievements) VALUES($1,'Test employer','Главный бухгалтер',1,2020,TRUE,'Обучаю специалистов','Автоматизация учёта')`, profileID); e != nil {
		t.Fatal(e)
	}
	profileView := request("GET", path, author, false, nil, 200)
	if !bytes.Contains(profileView, []byte("Обучаю специалистов")) || bytes.Contains(profileView, []byte("Private career preference")) {
		t.Fatal("incorrect instructor bio source")
	}
	if _, e = db.Exec(`UPDATE resumes SET visibility='private' WHERE id=$1`, profileID); e != nil {
		t.Fatal(e)
	}
	privateView := request("GET", path, author, false, nil, 200)
	if bytes.Contains(privateView, []byte("Обучаю специалистов")) {
		t.Fatal("private profile leaked into course")
	}
	if _, e = db.Exec(`UPDATE resumes SET visibility='public' WHERE id=$1`, profileID); e != nil {
		t.Fatal(e)
	}
	_ = json.Unmarshal(raw, &doc)
	request("GET", "/api/courses/view/"+doc.Slug, 0, false, nil, 404)
	request("POST", path+"/submit", other, false, nil, 404)
	request("POST", path+"/submit", author, false, nil, 200)
	_ = json.Unmarshal(request("GET", path, author, false, nil, 200), &doc)
	publicPath := "/api/courses/view/" + doc.Slug
	request("GET", publicPath, 0, false, nil, 404)
	request("POST", fmt.Sprintf("/api/admin/courses/%d/approve", id), author, false, nil, 401)
	request("POST", fmt.Sprintf("/api/admin/courses/%d/approve", id), 0, true, nil, 200)
	request("GET", publicPath, 0, false, nil, 200)
	favoritePath := fmt.Sprintf("/api/courses/favorites/%d", id)
	request("POST", favoritePath, other, false, map[string]bool{"favorite": true}, 200)
	favorite := request("GET", favoritePath, other, false, nil, 200)
	if !bytes.Contains(favorite, []byte(`"favorite":true`)) {
		t.Fatal("favorite not persisted")
	}
	// Migration replay preserves author edits and review-ready structures.
	if e = prepareCoursesDatabase(context.Background()); e != nil {
		t.Fatal(e)
	}
	request("GET", publicPath, 0, false, nil, 200)
	request("POST", path+"/hide", author, false, nil, 200)
	request("GET", publicPath, 0, false, nil, 404)
	_ = json.Unmarshal(request("GET", path, author, false, nil, 200), &saved)
	if len(saved.Modules) != 2 || saved.Modules[0].Lessons[1].ID != lessonID {
		t.Fatal("migration changed curriculum")
	}
	// Generic dictionary admin contract keeps inactive category IDs intact.
	dicts, e := loadDictionaries(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dicts {
		if d.Alias != "course_categories" {
			continue
		}
		off := false
		d.Items[0].Active = &off
		if e = saveDictionary(context.Background(), d.ID, d.Name, d.Alias, d.VacancyTitle, d.ResumeTitle, d.UseImportanceInVacancy, d.SingleChoice, d.Items); e != nil {
			t.Fatal(e)
		}
	}
	var active bool
	if e = db.QueryRow(`SELECT active FROM dictionary_items WHERE id=$1`, category).Scan(&active); e != nil || active {
		t.Fatal("category disable failed")
	}
	t.Log("draft/edit/reorder/preview/moderation/publication/hide/ownership/favorites/idempotence/category checks passed")
	if os.Getenv("COURSES_BROWSER_READY_FILE") != "" {
		_, e = db.Exec(`UPDATE dictionary_items SET active=TRUE WHERE id=$1`, category)
		if e != nil {
			t.Fatal(e)
		}
		request("POST", path+"/submit", author, false, nil, 200)
		request("POST", fmt.Sprintf("/api/admin/courses/%d/approve", id), 0, true, nil, 200)
		serveCourseBrowserFixture(t, author, id)
	}
}
