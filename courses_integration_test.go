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
	mux.HandleFunc("/api/admin/course-access", adminCourseAccess)
	mux.HandleFunc("/api/admin/users/", adminUserAction)
	adminResponse := httptest.NewRecorder()
	if err := createAdminSession(adminResponse, httptest.NewRequest("POST", "/api/admin/login", nil)); err != nil {
		t.Fatal(err)
	}
	adminSessionCookie := adminResponse.Result().Cookies()[0]
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
			r.AddCookie(adminSessionCookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	t.Run("restricted and public access", func(t *testing.T) {
		for _, uid := range []int64{0, author, other} {
			if raw := request("GET", "/api/courses/access", uid, false, nil, 200); !bytes.Contains(raw, []byte(`"allowed":false`)) {
				t.Fatal("restricted module visible")
			}
			for _, route := range []string{"/api/courses", "/api/courses/meta", "/api/courses/my", "/api/courses/my/1", "/api/courses/view/example", "/api/courses/favorites/1"} {
				request("GET", route, uid, false, nil, 404)
			}
		}
		request("GET", "/api/admin/course-access", author, false, nil, 401)
		request("PUT", "/api/admin/course-access", author, false, map[string]bool{"public_access": true}, 401)
		grant := fmt.Sprintf("/api/admin/users/%d/courses-access", author)
		request("PUT", grant, author, false, map[string]bool{"courses_access": true}, 401)
		request("PUT", grant, 0, true, map[string]any{"courses_access": "true"}, 400)
		request("PUT", grant, 0, true, map[string]bool{"courses_access": true}, 200)
		request("GET", "/api/courses/meta", author, false, nil, 200)
		request("GET", "/api/courses/meta", other, false, nil, 404)
		request("GET", "/api/courses/meta", 0, true, nil, 200)
		request("PUT", grant, 0, true, map[string]bool{"courses_access": false}, 200)
		request("GET", "/api/courses/meta", author, false, nil, 404)
		request("PUT", "/api/admin/users/9223372036854775807/courses-access", 0, true, map[string]bool{"courses_access": true}, 404)
		request("PUT", "/api/admin/course-access", 0, true, map[string]bool{"public_access": true}, 200)
		for _, uid := range []int64{0, author, other} {
			request("GET", "/api/courses/meta", uid, false, nil, 200)
		}
		if err := prepareCoursesDatabase(context.Background()); err != nil {
			t.Fatal(err)
		}
		request("GET", "/api/courses/meta", 0, false, nil, 200)
		request("PUT", "/api/admin/course-access", 0, true, map[string]bool{"public_access": false}, 200)
		request("GET", "/api/courses/meta", 0, false, nil, 404)
		// The existing publication lifecycle exercises the public mode.
		request("PUT", "/api/admin/course-access", 0, true, map[string]bool{"public_access": true}, 200)
	})
	request("POST", "/api/courses/my", 0, false, nil, 401)
	t.Run("lesson type dictionary lifecycle", func(t *testing.T) {
		var dictionaryID, itemID int64
		if err := db.QueryRow(`SELECT id FROM dictionaries WHERE alias='course_lesson_types'`).Scan(&dictionaryID); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`INSERT INTO dictionary_items(dictionary_id,value,icon,sort_order) VALUES($1,'Вебинар','/static/uploads/position-icons/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.svg',50) RETURNING id`, dictionaryID).Scan(&itemID); err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("lesson-%d", itemID)
		meta := request("GET", "/api/courses/meta", 0, false, nil, 200)
		if !bytes.Contains(meta, []byte(key)) {
			t.Fatal("new dictionary type missing from metadata")
		}
		var created struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(request("POST", "/api/courses/my", author, false, nil, 201), &created)
		p := fmt.Sprintf("/api/courses/my/%d", created.ID)
		var input courses.Input
		_ = json.Unmarshal(request("GET", p, author, false, nil, 200), &input)
		input.Modules = []courses.Module{{Title: "Модуль", Lessons: []courses.Lesson{{Title: "Вебинар", Type: key}}}}
		_ = json.Unmarshal(request("PUT", p, author, false, input, 200), &input)
		if _, err := db.Exec(`UPDATE dictionary_items SET value='Прямой эфир',active=FALSE,deleted_at=NOW() WHERE id=$1`, itemID); err != nil {
			t.Fatal(err)
		}
		meta = request("GET", "/api/courses/meta", 0, false, nil, 200)
		if bytes.Contains(meta, []byte(key)) {
			t.Fatal("deleted type selectable")
		}
		raw := request("PUT", p, author, false, input, 200)
		if !bytes.Contains(raw, []byte("Прямой эфир")) || !bytes.Contains(raw, []byte("type_icon")) {
			t.Fatal("existing lesson lost dictionary presentation")
		}
		_ = json.Unmarshal(raw, &input)
		input.Modules[0].Lessons = append(input.Modules[0].Lessons, courses.Lesson{Title: "Новый", Type: key})
		request("PUT", p, author, false, input, 400)
		input.Modules[0].Lessons = input.Modules[0].Lessons[:1]
		input.Modules[0].Lessons[0].Type = "lesson-9223372036854775807"
		request("PUT", p, author, false, input, 400)
		if err := prepareCoursesDatabase(context.Background()); err != nil {
			t.Fatal(err)
		}
		meta = request("GET", "/api/courses/meta", 0, false, nil, 200)
		if bytes.Contains(meta, []byte(key)) {
			t.Fatal("migration reactivated deleted type")
		}
	})
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
	v.FeatureIDs = nil // Exercise compatibility with the existing flags.
	v.BonusesColor = "amber"
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
	if !saved.Certificate || !saved.Chat || !saved.Corporate || saved.BonusesColor != "amber" || len(saved.FeatureIDs) == 0 {
		t.Fatal("legacy settings or bonus color lost")
	}
	if len(saved.Benefits) != 4 || saved.Benefits[0].Color != "mint" || saved.Benefits[3].Color != "rose" || saved.Benefits[3].Title != "" {
		t.Fatal("benefit colors or empty slots were not persisted")
	}
	request("PUT", path, author, false, v, 409)
	t.Run("custom course features", func(t *testing.T) {
		saved.CustomFeatures = []courses.Bonus{{Title: "  Проверка домашних заданий автором  "}, {Title: "Индивидуальная консультация"}}
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		if len(saved.CustomFeatures) != 2 || saved.CustomFeatures[0].ID <= 0 || saved.CustomFeatures[0].Title != "Проверка домашних заданий автором" {
			t.Fatal("custom features not persisted")
		}
		firstID := saved.CustomFeatures[0].ID
		meta := request("GET", "/api/courses/meta", 0, false, nil, 200)
		if bytes.Contains(meta, []byte(saved.CustomFeatures[0].Title)) {
			t.Fatal("custom feature leaked into dictionary")
		}
		preview := request("GET", fmt.Sprintf("/api/admin/courses/%d", id), 0, true, nil, 200)
		if !bytes.Contains(preview, []byte(saved.CustomFeatures[0].Title)) {
			t.Fatal("custom features missing in moderation preview")
		}
		bad := saved
		bad.Title = "Не должно сохраниться"
		bad.CustomFeatures = []courses.Bonus{{ID: firstID, Title: "Чат с автором и участниками"}}
		request("PUT", path, author, false, bad, 400)
		var unchanged courses.Input
		_ = json.Unmarshal(request("GET", path, author, false, nil, 200), &unchanged)
		if unchanged.Title != saved.Title || unchanged.Revision != saved.Revision || unchanged.CustomFeatures[0].Title != saved.CustomFeatures[0].Title {
			t.Fatal("duplicate update not rolled back")
		}
		// A child ID from another course cannot be adopted or changed.
		var second struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(request("POST", "/api/courses/my", other, false, nil, 201), &second)
		secondPath := fmt.Sprintf("/api/courses/my/%d", second.ID)
		var secondInput courses.Input
		_ = json.Unmarshal(request("GET", secondPath, other, false, nil, 200), &secondInput)
		secondInput.CustomFeatures = []courses.Bonus{{ID: firstID, Title: "Чужой пункт"}}
		request("PUT", secondPath, other, false, secondInput, 400)
		bad = saved
		bad.CustomFeatures = append([]courses.Bonus{}, saved.CustomFeatures...)
		bad.CustomFeatures = append(bad.CustomFeatures, bad.CustomFeatures[0])
		request("PUT", path, author, false, bad, 400)
		saved.CustomFeatures[0], saved.CustomFeatures[1] = saved.CustomFeatures[1], saved.CustomFeatures[0]
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		if saved.CustomFeatures[1].ID != firstID {
			t.Fatal("feature order or identity lost")
		}
		legacy := saved
		legacy.CustomFeatures = nil
		_ = json.Unmarshal(request("PUT", path, author, false, legacy, 200), &saved)
		if len(saved.CustomFeatures) != 2 {
			t.Fatal("older client erased custom features")
		}
		saved.CustomFeatures = saved.CustomFeatures[1:]
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		if len(saved.CustomFeatures) != 1 || saved.CustomFeatures[0].ID != firstID {
			t.Fatal("custom feature removal failed")
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM course_custom_features WHERE course_id=$1`, id).Scan(&count); err != nil || count != 1 {
			t.Fatal("custom feature removal not persisted")
		}
	})
	t.Run("course tariffs", func(t *testing.T) {
		saved.PricingMode = "tariffs"
		tariffOldPrice := int64(300)
		saved.Tariffs = []courses.Tariff{{Title: "Базовый", Description: "Материалы", Price: 100, OldPrice: &tariffOldPrice, Hours: 2.5}, {Title: "С поддержкой", Description: "Обратная связь", Price: 200, Hours: 5, Recommended: true}}
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		first := saved.Tariffs[0].ID
		if saved.Tariffs[0].OldPrice == nil || *saved.Tariffs[0].OldPrice != 300 || saved.Tariffs[0].Hours != 2.5 {
			t.Fatal("tariff price/hours not persisted")
		}
		if first <= 0 || len(saved.Tariffs) != 2 || !saved.Tariffs[1].Recommended {
			t.Fatal("tariffs not persisted")
		}
		saved.Tariffs[0], saved.Tariffs[1] = saved.Tariffs[1], saved.Tariffs[0]
		saved.Tariffs[0].Recommended = false
		saved.Tariffs[1].Recommended = true
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		if saved.Tariffs[1].ID != first || !saved.Tariffs[1].Recommended {
			t.Fatal("reorder/recommend failed")
		}
		var second struct{ ID int64 }
		_ = json.Unmarshal(request("POST", "/api/courses/my", other, false, nil, 201), &second)
		secondPath := fmt.Sprintf("/api/courses/my/%d", second.ID)
		var foreign courses.Input
		_ = json.Unmarshal(request("GET", secondPath, other, false, nil, 200), &foreign)
		foreign.PricingMode = "tariffs"
		foreign.Tariffs = saved.Tariffs
		request("PUT", secondPath, other, false, foreign, 400)
		legacy := saved
		legacy.PricingMode = ""
		legacy.Tariffs = nil
		_ = json.Unmarshal(request("PUT", path, author, false, legacy, 200), &saved)
		if saved.PricingMode != "tariffs" || len(saved.Tariffs) != 2 {
			t.Fatal("legacy erased pricing")
		}
		list := request("GET", "/api/courses/my", author, false, nil, 200)
		var rows []struct {
			ID       int64
			MinPrice int64 `json:"min_price"`
		}
		_ = json.Unmarshal(list, &rows)
		found := false
		for _, row := range rows {
			if row.ID == id {
				found = true
				if row.MinPrice != 100 {
					t.Fatal("minimum price incorrect")
				}
			}
		}
		if !found {
			t.Fatal("course missing")
		}
		saved.PricingMode = "single"
		_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
		if len(saved.Tariffs) != 2 || saved.Price != 590000 {
			t.Fatal("mode switch lost values")
		}
	})

	var customFeature int64
	if err := db.QueryRow(`INSERT INTO dictionary_items(dictionary_id,value,sort_order) SELECT id,'Еженедельные встречи',100 FROM dictionaries WHERE alias='course_features' RETURNING id`).Scan(&customFeature); err != nil {
		t.Fatal(err)
	}
	badFeatures := saved
	badFeatures.FeatureIDs = append(append([]int64{}, saved.FeatureIDs...), category)
	request("PUT", path, author, false, badFeatures, 400)
	saved.FeatureIDs = append(saved.FeatureIDs, customFeature)
	_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
	if len(saved.FeatureIDs) < 2 || saved.FeatureIDs[len(saved.FeatureIDs)-1] != customFeature {
		t.Fatal("custom dictionary feature not persisted")
	}
	if _, err := db.Exec(`UPDATE dictionary_items SET active=FALSE WHERE id=$1`, customFeature); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(request("PUT", path, author, false, saved, 200), &saved)
	var meta struct {
		Features []struct{ ID int64 } `json:"features"`
	}
	_ = json.Unmarshal(request("GET", "/api/courses/meta", 0, false, nil, 200), &meta)
	for _, feature := range meta.Features {
		if feature.ID == customFeature {
			t.Fatal("inactive feature offered in catalog")
		}
	}
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
	publicView := request("GET", publicPath, 0, false, nil, 200)
	if !bytes.Contains(publicView, []byte(saved.CustomFeatures[0].Title)) {
		t.Fatal("custom features missing from published course")
	}
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
