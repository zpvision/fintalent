package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"FinTalent/internal/courses"
)

func enqueueCourseInterestNotification(ctx context.Context, tx *sql.Tx, event courses.InterestNotification) error {
	var name, email, title string
	err := tx.QueryRowContext(ctx, `SELECT u.full_name,u.email,c.title FROM courses c JOIN users u ON u.id=c.owner_id WHERE c.id=$1 AND NOT u.is_blocked AND NOT u.is_system`, event.CourseID).Scan(&name, &email, &title)
	if err == sql.ErrNoRows {
		return nil // System/demo authors do not receive mail.
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(email) == "" {
		return nil
	}
	data := courseInterestEmailData(name, title, event)
	// The application ID survives repeated clicks and contact/tariff edits.
	// Admission and the application commit together, so retries do not lose mail.
	return enqueueNotification(ctx, tx, "event", name, email, "Новый потенциальный ученик — FinTalent Academy", data, fmt.Sprintf("course-interest-author:%d", event.InterestID))
}

func courseInterestEmailData(author, title string, event courses.InterestNotification) eventNotificationEmailData {
	tariff := event.TariffName
	if strings.TrimSpace(tariff) == "" {
		tariff = "Основной тариф"
	}
	return eventNotificationEmailData{
		RecipientName: author,
		Badge:         "FINTALENT ACADEMY · НОВАЯ ЗАЯВКА",
		Title:         "У вас новый потенциальный ученик!",
		Intro:         "Пользователь выбрал ваш курс и оставил контакты. Свяжитесь с ним, чтобы ответить на вопросы и обсудить обучение.",
		CardLabel:     "ВАШ КУРС",
		CardTitle:     title,
		Details:       fmt.Sprintf("Выбранный тариф: %s\nСтоимость: %s", tariff, courseInterestPrice(event.Price)),
		FactsTitle:    "КОНТАКТЫ УЧЕНИКА",
		Facts:         []eventNotificationEmailFact{{Label: "ФИО", Value: event.StudentName}, {Label: "Email", Value: event.Email}, {Label: "Телефон", Value: event.Phone}},
		ButtonText:    "Перейти в Академию",
		ButtonURL:     strings.TrimRight(applicationBaseURL(), "/") + "/profile/courses/" + strconv.FormatInt(event.CourseID, 10) + "/academy/analytics?tab=students&period=all",
		Accent:        template.CSS("#2563df"),
		Footer:        "Заявка доступна в Академии: аналитика курса → «Ученики». Это заявка на обучение, а не подтверждение оплаты.",
	}
}

func courseInterestPrice(kopecks int64) string {
	whole := strconv.FormatInt(kopecks/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + " " + whole[i:]
	}
	return fmt.Sprintf("%s,%02d ₽", whole, kopecks%100)
}
