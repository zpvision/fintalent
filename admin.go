package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const adminCookie = "fintalent_admin"

var adminSessionToken = newAdminToken()

type dictionary struct {
	ID                     int64            `json:"id"`
	Name                   string           `json:"name"`
	Alias                  string           `json:"alias"`
	Icon                   string           `json:"icon"`
	UseImportanceInVacancy bool             `json:"use_importance_in_vacancy"`
	SingleChoice           bool             `json:"single_choice"`
	VacancyTitle           string           `json:"vacancy_title"`
	ResumeTitle            string           `json:"resume_title"`
	Items                  []dictionaryItem `json:"items"`
	UpdatedAt              time.Time        `json:"updated_at"`
}

type dictionaryItem struct {
	ID      int64  `json:"id"`
	Value   string `json:"value"`
	Comment string `json:"comment"`
	Icon    string `json:"icon"`
	Order   int    `json:"order"`
}

var initialDictionaries = []struct {
	name  string
	alias string
	items []string
}{
	{"Должность", "position", []string{"Главный бухгалтер", "Заместитель главного бухгалтера", "Бухгалтер", "Помощник бухгалтера", "Бухгалтер по заработной плате", "Бухгалтер по первичной документации", "Бухгалтер по налогам", "Финансовый бухгалтер", "Аудитор", "Налоговый консультант", "Финансовый аналитик", "Экономист", "Другой вариант"}},
	{"Опыт", "experience", []string{"Нет опыта", "До 1 года", "1–3 года", "3–5 лет", "5–10 лет", "Более 10 лет"}},
	{"Сфера деятельности", "business_sector", []string{"Производство", "Торговля", "Услуги", "Строительство", "IT", "Маркетплейсы", "Общепит", "Медицина", "Образование", "Государственные учреждения", "Некоммерческие организации", "Логистика", "Другое"}},
	{"Размер компании", "company_size", []string{"До 10 сотрудников", "До 30", "До 100", "До 300", "Более 300"}},
	{"Участки", "accounting_areas", []string{"НДС", "УСН", "ОСНО", "Зарплата и кадры", "ТМЦ", "Банк и касса", "Основные средства", "Отчетность", "ВЭД", "Производство"}},
	{"Программы", "software", []string{"1С:Бухгалтерия", "1С:ЗУП", "1С:ERP", "СБИС", "Контур.Экстерн", "Диадок", "Excel", "Мое дело"}},
	{"Сколько компаний вели одновременно?", "companies_managed_simultaneously", []string{"1", "2-5", "6-10", "11-20", "20-50", "Более 50"}},
	{"Сколько юридических лиц вели в общей сложности?", "legal_entities_managed_total", []string{"1-5", "6-20", "21-50", "51-100", "Более 100"}},
	{"Объем первичных документов в месяц (примерно)?", "monthly_primary_documents", []string{"До 100", "100-500", "500-1000", "1000-5000", "Более 5000"}},
	{"Сколько сотрудников было в расчете?", "employees_in_payroll", []string{"До 10", "10-50", "51-100", "101-200", "Более 200"}},
	{"С максимальным оборотом каких компаний работали?", "maximum_company_turnover", []string{"До 30 млн ₽", "30-100 млн ₽", "100-500 млн ₽", "Более 500 млн ₽"}},
	{"Проходили налоговые проверки?", "tax_audits", []string{"Нет", "Да, 1–2 раза", "Да, регулярно"}},
}

var publicationDictionaries = []struct {
	name  string
	alias string
	items []string
}{
	{"Теги публикаций", "publication_tags", []string{
		"Бухгалтерский учёт", "Налоговый учёт", "Управленческий учёт", "Финансовый учёт", "НДС", "УСН", "ОСНО", "ПСН", "ЕНП", "НДФЛ",
		"Налог на прибыль", "Страховые взносы", "Заработная плата", "Кадровый учёт", "Отчётность", "Первичные документы", "ЭДО", "Электронная подпись", "1С:Бухгалтерия", "1С:ЗУП",
		"1С:ERP", "Excel", "Банк и касса", "Основные средства", "Нематериальные активы", "ТМЦ", "Инвентаризация", "Дебиторская задолженность", "Кредиторская задолженность", "Закрытие месяца",
		"Годовая отчётность", "Бухгалтерский баланс", "Декларация", "Камеральная проверка", "Выездная проверка", "Требования ФНС", "Налоговые риски", "Налоговая оптимизация", "Учётная политика", "Восстановление учёта",
		"Аудит", "МСФО", "РСБУ", "ВЭД", "Импорт", "Экспорт", "Маркировка", "Онлайн-касса", "Самозанятые", "Малый бизнес",
	}},
	{"Темы публикаций", "publication_topics", []string{
		"Бухгалтерский учёт для начинающих", "Организация бухгалтерского учёта", "Учётная политика организации", "Первичная документация", "Закрытие месяца", "Закрытие года", "Исправление ошибок в учёте", "Восстановление бухгалтерского учёта", "Бухгалтерская отчётность", "Бухгалтерский баланс",
		"Отчёт о финансовых результатах", "Налоговый учёт", "НДС", "Налог на прибыль", "НДФЛ", "Страховые взносы", "УСН доходы", "УСН доходы минус расходы", "Патентная система", "Общая система налогообложения",
		"Единый налоговый платёж", "Налоговые декларации", "Налоговые требования", "Камеральные проверки", "Выездные налоговые проверки", "Налоговые риски", "Налоговое планирование", "Расчёты с контрагентами", "Дебиторская задолженность", "Кредиторская задолженность",
		"Банк и касса", "Подотчётные лица", "Основные средства", "Нематериальные активы", "Материалы и товары", "Инвентаризация", "Производственный учёт", "Учёт в торговле", "Учёт в строительстве", "Учёт в сфере услуг",
		"Заработная плата", "Кадровое делопроизводство", "Отпуска и больничные", "Командировки", "Увольнение сотрудников", "Самозанятые и договоры ГПХ", "Электронный документооборот", "Электронная подпись", "Онлайн-кассы", "Маркировка товаров",
		"Работа в 1С:Бухгалтерии", "Работа в 1С:ЗУП", "Автоматизация учёта", "Excel для бухгалтера", "Внутренний контроль", "Бухгалтерский аудит", "МСФО", "Внешнеэкономическая деятельность", "Импорт и экспорт", "Финансовый анализ бизнеса",
	}},
}

func newAdminToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func prepareAdminDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS dictionaries (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(200) NOT NULL UNIQUE,
		alias VARCHAR(100),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS dictionary_items (
		id BIGSERIAL PRIMARY KEY, dictionary_id BIGINT NOT NULL REFERENCES dictionaries(id) ON DELETE CASCADE,
		value VARCHAR(500) NOT NULL, comment TEXT NOT NULL DEFAULT '', icon VARCHAR(500) NOT NULL DEFAULT '',
		sort_order INTEGER NOT NULL DEFAULT 0
	);
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS alias VARCHAR(100);
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS use_importance_in_vacancy BOOLEAN NOT NULL DEFAULT TRUE;
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS single_choice BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS vacancy_title VARCHAR(300) NOT NULL DEFAULT '';
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS resume_title VARCHAR(300) NOT NULL DEFAULT '';
	ALTER TABLE dictionaries ADD COLUMN IF NOT EXISTS icon VARCHAR(500) NOT NULL DEFAULT '';
	ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS comment TEXT NOT NULL DEFAULT '';
		ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS icon VARCHAR(500) NOT NULL DEFAULT '';
	CREATE UNIQUE INDEX IF NOT EXISTS dictionaries_alias_unique_idx ON dictionaries(alias) WHERE alias IS NOT NULL;`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE dictionaries SET icon='/api/assets/dictionary-icon/' || id || '.svg' WHERE BTRIM(COALESCE(icon,''))=''`)
	if err != nil {
		return err
	}
	for _, seed := range append(initialDictionaries, publicationDictionaries...) {
		var id int64
		err = db.QueryRowContext(ctx, `SELECT id FROM dictionaries WHERE alias=$1`, seed.alias).Scan(&id)
		if err == sql.ErrNoRows {
			err = db.QueryRowContext(ctx, `INSERT INTO dictionaries(name,alias) VALUES($1,$2)
				ON CONFLICT(name) DO UPDATE SET alias=EXCLUDED.alias RETURNING id`, seed.name, seed.alias).Scan(&id)
		}
		if err != nil {
			return err
		}
		var count int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dictionary_items WHERE dictionary_id=$1`, id).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			for order, value := range seed.items {
				if _, err = db.ExecContext(ctx, `INSERT INTO dictionary_items(dictionary_id,value,sort_order) VALUES($1,$2,$3)`, id, value, order); err != nil {
					return err
				}
			}
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE dictionary_items AS i
		SET icon = CASE d.alias
			WHEN 'position' THEN '/static/icons/positions/position-' || LPAD(i.sort_order::text, 2, '0') || '.svg'
			WHEN 'accounting_areas' THEN '/api/assets/accounting-area-icon/' || i.sort_order || '.png'
		END
		FROM dictionaries AS d
		WHERE d.id = i.dictionary_id
			AND (i.icon = '' OR (d.alias = 'position' AND i.icon LIKE '/api/assets/position-icon/%'))
			AND d.alias IN ('position', 'accounting_areas')`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE dictionary_items AS i
		SET icon = '/static/icons/dictionaries/' || d.alias || '-' || i.id || '.svg'
		FROM dictionaries AS d
		WHERE d.id = i.dictionary_id
			AND BTRIM(COALESCE(i.icon, '')) = ''
			AND i.id IN (
				14,15,16,17,18,19,
				20,21,22,23,24,25,26,27,28,29,30,31,32,
				33,34,35,36,37,
				56,57,58,59,60,61,
				62,63,64,65,66,
				67,68,69,70,71,
				72,73,74,75,76,
				77,78,79,80,
				81,82,83
			)`)
	if err != nil {
		return err
	}
	if err = prepareApplicantSurveyDatabase(ctx); err != nil {
		return err
	}
	if err = prepareOKVEDDatabase(ctx); err != nil {
		return err
	}
	return prepareVacancySurveyDatabase(ctx)
}

func registerAdminRoutes() {
	http.HandleFunc("/admin", serveFrontendPage("static/admin.html"))
	http.HandleFunc("/admin/", serveFrontendPage("static/admin.html"))
	http.HandleFunc("/api/admin/login", adminLogin)
	http.HandleFunc("/api/admin/logout", adminLogout)
	http.HandleFunc("/api/admin/session", adminSession)
	http.HandleFunc("/api/admin/dictionaries", adminDictionaries)
	http.HandleFunc("/api/admin/dictionaries/", adminDictionary)
	http.HandleFunc("/api/admin/okved", adminOKVED)
	http.HandleFunc("/api/admin/position-icons", adminPositionIconUpload)
	http.HandleFunc("/api/admin/users", adminUsers)
	http.HandleFunc("/api/admin/users/", adminUserAction)
	http.HandleFunc("/api/admin/zodiac-signs", adminZodiacSigns)
	http.HandleFunc("/api/assets/dictionary-icon/", dictionaryIconAsset)
	registerApplicantSurveyRoutes()
	registerVacancySurveyRoutes()
	registerTestCategoryRoutes()
	registerDutyRoutes()
}

type adminZodiacSign struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	StartMonth int    `json:"start_month"`
	StartDay   int    `json:"start_day"`
	EndMonth   int    `json:"end_month"`
	EndDay     int    `json:"end_day"`
	Icon       string `json:"icon"`
	SortOrder  int    `json:"sort_order"`
	Active     bool   `json:"active"`
}

func adminZodiacSigns(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeAdminJSON(w, http.StatusUnauthorized, map[string]string{"error": "Требуется вход администратора"})
		return
	}
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT id,code,name,start_month,start_day,end_month,end_day,icon,sort_order,is_active FROM zodiac_signs ORDER BY sort_order,id`)
		if err != nil {
			writeAdminJSON(w, 500, map[string]string{"error": "Не удалось загрузить знаки зодиака"})
			return
		}
		defer rows.Close()
		items := []adminZodiacSign{}
		for rows.Next() {
			var item adminZodiacSign
			if err = rows.Scan(&item.ID, &item.Code, &item.Name, &item.StartMonth, &item.StartDay, &item.EndMonth, &item.EndDay, &item.Icon, &item.SortOrder, &item.Active); err != nil {
				writeAdminJSON(w, 500, map[string]string{"error": "Не удалось загрузить знаки зодиака"})
				return
			}
			items = append(items, item)
		}
		writeAdminJSON(w, http.StatusOK, items)
		return
	}
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var items []adminZodiacSign
	if json.NewDecoder(r.Body).Decode(&items) != nil || len(items) != 12 {
		writeAdminJSON(w, 400, map[string]string{"error": "Справочник должен содержать 12 знаков"})
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAdminJSON(w, 500, map[string]string{"error": "Не удалось сохранить справочник"})
		return
	}
	defer tx.Rollback()
	for _, item := range items {
		item.Name, item.Icon = strings.TrimSpace(item.Name), strings.TrimSpace(item.Icon)
		if item.ID <= 0 || item.Name == "" || item.StartMonth < 1 || item.StartMonth > 12 || item.EndMonth < 1 || item.EndMonth > 12 || item.StartDay < 1 || item.StartDay > 31 || item.EndDay < 1 || item.EndDay > 31 {
			writeAdminJSON(w, 400, map[string]string{"error": "Проверьте названия и границы дат"})
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE zodiac_signs SET name=$1,start_month=$2,start_day=$3,end_month=$4,end_day=$5,icon=$6,sort_order=$7,is_active=$8,updated_at=NOW() WHERE id=$9`, item.Name, item.StartMonth, item.StartDay, item.EndMonth, item.EndDay, item.Icon, item.SortOrder, item.Active, item.ID); err != nil {
			writeAdminJSON(w, 500, map[string]string{"error": "Не удалось сохранить справочник"})
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAdminJSON(w, 500, map[string]string{"error": "Не удалось сохранить справочник"})
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]string{"message": "Справочник сохранён"})
}

func dictionaryIconAsset(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/assets/dictionary-icon/"), ".svg")
	id, err := strconv.ParseInt(tail, 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	var name string
	if err = db.QueryRowContext(r.Context(), `SELECT name FROM dictionaries WHERE id=$1`, id).Scan(&name); err != nil {
		http.NotFound(w, r)
		return
	}
	runes := []rune(strings.TrimSpace(name))
	mark := "?"
	if len(runes) > 0 {
		mark = strings.ToUpper(string(runes[0]))
	}
	glyphs := []string{
		`<path d="M20 19h24M20 29h24M20 39h15" stroke="black" stroke-width="4" stroke-linecap="round"/>`,
		`<circle cx="25" cy="24" r="7" fill="none" stroke="black" stroke-width="4"/><path d="M14 43c2-8 8-12 16-12s14 4 16 12" fill="none" stroke="black" stroke-width="4" stroke-linecap="round"/>`,
		`<path d="M18 18h28v28H18zM18 28h28M28 18v28" fill="none" stroke="black" stroke-width="4" stroke-linejoin="round"/>`,
		`<path d="M17 42l9-10 7 6 14-17" fill="none" stroke="black" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/><circle cx="47" cy="21" r="3" fill="black"/>`,
		`<path d="M19 23h26v23H19zM24 18v10M40 18v10M19 31h26" fill="none" stroke="black" stroke-width="4" stroke-linecap="round"/>`,
		`<path d="M32 16l5 10 11 2-8 8 2 11-10-5-10 5 2-11-8-8 11-2z" fill="none" stroke="black" stroke-width="3.5" stroke-linejoin="round"/>`,
	}
	glyph := glyphs[int(id%int64(len(glyphs)))]
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="6" y="6" width="52" height="52" rx="16" fill="black" opacity=".12"/>%s<text x="49" y="54" text-anchor="middle" font-family="Arial,sans-serif" font-size="13" font-weight="700" fill="black">%s</text></svg>`, glyph, html.EscapeString(mark))
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(svg))
}

func adminPositionIconUpload(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 300<<10)
	if err := r.ParseMultipartForm(256 << 10); err != nil {
		writeJSON(w, http.StatusBadRequest, "SVG-файл слишком большой")
		return
	}
	file, header, err := r.FormFile("icon")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Выберите SVG-файл")
		return
	}
	defer file.Close()
	if strings.ToLower(filepath.Ext(header.Filename)) != ".svg" {
		writeJSON(w, http.StatusBadRequest, "Поддерживаются только SVG-иконки")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, (256<<10)+1))
	if err != nil || len(data) == 0 || len(data) > 256<<10 || !safeSVG(data) {
		writeJSON(w, http.StatusBadRequest, "Некорректный или небезопасный SVG-файл")
		return
	}
	token := make([]byte, 12)
	if _, err = rand.Read(token); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить иконку")
		return
	}
	directory := filepath.Join("static", "uploads", "position-icons")
	if err = os.MkdirAll(directory, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось создать каталог иконок")
		return
	}
	name := hex.EncodeToString(token) + ".svg"
	if err = os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить иконку")
		return
	}
	writeAdminJSON(w, http.StatusCreated, map[string]string{"url": "/static/uploads/position-icons/" + name})
}

func safeSVG(data []byte) bool {
	value := strings.ToLower(string(data))
	if !strings.Contains(value, "<svg") || !strings.Contains(value, "</svg>") {
		return false
	}
	for _, forbidden := range []string{"<script", "<foreignobject", "javascript:", "data:text/html", "onload=", "onerror="} {
		if strings.Contains(value, forbidden) {
			return false
		}
	}
	return true
}

func adminLogin(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) || !parseForm(w, r) {
		return
	}
	expectedLogin, expectedPassword := strings.TrimSpace(os.Getenv("ADMIN_LOGIN")), os.Getenv("ADMIN_PASSWORD")
	if expectedLogin == "" || expectedPassword == "" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			writeJSON(w, http.StatusServiceUnavailable, "Вход администратора не настроен")
			return
		}
		expectedLogin, expectedPassword = "admin", "admin"
	}
	loginOK := subtle.ConstantTimeCompare([]byte(r.FormValue("login")), []byte(expectedLogin)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(expectedPassword)) == 1
	if !loginOK || !passwordOK {
		writeJSON(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: adminSessionToken, Path: "/", HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteStrictMode, MaxAge: 12 * 60 * 60})
	writeJSON(w, http.StatusOK, "Вход выполнен")
}

func adminLogout(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/", HttpOnly: true, Secure: secureCookies(), MaxAge: -1})
	writeJSON(w, http.StatusOK, "Выход выполнен")
}

func adminSession(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, "Требуется вход")
		return
	}
	writeJSON(w, http.StatusOK, "Авторизован")
}

func isAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(adminSessionToken)) == 1
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !isAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, "Требуется вход в админку")
		return false
	}
	return true
}

func adminDictionaries(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		dictionaries, err := loadDictionaries(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить справочники")
			return
		}
		writeAdminJSON(w, http.StatusOK, dictionaries)
	case http.MethodPost:
		var payload struct {
			Name                   string `json:"name"`
			Alias                  string `json:"alias"`
			UseImportanceInVacancy *bool  `json:"use_importance_in_vacancy"`
			SingleChoice           bool   `json:"single_choice"`
			VacancyTitle           string `json:"vacancy_title"`
			ResumeTitle            string `json:"resume_title"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		payload.Name, payload.Alias = strings.TrimSpace(payload.Name), strings.ToLower(strings.TrimSpace(payload.Alias))
		if payload.Name == "" || !validAlias(payload.Alias) {
			writeJSON(w, http.StatusBadRequest, "Укажите название и alias латиницей")
			return
		}
		var id int64
		useImportance := true
		if payload.UseImportanceInVacancy != nil {
			useImportance = *payload.UseImportanceInVacancy
		}
		vacancyTitle := strings.TrimSpace(payload.VacancyTitle)
		resumeTitle := strings.TrimSpace(payload.ResumeTitle)
		err := db.QueryRow(`INSERT INTO dictionaries(name,alias,use_importance_in_vacancy,single_choice,vacancy_title,resume_title) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, payload.Name, payload.Alias, useImportance, payload.SingleChoice, vacancyTitle, resumeTitle).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusConflict, "Справочник с таким названием уже существует")
			return
		}
		writeAdminJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func adminDictionary(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/admin/dictionaries/"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректный справочник")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var payload struct {
			Name                   string           `json:"name"`
			Alias                  string           `json:"alias"`
			UseImportanceInVacancy *bool            `json:"use_importance_in_vacancy"`
			SingleChoice           bool             `json:"single_choice"`
			VacancyTitle           string           `json:"vacancy_title"`
			ResumeTitle            string           `json:"resume_title"`
			Items                  []dictionaryItem `json:"items"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil || strings.TrimSpace(payload.Name) == "" || !validAlias(payload.Alias) {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		useImportance := true
		if payload.UseImportanceInVacancy != nil {
			useImportance = *payload.UseImportanceInVacancy
		}
		if err := saveDictionary(r.Context(), id, payload.Name, payload.Alias, payload.VacancyTitle, payload.ResumeTitle, useImportance, payload.SingleChoice, payload.Items); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось сохранить справочник")
			return
		}
		writeJSON(w, http.StatusOK, "Справочник сохранён")
	case http.MethodDelete:
		if _, err := db.Exec(`DELETE FROM dictionaries WHERE id=$1`, id); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось удалить справочник")
			return
		}
		writeJSON(w, http.StatusOK, "Справочник удалён")
	default:
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func loadDictionaries(ctx context.Context) ([]dictionary, error) {
	rows, err := db.QueryContext(ctx, `SELECT d.id,d.name,COALESCE(d.alias,''),COALESCE(d.icon,''),d.use_importance_in_vacancy,d.single_choice,d.vacancy_title,d.resume_title,d.updated_at,i.id,i.value,i.comment,i.icon,i.sort_order FROM dictionaries d LEFT JOIN dictionary_items i ON i.dictionary_id=d.id AND i.deleted_at IS NULL ORDER BY d.id,i.sort_order,i.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []dictionary{}
	indexes := map[int64]int{}
	for rows.Next() {
		var dID int64
		var name string
		var alias string
		var dictionaryIcon string
		var useImportance bool
		var singleChoice bool
		var vacancyTitle string
		var resumeTitle string
		var updated time.Time
		var itemID sql.NullInt64
		var value sql.NullString
		var comment sql.NullString
		var icon sql.NullString
		var order sql.NullInt64
		if err := rows.Scan(&dID, &name, &alias, &dictionaryIcon, &useImportance, &singleChoice, &vacancyTitle, &resumeTitle, &updated, &itemID, &value, &comment, &icon, &order); err != nil {
			return nil, err
		}
		idx, ok := indexes[dID]
		if !ok {
			idx = len(result)
			indexes[dID] = idx
			result = append(result, dictionary{ID: dID, Name: name, Alias: alias, Icon: dictionaryIcon, UseImportanceInVacancy: useImportance, SingleChoice: singleChoice, VacancyTitle: vacancyTitle, ResumeTitle: resumeTitle, Items: []dictionaryItem{}, UpdatedAt: updated})
		}
		if itemID.Valid {
			result[idx].Items = append(result[idx].Items, dictionaryItem{ID: itemID.Int64, Value: value.String, Comment: comment.String, Icon: icon.String, Order: int(order.Int64)})
		}
	}
	return result, rows.Err()
}

func saveDictionary(ctx context.Context, id int64, name, alias, vacancyTitle, resumeTitle string, useImportance, singleChoice bool, items []dictionaryItem) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	vacancyTitle = strings.TrimSpace(vacancyTitle)
	resumeTitle = strings.TrimSpace(resumeTitle)
	result, err := tx.ExecContext(ctx, `UPDATE dictionaries SET name=$1,alias=$2,vacancy_title=$3,resume_title=$4,use_importance_in_vacancy=$5,single_choice=$6,updated_at=NOW() WHERE id=$7`, strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(alias)), vacancyTitle, resumeTitle, useImportance, singleChoice, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.New("dictionary not found")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE dictionary_items SET active=FALSE,deleted_at=NOW() WHERE dictionary_id=$1`, id); err != nil {
		return err
	}
	order := 0
	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}
		if item.ID > 0 {
			result, updateErr := tx.ExecContext(ctx, `UPDATE dictionary_items SET value=$1,comment=$2,icon=$3,sort_order=$4,active=TRUE,deleted_at=NULL WHERE id=$5 AND dictionary_id=$6`, value, strings.TrimSpace(item.Comment), strings.TrimSpace(item.Icon), order, item.ID, id)
			if updateErr != nil {
				return updateErr
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return errors.New("dictionary item not found")
			}
		} else if _, err = tx.ExecContext(ctx, `INSERT INTO dictionary_items(dictionary_id,value,comment,icon,sort_order,active,deleted_at) VALUES($1,$2,$3,$4,$5,TRUE,NULL)`, id, value, strings.TrimSpace(item.Comment), strings.TrimSpace(item.Icon), order); err != nil {
			return err
		}
		order++
	}
	return tx.Commit()
}

func validAlias(alias string) bool {
	alias = strings.TrimSpace(strings.ToLower(alias))
	if alias == "" || len(alias) > 100 {
		return false
	}
	for i, r := range alias {
		if ((r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_') || (i == 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func writeAdminJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
