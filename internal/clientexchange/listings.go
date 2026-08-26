package clientexchange

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) getListingJSON(ctx context.Context, id, viewerID int64, ownerView bool) (json.RawMessage, error) {
	query := `SELECT jsonb_build_object(
		'id',l.id,'public_id',l.public_id,'seller_user_id',CASE WHEN l.seller_user_id=$2 THEN l.seller_user_id ELSE NULL END,
		'is_owner',l.seller_user_id=$2,'title',COALESCE(NULLIF(l.title,''),i.name),'industry',jsonb_build_object('id',i.id,'name',i.name),
		'employees',jsonb_build_object('id',er.id,'name',er.name),'tax_system',jsonb_build_object('id',ts.id,'name',ts.name),
		'revenue',jsonb_build_object('id',rr.id,'name',rr.name),'accounting_state',jsonb_build_object('id',ast.id,'name',ast.name,'color',ast.color),
		'transfer_reason',jsonb_build_object('id',tr.id,'name',tr.name),'transfer_type',jsonb_build_object('id',tt.id,'name',tt.name,'code',tt.code),
		'transfer_reason_comment',l.transfer_reason_comment,'transfer_price',l.transfer_price,'monthly_commission_percent',l.monthly_commission_percent,
		'commission_months',l.commission_months,'current_monthly_fee',l.current_monthly_fee,'operations_per_month',l.operations_per_month,'banks_count',l.banks_count,
		'has_vat',l.has_vat,'foreign_trade',l.foreign_trade,'bargain_allowed',l.bargain_allowed,'region',l.region,'city',l.city,
		'client_since',l.client_since,'desired_transfer_date',l.desired_transfer_date,'comment',l.comment,'status',l.status,'match_percent',l.match_percent,
		'views_count',l.views_count,'responses_count',(SELECT COUNT(*) FROM client_exchange_responses cr WHERE cr.listing_id=l.id AND cr.status<>'withdrawn'),
		'is_favorite',EXISTS(SELECT 1 FROM client_exchange_favorites f WHERE f.listing_id=l.id AND f.user_id=$2),
		'my_response_status',(SELECT cr.status FROM client_exchange_responses cr WHERE cr.listing_id=l.id AND cr.buyer_user_id=$2 ORDER BY cr.created_at DESC LIMIT 1),
		'marketplaces',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'name',d.name) ORDER BY d.sort_order) FROM client_exchange_listing_options o JOIN client_exchange_dictionary_items d ON d.id=o.item_id WHERE o.listing_id=l.id AND o.kind='marketplace'),'[]'::jsonb),
		'edo_providers',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'name',d.name) ORDER BY d.sort_order) FROM client_exchange_listing_options o JOIN client_exchange_dictionary_items d ON d.id=o.item_id WHERE o.listing_id=l.id AND o.kind='edo_provider'),'[]'::jsonb),
		'accounting_programs',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'name',d.name) ORDER BY d.sort_order) FROM client_exchange_listing_options o JOIN client_exchange_dictionary_items d ON d.id=o.item_id WHERE o.listing_id=l.id AND o.kind='accounting_program'),'[]'::jsonb),
		'private',CASE WHEN l.seller_user_id=$2 OR EXISTS(SELECT 1 FROM client_exchange_responses cr WHERE cr.listing_id=l.id AND cr.buyer_user_id=$2 AND cr.status='accepted') THEN jsonb_build_object('client_inn',l.client_inn,'client_legal_name',l.client_legal_name) ELSE NULL END,
		'seller',jsonb_build_object('name',CASE WHEN l.seller_user_id=$2 OR EXISTS(SELECT 1 FROM client_exchange_responses cr WHERE cr.listing_id=l.id AND cr.buyer_user_id=$2 AND cr.status='accepted') THEN u.full_name ELSE 'Бухгалтерская компания' END,'avatar',COALESCE(u.avatar_url,''),'region',l.region,'verified',true,'email',CASE WHEN l.seller_user_id=$2 OR EXISTS(SELECT 1 FROM client_exchange_responses cr WHERE cr.listing_id=l.id AND cr.buyer_user_id=$2 AND cr.status='accepted') THEN u.email ELSE NULL END),
		'published_at',l.published_at,'created_at',l.created_at,'updated_at',l.updated_at,'transferred_at',l.transferred_at,'current_step',l.current_step
	) FROM client_exchange_listings l JOIN users u ON u.id=l.seller_user_id
	LEFT JOIN client_exchange_dictionary_items i ON i.id=l.industry_id LEFT JOIN client_exchange_dictionary_items er ON er.id=l.employee_range_id
	LEFT JOIN client_exchange_dictionary_items ts ON ts.id=l.tax_system_id LEFT JOIN client_exchange_dictionary_items rr ON rr.id=l.revenue_range_id
	LEFT JOIN client_exchange_dictionary_items ast ON ast.id=l.accounting_state_id LEFT JOIN client_exchange_dictionary_items tr ON tr.id=l.transfer_reason_id
	LEFT JOIN client_exchange_dictionary_items tt ON tt.id=l.transfer_type_id WHERE l.id=$1 AND l.deleted_at IS NULL`
	if !ownerView {
		query += ` AND (l.status IN ('active','has_responses','buyer_selected','transfer_in_progress','transferred') OR l.seller_user_id=$2 OR l.selected_buyer_user_id=$2)`
	}
	var raw []byte
	if err := h.db.QueryRowContext(ctx, query, id, viewerID).Scan(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (h *Handler) listingRoute(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/client-exchange/listings/"), "/")
	parts := strings.Split(tail, "/")
	id := parseInt64(parts[0])
	if id < 1 {
		fail(w, 400, "Некорректный ID")
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if action != "" {
		h.listingAction(w, r, u, id, action)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if err := h.recordView(r.Context(), id, u.ID); err != nil && err != sql.ErrNoRows {
			fail(w, 500, "Не удалось записать просмотр")
			return
		}
		b, err := h.getListingJSON(r.Context(), id, u.ID, false)
		if err == sql.ErrNoRows {
			fail(w, 404, "Объявление не найдено")
			return
		}
		if err != nil {
			fail(w, 500, "Не удалось загрузить объявление")
			return
		}
		respond(w, 200, json.RawMessage(b))
	case http.MethodPut:
		if !h.owns(r.Context(), id, u.ID) {
			fail(w, 403, "Изменять объявление может только владелец")
			return
		}
		var currentStatus string
		if err := h.db.QueryRowContext(r.Context(), `SELECT status FROM client_exchange_listings WHERE id=$1`, id).Scan(&currentStatus); err != nil {
			fail(w, 404, "Объявление не найдено")
			return
		}
		if currentStatus == "buyer_selected" || currentStatus == "transfer_in_progress" || currentStatus == "transferred" {
			fail(w, 409, "Объявление нельзя редактировать на этапе передачи")
			return
		}
		var in ListingInput
		if !decode(w, r, &in) {
			return
		}
		if err := h.validateInput(r.Context(), in, false); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := h.updateListing(r.Context(), id, in); err != nil {
			fail(w, 400, err.Error())
			return
		}
		b, _ := h.getListingJSON(r.Context(), id, u.ID, true)
		respond(w, 200, json.RawMessage(b))
	case http.MethodDelete:
		if !h.owns(r.Context(), id, u.ID) && !h.admin(r) {
			fail(w, 403, "Удалить объявление может только владелец или администратор")
			return
		}
		var status string
		if err := h.db.QueryRowContext(r.Context(), `SELECT status FROM client_exchange_listings WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&status); err != nil {
			fail(w, 404, "Объявление не найдено")
			return
		}
		if status == "transferred" {
			fail(w, 409, "Историю успешной передачи удалить нельзя")
			return
		}
		_, err := h.db.ExecContext(r.Context(), `UPDATE client_exchange_listings SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
		if err != nil {
			fail(w, 500, "Не удалось удалить объявление")
			return
		}
		respond(w, 200, map[string]string{"message": "Объявление удалено"})
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}

func (h *Handler) updateListing(ctx context.Context, id int64, in ListingInput) error {
	_, err := h.db.ExecContext(ctx, `UPDATE client_exchange_listings SET title=$2,client_inn=$3,client_legal_name=$4,industry_id=$5,employee_range_id=$6,tax_system_id=$7,revenue_range_id=$8,accounting_state_id=$9,transfer_reason_id=$10,transfer_type_id=$11,transfer_reason_comment=$12,transfer_price=$13,monthly_commission_percent=$14,commission_months=$15,current_monthly_fee=$16,operations_per_month=$17,banks_count=$18,has_vat=$19,foreign_trade=$20,bargain_allowed=$21,region=$22,city=$23,client_since=$24,desired_transfer_date=$25,comment=$26,current_step=$27,updated_at=NOW() WHERE id=$1`, id, clean(in.Title, 240), strings.TrimSpace(in.ClientINN), clean(in.ClientLegalName, 500), in.IndustryID, in.EmployeeRangeID, in.TaxSystemID, in.RevenueRangeID, in.AccountingStateID, in.TransferReasonID, in.TransferTypeID, clean(in.TransferReasonComment, 2000), in.TransferPrice, in.MonthlyCommission, in.CommissionMonths, in.CurrentMonthlyFee, in.OperationsPerMonth, in.BanksCount, in.HasVAT, in.ForeignTrade, in.BargainAllowed, clean(in.Region, 200), clean(in.City, 200), nullableDate(in.ClientSince), nullableDate(in.DesiredTransferDate), clean(in.Comment, 5000), clamp(in.CurrentStep, 1, 6))
	if err != nil {
		return errors.New("не удалось сохранить объявление")
	}
	return h.saveOptions(ctx, id, in)
}

func (h *Handler) listingAction(w http.ResponseWriter, r *http.Request, u UserIdentity, id int64, action string) {
	if action == "responses" {
		if r.Method == http.MethodPost {
			h.createResponse(w, r, u, id)
		} else {
			fail(w, 405, "Метод не поддерживается")
		}
		return
	}
	if action == "favorite" {
		h.favorite(w, r, u, id)
		return
	}
	if r.Method != http.MethodPost {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if !h.owns(r.Context(), id, u.ID) {
		fail(w, 403, "Действие доступно только владельцу")
		return
	}
	var current string
	if err := h.db.QueryRowContext(r.Context(), `SELECT status FROM client_exchange_listings WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&current); err != nil {
		fail(w, 404, "Объявление не найдено")
		return
	}
	next := ""
	switch action {
	case "publish":
		if current == "draft" || current == "archived" || current == "cancelled" {
			next = "active"
		}
	case "archive":
		if current == "active" || current == "has_responses" {
			next = "archived"
		}
	case "cancel":
		if current != "transferred" {
			next = "cancelled"
		}
	case "start-transfer":
		if current == "buyer_selected" {
			next = "transfer_in_progress"
		}
	case "complete":
		if current == "buyer_selected" || current == "transfer_in_progress" {
			next = "transferred"
		}
	}
	if next == "" {
		fail(w, 409, "Недопустимый переход статуса")
		return
	}
	if action == "publish" {
		in, err := h.loadInput(r.Context(), id)
		if err != nil {
			fail(w, 500, "Не удалось проверить объявление")
			return
		}
		if err = h.validateInput(r.Context(), in, true); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	query := `UPDATE client_exchange_listings SET status=$2::varchar,updated_at=NOW(),published_at=CASE WHEN $2::varchar='active' THEN NOW() ELSE published_at END,transferred_at=CASE WHEN $2::varchar='transferred' THEN NOW() ELSE transferred_at END WHERE id=$1`
	if _, err := h.db.ExecContext(r.Context(), query, id, next); err != nil {
		fail(w, 500, "Не удалось изменить статус")
		return
	}
	if next == "transferred" {
		_, _ = h.db.ExecContext(r.Context(), `INSERT INTO client_exchange_notifications(user_id,type,title,message,listing_id)
			SELECT selected_buyer_user_id,'transfer_completed','Передача клиента завершена','Продавец отметил клиента переданным.',id
			FROM client_exchange_listings WHERE id=$1 AND selected_buyer_user_id IS NOT NULL`, id)
	}
	respond(w, 200, map[string]any{"status": next, "label": statusLabels()[next]})
}

func (h *Handler) loadInput(ctx context.Context, id int64) (ListingInput, error) {
	var x ListingInput
	var clientSince, desired sql.NullTime
	err := h.db.QueryRowContext(ctx, `SELECT title,client_inn,client_legal_name,industry_id,employee_range_id,tax_system_id,revenue_range_id,accounting_state_id,transfer_reason_id,transfer_type_id,transfer_reason_comment,transfer_price,monthly_commission_percent,commission_months,current_monthly_fee,operations_per_month,banks_count,has_vat,foreign_trade,bargain_allowed,region,city,client_since,desired_transfer_date,comment,current_step FROM client_exchange_listings WHERE id=$1`, id).Scan(&x.Title, &x.ClientINN, &x.ClientLegalName, &x.IndustryID, &x.EmployeeRangeID, &x.TaxSystemID, &x.RevenueRangeID, &x.AccountingStateID, &x.TransferReasonID, &x.TransferTypeID, &x.TransferReasonComment, &x.TransferPrice, &x.MonthlyCommission, &x.CommissionMonths, &x.CurrentMonthlyFee, &x.OperationsPerMonth, &x.BanksCount, &x.HasVAT, &x.ForeignTrade, &x.BargainAllowed, &x.Region, &x.City, &clientSince, &desired, &x.Comment, &x.CurrentStep)
	if clientSince.Valid {
		s := clientSince.Time.Format("2006-01-02")
		x.ClientSince = &s
	}
	if desired.Valid {
		s := desired.Time.Format("2006-01-02")
		x.DesiredTransferDate = &s
	}
	return x, err
}

func (h *Handler) owns(ctx context.Context, id, userID int64) bool {
	var ok bool
	_ = h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM client_exchange_listings WHERE id=$1 AND seller_user_id=$2 AND deleted_at IS NULL)`, id, userID).Scan(&ok)
	return ok
}
func (h *Handler) recordView(ctx context.Context, id, userID int64) error {
	res, err := h.db.ExecContext(ctx, `INSERT INTO client_exchange_views(listing_id,user_id) SELECT id,$2 FROM client_exchange_listings WHERE id=$1 AND seller_user_id<>$2 AND deleted_at IS NULL ON CONFLICT DO NOTHING`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		_, err = h.db.ExecContext(ctx, `UPDATE client_exchange_listings SET views_count=views_count+1 WHERE id=$1`, id)
	}
	return err
}

func (h *Handler) myListings(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.listByQuery(w, r, u.ID, `SELECT id FROM client_exchange_listings WHERE seller_user_id=$1 AND deleted_at IS NULL ORDER BY updated_at DESC`, u.ID)
}
func (h *Handler) myFavorites(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.listByQuery(w, r, u.ID, `SELECT l.id FROM client_exchange_favorites f JOIN client_exchange_listings l ON l.id=f.listing_id WHERE f.user_id=$1 AND l.deleted_at IS NULL ORDER BY f.created_at DESC`, u.ID)
}
func (h *Handler) listByQuery(w http.ResponseWriter, r *http.Request, viewer int64, query string, args ...any) {
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		fail(w, 500, "Не удалось загрузить объявления")
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			if b, e := h.getListingJSON(r.Context(), id, viewer, true); e == nil {
				items = append(items, b)
			}
		}
	}
	respond(w, 200, map[string]any{"items": items})
}

func (h *Handler) favorite(w http.ResponseWriter, r *http.Request, u UserIdentity, id int64) {
	switch r.Method {
	case http.MethodPost:
		_, err := h.db.ExecContext(r.Context(), `INSERT INTO client_exchange_favorites(user_id,listing_id) SELECT $1,id FROM client_exchange_listings WHERE id=$2 AND seller_user_id<>$1 AND deleted_at IS NULL ON CONFLICT DO NOTHING`, u.ID, id)
		if err != nil {
			fail(w, 400, "Не удалось добавить в избранное")
			return
		}
		respond(w, 200, map[string]bool{"favorite": true})
	case http.MethodDelete:
		_, _ = h.db.ExecContext(r.Context(), `DELETE FROM client_exchange_favorites WHERE user_id=$1 AND listing_id=$2`, u.ID, id)
		respond(w, 200, map[string]bool{"favorite": false})
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = strconv.Itoa
var _ = fmt.Sprintf
