package courses

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var raw json.RawMessage
	err := h.db.QueryRowContext(ctx, `WITH applications AS (
 SELECT i.id,i.course_id,c.title course_title,COALESCE(NULLIF(i.tariff_name,''),t.title,'') tariff_name,
 author.full_name author_name,COALESCE(NULLIF(i.student_name,''),u.full_name) student_name,
 COALESCE(NULLIF(i.contact_email,''),u.email) contact_email,i.contact_phone,i.first_clicked_at created_at
 FROM course_interests i JOIN courses c ON c.id=i.course_id JOIN users author ON author.id=c.owner_id JOIN users u ON u.id=i.user_id LEFT JOIN course_tariffs t ON t.id=i.tariff_id
 WHERE i.user_id IS NOT NULL), page_rows AS (SELECT * FROM applications ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET $1)
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.created_at DESC,p.id DESC) FROM page_rows p),'[]'),'total',(SELECT count(*) FROM applications),'page',$2::int,'page_size',50)`, (page-1)*50, page).Scan(&raw)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, raw)
}
