package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const adminCookie = "fintalent_admin"

var adminSessionToken = newAdminToken()

type dictionary struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Alias     string           `json:"alias"`
	Items     []dictionaryItem `json:"items"`
	UpdatedAt time.Time        `json:"updated_at"`
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
	ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS comment TEXT NOT NULL DEFAULT '';
	ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS icon VARCHAR(500) NOT NULL DEFAULT '';
	CREATE UNIQUE INDEX IF NOT EXISTS dictionaries_alias_unique_idx ON dictionaries(alias) WHERE alias IS NOT NULL;`)
	if err != nil {
		return err
	}
	for _, seed := range initialDictionaries {
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
	return nil
}

func registerAdminRoutes() {
	http.HandleFunc("/admin", servePage("static/admin.html"))
	http.HandleFunc("/admin/", servePage("static/admin.html"))
	http.HandleFunc("/api/admin/login", adminLogin)
	http.HandleFunc("/api/admin/logout", adminLogout)
	http.HandleFunc("/api/admin/session", adminSession)
	http.HandleFunc("/api/admin/dictionaries", adminDictionaries)
	http.HandleFunc("/api/admin/dictionaries/", adminDictionary)
}

func adminLogin(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) || !parseForm(w, r) {
		return
	}
	loginOK := subtle.ConstantTimeCompare([]byte(r.FormValue("login")), []byte("admin")) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte("admin")) == 1
	if !loginOK || !passwordOK {
		writeJSON(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: adminSessionToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 60 * 60})
	writeJSON(w, http.StatusOK, "Вход выполнен")
}

func adminLogout(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
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
			Name  string `json:"name"`
			Alias string `json:"alias"`
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
		err := db.QueryRow(`INSERT INTO dictionaries(name,alias) VALUES($1,$2) RETURNING id`, payload.Name, payload.Alias).Scan(&id)
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
			Name  string           `json:"name"`
			Alias string           `json:"alias"`
			Items []dictionaryItem `json:"items"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil || strings.TrimSpace(payload.Name) == "" || !validAlias(payload.Alias) {
			writeJSON(w, http.StatusBadRequest, "Некорректные данные")
			return
		}
		if err := saveDictionary(r.Context(), id, payload.Name, payload.Alias, payload.Items); err != nil {
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
	rows, err := db.QueryContext(ctx, `SELECT d.id,d.name,COALESCE(d.alias,''),d.updated_at,i.id,i.value,i.comment,i.icon,i.sort_order FROM dictionaries d LEFT JOIN dictionary_items i ON i.dictionary_id=d.id ORDER BY d.id,i.sort_order,i.id`)
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
		var updated time.Time
		var itemID sql.NullInt64
		var value sql.NullString
		var comment sql.NullString
		var icon sql.NullString
		var order sql.NullInt64
		if err := rows.Scan(&dID, &name, &alias, &updated, &itemID, &value, &comment, &icon, &order); err != nil {
			return nil, err
		}
		idx, ok := indexes[dID]
		if !ok {
			idx = len(result)
			indexes[dID] = idx
			result = append(result, dictionary{ID: dID, Name: name, Alias: alias, Items: []dictionaryItem{}, UpdatedAt: updated})
		}
		if itemID.Valid {
			result[idx].Items = append(result[idx].Items, dictionaryItem{ID: itemID.Int64, Value: value.String, Comment: comment.String, Icon: icon.String, Order: int(order.Int64)})
		}
	}
	return result, rows.Err()
}

func saveDictionary(ctx context.Context, id int64, name, alias string, items []dictionaryItem) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE dictionaries SET name=$1,alias=$2,updated_at=NOW() WHERE id=$3`, strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(alias)), id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errors.New("dictionary not found")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM dictionary_items WHERE dictionary_id=$1`, id); err != nil {
		return err
	}
	order := 0
	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO dictionary_items(dictionary_id,value,comment,icon,sort_order) VALUES($1,$2,$3,$4,$5)`, id, value, strings.TrimSpace(item.Comment), strings.TrimSpace(item.Icon), order); err != nil {
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
