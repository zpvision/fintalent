package accountingcompany

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type User struct {
	ID       int64
	FullName string
	Email    string
}

type UserResolver func(*http.Request) (User, error)
type AdminResolver func(*http.Request) bool

type Handler struct {
	db    *sql.DB
	user  UserResolver
	admin AdminResolver
}

type ServiceInput struct {
	ID         int64    `json:"id"`
	ServiceID  *int64   `json:"service_id"`
	CustomName string   `json:"custom_name"`
	PriceFrom  *float64 `json:"price_from"`
	PriceType  string   `json:"price_type"`
	SortOrder  int      `json:"sort_order"`
}

type TariffInput struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Subtitle  string   `json:"subtitle"`
	Price     *float64 `json:"price"`
	Period    string   `json:"period"`
	Benefits  []string `json:"benefits"`
	SortOrder int      `json:"sort_order"`
	Popular   bool     `json:"popular"`
	Active    bool     `json:"active"`
}

type CompanyInput struct {
	Name               string         `json:"name"`
	ShortDescription   string         `json:"short_description"`
	FullDescription    string         `json:"full_description"`
	Logo               string         `json:"logo"`
	City               string         `json:"city"`
	Address            string         `json:"address"`
	RemoteAllRussia    bool           `json:"remote_all_russia"`
	FoundedYear        *int           `json:"founded_year"`
	EmployeeCount      *int           `json:"employee_count"`
	INN                string         `json:"inn"`
	Phone              string         `json:"phone"`
	Email              string         `json:"email"`
	Website            string         `json:"website"`
	Telegram           string         `json:"telegram"`
	Whatsapp           string         `json:"whatsapp"`
	VK                 string         `json:"vk"`
	WorkHours          string         `json:"work_hours"`
	ManagerName        string         `json:"manager_name"`
	ManagerPosition    string         `json:"manager_position"`
	ManagerPhoto       string         `json:"manager_photo"`
	ManagerDescription string         `json:"manager_description"`
	ManagerUserID      *int64         `json:"manager_user_id"`
	AccentStyleID      *int64         `json:"accent_style_id"`
	HeaderImageType    string         `json:"header_image_type"`
	HeaderTemplateID   *int64         `json:"header_template_id"`
	CustomHeaderImage  string         `json:"custom_header_image"`
	Advantages         []string       `json:"advantages"`
	DirectionIDs       []int64        `json:"direction_ids"`
	KeyDirectionIDs    []int64        `json:"key_direction_ids"`
	TaxSystemIDs       []int64        `json:"tax_system_ids"`
	Services           []ServiceInput `json:"services"`
	Tariffs            []TariffInput  `json:"tariffs"`
	CurrentStep        int            `json:"current_step"`
}

func New(db *sql.DB, user UserResolver, admin AdminResolver) *Handler {
	return &Handler{db: db, user: user, admin: admin}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/accounting-companies/meta", h.meta)
	mux.HandleFunc("/api/accounting-companies/my", h.myCompany)
	mux.HandleFunc("/api/accounting-companies", h.companies)
	mux.HandleFunc("/api/accounting-companies/", h.companyRoute)
	mux.HandleFunc("/api/admin/accounting-companies/dictionaries", h.adminDictionaries)
	mux.HandleFunc("/api/admin/accounting-companies/dictionaries/", h.adminDictionary)
}

func response(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func failure(w http.ResponseWriter, status int, message string) {
	response(w, status, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		failure(w, http.StatusBadRequest, "Некорректные данные формы")
		return false
	}
	return true
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) (User, bool) {
	u, err := h.user(r)
	if err != nil {
		failure(w, http.StatusUnauthorized, "Требуется авторизация")
		return User{}, false
	}
	return u, true
}

func (h *Handler) optionalUser(r *http.Request) User {
	u, _ := h.user(r)
	return u
}

func (h *Handler) meta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	tables := []struct{ Key, Query string }{
		{"directions", `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'icon',icon,'description',description,'sort_order',sort_order,'active',active) FROM accounting_company_directions WHERE active AND deleted_at IS NULL ORDER BY sort_order,name`},
		{"services", `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'icon',icon,'category',category,'description',description,'sort_order',sort_order,'active',active) FROM accounting_company_service_catalog WHERE active AND deleted_at IS NULL ORDER BY sort_order,name`},
		{"headers", `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'image_url',image_url,'category',category,'sort_order',sort_order,'active',active) FROM accounting_company_header_templates WHERE active AND deleted_at IS NULL ORDER BY sort_order,name`},
		{"accents", `SELECT jsonb_build_object('id',id,'name',name,'color_key',color_key,'color_value',color_value,'sort_order',sort_order,'active',active) FROM accounting_company_accent_styles WHERE active AND deleted_at IS NULL ORDER BY sort_order,name`},
		{"tax_systems", `SELECT jsonb_build_object('id',id,'name',name,'slug',slug,'sort_order',sort_order,'active',active) FROM accounting_company_tax_systems WHERE active ORDER BY sort_order,name`},
	}
	result := map[string][]json.RawMessage{}
	for _, table := range tables {
		rows, err := h.db.QueryContext(r.Context(), table.Query)
		if err != nil {
			failure(w, 500, "Не удалось загрузить справочники")
			return
		}
		items := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if rows.Scan(&raw) == nil {
				items = append(items, raw)
			}
		}
		rows.Close()
		result[table.Key] = items
	}
	response(w, 200, result)
}

func (h *Handler) companies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.catalog(w, r)
	case http.MethodPost:
		u, ok := h.current(w, r)
		if !ok {
			return
		}
		var in CompanyInput
		if !decode(w, r, &in) {
			return
		}
		id, err := h.create(r.Context(), u, in)
		if err != nil {
			failure(w, 400, err.Error())
			return
		}
		h.sendCompany(w, r, id, u.ID, true)
	default:
		failure(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) myCompany(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	var id int64
	err := h.db.QueryRowContext(r.Context(), `SELECT id FROM accounting_companies WHERE owner_user_id=$1 AND deleted_at IS NULL`, u.ID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		response(w, 200, map[string]any{"company": nil})
		return
	}
	if err != nil {
		failure(w, 500, "Не удалось загрузить компанию")
		return
	}
	h.sendCompany(w, r, id, u.ID, true)
}

func (h *Handler) companyRoute(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/accounting-companies/"), "/")
	parts := strings.Split(tail, "/")
	if len(parts) == 0 || parts[0] == "" {
		failure(w, 404, "Компания не найдена")
		return
	}
	if parts[0] == "slug" && len(parts) > 1 {
		var id int64
		if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM accounting_companies WHERE slug=$1 AND deleted_at IS NULL`, parts[1]).Scan(&id); err != nil {
			failure(w, 404, "Компания не найдена")
			return
		}
		h.sendCompany(w, r, id, h.optionalUser(r).ID, false)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		failure(w, 404, "Компания не найдена")
		return
	}
	if len(parts) > 1 {
		switch parts[1] {
		case "publish":
			h.changeStatus(w, r, id, "published")
		case "archive":
			h.changeStatus(w, r, id, "archived")
		case "passport":
			h.passport(w, r, id)
		case "reviews":
			h.reviews(w, r, id)
		default:
			failure(w, 404, "Раздел не найден")
		}
		return
	}
	switch r.Method {
	case http.MethodGet:
		u := h.optionalUser(r)
		h.sendCompany(w, r, id, u.ID, false)
	case http.MethodPut:
		u, ok := h.current(w, r)
		if !ok {
			return
		}
		var in CompanyInput
		if !decode(w, r, &in) {
			return
		}
		if err := h.update(r.Context(), id, u.ID, in); err != nil {
			failure(w, 400, err.Error())
			return
		}
		h.sendCompany(w, r, id, u.ID, true)
	case http.MethodDelete:
		u, ok := h.current(w, r)
		if !ok {
			return
		}
		res, e := h.db.ExecContext(r.Context(), `UPDATE accounting_companies SET deleted_at=NOW(),status='archived',updated_at=NOW() WHERE id=$1 AND (owner_user_id=$2 OR $3) AND deleted_at IS NULL`, id, u.ID, h.admin(r))
		if e != nil {
			failure(w, 500, "Не удалось удалить компанию")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			failure(w, 403, "Недостаточно прав")
			return
		}
		response(w, 200, map[string]bool{"ok": true})
	default:
		failure(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) create(ctx context.Context, u User, in CompanyInput) (int64, error) {
	var existing int64
	if h.db.QueryRowContext(ctx, `SELECT id FROM accounting_companies WHERE owner_user_id=$1 AND deleted_at IS NULL`, u.ID).Scan(&existing) == nil {
		return existing, nil
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "Новая бухгалтерская компания"
	}
	if in.Email == "" {
		in.Email = u.Email
	}
	if in.ManagerName == "" {
		in.ManagerName = u.FullName
	}
	if in.HeaderImageType == "" {
		in.HeaderImageType = "template"
	}
	var id int64
	slug := slugify(in.Name) + "-" + strconv.FormatInt(time.Now().Unix()%1000000, 10)
	err := h.db.QueryRowContext(ctx, `INSERT INTO accounting_companies(owner_user_id,name,slug,email,manager_name,header_image_type,current_step) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, u.ID, clean(in.Name, 240), slug, clean(in.Email, 254), clean(in.ManagerName, 240), in.HeaderImageType, clamp(in.CurrentStep, 1, 5)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("не удалось создать компанию")
	}
	if err = h.update(ctx, id, u.ID, in); err != nil {
		return 0, err
	}
	return id, nil
}

func (h *Handler) update(ctx context.Context, id, owner int64, in CompanyInput) error {
	if err := validate(in); err != nil {
		return err
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	advantages, _ := json.Marshal(limitStrings(in.Advantages, 8, 180))
	res, err := tx.ExecContext(ctx, `UPDATE accounting_companies SET name=$3,short_description=$4,full_description=$5,logo=$6,city=$7,address=$8,remote_all_russia=$9,founded_year=$10,employee_count=$11,inn=$12,phone=$13,email=$14,website=$15,telegram=$16,whatsapp=$17,vk=$18,work_hours=$19,manager_name=$20,manager_position=$21,manager_photo=$22,manager_description=$23,manager_user_id=$24,accent_style_id=$25,header_image_type=$26,header_template_id=$27,custom_header_image=$28,advantages=$29,current_step=$30,updated_at=NOW() WHERE id=$1 AND owner_user_id=$2 AND deleted_at IS NULL`, id, owner, clean(in.Name, 240), clean(in.ShortDescription, 500), clean(in.FullDescription, 12000), in.Logo, clean(in.City, 180), clean(in.Address, 500), in.RemoteAllRussia, in.FoundedYear, in.EmployeeCount, strings.TrimSpace(in.INN), clean(in.Phone, 80), clean(in.Email, 254), clean(in.Website, 500), clean(in.Telegram, 500), clean(in.Whatsapp, 500), clean(in.VK, 500), clean(in.WorkHours, 180), clean(in.ManagerName, 240), clean(in.ManagerPosition, 180), in.ManagerPhoto, clean(in.ManagerDescription, 700), in.ManagerUserID, in.AccentStyleID, in.HeaderImageType, in.HeaderTemplateID, in.CustomHeaderImage, advantages, clamp(in.CurrentStep, 1, 5))
	if err != nil {
		return fmt.Errorf("не удалось сохранить компанию")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("недостаточно прав")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM accounting_company_direction_links WHERE company_id=$1`, id); err != nil {
		return err
	}
	keys := idSet(in.KeyDirectionIDs)
	for i, directionID := range uniqueIDs(in.DirectionIDs, 10) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO accounting_company_direction_links(company_id,direction_id,is_key,sort_order) SELECT $1,id,$3,$4 FROM accounting_company_directions WHERE id=$2 AND active AND deleted_at IS NULL`, id, directionID, keys[directionID], i); err != nil {
			return fmt.Errorf("некорректное направление")
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM accounting_company_tax_system_links WHERE company_id=$1`, id); err != nil {
		return err
	}
	for _, taxID := range uniqueIDs(in.TaxSystemIDs, 12) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO accounting_company_tax_system_links(company_id,tax_system_id) SELECT $1,id FROM accounting_company_tax_systems WHERE id=$2 AND active`, id, taxID); err != nil {
			return fmt.Errorf("некорректная система налогообложения")
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM accounting_company_services WHERE company_id=$1`, id); err != nil {
		return err
	}
	for i, s := range in.Services {
		if i >= 30 {
			break
		}
		if s.PriceFrom != nil && *s.PriceFrom < 0 {
			return fmt.Errorf("цена услуги не может быть отрицательной")
		}
		pt := s.PriceType
		if !allowedPriceType(pt) {
			pt = "from_month"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO accounting_company_services(company_id,service_id,custom_name,price_from,price_type,sort_order) SELECT $1,id,$3,$4,$5,$6 FROM accounting_company_service_catalog WHERE id=$2 AND active AND deleted_at IS NULL`, id, s.ServiceID, clean(s.CustomName, 220), s.PriceFrom, pt, i)
		if err != nil {
			return fmt.Errorf("некорректная услуга")
		}
	}
	if len(in.Tariffs) > 5 {
		return fmt.Errorf("можно добавить не более 5 тарифов")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM accounting_company_tariffs WHERE company_id=$1`, id); err != nil {
		return err
	}
	for i, t := range in.Tariffs {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		if t.Price != nil && *t.Price < 0 {
			return fmt.Errorf("цена тарифа не может быть отрицательной")
		}
		benefits, _ := json.Marshal(limitStrings(t.Benefits, 12, 180))
		_, err = tx.ExecContext(ctx, `INSERT INTO accounting_company_tariffs(company_id,name,subtitle,price,period,benefits,sort_order,popular,active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, clean(t.Name, 160), clean(t.Subtitle, 300), t.Price, clean(t.Period, 80), benefits, i, t.Popular, t.Active)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func validate(in CompanyInput) error {
	if len([]rune(strings.TrimSpace(in.Name))) < 2 {
		return fmt.Errorf("укажите название компании")
	}
	if in.FoundedYear != nil && (*in.FoundedYear < 1900 || *in.FoundedYear > time.Now().Year()) {
		return fmt.Errorf("проверьте год начала работы")
	}
	if in.EmployeeCount != nil && *in.EmployeeCount < 0 {
		return fmt.Errorf("количество сотрудников не может быть отрицательным")
	}
	inn := strings.TrimSpace(in.INN)
	if inn != "" && !regexp.MustCompile(`^(\d{10}|\d{12})$`).MatchString(inn) {
		return fmt.Errorf("ИНН должен содержать 10 или 12 цифр")
	}
	if len(uniqueIDs(in.DirectionIDs, 100)) > 10 {
		return fmt.Errorf("можно выбрать не более 10 направлений")
	}
	if len(uniqueIDs(in.KeyDirectionIDs, 100)) > 3 {
		return fmt.Errorf("можно отметить не более 3 ключевых направлений")
	}
	if in.HeaderImageType != "template" && in.HeaderImageType != "custom" {
		return fmt.Errorf("некорректный тип изображения")
	}
	return nil
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request, id int64, status string) {
	if r.Method != http.MethodPost {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	newSlug := ""
	if status == "published" {
		var name string
		var services, directions, tariffs int
		err := h.db.QueryRowContext(r.Context(), `SELECT name,(SELECT count(*) FROM accounting_company_services WHERE company_id=c.id),(SELECT count(*) FROM accounting_company_direction_links WHERE company_id=c.id),(SELECT count(*) FROM accounting_company_tariffs WHERE company_id=c.id AND active) FROM accounting_companies c WHERE id=$1 AND owner_user_id=$2 AND deleted_at IS NULL`, id, u.ID).Scan(&name, &services, &directions, &tariffs)
		if err != nil {
			failure(w, 403, "Недостаточно прав")
			return
		}
		if len(strings.TrimSpace(name)) < 2 || services < 1 || directions < 1 || tariffs < 1 {
			failure(w, 400, "Для публикации добавьте название, направление, услугу и тариф")
			return
		}
		newSlug = slugify(name)
		if newSlug == "" {
			newSlug = "company"
		}
		newSlug += "-" + strconv.FormatInt(id, 10)
	}
	res, err := h.db.ExecContext(r.Context(), `UPDATE accounting_companies SET status=$3::varchar,slug=CASE WHEN $3::varchar='published' THEN $4 ELSE slug END,published_at=CASE WHEN $3::varchar='published' THEN COALESCE(published_at,NOW()) ELSE published_at END,updated_at=NOW() WHERE id=$1 AND owner_user_id=$2 AND deleted_at IS NULL`, id, u.ID, status, newSlug)
	if err != nil {
		failure(w, 500, "Не удалось изменить статус")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		failure(w, 403, "Недостаточно прав")
		return
	}
	h.sendCompany(w, r, id, u.ID, true)
}

func slugify(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	mapRunes := map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ы': "y", 'э': "e", 'ю': "yu", 'я': "ya"}
	var b strings.Builder
	dash := false
	for _, r := range v {
		if x, ok := mapRunes[r]; ok {
			b.WriteString(x)
			dash = false
		} else if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
func clean(v string, n int) string {
	r := []rune(strings.TrimSpace(v))
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func uniqueIDs(ids []int64, max int) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
			if len(out) >= max {
				break
			}
		}
	}
	return out
}
func idSet(ids []int64) map[int64]bool {
	m := map[int64]bool{}
	for _, id := range uniqueIDs(ids, 3) {
		m[id] = true
	}
	return m
}
func limitStrings(in []string, maxItems, maxLen int) []string {
	out := []string{}
	for _, v := range in {
		v = clean(v, maxLen)
		if v != "" {
			out = append(out, v)
		}
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
func allowedPriceType(v string) bool {
	for _, x := range []string{"from_month", "month", "from_hour", "hour", "from_once", "request"} {
		if v == x {
			return true
		}
	}
	return false
}

func (h *Handler) sendCompany(w http.ResponseWriter, r *http.Request, id, userID int64, forceOwner bool) {
	raw, owner, status, err := h.companyJSON(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			failure(w, 404, "Компания не найдена")
		} else {
			failure(w, 500, "Не удалось загрузить компанию")
		}
		return
	}
	isOwner := userID > 0 && userID == owner
	if status != "published" && !isOwner && !h.admin(r) {
		failure(w, 404, "Компания не найдена")
		return
	}
	if forceOwner && !isOwner && !h.admin(r) {
		failure(w, 403, "Недостаточно прав")
		return
	}
	response(w, 200, map[string]any{"company": json.RawMessage(raw), "is_owner": isOwner})
}

func (h *Handler) companyJSON(ctx context.Context, id, userID int64) ([]byte, int64, string, error) {
	var raw []byte
	var owner int64
	var status string
	err := h.db.QueryRowContext(ctx, `SELECT c.owner_user_id,c.status,jsonb_build_object(
	'id',c.id,'owner_user_id',c.owner_user_id,'name',c.name,'slug',c.slug,'short_description',c.short_description,'full_description',c.full_description,
	'logo',c.logo,'city',c.city,'address',c.address,'remote_all_russia',c.remote_all_russia,'founded_year',c.founded_year,'employee_count',c.employee_count,
	'inn',CASE WHEN c.owner_user_id=$2 THEN c.inn ELSE '' END,'phone',c.phone,'email',c.email,'website',c.website,'telegram',c.telegram,'whatsapp',c.whatsapp,'vk',c.vk,'work_hours',c.work_hours,
	'manager_name',c.manager_name,'manager_position',c.manager_position,'manager_photo',c.manager_photo,'manager_description',c.manager_description,'manager_user_id',c.manager_user_id,
	'accent',CASE WHEN a.id IS NULL THEN NULL ELSE jsonb_build_object('id',a.id,'name',a.name,'color_key',a.color_key,'color_value',a.color_value) END,
	'accent_style_id',c.accent_style_id,'header_image_type',c.header_image_type,'header_template_id',c.header_template_id,'custom_header_image',c.custom_header_image,
	'header_image',CASE WHEN c.header_image_type='custom' AND c.custom_header_image<>'' THEN c.custom_header_image ELSE ht.image_url END,
	'advantages',c.advantages,'status',c.status,'verified',c.verified,'current_step',c.current_step,'published_at',c.published_at,'created_at',c.created_at,'updated_at',c.updated_at,
	'directions',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'name',d.name,'slug',d.slug,'icon',d.icon,'is_key',l.is_key) ORDER BY l.sort_order,d.sort_order) FROM accounting_company_direction_links l JOIN accounting_company_directions d ON d.id=l.direction_id WHERE l.company_id=c.id),'[]'::jsonb),
	'direction_ids',COALESCE((SELECT jsonb_agg(direction_id ORDER BY sort_order) FROM accounting_company_direction_links WHERE company_id=c.id),'[]'::jsonb),
	'key_direction_ids',COALESCE((SELECT jsonb_agg(direction_id ORDER BY sort_order) FROM accounting_company_direction_links WHERE company_id=c.id AND is_key),'[]'::jsonb),
	'tax_systems',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'name',t.name,'slug',t.slug) ORDER BY t.sort_order) FROM accounting_company_tax_system_links l JOIN accounting_company_tax_systems t ON t.id=l.tax_system_id WHERE l.company_id=c.id),'[]'::jsonb),
	'tax_system_ids',COALESCE((SELECT jsonb_agg(tax_system_id) FROM accounting_company_tax_system_links WHERE company_id=c.id),'[]'::jsonb),
	'services',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',s.id,'service_id',s.service_id,'name',COALESCE(NULLIF(s.custom_name,''),sc.name),'custom_name',s.custom_name,'icon',sc.icon,'price_from',s.price_from,'price_type',s.price_type,'sort_order',s.sort_order) ORDER BY s.sort_order,s.id) FROM accounting_company_services s LEFT JOIN accounting_company_service_catalog sc ON sc.id=s.service_id WHERE s.company_id=c.id),'[]'::jsonb),
	'tariffs',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'name',t.name,'subtitle',t.subtitle,'price',t.price,'period',t.period,'benefits',t.benefits,'sort_order',t.sort_order,'popular',t.popular,'active',t.active) ORDER BY t.sort_order,t.id) FROM accounting_company_tariffs t WHERE t.company_id=c.id AND (t.active OR c.owner_user_id=$2)),'[]'::jsonb),
	'reviews',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',rv.id,'author_name',rv.author_name,'author_company',rv.author_company,'text',rv.text,'rating',rv.rating,'created_at',rv.created_at) ORDER BY rv.created_at DESC) FROM accounting_company_reviews rv WHERE rv.company_id=c.id AND rv.status='published'),'[]'::jsonb),
	'passport_summary',(SELECT CASE WHEN count(*)=0 THEN NULL ELSE jsonb_build_object('overall_index',round(avg(ta.percent),0),'tests_count',count(*),'specialists_count',count(DISTINCT i.employee_id)) END FROM company_test_invitations i JOIN test_attempts ta ON ta.id=i.attempt_id WHERE i.owner_user_id=c.owner_user_id AND i.status='finished' AND ta.status='finished'),
	'completeness',LEAST(100,(CASE WHEN c.name<>'' THEN 12 ELSE 0 END)+(CASE WHEN c.logo<>'' THEN 12 ELSE 0 END)+(CASE WHEN c.phone<>'' OR c.email<>'' THEN 12 ELSE 0 END)+(CASE WHEN c.short_description<>'' THEN 10 ELSE 0 END)+(CASE WHEN c.manager_name<>'' THEN 10 ELSE 0 END)+(CASE WHEN EXISTS(SELECT 1 FROM accounting_company_direction_links WHERE company_id=c.id) THEN 14 ELSE 0 END)+(CASE WHEN EXISTS(SELECT 1 FROM accounting_company_services WHERE company_id=c.id) THEN 15 ELSE 0 END)+(CASE WHEN EXISTS(SELECT 1 FROM accounting_company_tariffs WHERE company_id=c.id) THEN 15 ELSE 0 END))
	) FROM accounting_companies c LEFT JOIN accounting_company_accent_styles a ON a.id=c.accent_style_id LEFT JOIN accounting_company_header_templates ht ON ht.id=c.header_template_id WHERE c.id=$1 AND c.deleted_at IS NULL`, id, userID).Scan(&owner, &status, &raw)
	return raw, owner, status, err
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseInt(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	limit := parseInt(q.Get("limit"), 12)
	if limit < 1 {
		limit = 12
	}
	if limit > 48 {
		limit = 48
	}
	where := []string{"c.status='published'", "c.deleted_at IS NULL"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		args = append(args, v)
		n := len(args)
		where = append(where, fmt.Sprintf("(c.name ILIKE '%%'||$%d||'%%' OR c.short_description ILIKE '%%'||$%d||'%%' OR c.city ILIKE '%%'||$%d||'%%')", n, n, n))
	}
	if v := strings.TrimSpace(q.Get("city")); v != "" {
		add("c.city=$%d", v)
	}
	if v := parseInt64(q.Get("direction_id")); v > 0 {
		add("EXISTS(SELECT 1 FROM accounting_company_direction_links dl WHERE dl.company_id=c.id AND dl.direction_id=$%d)", v)
	}
	if v := parseInt64(q.Get("service_id")); v > 0 {
		add("EXISTS(SELECT 1 FROM accounting_company_services cs WHERE cs.company_id=c.id AND cs.service_id=$%d)", v)
	}
	if v := parseInt64(q.Get("tax_system_id")); v > 0 {
		add("EXISTS(SELECT 1 FROM accounting_company_tax_system_links tl WHERE tl.company_id=c.id AND tl.tax_system_id=$%d)", v)
	}
	if q.Get("online") == "true" {
		where = append(where, "c.remote_all_russia=TRUE")
	}
	if v := parseFloat64(q.Get("price_to")); v != nil {
		add("EXISTS(SELECT 1 FROM accounting_company_services cs WHERE cs.company_id=c.id AND cs.price_from<=$%d)", *v)
	}
	if q.Get("passport") == "true" {
		where = append(where, "(EXISTS(SELECT 1 FROM accounting_company_competency_scores cs WHERE cs.company_id=c.id) OR EXISTS(SELECT 1 FROM company_test_invitations i WHERE i.owner_user_id=c.owner_user_id AND i.status='finished'))")
	}
	from := " FROM accounting_companies c WHERE " + strings.Join(where, " AND ")
	var total int
	if err := h.db.QueryRowContext(r.Context(), "SELECT count(*)"+from, args...).Scan(&total); err != nil {
		failure(w, 500, "Не удалось загрузить каталог")
		return
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := h.db.QueryContext(r.Context(), "SELECT c.id"+from+fmt.Sprintf(" ORDER BY c.verified DESC,c.published_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		failure(w, 500, "Не удалось загрузить каталог")
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			if raw, _, _, e := h.companyJSON(r.Context(), id, 0); e == nil {
				items = append(items, raw)
			}
		}
	}
	response(w, 200, map[string]any{"items": items, "page": page, "limit": limit, "total": total, "pages": maxInt(1, (total+limit-1)/limit)})
}

func (h *Handler) passport(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodGet {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	var owner int64
	var status string
	if err := h.db.QueryRowContext(r.Context(), `SELECT owner_user_id,status FROM accounting_companies WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&owner, &status); err != nil {
		failure(w, 404, "Компания не найдена")
		return
	}
	u := h.optionalUser(r)
	if status != "published" && u.ID != owner && !h.admin(r) {
		failure(w, 404, "Компания не найдена")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `WITH results AS (
	 SELECT v.title competency,a.percent,e.id specialist_id,i.finished_at,a.id attempt_id,e.full_name specialist_name
	 FROM company_test_invitations i JOIN company_test_employees e ON e.id=i.employee_id JOIN test_attempts a ON a.id=i.attempt_id JOIN test_versions v ON v.id=a.test_version_id
	 WHERE i.owner_user_id=$1 AND i.status='finished' AND a.status='finished'
	 UNION ALL
	 SELECT v.title,a.percent,a.user_id,a.finished_at,a.id,u.full_name
	 FROM test_attempts a JOIN test_versions v ON v.id=a.test_version_id JOIN users u ON u.id=a.user_id
	 WHERE a.status='finished' AND (a.user_id=$1 OR a.user_id IN (SELECT user_id FROM accounting_company_team WHERE company_id=$2 AND active AND user_id IS NOT NULL))
	), grouped AS (SELECT competency,round(avg(percent),2) score,count(DISTINCT specialist_id) specialists,count(*) tests,max(finished_at) last_at FROM results GROUP BY competency)
	SELECT competency,score,specialists,tests,last_at FROM grouped ORDER BY score DESC,competency`, owner, id)
	if err != nil {
		failure(w, 500, "Не удалось сформировать Паспорт компетенций")
		return
	}
	defer rows.Close()
	type score struct {
		Name        string     `json:"name"`
		Percent     float64    `json:"percent"`
		Specialists int        `json:"specialists_count"`
		Tests       int        `json:"tests_count"`
		LastAt      *time.Time `json:"last_confirmed_at"`
	}
	scores := []score{}
	totalTests := 0
	specialistsMax := 0
	sum := 0.0
	for rows.Next() {
		var s score
		var last sql.NullTime
		if rows.Scan(&s.Name, &s.Percent, &s.Specialists, &s.Tests, &last) == nil {
			if last.Valid {
				s.LastAt = &last.Time
			}
			scores = append(scores, s)
			totalTests += s.Tests
			if s.Specialists > specialistsMax {
				specialistsMax = s.Specialists
			}
			sum += s.Percent
		}
	}
	index := 0.0
	if len(scores) > 0 {
		index = float64(int(sum/float64(len(scores))*100)) / 100
	}
	var history []json.RawMessage
	historyRows, e := h.db.QueryContext(r.Context(), `SELECT jsonb_build_object('test_title',v.title,'specialist_name',e.full_name,'percent',a.percent,'finished_at',a.finished_at,'passed',a.passed) FROM company_test_invitations i JOIN company_test_employees e ON e.id=i.employee_id JOIN test_attempts a ON a.id=i.attempt_id JOIN test_versions v ON v.id=a.test_version_id WHERE i.owner_user_id=$1 AND i.status='finished' ORDER BY a.finished_at DESC LIMIT 50`, owner)
	if e == nil {
		defer historyRows.Close()
		history = []json.RawMessage{}
		for historyRows.Next() {
			var raw []byte
			if historyRows.Scan(&raw) == nil {
				history = append(history, raw)
			}
		}
	}
	response(w, 200, map[string]any{"company_id": id, "overall_index": index, "confirmed_competencies": len(scores), "tests_count": totalTests, "specialists_count": specialistsMax, "scores": scores, "history": history})
}

func (h *Handler) reviews(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		failure(w, 405, "Метод не поддерживается")
		return
	}
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	var in struct {
		Text          string `json:"text"`
		Rating        int    `json:"rating"`
		AuthorCompany string `json:"author_company"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Rating < 1 || in.Rating > 5 || len([]rune(strings.TrimSpace(in.Text))) < 10 {
		failure(w, 400, "Добавьте текст отзыва и оценку от 1 до 5")
		return
	}
	var owner int64
	if h.db.QueryRowContext(r.Context(), `SELECT owner_user_id FROM accounting_companies WHERE id=$1 AND status='published' AND deleted_at IS NULL`, id).Scan(&owner) != nil {
		failure(w, 404, "Компания не найдена")
		return
	}
	if owner == u.ID {
		failure(w, 400, "Нельзя оставить отзыв своей компании")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO accounting_company_reviews(company_id,author_user_id,author_name,author_company,text,rating) VALUES($1,$2,$3,$4,$5,$6)`, id, u.ID, clean(u.FullName, 180), clean(in.AuthorCompany, 220), clean(in.Text, 4000), in.Rating)
	if err != nil {
		failure(w, 500, "Не удалось отправить отзыв")
		return
	}
	response(w, 201, map[string]any{"ok": true, "message": "Отзыв отправлен на модерацию"})
}

func parseInt(v string, fallback int) int {
	n, e := strconv.Atoi(v)
	if e != nil {
		return fallback
	}
	return n
}
func parseInt64(v string) int64 { n, _ := strconv.ParseInt(v, 10, 64); return n }
func parseFloat64(v string) *float64 {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	n, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return nil
	}
	return &n
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
