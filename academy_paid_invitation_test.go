package main

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestAcademyInvitationTemplateEscapesAndLinks(t *testing.T) {
	tmpl, err := template.New("invitation").Parse(eventNotificationEmailTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	err = tmpl.Execute(&rendered, eventNotificationEmailData{Title: "Добро пожаловать на курс!", RecipientName: "Ученик", CardTitle: "<script>bad</script>", Accent: template.CSS("#2563df"), ButtonText: "Начать обучение", ButtonURL: "https://fintalent.ru/profile/learning/42", SecondaryText: "Все мои курсы", SecondaryURL: "https://fintalent.ru/profile/learning"})
	if err != nil {
		t.Fatal(err)
	}
	html := rendered.String()
	for _, s := range []string{`href="https://fintalent.ru/profile/learning/42"`, `href="https://fintalent.ru/profile/learning"`, `&lt;script&gt;`} {
		if !strings.Contains(html, s) {
			t.Fatal("invitation missing safe content or learning links")
		}
	}
	if strings.Contains(html, "<script>bad") {
		t.Fatal("unescaped course title")
	}
}
