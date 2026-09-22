package main

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestEventNotificationTemplateRendersAction(t *testing.T) {
	tmpl, err := template.New("event-notification").Parse(eventNotificationEmailTemplate)
	if err != nil {
		t.Fatalf("parse event email template: %v", err)
	}
	var rendered bytes.Buffer
	data := eventNotificationEmailData{
		RecipientName: "Анна",
		Badge:         "Помощь коллегам",
		Title:         "Запрос принят",
		Intro:         "Специалист ответил на запрос.",
		CardLabel:     "Ответ",
		CardTitle:     "Налоговый учёт",
		Details:       "Свяжитесь со мной по телефону.",
		ButtonText:    "Открыть запрос",
		ButtonURL:     "https://fintalent.ru/profile?section=help",
		Accent:        template.CSS("#0b986c"),
	}
	if err = tmpl.Execute(&rendered, data); err != nil {
		t.Fatalf("render event email template: %v", err)
	}
	html := rendered.String()
	for _, expected := range []string{"Анна", "Запрос принят", data.ButtonURL, "#0b986c"} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered email does not contain %q", expected)
		}
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Error("rendered email contains a sanitized unsafe template value")
	}
}
