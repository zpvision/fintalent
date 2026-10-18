package academy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (h *Handler) communicationAccess(r *http.Request, course, user int64) bool {
	var yes bool
	_ = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=$2 WHERE c.id=$1 AND NOT u.is_blocked AND NOT u.is_system AND(c.owner_id=$2 OR EXISTS(SELECT 1 FROM academy_cohort_staff s JOIN academy_cohorts ch ON ch.id=s.cohort_id WHERE ch.course_id=c.id AND s.user_id=$2 AND s.can_communicate)))`, course, user).Scan(&yes)
	return yes
}
func (h *Handler) communicationRoute(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if !h.communicationAccess(r, course, user) {
		fail(w, 403, "Не выдано право управления коммуникациями")
		return
	}
	if len(path) == 0 {
		fail(w, 404, "Раздел не найден")
		return
	}
	switch path[0] {
	case "audience":
		h.communicationAudience(w, r, course, user)
	case "campaigns":
		h.campaigns(w, r, course, user, path[1:])
	case "templates":
		h.emailTemplates(w, r, course, user, path[1:])
	case "automations":
		h.automations(w, r, course, user, path[1:])
	case "deliveries":
		h.communicationHistory(w, r, course, user, path[1:])
	case "preview":
		h.messagePreview(w, r, course, user, false)
	case "test":
		h.messagePreview(w, r, course, user, true)
	default:
		fail(w, 404, "Раздел не найден")
	}
}
func (h *Handler) communicationAudience(w http.ResponseWriter, r *http.Request, course, user int64) {
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var b struct {
		Audience Segment `json:"audience"`
		Channel  string  `json:"channel"`
		Category string  `json:"category"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !oneOf(b.Channel, "email", "in_app") || !oneOf(b.Category, "service", "marketing") {
		problem(w, ErrInvalid)
		return
	}
	items, err := audience(r.Context(), h.db, b.Audience, course, user)
	if err != nil {
		problem(w, err)
		return
	}
	eligible := 0
	for _, p := range items {
		if recipientEligible(p, b.Channel, b.Category) {
			eligible++
		}
	}
	preview := items
	if len(preview) > 100 {
		preview = preview[:100]
	}
	respond(w, 200, map[string]any{"count": len(items), "eligible": eligible, "excluded": len(items) - eligible, "items": preview, "preview_limit": 100})
}
func (h *Handler) campaigns(w http.ResponseWriter, r *http.Request, course, user int64, path []string) {
	if len(path) == 0 && r.Method == "GET" {
		var raw json.RawMessage
		before := queryInt(r, "before")
		if before == 0 {
			before = 9223372036854775807
		}
		err := h.db.QueryRowContext(r.Context(), `WITH visible AS(SELECT c.*,d.total recipients,d.sent,d.queued,d.failed,d.sending FROM academy_campaigns c LEFT JOIN LATERAL(SELECT count(*) total,count(*) FILTER(WHERE status IN('sent','delivered')) sent,count(*) FILTER(WHERE status='queued') queued,count(*) FILTER(WHERE status='failed') failed,count(*) FILTER(WHERE status='sending') sending FROM academy_message_deliveries WHERE campaign_id=c.id)d ON true WHERE c.course_id=$1 AND c.deleted_at IS NULL AND(c.created_by=$2 OR EXISTS(SELECT 1 FROM courses WHERE id=$1 AND owner_id=$2))) SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id DESC) FROM(SELECT * FROM visible WHERE id<$3 AND($4='' OR status=$4) ORDER BY id DESC LIMIT 100)x),'[]'),'stats',(SELECT jsonb_build_object('total',count(*),'sent',COALESCE(sum(sent),0),'queued',COALESCE(sum(queued+sending),0),'failed',COALESCE(sum(failed),0)) FROM visible))`, course, user, before, r.URL.Query().Get("status")).Scan(&raw)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, raw)
		return
	}
	if len(path) == 0 && r.Method == "POST" {
		h.saveCampaign(w, r, course, user, nil)
		return
	}
	if len(path) == 0 {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	id, _ := strconv.ParseInt(path[0], 10, 64)
	c, err := loadCampaign(r.Context(), h.db, course, id, user, false)
	if err != nil {
		problem(w, err)
		return
	}
	if len(path) == 1 {
		switch r.Method {
		case "GET":
			respond(w, 200, c)
		case "PUT":
			h.saveCampaign(w, r, course, user, c)
		case "DELETE":
			tx, er := h.db.BeginTx(r.Context(), nil)
			if er != nil {
				problem(w, er)
				return
			}
			defer tx.Rollback()
			c, err = loadCampaign(r.Context(), tx, course, id, user, true)
			if err == nil && c.Status != "draft" {
				err = inputError{"Удалить можно только черновик"}
			}
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `UPDATE academy_campaigns SET deleted_at=now() WHERE id=$1`, id)
			}
			if err == nil {
				err = communicationAudit(r.Context(), tx, course, user, "campaign", id, "deleted", nil)
			}
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 200, map[string]bool{"deleted": true})
		default:
			fail(w, 405, "Метод не поддерживается")
		}
		return
	}
	if len(path) != 2 || r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	switch path[1] {
	case "duplicate":
		c.ID = 0
		if utf8.RuneCountInString(c.Name) > 170 {
			c.Name = string([]rune(c.Name)[:170])
		}
		c.Name += " · копия"
		c.Status = "draft"
		c.SendAt = nil
		c.Revision = 0
		c.RequestKey = Key()
		h.insertCampaign(w, r, course, user, c)
	case "launch":
		h.launchCampaign(w, r, c, user)
	case "cancel":
		tx, er := h.db.BeginTx(r.Context(), nil)
		if er != nil {
			problem(w, er)
			return
		}
		defer tx.Rollback()
		c, err = loadCampaign(r.Context(), tx, course, id, user, true)
		if err == nil && !oneOf(c.Status, "scheduled", "sending", "cancelled") {
			fail(w, 409, "Отменить можно запланированную или выполняющуюся рассылку")
			return
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_message_deliveries SET status='cancelled',error_code='campaign_cancelled' WHERE campaign_id=$1 AND status='queued'`, id)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE academy_campaigns SET status='cancelled',cancelled_at=now(),revision=revision+1 WHERE id=$1`, id)
		}
		if err == nil {
			err = communicationAudit(r.Context(), tx, course, user, "campaign", id, "cancelled", nil)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"cancelled": true})
	default:
		fail(w, 404, "Действие не найдено")
	}
}
func (h *Handler) validateCampaign(c *Campaign) error {
	c.Name = strings.TrimSpace(c.Name)
	if utf8.RuneCountInString(c.Name) < 1 || utf8.RuneCountInString(c.Name) > 180 || !oneOf(c.Channel, "email", "in_app") || !oneOf(c.Category, "service", "marketing") {
		return ErrInvalid
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil || c.Timezone == "" || c.Timezone == "Local" {
		return inputError{"Укажите временную зону IANA"}
	}
	if _, _, err := segmentSQL(c.Audience, c.CourseID, c.Creator); err != nil {
		return err
	}
	return ValidateMessage(&c.Message, h.base)
}
func (h *Handler) saveCampaign(w http.ResponseWriter, r *http.Request, course, user int64, old *Campaign) {
	var c Campaign
	if !decode(w, r, &c) {
		return
	}
	c.CourseID = course
	c.Creator = user
	if err := h.validateCampaign(&c); err != nil {
		problem(w, err)
		return
	}
	if old == nil {
		h.insertCampaign(w, r, course, user, &c)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	old, err = loadCampaign(r.Context(), tx, course, old.ID, user, true)
	if err != nil {
		problem(w, err)
		return
	}
	if old.Status != "draft" {
		fail(w, 409, "После запуска письмо зафиксировано. Создайте копию для изменений")
		return
	}
	if c.Revision != old.Revision {
		problem(w, ErrConflict)
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE academy_campaigns SET name=$2,message=$3,audience=$4,channel=$5,category=$6,timezone=$7,send_at=$8,revision=revision+1,updated_at=now() WHERE id=$1`, old.ID, c.Name, jsonValue(c.Message), jsonValue(c.Audience), c.Channel, c.Category, c.Timezone, c.SendAt)
	if err == nil {
		err = communicationAudit(r.Context(), tx, course, user, "campaign", old.ID, "updated", nil)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	c.ID = old.ID
	c.Status = "draft"
	c.Revision++
	respond(w, 200, c)
}
func (h *Handler) insertCampaign(w http.ResponseWriter, r *http.Request, course, user int64, c *Campaign) {
	if !keyPattern.MatchString(c.RequestKey) {
		problem(w, ErrInvalid)
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_campaigns(course_id,created_by,name,message,audience,channel,category,timezone,send_at,request_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(request_key) DO NOTHING RETURNING id`, course, user, c.Name, jsonValue(c.Message), jsonValue(c.Audience), c.Channel, c.Category, c.Timezone, c.SendAt, c.RequestKey).Scan(&c.ID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_campaigns WHERE request_key=$1 AND course_id=$2 AND created_by=$3`, c.RequestKey, course, user).Scan(&c.ID)
	}
	if err == nil {
		err = communicationAudit(r.Context(), tx, course, user, "campaign", c.ID, "created", nil)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	saved, err := loadCampaign(r.Context(), h.db, course, c.ID, user, false)
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 201, saved)
}
func (h *Handler) launchCampaign(w http.ResponseWriter, r *http.Request, c *Campaign, user int64) {
	var b struct {
		Confirm  bool  `json:"confirm"`
		Revision int64 `json:"revision"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !b.Confirm {
		fail(w, 400, "Подтвердите запуск рассылки")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	c, err = loadCampaign(r.Context(), tx, c.CourseID, c.ID, user, true)
	if err != nil {
		problem(w, err)
		return
	}
	if c.Status != "draft" {
		respond(w, 200, map[string]any{"id": c.ID, "status": c.Status})
		return
	}
	if c.Revision != b.Revision {
		problem(w, ErrConflict)
		return
	}
	// Only the creator's area can be sent, even when the owner launches a staff-created draft.
	items, err := audience(r.Context(), tx, c.Audience, c.CourseID, c.Creator)
	if err != nil {
		problem(w, err)
		return
	}
	due := time.Now()
	if c.SendAt != nil {
		due = *c.SendAt
		if due.Before(time.Now().Add(-time.Minute)) {
			fail(w, 400, "Выберите будущее время или отправку сейчас")
			return
		}
	}
	selected := []map[string]int64{}
	for _, p := range items {
		if recipientEligible(p, c.Channel, c.Category) {
			selected = append(selected, map[string]int64{"eid": p.EnrollmentID, "uid": p.UserID})
		}
	}
	count := len(selected)
	if count > 0 {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO academy_message_deliveries(course_id,actor_id,campaign_id,enrollment_id,recipient_id,channel,category,message,dedup_key,next_attempt_at) SELECT $1,$2,$3::bigint,x.eid,x.uid,$4,$5,$6,'campaign:'||($3::bigint)::text||':'||x.uid,$7 FROM jsonb_to_recordset($8::jsonb) AS x(eid bigint,uid bigint) ON CONFLICT(dedup_key) DO NOTHING`, c.CourseID, c.Creator, c.ID, c.Channel, c.Category, jsonValue(c.Message), due, jsonValue(selected))
		if err != nil {
			problem(w, err)
			return
		}
	}
	if count == 0 {
		fail(w, 400, "Нет получателей с действующим доступом и подходящими настройками уведомлений")
		return
	}
	status := "sending"
	if due.After(time.Now()) {
		status = "scheduled"
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE academy_campaigns SET status=$2,launched_at=now(),revision=revision+1 WHERE id=$1`, c.ID, status)
	if err == nil {
		err = communicationAudit(r.Context(), tx, c.CourseID, user, "campaign", c.ID, "launched", map[string]any{"recipients": count, "send_at": due})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 200, map[string]any{"id": c.ID, "status": status, "recipients": count})
}

func (h *Handler) messagePreview(w http.ResponseWriter, r *http.Request, course, user int64, test bool) {
	if r.Method != "POST" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	var b struct {
		Message Message `json:"message"`
		Email   string  `json:"email"`
	}
	if !decode(w, r, &b) {
		return
	}
	var name, email string
	err := h.db.QueryRowContext(r.Context(), `SELECT full_name,email FROM users WHERE id=$1 AND NOT is_blocked`, user).Scan(&name, &email)
	if err != nil {
		problem(w, err)
		return
	}
	subject, body, err := RenderMessage(b.Message, sampleVariables(h.base, name), h.base, "")
	if err != nil {
		problem(w, err)
		return
	}
	if !test {
		respond(w, 200, map[string]any{"html": body, "subject": subject, "variables": communicationVariables, "sample": true})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(b.Email), email) {
		fail(w, 400, "Тестовое письмо можно отправить только на email вашего аккаунта")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock($1)`, user)
	var count int
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM academy_message_deliveries WHERE actor_id=$1 AND test AND queued_at>now()-INTERVAL '1 hour'`, user).Scan(&count)
	}
	if err != nil {
		problem(w, err)
		return
	}
	if count >= 3 {
		fail(w, 429, "Не более трёх тестовых писем в час")
		return
	}
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO academy_message_deliveries(course_id,actor_id,recipient_id,channel,category,message,dedup_key,next_attempt_at,test) VALUES($1,$2,$2,'email','service',$3,$4,now(),true) RETURNING id`, course, user, jsonValue(b.Message), "test:"+Key()).Scan(&id)
	if err == nil {
		err = communicationAudit(r.Context(), tx, course, user, "test", id, "queued", nil)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		problem(w, err)
		return
	}
	respond(w, 202, map[string]any{"id": id, "status": "queued", "message": "Тестовое письмо поставлено в очередь. Результат появится в истории отправок"})
}

func (h *Handler) SetCommunicationBase(base string) { h.base = strings.TrimRight(base, "/") }
