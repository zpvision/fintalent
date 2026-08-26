package clientexchange

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,99}$`)

func validKind(kind string) bool {
	for _, x := range DictionaryKinds {
		if x == kind {
			return true
		}
	}
	return false
}

func (h *Handler) adminDictionaries(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход в админку")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if r.Method == http.MethodPost {
		var x DictionaryItem
		if !decode(w, r, &x) {
			return
		}
		if !validKind(x.Kind) || !codePattern.MatchString(x.Code) || strings.TrimSpace(x.Name) == "" {
			fail(w, 400, "Проверьте тип, code и название")
			return
		}
		if x.Color == "" {
			x.Color = "blue"
		}
		err := h.db.QueryRowContext(r.Context(), `INSERT INTO client_exchange_dictionary_items(kind,code,name,description,min_value,max_value,color,legal_name,operator_code,sort_order,active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, x.Kind, x.Code, clean(x.Name, 300), clean(x.Description, 3000), x.MinValue, x.MaxValue, clean(x.Color, 30), clean(x.LegalName, 300), clean(x.OperatorCode, 100), x.SortOrder, x.Active).Scan(&x.ID)
		if err != nil {
			fail(w, 409, "Такой code уже существует в этом справочнике")
			return
		}
		respond(w, 201, x)
		return
	}
	kind := r.URL.Query().Get("kind")
	args := []any{}
	where := "deleted_at IS NULL"
	if kind != "" {
		if !validKind(kind) {
			fail(w, 400, "Неизвестный справочник")
			return
		}
		args = append(args, kind)
		where += " AND kind=$1"
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT d.id,d.kind,d.code,d.name,d.description,d.min_value,d.max_value,d.color,d.legal_name,d.operator_code,d.sort_order,d.active,
		EXISTS(SELECT 1 FROM client_exchange_listings l WHERE l.deleted_at IS NULL AND (l.industry_id=d.id OR l.employee_range_id=d.id OR l.tax_system_id=d.id OR l.revenue_range_id=d.id OR l.accounting_state_id=d.id OR l.transfer_reason_id=d.id OR l.transfer_type_id=d.id))
		OR EXISTS(SELECT 1 FROM client_exchange_listing_options o WHERE o.item_id=d.id)
		FROM client_exchange_dictionary_items d WHERE `+where+` ORDER BY d.kind,d.sort_order,d.name`, args...)
	if err != nil {
		fail(w, 500, "Не удалось загрузить справочники")
		return
	}
	defer rows.Close()
	items := []DictionaryItem{}
	for rows.Next() {
		var x DictionaryItem
		var min, max sql.NullFloat64
		if rows.Scan(&x.ID, &x.Kind, &x.Code, &x.Name, &x.Description, &min, &max, &x.Color, &x.LegalName, &x.OperatorCode, &x.SortOrder, &x.Active, &x.Used) != nil {
			continue
		}
		if min.Valid {
			x.MinValue = &min.Float64
		}
		if max.Valid {
			x.MaxValue = &max.Float64
		}
		items = append(items, x)
	}
	respond(w, 200, map[string]any{"items": items, "kinds": DictionaryKinds})
}

func (h *Handler) adminDictionary(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход в админку")
		return
	}
	id := parseInt64(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/client-exchange/dictionaries/"), "/"))
	if id < 1 {
		fail(w, 400, "Некорректный ID")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var x DictionaryItem
		if !decode(w, r, &x) {
			return
		}
		if !validKind(x.Kind) || !codePattern.MatchString(x.Code) || strings.TrimSpace(x.Name) == "" {
			fail(w, 400, "Проверьте тип, code и название")
			return
		}
		if x.Color == "" {
			x.Color = "blue"
		}
		res, err := h.db.ExecContext(r.Context(), `UPDATE client_exchange_dictionary_items SET kind=$2,code=$3,name=$4,description=$5,min_value=$6,max_value=$7,color=$8,legal_name=$9,operator_code=$10,sort_order=$11,active=$12,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, x.Kind, x.Code, clean(x.Name, 300), clean(x.Description, 3000), x.MinValue, x.MaxValue, clean(x.Color, 30), clean(x.LegalName, 300), clean(x.OperatorCode, 100), x.SortOrder, x.Active)
		if err != nil {
			fail(w, 409, "Не удалось сохранить: проверьте уникальность code")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			fail(w, 404, "Значение не найдено")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
	case http.MethodDelete:
		var used bool
		if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM client_exchange_listings l WHERE l.deleted_at IS NULL AND (l.industry_id=$1 OR l.employee_range_id=$1 OR l.tax_system_id=$1 OR l.revenue_range_id=$1 OR l.accounting_state_id=$1 OR l.transfer_reason_id=$1 OR l.transfer_type_id=$1)) OR EXISTS(SELECT 1 FROM client_exchange_listing_options WHERE item_id=$1)`, id).Scan(&used); err != nil {
			fail(w, 500, "Не удалось проверить использование")
			return
		}
		if used {
			_, _ = h.db.ExecContext(r.Context(), `UPDATE client_exchange_dictionary_items SET active=FALSE,updated_at=NOW() WHERE id=$1`, id)
			fail(w, 409, "Значение используется и было безопасно отключено вместо удаления")
			return
		}
		_, err := h.db.ExecContext(r.Context(), `UPDATE client_exchange_dictionary_items SET active=FALSE,deleted_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
		if err != nil {
			fail(w, 500, "Не удалось удалить значение")
			return
		}
		respond(w, 200, map[string]bool{"deleted": true})
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}
