package main

import (
	"bytes"
	"encoding/base64"
	htmlstd "html"
	"html/template"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"FinTalent/internal/courses"
)

func TestCourseInterestEmailTemplate(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://fintalent.ru/")
	event := courses.InterestNotification{CourseID: 42, StudentName: "Анна Петрова", Email: "student@example.invalid", Phone: "+79991234567", TariffName: "С сопровождением", Price: 1490000}
	data := courseInterestEmailData("Мария", "Финансовая модель: от данных к решениям", event)
	tmpl, err := template.New("event").Parse(eventNotificationEmailTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, data); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{data.CardTitle, event.StudentName, event.Email, event.Phone, event.TariffName, "14 900,00 ₽", "Перейти в Академию", "/profile/courses/42/academy/analytics?tab=students&amp;period=all", "КОНТАКТЫ УЧЕНИКА"} {
		if !strings.Contains(html.String(), expected) && !strings.Contains(htmlstd.UnescapeString(html.String()), expected) {
			t.Errorf("missing email content: %s", expected)
		}
	}
	if strings.Contains(html.String(), "ZgotmplZ") {
		t.Fatal("unsafe template value")
	}
	message, err := buildHTMLMessage(mail.Address{Address: "noreply@example.invalid"}, mail.Address{Address: "author@example.invalid"}, "Новый потенциальный ученик", html.Bytes())
	if err != nil || !bytes.Contains(bytes.ToLower(message), []byte("content-id: <fintalent-logo>")) {
		t.Fatal("email MIME or embedded logo failed", err)
	}
	if dir := os.Getenv("COURSE_INTEREST_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		preview := strings.ReplaceAll(html.String(), "cid:fintalent-logo", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(emailLogo))
		if err := os.WriteFile(filepath.Join(dir, "course-interest-email.html"), []byte(preview), 0600); err != nil {
			t.Fatal(err)
		}
	}
	data.CardTitle = `<script>alert('course')</script>`
	data.Facts[0].Value = `<img src=x onerror=alert('student')>`
	html.Reset()
	if err := tmpl.Execute(&html, data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html.String(), "<script>") || strings.Contains(html.String(), "<img src=x") || !strings.Contains(html.String(), "&lt;script&gt;") {
		t.Fatal("user content was not escaped")
	}
}

func TestCourseInterestPrice(t *testing.T) {
	for amount, want := range map[int64]string{0: "0,00 ₽", 1: "0,01 ₽", 199: "1,99 ₽", 150000: "1 500,00 ₽", 123456789: "1 234 567,89 ₽"} {
		if got := courseInterestPrice(amount); got != want {
			t.Errorf("price(%d) = %q, want %q", amount, got, want)
		}
	}
	data := courseInterestEmailData("Автор", "Бесплатный курс", courses.InterestNotification{})
	if !strings.Contains(data.Details, "Основной тариф") || !strings.Contains(data.Details, "0,00 ₽") {
		t.Fatal("single-price course label missing")
	}
}
