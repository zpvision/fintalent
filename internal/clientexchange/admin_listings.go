package clientexchange

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Admin contract: GET listings?q=&status=&page= returns 30 rows and a total;
// GET listings/{id} returns listing metadata and a complete ListingInput.
// PUT {input,updated_at}, POST {id}/archive {updated_at}, DELETE {id} {updated_at}
// require an admin session and the version read by the administrator.
// Deletion is soft; cards involved in a transfer remain read-only.
type adminListingSummary struct {
	ID             int64      `json:"id"`
	PublicID       string     `json:"public_id"`
	Title          string     `json:"title"`
	SellerID       int64      `json:"seller_user_id"`
	SellerName     string     `json:"seller_name"`
	SellerEmail    string     `json:"seller_email"`
	Status         string     `json:"status"`
	City           string     `json:"city"`
	TransferPrice  *float64   `json:"transfer_price"`
	ViewsCount     int        `json:"views_count"`
	ResponsesCount int        `json:"responses_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	PublishedAt    *time.Time `json:"published_at"`
}

const adminListingColumns = `l.id,l.public_id,COALESCE(NULLIF(l.title,''),d.name,''),l.seller_user_id,u.full_name,u.email,l.status,l.city,l.transfer_price,l.views_count,
 (SELECT count(*) FROM client_exchange_responses r WHERE r.listing_id=l.id AND r.status<>'withdrawn'),l.created_at,l.updated_at,l.published_at`
const adminListingFrom = ` FROM client_exchange_listings l JOIN users u ON u.id=l.seller_user_id LEFT JOIN client_exchange_dictionary_items d ON d.id=l.industry_id `

func scanAdminListing(row interface{ Scan(...any) error }) (adminListingSummary, error) {
	var x adminListingSummary
	err := row.Scan(&x.ID, &x.PublicID, &x.Title, &x.SellerID, &x.SellerName, &x.SellerEmail, &x.Status, &x.City, &x.TransferPrice, &x.ViewsCount, &x.ResponsesCount, &x.CreatedAt, &x.UpdatedAt, &x.PublishedAt)
	return x, err
}

func editableListingStatus(status string) bool {
	return status == "draft" || status == "active" || status == "has_responses" || status == "archived" || status == "cancelled"
}

func (h *Handler) adminListings(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход в админку")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	page := parseInt(r.URL.Query().Get("page"), 1)
	if page < 1 || page > 100000 {
		fail(w, 400, "Некорректная страница")
		return
	}
	where, args := ` WHERE l.deleted_at IS NULL`, []any{}
	if status := r.URL.Query().Get("status"); status != "" {
		if !editableListingStatus(status) && status != "buyer_selected" && status != "transfer_in_progress" && status != "transferred" {
			fail(w, 400, "Неизвестный статус")
			return
		}
		args = append(args, status)
		where += fmt.Sprintf(" AND l.status=$%d", len(args))
	}
	if q := clean(r.URL.Query().Get("q"), 200); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		where += fmt.Sprintf(` AND (l.title ILIKE $%[1]d OR u.full_name ILIKE $%[1]d OR u.email ILIKE $%[1]d OR l.client_legal_name ILIKE $%[1]d OR l.client_inn ILIKE $%[1]d OR l.public_id::text ILIKE $%[1]d OR l.id::text ILIKE $%[1]d OR u.id::text ILIKE $%[1]d)`, n)
	}
	var total int
	if err := h.db.QueryRowContext(r.Context(), "SELECT count(*)"+adminListingFrom+where, args...).Scan(&total); err != nil {
		fail(w, 500, "Не удалось загрузить карточки")
		return
	}
	args = append(args, (page-1)*30)
	rows, err := h.db.QueryContext(r.Context(), "SELECT "+adminListingColumns+adminListingFrom+where+fmt.Sprintf(" ORDER BY l.updated_at DESC,l.id DESC LIMIT 30 OFFSET $%d", len(args)), args...)
	if err != nil {
		fail(w, 500, "Не удалось загрузить карточки")
		return
	}
	defer rows.Close()
	items := []adminListingSummary{}
	for rows.Next() {
		x, err := scanAdminListing(rows)
		if err != nil {
			fail(w, 500, "Не удалось прочитать карточки")
			return
		}
		items = append(items, x)
	}
	if rows.Err() != nil {
		fail(w, 500, "Не удалось прочитать карточки")
		return
	}
	respond(w, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": 30})
}

func (h *Handler) adminListing(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход в админку")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/client-exchange/listings/"), "/")
	id := parseInt64(parts[0])
	archive := len(parts) == 2 && parts[1] == "archive"
	if id < 1 || len(parts) > 1 && !archive {
		fail(w, 400, "Некорректный адрес карточки")
		return
	}
	if archive && r.Method != http.MethodPost || !archive && r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var body struct {
		Input     *ListingInput `json:"input"`
		UpdatedAt time.Time     `json:"updated_at"`
	}
	if r.Method != http.MethodGet {
		if !decode(w, r, &body) {
			return
		}
		if r.Method == http.MethodPut && body.Input == nil {
			fail(w, 400, "Не переданы данные карточки")
			return
		}
	}
	// One snapshot for details; the same row lock serializes mutations with seller actions.
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		fail(w, 500, "Не удалось открыть карточку")
		return
	}
	defer tx.Rollback()
	x, err := scanAdminListing(tx.QueryRowContext(r.Context(), "SELECT "+adminListingColumns+adminListingFrom+" WHERE l.id=$1 AND l.deleted_at IS NULL FOR UPDATE OF l", id))
	if err == sql.ErrNoRows {
		fail(w, 404, "Карточка не найдена")
		return
	}
	if err != nil {
		fail(w, 500, "Не удалось прочитать карточку")
		return
	}
	if r.Method == http.MethodGet {
		input, err := loadListingInput(r.Context(), tx, id)
		if err != nil {
			fail(w, 500, "Не удалось прочитать данные карточки")
			return
		}
		respond(w, 200, map[string]any{"listing": x, "input": input})
		return
	}
	if !body.UpdatedAt.Equal(x.UpdatedAt) {
		fail(w, 409, "Карточка уже изменилась. Закройте окно и откройте её заново")
		return
	}
	if !editableListingStatus(x.Status) {
		fail(w, 409, "Карточку нельзя изменять или удалять на этапе передачи. История сделки сохраняется")
		return
	}
	var result sql.Result
	if archive {
		if x.Status != "active" && x.Status != "has_responses" {
			fail(w, 409, "Карточка уже снята с публикации")
			return
		}
		result, err = tx.ExecContext(r.Context(), `UPDATE client_exchange_listings SET status='archived',updated_at=clock_timestamp() WHERE id=$1`, id)
	} else if r.Method == http.MethodDelete {
		result, err = tx.ExecContext(r.Context(), `UPDATE client_exchange_listings SET deleted_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1`, id)
	} else {
		if err := validateListingInput(r.Context(), tx, id, *body.Input, x.Status == "active" || x.Status == "has_responses"); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := validateAdminListingInput(*body.Input); err != nil {
			fail(w, 400, err.Error())
			return
		}
		err = h.updateListingTx(r.Context(), tx, id, *body.Input)
	}
	if err != nil {
		if archive || r.Method == http.MethodDelete {
			fail(w, 500, "Не удалось изменить карточку")
		} else {
			fail(w, 400, "Не удалось сохранить карточку. Проверьте выбранные значения")
		}
		return
	}
	if result != nil {
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			fail(w, 409, "Карточка уже изменилась. Обновите страницу")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, "Не удалось сохранить карточку")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func validateAdminListingInput(in ListingInput) error {
	if in.OperationsPerMonth != nil && *in.OperationsPerMonth < 0 || in.BanksCount != nil && *in.BanksCount < 0 || in.CommissionMonths != nil && *in.CommissionMonths < 0 {
		return fmt.Errorf("Количество не может быть отрицательным")
	}
	for _, value := range []*string{in.ClientSince, in.DesiredTransferDate} {
		if value != nil && strings.TrimSpace(*value) != "" {
			if _, err := time.Parse("2006-01-02", *value); err != nil {
				return fmt.Errorf("Проверьте даты")
			}
		}
	}
	return nil
}
