package clientexchange

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) createResponse(w http.ResponseWriter, r *http.Request, u UserIdentity, listingID int64) {
	var in ResponseInput
	if !decode(w, r, &in) {
		return
	}
	if in.ProposedPrice != nil && *in.ProposedPrice < 0 {
		fail(w, 400, "Сумма не может быть отрицательной")
		return
	}
	if len([]rune(strings.TrimSpace(in.Comment))) > 3000 {
		fail(w, 400, "Комментарий слишком длинный")
		return
	}
	var seller int64
	var title, status string
	var original sql.NullFloat64
	var bargain bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT seller_user_id,COALESCE(NULLIF(title,''),'Клиент'),status,transfer_price,bargain_allowed FROM client_exchange_listings WHERE id=$1 AND deleted_at IS NULL`, listingID).Scan(&seller, &title, &status, &original, &bargain); err != nil {
		fail(w, 404, "Объявление не найдено")
		return
	}
	if seller == u.ID {
		fail(w, 409, "Нельзя откликнуться на собственное объявление")
		return
	}
	if status != "active" && status != "has_responses" {
		fail(w, 409, "Объявление не принимает предложения")
		return
	}
	if in.ProposedPrice != nil && !bargain && !in.AcceptOriginalPrice {
		fail(w, 409, "Торг по этому объявлению не предусмотрен")
		return
	}
	if !in.AcceptOriginalPrice && !in.ReadyToDiscuss && in.ProposedPrice == nil {
		fail(w, 400, "Укажите вариант предложения")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Не удалось отправить предложение")
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO client_exchange_responses(listing_id,buyer_user_id,proposed_price,accept_original_price,ready_to_discuss,comment) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, listingID, u.ID, in.ProposedPrice, in.AcceptOriginalPrice, in.ReadyToDiscuss, clean(in.Comment, 3000)).Scan(&id)
	if err != nil {
		fail(w, 409, "Вы уже отправили активное предложение")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE client_exchange_listings SET status='has_responses',updated_at=NOW() WHERE id=$1 AND status='active'`, listingID)
	if err != nil {
		fail(w, 500, "Не удалось отправить предложение")
		return
	}
	message := "Компания заинтересовалась вашим клиентом"
	if in.ProposedPrice != nil {
		message += " и предложила " + formatMoney(*in.ProposedPrice) + " ₽"
	} else if in.AcceptOriginalPrice && original.Valid {
		message += " и готова принять цену " + formatMoney(original.Float64) + " ₽"
	} else {
		message += " и готова обсудить условия"
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO client_exchange_notifications(user_id,type,title,message,listing_id,response_id) VALUES($1,'new_response',$2,$3,$4,$5)`, seller, "Новое предложение по клиенту «"+title+"»", message, listingID, id)
	if err != nil {
		fail(w, 500, "Не удалось отправить уведомление")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Не удалось отправить предложение")
		return
	}
	respond(w, 201, map[string]any{"id": id, "status": "pending", "message": "Предложение отправлено"})
}

func (h *Handler) responseRoute(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/client-exchange/responses/"), "/")
	parts := strings.Split(tail, "/")
	id := parseInt64(parts[0])
	if id < 1 || len(parts) < 2 {
		fail(w, 404, "Действие не найдено")
		return
	}
	if r.Method != http.MethodPost {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	switch parts[1] {
	case "accept":
		h.acceptResponse(w, r, u, id)
	case "reject":
		h.rejectResponse(w, r, u, id)
	case "withdraw":
		h.withdrawResponse(w, r, u, id)
	default:
		fail(w, 404, "Действие не найдено")
	}
}

func (h *Handler) acceptResponse(w http.ResponseWriter, r *http.Request, u UserIdentity, responseID int64) {
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Не удалось выбрать покупателя")
		return
	}
	defer tx.Rollback()
	var listingID, buyerID int64
	var title, status string
	err = tx.QueryRowContext(r.Context(), `SELECT l.id,cr.buyer_user_id,COALESCE(NULLIF(l.title,''),'Клиент'),l.status FROM client_exchange_responses cr JOIN client_exchange_listings l ON l.id=cr.listing_id WHERE cr.id=$1 AND l.seller_user_id=$2 AND cr.status='pending' FOR UPDATE`, responseID, u.ID).Scan(&listingID, &buyerID, &title, &status)
	if err == sql.ErrNoRows {
		fail(w, 403, "Предложение недоступно или уже обработано")
		return
	}
	if err != nil {
		fail(w, 500, "Не удалось выбрать покупателя")
		return
	}
	if status != "has_responses" && status != "active" {
		fail(w, 409, "В текущем статусе выбрать покупателя нельзя")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE client_exchange_responses SET status=CASE WHEN id=$1 THEN 'accepted' ELSE 'rejected' END,updated_at=NOW() WHERE listing_id=$2 AND status='pending'`, responseID, listingID); err != nil {
		fail(w, 500, "Не удалось обработать предложения")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE client_exchange_listings SET status='buyer_selected',selected_buyer_user_id=$2,updated_at=NOW() WHERE id=$1`, listingID, buyerID); err != nil {
		fail(w, 500, "Не удалось изменить объявление")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO client_exchange_notifications(user_id,type,title,message,listing_id,response_id) VALUES($1,'response_accepted',$2,'Продавец выбрал ваше предложение. Контакты сторон теперь доступны.',$3,$4),($5,'buyer_selected',$6,'Вы выбрали покупателя. Контакты сторон теперь доступны.',$3,$4)`, buyerID, "Ваше предложение принято: «"+title+"»", listingID, responseID, u.ID, "Покупатель выбран: «"+title+"»"); err != nil {
		fail(w, 500, "Не удалось отправить уведомления")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "Не удалось выбрать покупателя")
		return
	}
	respond(w, 200, map[string]string{"status": "accepted", "message": "Компания выбрана, контакты открыты"})
}

func (h *Handler) rejectResponse(w http.ResponseWriter, r *http.Request, u UserIdentity, responseID int64) {
	var listing, buyer int64
	var title string
	err := h.db.QueryRowContext(r.Context(), `UPDATE client_exchange_responses cr SET status='rejected',updated_at=NOW() FROM client_exchange_listings l WHERE cr.id=$1 AND l.id=cr.listing_id AND l.seller_user_id=$2 AND cr.status='pending' RETURNING l.id,cr.buyer_user_id,COALESCE(NULLIF(l.title,''),'Клиент')`, responseID, u.ID).Scan(&listing, &buyer, &title)
	if err != nil {
		fail(w, 403, "Предложение недоступно")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO client_exchange_notifications(user_id,type,title,message,listing_id,response_id) VALUES($1,'response_rejected',$2,'Продавец выбрал другое предложение или отказался от передачи.',$3,$4)`, buyer, "Предложение отклонено: «"+title+"»", listing, responseID)
	respond(w, 200, map[string]string{"status": "rejected"})
}
func (h *Handler) withdrawResponse(w http.ResponseWriter, r *http.Request, u UserIdentity, responseID int64) {
	res, err := h.db.ExecContext(r.Context(), `UPDATE client_exchange_responses SET status='withdrawn',updated_at=NOW() WHERE id=$1 AND buyer_user_id=$2 AND status='pending'`, responseID, u.ID)
	if err != nil {
		fail(w, 500, "Не удалось отозвать предложение")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 409, "Предложение уже обработано")
		return
	}
	respond(w, 200, map[string]string{"status": "withdrawn"})
}

func (h *Handler) myResponses(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.responseList(w, r, `cr.buyer_user_id=$1`, u.ID, u.ID)
}
func (h *Handler) receivedResponses(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.responseList(w, r, `l.seller_user_id=$1`, u.ID, u.ID)
}
func (h *Handler) responseList(w http.ResponseWriter, r *http.Request, condition string, arg, viewer int64) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT cr.id,cr.listing_id,cr.buyer_user_id,u.full_name,u.email,COALESCE(u.avatar_url,''),cr.proposed_price,cr.accept_original_price,cr.ready_to_discuss,cr.comment,cr.status,cr.created_at,l.status,l.selected_buyer_user_id,COALESCE(NULLIF(l.title,''),di.name),l.region,su.full_name,su.email FROM client_exchange_responses cr JOIN client_exchange_listings l ON l.id=cr.listing_id JOIN users u ON u.id=cr.buyer_user_id JOIN users su ON su.id=l.seller_user_id LEFT JOIN client_exchange_dictionary_items di ON di.id=l.industry_id WHERE `+condition+` AND l.deleted_at IS NULL ORDER BY cr.created_at DESC`, arg)
	if err != nil {
		fail(w, 500, "Не удалось загрузить предложения")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, lid, bid int64
		var buyer, email, avatar, comment, status, lstatus, title, region, sellerName, sellerEmail string
		var price sql.NullFloat64
		var accept, discuss bool
		var created any
		var selected sql.NullInt64
		if rows.Scan(&id, &lid, &bid, &buyer, &email, &avatar, &price, &accept, &discuss, &comment, &status, &created, &lstatus, &selected, &title, &region, &sellerName, &sellerEmail) != nil {
			continue
		}
		contactAllowed := status == "accepted" && (viewer == bid || selected.Int64 == bid)
		item := map[string]any{"id": id, "listing_id": lid, "buyer_user_id": bid, "buyer_name": buyer, "buyer_avatar": avatar, "proposed_price": nil, "accept_original_price": accept, "ready_to_discuss": discuss, "comment": comment, "status": status, "created_at": created, "listing_status": lstatus, "listing_title": title, "region": region}
		if price.Valid {
			item["proposed_price"] = price.Float64
		}
		if contactAllowed {
			if viewer == bid {
				item["contact_name"] = sellerName
				item["contact_email"] = sellerEmail
			} else {
				item["contact_name"] = buyer
				item["contact_email"] = email
			}
		}
		items = append(items, item)
	}
	respond(w, 200, map[string]any{"items": items})
}

func (h *Handler) notifications(w http.ResponseWriter, r *http.Request) {
	u, ok := h.current(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		_, err := h.db.ExecContext(r.Context(), `UPDATE client_exchange_notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, u.ID)
		if err != nil {
			fail(w, 500, "Не удалось прочитать уведомления")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,type,title,message,listing_id,response_id,read_at,created_at FROM client_exchange_notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, u.ID)
	if err != nil {
		fail(w, 500, "Не удалось загрузить уведомления")
		return
	}
	defer rows.Close()
	items := []Notification{}
	unread := 0
	for rows.Next() {
		var x Notification
		var lid, rid sql.NullInt64
		var read sql.NullTime
		if rows.Scan(&x.ID, &x.Type, &x.Title, &x.Message, &lid, &rid, &read, &x.CreatedAt) != nil {
			continue
		}
		if lid.Valid {
			x.ListingID = &lid.Int64
		}
		if rid.Valid {
			x.ResponseID = &rid.Int64
		}
		if read.Valid {
			x.ReadAt = &read.Time
		} else {
			unread++
		}
		items = append(items, x)
	}
	respond(w, 200, map[string]any{"items": items, "unread": unread})
}

func formatMoney(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
