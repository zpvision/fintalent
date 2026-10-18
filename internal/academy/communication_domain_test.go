package academy

import (
	"strings"
	"testing"
)

func TestCommunicationRendererEscapesPersonalization(t *testing.T) {
	m := Message{Subject: "Здравствуйте {{student.first_name}}", Blocks: []EmailBlock{{Key: Key(), Type: "text", Rich: []RichBlock{{Type: "paragraph", Text: "<b>{{student.name}}</b><img src=x onerror=alert(1)>"}}}, {Key: Key(), Type: "button", Text: "Открыть", URL: "{{lesson.url}}"}}}
	v := sampleVariables("https://fintalent.ru", "<script>alert(1)</script>")
	subject, body, err := RenderMessage(m, v, "https://fintalent.ru", "https://fintalent.ru/academy/unsubscribe/test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "<script>") || strings.Contains(body, "<script>") || strings.Contains(body, "onerror") || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, `role="presentation"`) {
		t.Fatal("unsafe email", body)
	}
}
func TestCommunicationWhitelist(t *testing.T) {
	for _, c := range []Condition{{Field: "name; DROP TABLE users", Op: "eq", Value: "1"}, {Field: "progress", Op: "OR 1=1", Value: "1"}, {Field: "progress", Op: "lt", Value: "NaN"}, {Field: "progress", Op: "gt", Value: "101"}, {Field: "tag", Op: "eq", Value: "0"}} {
		if _, _, err := segmentSQL(Segment{Join: "and", Conditions: []Condition{c}}, 1, 2); err == nil {
			t.Fatal("unsafe condition", c)
		}
	}
	q, args, err := segmentSQL(Segment{Join: "or", Conditions: []Condition{{Field: "progress", Op: "lt", Value: "40"}, {Field: "activity_days", Op: "gt", Value: "5"}}}, 1, 2)
	if err != nil || !strings.Contains(q, " OR ") || len(args) != 4 || !strings.Contains(q, "academy_can_communicate") {
		t.Fatal(q, args, err)
	}
}
func TestCommunicationTemplatesAndUnsafeURLs(t *testing.T) {
	for _, v := range builtinTemplates() {
		m := v["message"].(Message)
		if err := ValidateMessage(&m, "https://fintalent.ru"); err != nil {
			t.Fatal(v["name"], err)
		}
	}
	for _, url := range []string{"javascript:alert(1)", "data:text/html,test", "//evil.test/path", "https://user:pass@example.org/"} {
		if communicationURL(url, "https://fintalent.ru", false) {
			t.Fatal(url)
		}
	}
	m := Message{Subject: "{{user.password}}", Blocks: []EmailBlock{{Key: Key(), Type: "text", Text: "Test"}}}
	if ValidateMessage(&m, "https://fintalent.ru") == nil {
		t.Fatal("unknown variable accepted")
	}
	if communicationURL("https://tracker.example/pixel", "https://fintalent.ru", true) {
		t.Fatal("external tracking image accepted")
	}
}
