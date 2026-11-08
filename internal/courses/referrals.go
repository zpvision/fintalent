package courses

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

const referralCookie = "fintalent_course_source"

var referralCode = regexp.MustCompile(`^[a-z0-9_-]{3,64}$`)

func normalizeReferral(name, code string) (string, string, bool) {
	name = strings.TrimSpace(name)
	code = strings.ToLower(strings.TrimSpace(code))
	return name, code, utf8.RuneCountInString(name) > 0 && utf8.RuneCountInString(name) <= 120 && referralCode.MatchString(code)
}

func referralSameOrigin(r *http.Request) bool { return http.NewCrossOriginProtection().Check(r) == nil }

// Ownership is checked under a transaction lock; administrative access is explicit.
func (h *Handler) referrals(w http.ResponseWriter, r *http.Request, course, user int64, action string) {
	if r.Method != "GET" && !referralSameOrigin(r) {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	admin := h.admin(r)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	// Keep ownership stable through a mutation or a concurrent course transfer.
	var authorized int64
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM courses WHERE id=$1 AND (owner_id=$2 OR $3) FOR SHARE`, course, user, admin).Scan(&authorized)
	if err == sql.ErrNoRows {
		fail(w, 404, "Курс не найден")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	switch {
	case r.Method == "GET" && action == "referrals":
		var raw json.RawMessage
		err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.id),'[]') FROM (SELECT l.id,l.name,l.code,c.slug,c.status FROM course_referral_links l JOIN courses c ON c.id=l.course_id WHERE l.course_id=$1 AND l.deleted_at IS NULL) x`, course).Scan(&raw)
		if err != nil {
			dbError(w, err)
			return
		}
		respond(w, 200, raw)
	case r.Method == "POST" && action == "referrals":
		var in struct {
			Name string `json:"name"`
			Code string `json:"code"`
		}
		if !decode(w, r, &in) {
			return
		}
		name, code, valid := normalizeReferral(in.Name, in.Code)
		if !valid {
			fail(w, 400, "Укажите название до 120 символов и код из 3–64 латинских букв, цифр, дефисов или подчёркиваний")
			return
		}
		var id int64
		err = tx.QueryRowContext(r.Context(), `INSERT INTO course_referral_links(course_id,name,code,created_by,created_by_admin) VALUES($1,$2,$3,NULLIF($4,0),$5) RETURNING id`, course, name, code, user, admin).Scan(&id)
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			fail(w, 409, "Этот код уже использовался в курсе, в том числе в удалённой ссылке")
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			dbError(w, err)
			return
		}
		respond(w, 201, map[string]any{"id": id})
	case r.Method == "DELETE" && strings.HasPrefix(action, "referrals/"):
		id, e := strconv.ParseInt(strings.TrimPrefix(action, "referrals/"), 10, 64)
		if e != nil || id <= 0 {
			fail(w, 404, "Ссылка не найдена")
			return
		}
		res, e := tx.ExecContext(r.Context(), `UPDATE course_referral_links SET deleted_at=COALESCE(deleted_at,now()) WHERE id=$1 AND course_id=$2`, id, course)
		if e != nil {
			dbError(w, e)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			fail(w, 404, "Ссылка не найдена")
			return
		}
		if e = tx.Commit(); e != nil {
			dbError(w, e)
			return
		}
		respond(w, 200, map[string]bool{"deleted": true})
	default:
		fail(w, 405, "Метод не поддерживается")
	}
}

func guestHash(r *http.Request) string {
	c, e := r.Cookie(referralCookie)
	if e != nil || len(c.Value) != 64 {
		return ""
	}
	b, e := hex.DecodeString(c.Value)
	if e != nil {
		return ""
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
