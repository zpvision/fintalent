package clientexchange

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
)

type UserResolver func(*http.Request) (UserIdentity, error)
type AdminResolver func(*http.Request) bool

type Handler struct {
	db    *sql.DB
	user  UserResolver
	admin AdminResolver
}

func New(db *sql.DB, user UserResolver, admin AdminResolver) *Handler {
	return &Handler{db: db, user: user, admin: admin}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/client-exchange/meta", h.meta)
	mux.HandleFunc("/api/client-exchange/listings", h.listings)
	mux.HandleFunc("/api/client-exchange/listings/", h.listingRoute)
	mux.HandleFunc("/api/client-exchange/responses/", h.responseRoute)
	mux.HandleFunc("/api/client-exchange/my/listings", h.myListings)
	mux.HandleFunc("/api/client-exchange/my/responses", h.myResponses)
	mux.HandleFunc("/api/client-exchange/my/received", h.receivedResponses)
	mux.HandleFunc("/api/client-exchange/my/favorites", h.myFavorites)
	mux.HandleFunc("/api/client-exchange/notifications", h.notifications)
	mux.HandleFunc("/api/admin/client-exchange/dictionaries", h.adminDictionaries)
	mux.HandleFunc("/api/admin/client-exchange/dictionaries/", h.adminDictionary)
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		fail(w, http.StatusBadRequest, "Некорректные данные формы")
		return false
	}
	return true
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) (UserIdentity, bool) {
	u, err := h.user(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "Требуется авторизация")
		return UserIdentity{}, false
	}
	return u, true
}

func (h *Handler) meta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if _, ok := h.current(w, r); !ok {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,kind,code,name,description,min_value,max_value,color,icon,legal_name,operator_code,sort_order,active FROM client_exchange_dictionary_items WHERE active AND deleted_at IS NULL ORDER BY kind,sort_order,name`)
	if err != nil {
		fail(w, 500, "Не удалось загрузить справочники")
		return
	}
	defer rows.Close()
	result := map[string][]DictionaryItem{}
	for rows.Next() {
		var x DictionaryItem
		var min, max sql.NullFloat64
		if err = rows.Scan(&x.ID, &x.Kind, &x.Code, &x.Name, &x.Description, &min, &max, &x.Color, &x.Icon, &x.LegalName, &x.OperatorCode, &x.SortOrder, &x.Active); err != nil {
			fail(w, 500, "Не удалось загрузить справочники")
			return
		}
		if min.Valid {
			x.MinValue = &min.Float64
		}
		if max.Valid {
			x.MaxValue = &max.Float64
		}
		result[x.Kind] = append(result[x.Kind], x)
	}
	var active, added, transferred, companies int
	var average sql.NullFloat64
	_ = h.db.QueryRowContext(r.Context(), `SELECT
		COUNT(*) FILTER(WHERE status IN ('active','has_responses')),
		COUNT(*) FILTER(WHERE published_at >= date_trunc('month',NOW())),
		COUNT(*) FILTER(WHERE status='transferred'),
		COUNT(DISTINCT seller_user_id) FILTER(WHERE status<>'draft'),
		AVG(transfer_price) FILTER(WHERE status IN ('active','has_responses') AND transfer_price IS NOT NULL)
		FROM client_exchange_listings WHERE deleted_at IS NULL`).Scan(&active, &added, &transferred, &companies, &average)
	stats := map[string]any{"active": active, "added_month": added, "transferred": transferred, "companies": companies, "average_price": nil}
	if average.Valid {
		stats["average_price"] = average.Float64
	}
	respond(w, 200, map[string]any{"dictionaries": result, "statuses": statusLabels(), "stats": stats})
}

func statusLabels() map[string]string {
	return map[string]string{"draft": "Черновик", "active": "Активно", "has_responses": "Есть предложения", "buyer_selected": "Компания выбрана", "transfer_in_progress": "Передача", "transferred": "Передан", "archived": "Архив", "cancelled": "Снято"}
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (h *Handler) listings(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.catalog(w, r, u)
	case http.MethodPost:
		var input ListingInput
		if !decode(w, r, &input) {
			return
		}
		id, err := h.createListing(r.Context(), u.ID, input)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		data, err := h.getListingJSON(r.Context(), id, u.ID, false)
		if err != nil {
			fail(w, 500, "Не удалось открыть объявление")
			return
		}
		respond(w, 201, json.RawMessage(data))
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request, u UserIdentity) {
	q := r.URL.Query()
	page := clamp(parseInt(q.Get("page"), 1), 1, 100000)
	limit := clamp(parseInt(q.Get("limit"), 12), 1, 48)
	where := []string{"l.deleted_at IS NULL", "l.status IN ('active','has_responses')"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		args = append(args, v)
		n := len(args)
		where = append(where, fmt.Sprintf("(l.title ILIKE '%%'||$%d||'%%' OR i.name ILIKE '%%'||$%d||'%%' OR l.city ILIKE '%%'||$%d||'%%')", n, n, n))
	}
	if v := strings.TrimSpace(q.Get("region")); v != "" {
		add("l.region=$%d", v)
	}
	if v := parseInt64(q.Get("industry_id")); v > 0 {
		args = append(args, v)
		n := len(args)
		where = append(where, fmt.Sprintf("(l.industry_id=$%d OR EXISTS(SELECT 1 FROM client_exchange_listing_options xo WHERE xo.listing_id=l.id AND xo.kind='industry' AND xo.item_id=$%d))", n, n))
	}
	for _, f := range []struct{ name, col string }{{"tax_system_id", "l.tax_system_id"}, {"revenue_range_id", "l.revenue_range_id"}, {"employee_range_id", "l.employee_range_id"}, {"transfer_type_id", "l.transfer_type_id"}, {"accounting_state_id", "l.accounting_state_id"}} {
		if v := parseInt64(q.Get(f.name)); v > 0 {
			add(f.col+"=$%d", v)
		}
	}
	if v := q.Get("foreign_trade"); v != "" {
		add("l.foreign_trade=$%d", v == "true")
	}
	if v := parseInt64(q.Get("marketplace_id")); v > 0 {
		add("EXISTS(SELECT 1 FROM client_exchange_listing_options xo WHERE xo.listing_id=l.id AND xo.kind='marketplace' AND xo.item_id=$%d)", v)
	}
	if v := parseInt64(q.Get("edo_provider_id")); v > 0 {
		add("EXISTS(SELECT 1 FROM client_exchange_listing_options xo WHERE xo.listing_id=l.id AND xo.kind='edo_provider' AND xo.item_id=$%d)", v)
	}
	if v := parseFloat(q.Get("fee_from")); v != nil {
		add("l.current_monthly_fee >= $%d", *v)
	}
	if v := parseFloat(q.Get("fee_to")); v != nil {
		add("l.current_monthly_fee <= $%d", *v)
	}
	if v := parseFloat(q.Get("price_from")); v != nil {
		add("l.transfer_price >= $%d", *v)
	}
	if v := parseFloat(q.Get("price_to")); v != nil {
		add("l.transfer_price <= $%d", *v)
	}
	order := map[string]string{"new": "l.published_at DESC", "price_asc": "l.transfer_price ASC NULLS LAST", "price_desc": "l.transfer_price DESC NULLS LAST", "fee_desc": "l.current_monthly_fee DESC NULLS LAST", "revenue_desc": "rr.sort_order DESC"}[q.Get("sort")]
	if order == "" {
		order = "l.published_at DESC"
	}
	from := ` FROM client_exchange_listings l LEFT JOIN client_exchange_dictionary_items i ON i.id=l.industry_id LEFT JOIN client_exchange_dictionary_items rr ON rr.id=l.revenue_range_id WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+from, args...).Scan(&total); err != nil {
		fail(w, 500, "Не удалось загрузить каталог")
		return
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := h.db.QueryContext(r.Context(), `SELECT l.id FROM client_exchange_listings l LEFT JOIN client_exchange_dictionary_items i ON i.id=l.industry_id LEFT JOIN client_exchange_dictionary_items rr ON rr.id=l.revenue_range_id WHERE `+strings.Join(where, " AND ")+fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", order, len(args)-1, len(args)), args...)
	if err != nil {
		fail(w, 500, "Не удалось загрузить каталог")
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			if b, e := h.getListingJSON(r.Context(), id, u.ID, false); e == nil {
				items = append(items, b)
			}
		}
	}
	respond(w, 200, map[string]any{"items": items, "page": page, "limit": limit, "total": total, "pages": max(1, (total+limit-1)/limit)})
}

func (h *Handler) createListing(ctx context.Context, userID int64, input ListingInput) (int64, error) {
	normalizeIndustryIDs(&input)
	normalizeTransferReasonIDs(&input)
	if err := h.validateInput(ctx, input, false); err != nil {
		return 0, err
	}
	var id int64
	err := h.db.QueryRowContext(ctx, `INSERT INTO client_exchange_listings(seller_user_id,title,client_inn,client_legal_name,industry_id,employee_range_id,tax_system_id,revenue_range_id,accounting_state_id,transfer_reason_id,transfer_type_id,transfer_reason_comment,transfer_price,monthly_commission_percent,commission_months,current_monthly_fee,operations_per_month,banks_count,has_vat,foreign_trade,bargain_allowed,region,city,client_since,desired_transfer_date,comment,current_step) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27) RETURNING id`, userID, clean(input.Title, 240), strings.TrimSpace(input.ClientINN), clean(input.ClientLegalName, 500), input.IndustryID, input.EmployeeRangeID, input.TaxSystemID, input.RevenueRangeID, input.AccountingStateID, input.TransferReasonID, input.TransferTypeID, clean(input.TransferReasonComment, 2000), input.TransferPrice, input.MonthlyCommission, input.CommissionMonths, input.CurrentMonthlyFee, input.OperationsPerMonth, input.BanksCount, input.HasVAT, input.ForeignTrade, input.BargainAllowed, clean(input.Region, 200), clean(input.City, 200), nullableDate(input.ClientSince), nullableDate(input.DesiredTransferDate), clean(input.Comment, 5000), clamp(input.CurrentStep, 1, 6)).Scan(&id)
	if err != nil {
		return 0, errors.New("не удалось создать объявление")
	}
	if err = h.saveOptions(ctx, id, input); err != nil {
		return 0, err
	}
	return id, nil
}

func (h *Handler) validateInput(ctx context.Context, in ListingInput, publishing bool) error {
	normalizeIndustryIDs(&in)
	normalizeTransferReasonIDs(&in)
	if in.ClientINN != "" && !validINN(in.ClientINN) {
		return errors.New("ИНН должен содержать 10 или 12 цифр")
	}
	if in.TransferPrice != nil && *in.TransferPrice < 0 {
		return errors.New("Цена не может быть отрицательной")
	}
	if in.CurrentMonthlyFee != nil && *in.CurrentMonthlyFee < 0 {
		return errors.New("Абонплата не может быть отрицательной")
	}
	if in.MonthlyCommission != nil && (*in.MonthlyCommission < 0 || *in.MonthlyCommission > 100) {
		return errors.New("Комиссия должна быть от 0 до 100%")
	}
	if publishing {
		if len(in.IndustryIDs) == 0 || in.EmployeeRangeID == nil || in.TaxSystemID == nil || in.RevenueRangeID == nil || in.AccountingStateID == nil || in.TransferReasonID == nil || in.TransferTypeID == nil || strings.TrimSpace(in.City) == "" {
			return errors.New("Заполните обязательные поля всех шагов")
		}
	}
	ids := []struct {
		id   *int64
		kind string
	}{{in.IndustryID, "industry"}, {in.EmployeeRangeID, "employee_range"}, {in.TaxSystemID, "tax_system"}, {in.RevenueRangeID, "revenue_range"}, {in.AccountingStateID, "accounting_state"}, {in.TransferReasonID, "transfer_reason"}, {in.TransferTypeID, "transfer_type"}}
	for _, x := range ids {
		if x.id != nil {
			var ok bool
			if err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_exchange_dictionary_items WHERE id=$1 AND kind=$2 AND active AND deleted_at IS NULL)`, *x.id, x.kind).Scan(&ok); err != nil || !ok {
				return fmt.Errorf("некорректное значение справочника %s", x.kind)
			}
		}
	}
	for _, id := range in.IndustryIDs {
		var ok bool
		if err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_exchange_dictionary_items WHERE id=$1 AND kind='industry' AND active AND deleted_at IS NULL)`, id).Scan(&ok); err != nil || !ok {
			return errors.New("invalid industry")
		}
	}
	for _, id := range in.TransferReasonIDs {
		var ok bool
		if err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_exchange_dictionary_items WHERE id=$1 AND kind='transfer_reason' AND active AND deleted_at IS NULL)`, id).Scan(&ok); err != nil || !ok {
			return errors.New("invalid transfer reason")
		}
	}
	return nil
}

func (h *Handler) saveOptions(ctx context.Context, listingID int64, in ListingInput) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM client_exchange_listing_options WHERE listing_id=$1`, listingID); err != nil {
		return err
	}
	normalizeIndustryIDs(&in)
	normalizeTransferReasonIDs(&in)
	sets := []struct {
		kind string
		ids  []int64
	}{{"industry", in.IndustryIDs}, {"marketplace", in.MarketplaceIDs}, {"edo_provider", in.EDOProviderIDs}, {"accounting_program", in.AccountingProgramIDs}, {"transfer_reason", in.TransferReasonIDs}}
	for _, set := range sets {
		for _, id := range set.ids {
			var ok bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_exchange_dictionary_items WHERE id=$1 AND kind=$2 AND active AND deleted_at IS NULL)`, id, set.kind).Scan(&ok); err != nil || !ok {
				return errors.New("Выбран недоступный вариант")
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO client_exchange_listing_options(listing_id,item_id,kind) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, listingID, id, set.kind); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func normalizeTransferReasonIDs(in *ListingInput) {
	seen := map[int64]bool{}
	ids := []int64{}
	if in.TransferReasonID != nil && *in.TransferReasonID > 0 {
		seen[*in.TransferReasonID] = true
		ids = append(ids, *in.TransferReasonID)
	}
	for _, id := range in.TransferReasonIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	in.TransferReasonIDs = ids
	if len(ids) > 0 {
		in.TransferReasonID = &in.TransferReasonIDs[0]
	}
}

func normalizeIndustryIDs(in *ListingInput) {
	seen := map[int64]bool{}
	ids := []int64{}
	if in.IndustryID != nil && *in.IndustryID > 0 {
		seen[*in.IndustryID] = true
		ids = append(ids, *in.IndustryID)
	}
	for _, id := range in.IndustryIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	in.IndustryIDs = ids
	if len(ids) > 0 {
		first := ids[0]
		in.IndustryID = &first
		return
	}
	in.IndustryID = nil
}

func parseInt(v string, d int) int {
	n, e := strconv.Atoi(v)
	if e != nil {
		return d
	}
	return n
}
func parseInt64(v string) int64 { n, _ := strconv.ParseInt(v, 10, 64); return n }
func parseFloat(v string) *float64 {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	n, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return nil
	}
	return &n
}
func clean(v string, n int) string {
	v = strings.TrimSpace(v)
	r := []rune(v)
	if len(r) > n {
		v = string(r[:n])
	}
	return v
}
func nullableDate(v *string) any {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return strings.TrimSpace(*v)
}

var innPattern = regexp.MustCompile(`^(\d{10}|\d{12})$`)

func validINN(v string) bool { return innPattern.MatchString(strings.TrimSpace(v)) }
