package academy

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type PaidInvitationEnqueuer func(context.Context, *sql.Tx, int64, int64, int64, int64) (int64, error)

func (h *Handler) SetPaidInvitationEnqueuer(fn PaidInvitationEnqueuer) { h.paidInvitation = fn }

func commissionBasisPoints(standard, partner int, referral bool) int {
	if referral {
		return partner
	}
	return standard
}

func (h *Handler) markInterestPaid(w http.ResponseWriter, r *http.Request, course, actor int64, path []string) {
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	if http.NewCrossOriginProtection().Check(r) != nil {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	if len(path) != 2 || path[1] != "paid" {
		fail(w, 404, "Заявка не найдена")
		return
	}
	id, err := strconv.ParseInt(path[0], 10, 64)
	if err != nil || id <= 0 {
		problem(w, ErrInvalid)
		return
	}
	var in struct {
		Confirm bool  `json:"confirm"`
		Version int64 `json:"version_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !in.Confirm || in.Version <= 0 {
		fail(w, 400, "Подтвердите получение оплаты и выберите опубликованную версию программы")
		return
	}
	if h.paidInvitation == nil {
		fail(w, 503, "Отправка приглашений временно недоступна")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := h.beginTx(ctx, nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	var months int
	err = tx.QueryRowContext(ctx, `SELECT access_months FROM courses WHERE id=$1 AND(owner_id=$2 OR $3) FOR UPDATE`, course, actor, adminPrincipal(ctx).Can("cohorts")).Scan(&months)
	if err != nil {
		problem(w, err)
		return
	}
	var student int64
	var paid sql.NullTime
	var enrollment sql.NullInt64
	var override sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT user_id,paid_at,paid_enrollment_id,commission_override_bps FROM course_interests WHERE id=$1 AND course_id=$2 AND user_id IS NOT NULL FOR UPDATE`, id, course).Scan(&student, &paid, &enrollment, &override)
	if err != nil {
		problem(w, err)
		return
	}
	if paid.Valid {
		respond(w, 200, map[string]any{"paid": true, "existing": true, "enrollment_id": enrollment.Int64})
		return
	}
	var source sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT referral_link_id FROM course_referral_attributions WHERE user_id=$1 AND course_id=$2`, student, course).Scan(&source)
	if err != nil && err != sql.ErrNoRows {
		problem(w, err)
		return
	}
	var standard, partner int
	err = tx.QueryRowContext(ctx, `SELECT standard_bps,partner_bps FROM course_commission_terms WHERE course_id=$1 FOR SHARE`, course).Scan(&standard, &partner)
	if err != nil {
		problem(w, err)
		return
	}
	// Reuse a current enrollment, including a cohort; never bypass suspended or
	// expired access by silently creating independent access over the same release.
	var eid int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 AND program_version_id=$3 AND access_status='active' AND(expires_at IS NULL OR expires_at>now()) ORDER BY id DESC LIMIT 1`, course, student, in.Version).Scan(&eid)
	if err == sql.ErrNoRows {
		var restricted bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_enrollments WHERE course_id=$1 AND user_id=$2 AND program_version_id=$3 AND(access_status<>'active' OR expires_at<=now()))`, course, student, in.Version).Scan(&restricted)
		if err != nil {
			problem(w, err)
			return
		}
		if restricted {
			fail(w, 409, "У ученика ограничен или истёк доступ к этой версии. Сначала восстановите его в карточке ученика")
			return
		}
		var raw []byte
		err = tx.QueryRowContext(ctx, `SELECT v.content FROM academy_current_program_versions v JOIN academy_current_version_releases rel ON rel.version_id=v.id WHERE rel.course_id=$1 AND v.id=$2`, course, in.Version).Scan(&raw)
		if err == sql.ErrNoRows {
			fail(w, 400, "Сначала опубликуйте выбранную версию учебной программы")
			return
		}
		if err != nil {
			problem(w, err)
			return
		}
		var program Program
		if err = json.Unmarshal(raw, &program); err != nil {
			problem(w, err)
			return
		}
		if len(publishedLessons(program, false)) == 0 || len(CheckReadiness(program).Blockers) > 0 {
			fail(w, 409, "Эта версия не готова к самостоятельному обучению. Для групповой программы сначала зачислите ученика в подходящий поток")
			return
		}
		var expires *time.Time
		if months > 0 {
			at := time.Now().AddDate(0, months, 0)
			expires = &at
		}
		eid, _, err = ensureIndependentEnrollment(ctx, tx, course, student, in.Version, actor, expires, "interest-paid:"+strconv.FormatInt(id, 10))
	}
	if err != nil {
		problem(w, err)
		return
	}
	var allowed bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN courses c ON c.id=$1 JOIN users owner ON owner.id=c.owner_id WHERE u.id=$2 AND NOT u.is_blocked AND NOT u.is_system AND NOT owner.is_blocked)`, course, student).Scan(&allowed)
	if err != nil {
		problem(w, err)
		return
	}
	if !allowed {
		fail(w, 409, "Ученик или автор курса заблокирован")
		return
	}
	queueID, err := h.paidInvitation(ctx, tx, course, eid, student, id)
	if err != nil {
		problem(w, err)
		return
	}
	bps := commissionBasisPoints(standard, partner, source.Valid)
	if override.Valid {
		bps = int(override.Int64)
	}
	_, err = tx.ExecContext(ctx, `UPDATE course_interests SET paid_at=now(),paid_by=NULLIF($3,0),paid_commission_bps=$4,paid_referral_link_id=$5,paid_enrollment_id=$6,invitation_outbox_id=$7 WHERE id=$1 AND course_id=$2 AND paid_at IS NULL`, id, course, actor, bps, source, eid, queueID)
	if err == nil {
		err = event(ctx, tx, &Enrollment{ID: eid}, actor, "external_payment_confirmed", "interest-paid:"+strconv.FormatInt(id, 10), "", map[string]any{"interest_id": id, "commission_bps": bps, "referral_link_id": nullable(source.Int64), "invitation_outbox_id": queueID})
	}
	if err == nil {
		err = commitAcademy(ctx, tx)
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"paid": true, "enrollment_id": eid, "invitation_status": "pending", "commission_percent": float64(bps) / 100})
}
