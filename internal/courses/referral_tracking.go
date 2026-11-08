package courses

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func visitorCookie(w http.ResponseWriter, r *http.Request, token string) {
	if token != "" {
		http.SetCookie(w, &http.Cookie{Name: referralCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode, MaxAge: 34560000})
	}
}

// Rotate the consumed browser identity on successful login, preventing another
// account using this browser later from inheriting the first account's claims.
func ClearReferralGuestCookie(w http.ResponseWriter, r *http.Request) {
	if guestHash(r) != "" {
		http.SetCookie(w, &http.Cookie{Name: referralCookie, Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
}

// Acquire the guest row before link locks, consistently with login binding.
func lockReferralGuest(ctx context.Context, tx *sql.Tx, r *http.Request) (hash, token string, claimed bool, err error) {
	hash = guestHash(r)
	if hash == "" {
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return
		}
		token = hex.EncodeToString(b)
		s := sha256.Sum256(b)
		hash = hex.EncodeToString(s[:])
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO course_referral_guests(token_hash) VALUES($1) ON CONFLICT DO NOTHING`, hash)
	if err != nil {
		return
	}
	var user sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT claimed_by FROM course_referral_guests WHERE token_hash=$1 FOR UPDATE`, hash).Scan(&user)
	claimed = user.Valid
	return
}

func isPreviewBot(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	for _, s := range []string{"bot", "spider", "crawler", "facebookexternalhit", "slack-imgproxy", "vkshare"} {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

func (h *Handler) referralVisit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if !referralSameOrigin(r) {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	var in struct {
		Slug string `json:"slug"`
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	_, code, valid := normalizeReferral("visit", in.Code)
	if !valid || len(in.Slug) > 200 || isPreviewBot(r) {
		w.WriteHeader(204)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		dbError(w, e)
		return
	}
	defer tx.Rollback()
	// Invalid/deleted codes cannot allocate visitor sessions.
	var course, link int64
	e = tx.QueryRowContext(ctx, `SELECT c.id,l.id FROM courses c JOIN users u ON u.id=c.owner_id JOIN course_referral_links l ON l.course_id=c.id WHERE c.slug=$1 AND c.status='published' AND NOT u.is_blocked AND l.code=$2 AND l.deleted_at IS NULL`, in.Slug, code).Scan(&course, &link)
	if e == sql.ErrNoRows {
		w.WriteHeader(204)
		return
	}
	if e != nil {
		dbError(w, e)
		return
	}
	hash, token, claimed, e := lockReferralGuest(ctx, tx, r)
	if e != nil {
		dbError(w, e)
		return
	}
	e = tx.QueryRowContext(ctx, `SELECT id FROM course_referral_links WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, link).Scan(&link)
	if e == sql.ErrNoRows {
		w.WriteHeader(204)
		return
	}
	if e != nil {
		dbError(w, e)
		return
	}
	user, _ := h.user(r)
	visitor := "g:" + hash
	if user > 0 {
		visitor = "u:" + strconv.FormatInt(user, 10)
		_, e = tx.ExecContext(ctx, `INSERT INTO course_referral_attributions(user_id,course_id,referral_link_id,first_clicked_at) VALUES($1,$2,$3,now()) ON CONFLICT(user_id,course_id) DO NOTHING`, user, course, link)
	} else if !claimed {
		_, e = tx.ExecContext(ctx, `INSERT INTO course_referral_guest_claims(token_hash,course_id,referral_link_id) VALUES($1,$2,$3) ON CONFLICT(token_hash,course_id) DO NOTHING`, hash, course, link)
	}
	if e != nil {
		dbError(w, e)
		return
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO course_referral_visitors(referral_link_id,visitor) VALUES($1,$2) ON CONFLICT(referral_link_id,visitor) DO UPDATE SET visits=course_referral_visitors.visits+1,last_at=now() WHERE course_referral_visitors.last_at<now()-interval '2 seconds'`, link, visitor)
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		dbError(w, e)
		return
	}
	visitorCookie(w, r, token)
	w.WriteHeader(204)
}

func (h *Handler) courseInterest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if !referralSameOrigin(r) {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	var in struct {
		Slug   string `json:"slug"`
		Tariff int64  `json:"tariff_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Slug) > 200 || in.Tariff < 0 {
		fail(w, 400, "Некорректный курс или тариф")
		return
	}
	if isPreviewBot(r) {
		w.WriteHeader(204)
		return
	}
	user, ok := h.current(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		dbError(w, e)
		return
	}
	defer tx.Rollback()
	var course int64
	var pricing string
	e = tx.QueryRowContext(ctx, `SELECT c.id,c.pricing_mode FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.slug=$1 AND c.status='published' AND NOT u.is_blocked`, in.Slug).Scan(&course, &pricing)
	if e != nil {
		dbError(w, e)
		return
	}
	if in.Tariff > 0 {
		var valid bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM course_tariffs WHERE id=$1 AND course_id=$2)`, in.Tariff, course).Scan(&valid)
		if e != nil {
			dbError(w, e)
			return
		}
		if !valid || pricing != "tariffs" {
			fail(w, 400, "Тариф не принадлежит этому курсу")
			return
		}
	}
	var source sql.NullInt64
	e = tx.QueryRowContext(ctx, `SELECT referral_link_id FROM course_referral_attributions WHERE user_id=$1 AND course_id=$2`, user, course).Scan(&source)
	if e != nil && e != sql.ErrNoRows {
		dbError(w, e)
		return
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO course_interests(course_id,user_id,referral_link_id,tariff_id) VALUES($1,$2,$3,NULLIF($4,0)) ON CONFLICT(course_id,user_id) WHERE user_id IS NOT NULL DO UPDATE SET referral_link_id=COALESCE(course_interests.referral_link_id,EXCLUDED.referral_link_id)`, course, user, source, in.Tariff)
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		dbError(w, e)
		return
	}
	w.WriteHeader(204)
}

// Called after credential verification. The account binding is permanent and
// consumes a guest session so that a shared browser cannot reassign it later.
func BindReferralGuest(ctx context.Context, db *sql.DB, r *http.Request, user int64) error {
	hash := guestHash(r)
	if hash == "" {
		return nil
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var claimed sql.NullInt64
	e = tx.QueryRowContext(ctx, `SELECT claimed_by FROM course_referral_guests WHERE token_hash=$1 FOR UPDATE`, hash).Scan(&claimed)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if claimed.Valid {
		return nil
	}
	// Lock links in a stable order so deletion cannot race a new account binding.
	rows, e := tx.QueryContext(ctx, `SELECT l.id FROM course_referral_links l JOIN course_referral_guest_claims g ON g.referral_link_id=l.id WHERE g.token_hash=$1 AND l.deleted_at IS NULL ORDER BY l.id FOR SHARE OF l`, hash)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO course_referral_attributions(user_id,course_id,referral_link_id,first_clicked_at) SELECT $1,g.course_id,g.referral_link_id,g.first_clicked_at FROM course_referral_guest_claims g JOIN course_referral_links l ON l.id=g.referral_link_id WHERE g.token_hash=$2 AND l.deleted_at IS NULL ORDER BY g.course_id ON CONFLICT(user_id,course_id) DO NOTHING`, user, hash)
	if e != nil {
		return e
	}
	// Merge guest interest into the account without turning it into an enrollment.
	_, e = tx.ExecContext(ctx, `INSERT INTO course_interests(course_id,user_id,referral_link_id,tariff_id,first_clicked_at) SELECT i.course_id,$1,COALESCE(a.referral_link_id,i.referral_link_id),i.tariff_id,i.first_clicked_at FROM course_interests i LEFT JOIN course_referral_attributions a ON a.course_id=i.course_id AND a.user_id=$1 WHERE i.guest_hash=$2 AND i.user_id IS NULL ORDER BY i.course_id ON CONFLICT(course_id,user_id) WHERE user_id IS NOT NULL DO UPDATE SET first_clicked_at=LEAST(course_interests.first_clicked_at,EXCLUDED.first_clicked_at),referral_link_id=COALESCE(course_interests.referral_link_id,EXCLUDED.referral_link_id)`, user, hash)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `DELETE FROM course_interests WHERE guest_hash=$1 AND user_id IS NULL`, hash)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO course_referral_visitors(referral_link_id,visitor,visits,first_at,last_at) SELECT referral_link_id,$1,visits,first_at,last_at FROM course_referral_visitors WHERE visitor=$2 ORDER BY referral_link_id ON CONFLICT(referral_link_id,visitor) DO UPDATE SET visits=course_referral_visitors.visits+EXCLUDED.visits,first_at=LEAST(course_referral_visitors.first_at,EXCLUDED.first_at),last_at=GREATEST(course_referral_visitors.last_at,EXCLUDED.last_at)`, "u:"+strconv.FormatInt(user, 10), "g:"+hash)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `DELETE FROM course_referral_visitors WHERE visitor=$1`, "g:"+hash)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `UPDATE course_referral_guests SET claimed_by=$2 WHERE token_hash=$1`, hash, user)
	if e != nil {
		return e
	}
	return tx.Commit()
}
