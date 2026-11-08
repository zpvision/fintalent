package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"strconv"
	"strings"
)

func enqueueAcademyPaidInvitation(ctx context.Context, tx *sql.Tx, course, enrollment, student, interest int64) (int64, error) {
	var name, email, title string
	if err := tx.QueryRowContext(ctx, `SELECT u.full_name,u.email,c.title FROM users u CROSS JOIN courses c WHERE u.id=$1 AND c.id=$2 AND NOT u.is_blocked AND NOT u.is_system`, student, course).Scan(&name, &email, &title); err != nil {
		return 0, err
	}
	base := strings.TrimRight(applicationBaseURL(), "/")
	data := eventNotificationEmailData{RecipientName: name, Badge: "FINTALENT ACADEMY", Title: "Добро пожаловать на курс!", Intro: "Автор курса подтвердил получение вашей оплаты. Доступ к обучению открыт — можно начинать!", CardLabel: "ВАШ КУРС", CardTitle: title, Details: "Материалы, уроки и задания уже ждут вас в учебной комнате. Войдите в тот аккаунт FinTalent, с которого вы оставляли заявку.", ButtonText: "Начать обучение", ButtonURL: base + "/profile/learning/" + strconv.FormatInt(enrollment, 10), SecondaryText: "Все мои курсы", SecondaryURL: base + "/profile/learning", Accent: template.CSS("#2563df"), Footer: "Это приглашение подтверждает открытие доступа. Оплата принята автором курса вне сайта FinTalent."}
	key := fmt.Sprintf("academy-paid-interest:%d", interest)
	if err := enqueueNotification(ctx, tx, "event", name, email, "Доступ к курсу открыт — FinTalent Academy", data, key); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM notification_outbox WHERE dedup_key=$1`, key).Scan(&id)
	return id, err
}
