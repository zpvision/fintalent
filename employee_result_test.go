package main

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmployeeResultReviewRequiresLogin(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/employee-testing/result-review/123", nil)
	w := httptest.NewRecorder()
	employeeTestingResultReview(w, r)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Войдите") {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("private result cache policy: %q", got)
	}
}

func TestEmployeeResultEmailEscapesContent(t *testing.T) {
	tmpl, err := template.New("result").Parse(employeeTestResultEmailTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, employeeTestResultEmailData{EmployeeName: `<script>alert(1)</script>`, ResultURL: "https://fintalent.ru/employee-result?invitation=123"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body.String(), "<script>alert(1)</script>") || !strings.Contains(body.String(), "&lt;script&gt;") {
		t.Fatal("email template did not escape employee name")
	}
}
