package courses

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// GET accepts page, author_id and case-insensitive student_name/email substrings.
// total/summary cover every matching row, overall ignores filters and pagination.
// selected_price and total_price use the existing course currency unit: kopecks.
func (h *Handler) adminStudents(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход администратора")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	page := 1
	if value := r.URL.Query().Get("page"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100000 {
			fail(w, 400, "Некорректная страница")
			return
		}
		page = n
	}
	var authorID int64
	if value := r.URL.Query().Get("author_id"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			fail(w, 400, "Некорректный автор")
			return
		}
		authorID = id
	}
	name, email := strings.TrimSpace(r.URL.Query().Get("student_name")), strings.TrimSpace(r.URL.Query().Get("email"))
	if utf8.RuneCountInString(name) > 200 || utf8.RuneCountInString(email) > 254 {
		fail(w, 400, "Слишком длинное значение фильтра")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var raw json.RawMessage
	err := h.db.QueryRowContext(ctx, `WITH applications AS (
 SELECT i.id,i.user_id,i.course_id,c.owner_id author_id,c.title course_title,COALESCE(NULLIF(i.tariff_name,''),t.title,'') tariff_name,
 author.full_name author_name,COALESCE(NULLIF(i.student_name,''),u.full_name) student_name,
 COALESCE(NULLIF(i.contact_email,''),u.email) contact_email,i.contact_phone,i.first_clicked_at created_at,
 COALESCE(i.selected_price,CASE WHEN i.tariff_id IS NOT NULL THEN t.price WHEN c.pricing_mode<>'tariffs' THEN c.price END) selected_price,
 (i.selected_price IS NULL) price_is_current,
 (i.paid_at IS NULL AND i.paid_enrollment_id IS NULL AND i.invitation_outbox_id IS NULL) can_delete
 FROM course_interests i JOIN courses c ON c.id=i.course_id JOIN users author ON author.id=c.owner_id JOIN users u ON u.id=i.user_id LEFT JOIN course_tariffs t ON t.id=i.tariff_id AND t.course_id=i.course_id
 WHERE i.user_id IS NOT NULL), filtered AS (
 SELECT * FROM applications WHERE ($3::bigint=0 OR author_id=$3) AND ($4::text='' OR strpos(lower(student_name),lower($4))>0) AND ($5::text='' OR strpos(lower(contact_email),lower($5))>0)
 ), page_rows AS (SELECT * FROM filtered ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET $1)
 SELECT jsonb_build_object(
 'items',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.created_at DESC,p.id DESC) FROM page_rows p),'[]'),
 'total',(SELECT count(*) FROM filtered),'page',$2::int,'page_size',50,
 'authors',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY a.name,a.id) FROM (SELECT DISTINCT author_id id,author_name name FROM applications) a),'[]'),
 'summary',(SELECT jsonb_build_object('students',count(DISTINCT user_id),'total_price',COALESCE(sum(selected_price),0),'unknown_prices',count(*) FILTER(WHERE selected_price IS NULL),'current_prices',count(*) FILTER(WHERE price_is_current AND selected_price IS NOT NULL)) FROM filtered),
 'overall',(SELECT jsonb_build_object('applications',count(*),'students',count(DISTINCT user_id),'total_price',COALESCE(sum(selected_price),0),'unknown_prices',count(*) FILTER(WHERE selected_price IS NULL),'current_prices',count(*) FILTER(WHERE price_is_current AND selected_price IS NOT NULL)) FROM applications)
 )`, (page-1)*50, page, authorID, name, email).Scan(&raw)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, raw)
}

// DELETE removes only an unpaid expression of interest, never an account,
// enrollment or a confirmed payment. PostgreSQL rechecks the payment predicate
// after waiting for a concurrent payment update of this row.
func (h *Handler) adminDeleteStudent(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		fail(w, 401, "Требуется вход администратора")
		return
	}
	if r.Method != http.MethodDelete {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/admin/courses/students/"), 10, 64)
	if err != nil || id < 1 {
		fail(w, 400, "Некорректная заявка")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var deleted int64
	err = h.db.QueryRowContext(ctx, `DELETE FROM course_interests WHERE id=$1 AND user_id IS NOT NULL AND paid_at IS NULL AND paid_enrollment_id IS NULL AND invitation_outbox_id IS NULL RETURNING id`, id).Scan(&deleted)
	if err == sql.ErrNoRows {
		var exists bool
		if err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM course_interests WHERE id=$1 AND user_id IS NOT NULL)`, id).Scan(&exists); err != nil {
			dbError(w, err)
		} else if exists {
			fail(w, 409, "Оплаченная заявка или заявка с зачислением защищена от удаления")
		} else {
			fail(w, 404, "Заявка не найдена")
		}
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
