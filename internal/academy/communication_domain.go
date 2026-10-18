package academy

import (
	"fmt"
	"github.com/microcosm-cc/bluemonday"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type EmailBlock struct {
	Key    string      `json:"key"`
	Type   string      `json:"type"`
	Text   string      `json:"text"`
	URL    string      `json:"url,omitempty"`
	Align  string      `json:"align"`
	Color  string      `json:"color,omitempty"`
	Rich   []RichBlock `json:"rich,omitempty"`
	Height int         `json:"height,omitempty"`
}
type Message struct {
	Subject   string       `json:"subject"`
	Preheader string       `json:"preheader"`
	Blocks    []EmailBlock `json:"blocks"`
}
type Segment struct {
	Mode        string      `json:"mode,omitempty"`
	Join        string      `json:"join"`
	Conditions  []Condition `json:"conditions"`
	Cohorts     []int64     `json:"cohort_ids"`
	Enrollments []int64     `json:"enrollment_ids"`
}
type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}
type CommunicationStep struct {
	Key          string  `json:"key"`
	Channel      string  `json:"channel"`
	Category     string  `json:"category"`
	Target       string  `json:"target"`
	Timing       string  `json:"timing"`
	DelayMinutes int     `json:"delay_minutes"`
	At           string  `json:"at,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
	Message      Message `json:"message"`
}
type AutomationConfig struct {
	Audience     Segment             `json:"audience"`
	Steps        []CommunicationStep `json:"steps"`
	InactiveDays int                 `json:"inactive_days"`
	LeadDays     int                 `json:"lead_days"`
}
type Campaign struct {
	ID         int64      `json:"id"`
	CourseID   int64      `json:"course_id"`
	Creator    int64      `json:"created_by"`
	Name       string     `json:"name"`
	Message    Message    `json:"message"`
	Audience   Segment    `json:"audience"`
	Channel    string     `json:"channel"`
	Category   string     `json:"category"`
	Timezone   string     `json:"timezone"`
	SendAt     *time.Time `json:"send_at"`
	Status     string     `json:"status"`
	Revision   int64      `json:"revision"`
	RequestKey string     `json:"request_key"`
}
type Automation struct {
	ID          int64            `json:"id"`
	CourseID    int64            `json:"course_id"`
	Creator     int64            `json:"created_by"`
	Name        string           `json:"name"`
	Trigger     string           `json:"trigger"`
	Config      AutomationConfig `json:"config"`
	Status      string           `json:"status"`
	Revision    int64            `json:"revision"`
	Generation  int64            `json:"generation"`
	ActiveSince *time.Time       `json:"active_since"`
	RequestKey  string           `json:"request_key"`
}

var communicationVariables = []string{"student.first_name", "student.name", "course.title", "cohort.name", "teacher.name", "curator.name", "progress.percent", "course.url", "lesson.url", "assignment.title", "deadline", "cohort.starts_at"}
var variablePattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func interpolate(s string, variables map[string]string, escape bool) string {
	return variablePattern.ReplaceAllStringFunc(s, func(token string) string {
		key := strings.TrimSpace(variablePattern.FindStringSubmatch(token)[1])
		v := variables[key]
		if escape {
			return html.EscapeString(v)
		}
		return v
	})
}
func validateVariables(s string) error {
	for _, v := range variablePattern.FindAllStringSubmatch(s, -1) {
		found := false
		for _, key := range communicationVariables {
			if strings.TrimSpace(v[1]) == key {
				found = true
			}
		}
		if !found {
			return inputError{"Неизвестная переменная: " + v[1]}
		}
	}
	return nil
}
func communicationURL(value, base string, image bool) bool {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || len(value) > 2000 {
		return false
	}
	origin, _ := url.Parse(base)
	if u.Scheme == "" && u.Host == "" && strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(value, "//") {
		u = origin.ResolveReference(u)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		return false
	}
	if u.Hostname() == "" {
		return false
	}
	return !image || (u.Host == origin.Host && (strings.HasPrefix(u.Path, "/uploads/") || strings.HasPrefix(u.Path, "/static/") || strings.HasPrefix(u.Path, "/api/images/")))
}
func ValidateMessage(m *Message, base string) error {
	m.Subject = strings.TrimSpace(m.Subject)
	if utf8.RuneCountInString(m.Subject) < 1 || utf8.RuneCountInString(m.Subject) > 200 || strings.ContainsAny(m.Subject, "\r\n") || utf8.RuneCountInString(m.Preheader) > 300 || len(m.Blocks) < 1 || len(m.Blocks) > 60 {
		return inputError{"Заполните тему (до 200 символов) и от 1 до 60 блоков письма"}
	}
	seen := map[string]bool{}
	samples := sampleVariables(base, "Автор")
	if err := validateVariables(m.Subject + m.Preheader); err != nil {
		return err
	}
	for i := range m.Blocks {
		b := &m.Blocks[i]
		if !keyPattern.MatchString(b.Key) || seen[b.Key] || !oneOf(b.Type, "heading", "text", "image", "button", "divider", "spacer", "info", "signature") {
			return inputError{"Проверьте типы и идентификаторы блоков"}
		}
		seen[b.Key] = true
		if b.Align == "" {
			b.Align = "left"
		}
		if !oneOf(b.Align, "left", "center", "right") || len(b.Text) > 50000 || len(b.Rich) > 200 || b.Height < 0 || b.Height > 120 {
			return ErrInvalid
		}
		if b.Color == "" {
			b.Color = "#3569e8"
		}
		if !colorPattern.MatchString(b.Color) {
			return inputError{"Цвет задаётся шестизначным HEX-кодом"}
		}
		if err := validateVariables(b.Text + b.URL + jsonValue(b.Rich)); err != nil {
			return err
		}
		for j := range b.Rich {
			r := &b.Rich[j]
			if !oneOf(r.Type, "paragraph", "header", "list", "quote", "table") {
				return inputError{"Неподдерживаемый формат текста"}
			}
			r.Text = inlinePolicy.Sanitize(r.Text)
			for k := range r.Items {
				r.Items[k] = inlinePolicy.Sanitize(r.Items[k])
			}
			for k := range r.Rows {
				for n := range r.Rows[k] {
					r.Rows[k][n] = inlinePolicy.Sanitize(r.Rows[k][n])
				}
			}
		}
		if oneOf(b.Type, "button", "image") && !communicationURL(interpolate(b.URL, samples, false), base, b.Type == "image") {
			return inputError{"Используйте безопасную ссылку HTTPS. Изображения — только из публичного хранилища FinTalent"}
		}
	}
	return nil
}
func sampleVariables(base, name string) map[string]string {
	return map[string]string{"student.first_name": strings.Fields(name + " Автор")[0], "student.name": name, "course.title": "Ваш учебный курс", "cohort.name": "Ваш поток", "teacher.name": "Преподаватель", "curator.name": "Куратор", "progress.percent": "40", "course.url": base + "/profile/learning", "lesson.url": base + "/profile/learning", "assignment.title": "Практическое задание", "deadline": "срок не установлен", "cohort.starts_at": "дата не указана"}
}
func RenderMessage(m Message, variables map[string]string, base, unsubscribe string) (string, string, error) {
	if err := ValidateMessage(&m, base); err != nil {
		return "", "", err
	}
	subject := interpolate(m.Subject, variables, false)
	if strings.ContainsAny(subject, "\r\n") {
		return "", "", ErrInvalid
	}
	var out strings.Builder
	out.WriteString(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;background:#f3f6fc;color:#233958;font-family:Arial,sans-serif"><div style="display:none;max-height:0;overflow:hidden;mso-hide:all">` + html.EscapeString(interpolate(m.Preheader, variables, false)) + `</div><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center" style="padding:24px 12px"><table role="presentation" width="600" cellspacing="0" cellpadding="0" style="width:100%;max-width:600px;background:white;border-radius:16px"><tr><td style="padding:28px 28px 14px;color:#3569e8;font-weight:bold;letter-spacing:2px">FinTalent Academy</td></tr>`)
	for _, b := range m.Blocks {
		align := b.Align
		text := html.EscapeString(interpolate(b.Text, variables, false))
		out.WriteString(`<tr><td align="` + align + `" style="padding:12px 28px;text-align:` + align + `;font-size:16px;line-height:1.7;overflow-wrap:anywhere">`)
		switch b.Type {
		case "heading":
			out.WriteString(`<h1 style="font-size:26px;line-height:1.3;margin:0;color:#233958">` + text + `</h1>`)
		case "text":
			out.WriteString(strings.ReplaceAll(text, "\n", "<br>"))
			for _, r := range b.Rich {
				out.WriteString(renderEmailRich(r, variables))
			}
		case "info":
			out.WriteString(`<table role="presentation" width="100%"><tr><td style="padding:18px;background:#edf4ff;border-left:3px solid #3569e8;border-radius:8px">` + strings.ReplaceAll(text, "\n", "<br>") + `</td></tr></table>`)
		case "signature":
			out.WriteString(`<p style="color:#71809a;border-top:1px solid #e4ebf6;padding-top:18px">` + strings.ReplaceAll(text, "\n", "<br>") + `</p>`)
		case "divider":
			out.WriteString(`<hr style="border:0;border-top:1px solid #e4ebf6">`)
		case "spacer":
			out.WriteString(`<div style="height:` + strconv.Itoa(b.Height) + `px;line-height:1px">&nbsp;</div>`)
		case "image", "button":
			link := interpolate(b.URL, variables, false)
			if !communicationURL(link, base, b.Type == "image") {
				return "", "", inputError{"Ссылка после подстановки переменных недопустима"}
			}
			u, _ := url.Parse(link)
			origin, _ := url.Parse(base)
			link = origin.ResolveReference(u).String()
			if b.Type == "image" {
				out.WriteString(`<img src="` + html.EscapeString(link) + `" alt="` + text + `" width="544" style="width:100%;max-width:544px;height:auto;border:0;border-radius:10px">`)
			} else {
				out.WriteString(`<table role="presentation" cellspacing="0" cellpadding="0" align="` + align + `"><tr><td bgcolor="` + b.Color + `" style="border-radius:9px"><a href="` + html.EscapeString(link) + `" style="display:inline-block;padding:14px 24px;color:white;text-decoration:none;font-weight:bold">` + text + `</a></td></tr></table>`)
			}
		}
		out.WriteString(`</td></tr>`)
	}
	out.WriteString(`<tr><td style="padding:28px;color:#8b97ac;font-size:12px;line-height:1.6">Письмо отправлено через FinTalent Academy.`)
	if unsubscribe != "" {
		out.WriteString(`<br><a href="` + html.EscapeString(unsubscribe) + `" style="color:#7184a4">Настроить уведомления или отписаться</a>`)
	}
	out.WriteString(`</td></tr></table></td></tr></table></body></html>`)
	return subject, out.String(), nil
}
func renderEmailRich(r RichBlock, vars map[string]string) string {
	safe := func(s string) string { return inlinePolicy.Sanitize(interpolate(s, vars, true)) }
	switch r.Type {
	case "header":
		return "<h2>" + safe(r.Text) + "</h2>"
	case "quote":
		return `<blockquote style="border-left:3px solid #b9cdf5;padding-left:16px;color:#71809a">` + safe(r.Text) + "</blockquote>"
	case "list":
		s := "<ul>"
		for _, v := range r.Items {
			s += "<li>" + safe(v) + "</li>"
		}
		return s + "</ul>"
	case "table":
		s := `<table width="100%" cellpadding="8" cellspacing="0" style="border-collapse:collapse">`
		for _, row := range r.Rows {
			s += "<tr>"
			for _, v := range row {
				s += `<td style="border:1px solid #e2e9f4">` + safe(v) + `</td>`
			}
			s += "</tr>"
		}
		return s + "</table>"
	}
	return "<p>" + safe(r.Text) + "</p>"
}
func validateCommunicationStep(s *CommunicationStep, base string) error {
	if !keyPattern.MatchString(s.Key) || !oneOf(s.Channel, "email", "in_app") || !oneOf(s.Category, "service", "marketing") || !oneOf(s.Target, "student", "curator", "owner") || !oneOf(s.Timing, "immediate", "delay", "before_deadline", "at") || s.DelayMinutes < 0 || s.DelayMinutes > 525600 {
		return ErrInvalid
	}
	if s.Timezone != "" {
		if _, err := time.LoadLocation(s.Timezone); err != nil {
			return inputError{"Неизвестная временная зона"}
		}
	}
	if s.Timing == "at" {
		if _, err := time.Parse(time.RFC3339, s.At); err != nil {
			return inputError{"Укажите абсолютное время действия"}
		}
	}
	return ValidateMessage(&s.Message, base)
}
func builtinTemplates() []map[string]any {
	items := []map[string]any{}
	for i, v := range [][3]string{{"Добро пожаловать", "Добро пожаловать на {{course.title}}", "Здравствуйте, {{student.first_name}}! Ваше обучение уже доступно. Начните с первого урока."}, {"Старт завтра", "Скоро начнётся {{cohort.name}}", "Подготовьтесь к обучению. Дата старта: {{cohort.starts_at}}."}, {"Открыт модуль", "Новые материалы курса {{course.title}}", "Для вас открыт следующий этап обучения. Продолжайте с того места, где остановились."}, {"Напоминание о задании", "Практика: {{assignment.title}}", "Напоминаем о задании. Срок: {{deadline}}. Нужна помощь? Обратитесь к куратору {{curator.name}}."}, {"Работа проверена", "Результат вашей практической работы", "Преподаватель проверил вашу работу. Откройте учебную комнату, чтобы увидеть оценку и комментарий."}, {"На доработку", "Есть обратная связь по вашей работе", "Прочитайте рекомендации преподавателя и отправьте исправленную работу."}, {"Возвращайтесь", "Продолжим обучение, {{student.first_name}}?", "Ваш прогресс — {{progress.percent}}%. Сделайте следующий небольшой шаг к своей цели."}, {"Курс завершён", "Поздравляем с завершением {{course.title}}!", "Вы завершили обучение. Спасибо за вашу работу и стремление развиваться."}} {
		items = append(items, map[string]any{"id": fmt.Sprintf("builtin-%d", i+1), "name": v[0], "builtin": true, "message": Message{Subject: v[1], Preheader: v[0], Blocks: []EmailBlock{{Key: fmt.Sprintf("heading-%d", i), Type: "heading", Text: v[0], Align: "left"}, {Key: fmt.Sprintf("message-%d", i), Type: "text", Text: v[2], Align: "left"}, {Key: fmt.Sprintf("action--%d", i), Type: "button", Text: "Открыть обучение", URL: "{{lesson.url}}", Color: "#3569e8", Align: "left"}, {Key: fmt.Sprintf("signature-%d", i), Type: "signature", Text: "С уважением, {{teacher.name}}\nFinTalent Academy", Align: "left"}}}})
	}
	return items
}

// A notification carries the authored message, not just the email preheader.
func communicationPlainText(m Message, vars map[string]string) string {
	parts := []string{}
	for _, b := range m.Blocks {
		if b.Text != "" {
			parts = append(parts, interpolate(b.Text, vars, false))
		}
		for _, r := range b.Rich {
			parts = append(parts, html.UnescapeString(bluemonday.StrictPolicy().Sanitize(interpolate(r.Text, vars, true))))
			for _, i := range r.Items {
				parts = append(parts, html.UnescapeString(bluemonday.StrictPolicy().Sanitize(interpolate(i, vars, true))))
			}
			for _, row := range r.Rows {
				for _, cell := range row {
					parts = append(parts, html.UnescapeString(bluemonday.StrictPolicy().Sanitize(interpolate(cell, vars, true))))
				}
			}
		}
		if b.Type == "button" && b.URL != "" {
			parts = append(parts, interpolate(b.URL, vars, false))
		}
	}
	text := []rune(strings.Join(parts, "\n\n"))
	if len(text) > 4000 {
		text = text[:4000]
	}
	return string(text)
}
