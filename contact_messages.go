package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

//go:embed migrations/061_contact_messages.sql
var contactMessagesFS embed.FS

func prepareContactMessagesDatabase(ctx context.Context) error {
	schema, err := contactMessagesFS.ReadFile("migrations/061_contact_messages.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func registerContactMessageRoutes() {
	http.HandleFunc("/api/v1/contact-threads", contactThreads)
	http.HandleFunc("/api/v1/contact-threads/", contactThreadAction)
}

func contactThreads(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Войдите в аккаунт")
		return
	}
	if r.Method == http.MethodPost {
		createContactThread(w, r, u)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT t.id,t.subject,t.status,t.sender_id,t.recipient_id,t.updated_at,
		CASE WHEN t.sender_id=$1 THEN ru.full_name ELSE su.full_name END,
		CASE WHEN t.sender_id=$1 THEN COALESCE(ru.avatar_url,'') ELSE COALESCE(su.avatar_url,'') END,
		COALESCE((SELECT body FROM contact_messages WHERE thread_id=t.id ORDER BY id DESC LIMIT 1),''),
		(SELECT COUNT(*) FROM contact_messages WHERE thread_id=t.id),
		(SELECT COUNT(*) FROM contact_messages m WHERE m.thread_id=t.id AND m.author_id<>$1
			AND m.created_at>COALESCE(CASE WHEN t.sender_id=$1 THEN t.sender_read_at ELSE t.recipient_read_at END,'epoch'::timestamptz))
		FROM contact_threads t JOIN users su ON su.id=t.sender_id JOIN users ru ON ru.id=t.recipient_id
		WHERE t.sender_id=$1 OR t.recipient_id=$1 ORDER BY t.updated_at DESC`, u.ID)
	if err != nil {
		log.Printf("contact threads query: %v", err)
		writeJSON(w, 500, "Не удалось загрузить сообщения")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, sid, rid int64
		var subject, status, name, avatar, last string
		var updated any
		var count, unread int
		if err = rows.Scan(&id, &subject, &status, &sid, &rid, &updated, &name, &avatar, &last, &count, &unread); err != nil {
			writeJSON(w, 500, "Не удалось загрузить сообщения")
			return
		}
		items = append(items, map[string]any{"id": id, "subject": subject, "status": status, "sender_id": sid, "recipient_id": rid, "incoming": rid == u.ID, "person": map[string]any{"name": name, "avatar": avatar}, "last_message": last, "messages_count": count, "unread_count": unread, "updated_at": updated})
	}
	if err = rows.Err(); err != nil {
		writeJSON(w, 500, "Не удалось загрузить сообщения")
		return
	}
	writeAdminJSON(w, 200, items)
}

func createContactThread(w http.ResponseWriter, r *http.Request, u *user) {
	var p struct {
		ResumeID         int64 `json:"resume_id"`
		Subject, Message string
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&p) != nil {
		writeJSON(w, 400, "Некорректные данные")
		return
	}
	p.Subject = strings.TrimSpace(p.Subject)
	p.Message = strings.TrimSpace(p.Message)
	if len([]rune(p.Subject)) > 200 || p.ResumeID < 1 || len([]rune(p.Message)) < 10 || len([]rune(p.Message)) > 2000 {
		writeJSON(w, 400, "Напишите сообщение от 10 до 2000 символов")
		return
	}
	var recipient int64
	var recipientName, recipientEmail string
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось отправить запрос")
		return
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(r.Context(), `SELECT r.user_id,u.full_name,u.email FROM resumes r JOIN users u ON u.id=r.user_id WHERE r.id=$1 AND r.deleted_at IS NULL AND r.status='published' AND NOT u.is_blocked AND NOT u.is_system AND (r.visibility='public' OR EXISTS(SELECT 1 FROM resume_help_topics h JOIN help_topics ht ON ht.id=h.topic_id WHERE h.resume_id=r.id AND ht.is_active AND ht.deleted_at IS NULL)) FOR SHARE OF r,u`, p.ResumeID).Scan(&recipient, &recipientName, &recipientEmail); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("contact profile access: %v", err)
			writeJSON(w, 500, "Не удалось проверить профиль")
			return
		}
		writeJSON(w, 404, "Профиль недоступен")
		return
	}
	if recipient == u.ID {
		writeJSON(w, 400, "Нельзя отправить запрос самому себе")
		return
	}
	var sender int64
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM users WHERE id=$1 AND NOT is_blocked AND NOT is_system FOR SHARE`, u.ID).Scan(&sender); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, 401, "Войдите в аккаунт")
		} else {
			writeJSON(w, 500, "Не удалось проверить отправителя")
		}
		return
	}
	if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", fmt.Sprintf("contact:%d:%d", u.ID, recipient)); err != nil {
		writeJSON(w, 500, "Не удалось отправить запрос")
		return
	}
	var blocked bool
	err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM contact_threads WHERE sender_id=$1 AND recipient_id=$2 AND status='blocked')`, u.ID, recipient).Scan(&blocked)
	if err != nil {
		writeJSON(w, 500, "Не удалось проверить запрос")
		return
	}
	if blocked {
		writeJSON(w, 403, "Пользователь ограничил новые сообщения")
		return
	}
	var recent int
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM contact_threads WHERE sender_id=$1 AND recipient_id=$2 AND created_at>NOW()-INTERVAL '7 days'`, u.ID, recipient).Scan(&recent)
	if err != nil {
		writeJSON(w, 500, "Не удалось проверить запрос")
		return
	}
	if recent >= 2 {
		writeJSON(w, 429, "Можно отправить не более двух запросов этому специалисту за 7 дней")
		return
	}
	var id int64
	if err = tx.QueryRowContext(r.Context(), `INSERT INTO contact_threads(sender_id,recipient_id,resume_id,subject,sender_read_at) VALUES($1,$2,$3,$4,NOW()) RETURNING id`, u.ID, recipient, p.ResumeID, p.Subject).Scan(&id); err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO contact_messages(thread_id,author_id,body) VALUES($1,$2,$3)`, id, u.ID, p.Message)
	}
	if err == nil {
		data := eventNotificationEmailData{RecipientName: recipientName, Badge: "НОВЫЙ ЗАПРОС", Title: "С вами хотят связаться", Intro: u.FullName + " отправил(а) запрос через профессиональный профиль.", CardLabel: "Сообщение", CardTitle: p.Message, ButtonText: "Открыть сообщения", ButtonURL: applicationBaseURL() + "/profile?section=messages", Accent: template.CSS("#5b5bd6"), Footer: "Ответьте на запрос в личном кабинете FinTalent."}
		err = enqueueNotification(r.Context(), tx, "event", recipientName, recipientEmail, "Новый запрос на связь — FinTalent", data, "")
	}
	if err != nil || tx.Commit() != nil {
		writeJSON(w, 500, "Не удалось отправить запрос")
		return
	}
	writeAdminJSON(w, 201, map[string]any{"id": id, "message": "Запрос отправлен"})
}

func contactThreadAction(w http.ResponseWriter, r *http.Request) {
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, 401, "Войдите в аккаунт")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/contact-threads/"), "/"), "/")
	if len(parts) < 1 {
		writeJSON(w, 404, "Не найдено")
		return
	}
	id, _ := strconv.ParseInt(parts[0], 10, 64)
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	var sid, rid int64
	var status string
	if db.QueryRowContext(r.Context(), `SELECT sender_id,recipient_id,status FROM contact_threads WHERE id=$1`, id).Scan(&sid, &rid, &status) != nil || (u.ID != sid && u.ID != rid) {
		writeJSON(w, 404, "Диалог не найден")
		return
	}
	if r.Method == http.MethodGet && action == "messages" {
		_, _ = db.ExecContext(r.Context(), `UPDATE contact_threads SET
			sender_read_at=CASE WHEN sender_id=$2 THEN NOW() ELSE sender_read_at END,
			recipient_read_at=CASE WHEN recipient_id=$2 THEN NOW() ELSE recipient_read_at END
			WHERE id=$1`, id, u.ID)
		rows, e := db.QueryContext(r.Context(), `SELECT m.id,m.author_id,u.full_name,m.body,m.created_at FROM contact_messages m JOIN users u ON u.id=m.author_id WHERE m.thread_id=$1 ORDER BY m.id`, id)
		if e != nil {
			writeJSON(w, 500, "Не удалось загрузить переписку")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var mid, aid int64
			var name, body string
			var at any
			if e = rows.Scan(&mid, &aid, &name, &body, &at); e != nil {
				writeJSON(w, 500, "Не удалось загрузить переписку")
				return
			}
			out = append(out, map[string]any{"id": mid, "author_id": aid, "author_name": name, "body": body, "created_at": at, "mine": aid == u.ID})
		}
		if e = rows.Err(); e != nil {
			writeJSON(w, 500, "Не удалось загрузить переписку")
			return
		}
		writeAdminJSON(w, 200, out)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, "Не удалось выполнить действие")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", fmt.Sprintf("contact:%d:%d", sid, rid)); err != nil {
		writeJSON(w, 500, "Не удалось выполнить действие")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT t.sender_id,t.recipient_id,t.status FROM contact_threads t JOIN users su ON su.id=t.sender_id JOIN users ru ON ru.id=t.recipient_id WHERE t.id=$1 AND (t.sender_id=$2 OR t.recipient_id=$2) AND NOT su.is_blocked AND NOT ru.is_blocked FOR UPDATE OF t FOR SHARE OF su,ru`, id, u.ID).Scan(&sid, &rid, &status); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("contact action access: %v", err)
			writeJSON(w, 500, "Не удалось выполнить действие")
			return
		}
		writeJSON(w, 404, "Диалог недоступен")
		return
	}
	var p struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&p)
	switch action {
	case "accept", "decline", "block":
		allowed := u.ID == rid && (status == "pending" || (action == "block" && status == "accepted"))
		if !allowed {
			writeJSON(w, 409, "Действие недоступно")
			return
		}
		next := map[string]string{"accept": "accepted", "decline": "declined", "block": "blocked"}[action]
		_, err = tx.ExecContext(r.Context(), `UPDATE contact_threads SET status=$1,updated_at=NOW() WHERE id=$2`, next, id)
	case "messages":
		if status != "accepted" {
			writeJSON(w, 409, "Переписка ещё не открыта")
			return
		}
		p.Message = strings.TrimSpace(p.Message)
		if len([]rune(p.Message)) < 1 || len([]rune(p.Message)) > 4000 {
			writeJSON(w, 400, "Сообщение должно быть от 1 до 4000 символов")
			return
		}
		_, err = tx.ExecContext(r.Context(), `WITH ins AS (INSERT INTO contact_messages(thread_id,author_id,body) VALUES($1,$2,$3) RETURNING 1) UPDATE contact_threads SET updated_at=NOW() WHERE id=$1 AND EXISTS(SELECT 1 FROM ins)`, id, u.ID, p.Message)
	case "report":
		p.Reason = strings.TrimSpace(p.Reason)
		if len([]rune(p.Reason)) < 5 || len([]rune(p.Reason)) > 1000 {
			writeJSON(w, 400, "Опишите причину жалобы")
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO contact_reports(thread_id,reporter_id,reason) VALUES($1,$2,$3) ON CONFLICT(thread_id,reporter_id) DO UPDATE SET reason=EXCLUDED.reason,created_at=NOW()`, id, u.ID, p.Reason)
		if err == nil {
			data := eventNotificationEmailData{RecipientName: "Команда FinTalent", Badge: "ЖАЛОБА", Title: "Жалоба на переписку", Intro: fmt.Sprintf("Диалог №%d", id), CardLabel: "Причина", CardTitle: p.Reason, ButtonText: "Открыть FinTalent", ButtonURL: applicationBaseURL() + "/admin"}
			err = enqueueNotification(r.Context(), tx, "event", "FinTalent", "info@fintalent.ru", "Жалоба на переписку — FinTalent", data, "")
		}
	default:
		writeJSON(w, 404, "Действие не найдено")
		return
	}
	if err != nil {
		writeJSON(w, 500, "Не удалось выполнить действие")
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, "Не удалось выполнить действие")
		return
	}
	writeAdminJSON(w, 200, map[string]string{"message": "Готово"})
}

func sendContactReportEmail(threadID int64, reporter *user, reason string) {
	data := eventNotificationEmailData{RecipientName: "Команда FinTalent", Badge: "ЖАЛОБА", Title: "Жалоба на переписку", Intro: fmt.Sprintf("Пользователь %s (%s) пожаловался на диалог №%d.", reporter.FullName, reporter.Email, threadID), CardLabel: "Причина", CardTitle: reason, ButtonText: "Открыть FinTalent", ButtonURL: applicationBaseURL() + "/admin", Accent: template.CSS("#dc3545"), Footer: "Проверьте переписку и примите меры."}
	if err := sendEventNotificationEmail("FinTalent", "info@fintalent.ru", "Жалоба на переписку — FinTalent", data); err != nil {
		log.Printf("contact report email: %v", err)
	}
}
