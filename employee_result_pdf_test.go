package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"FinTalent/internal/testmodule/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func employeePDFFixture() *domain.Attempt {
	finished := time.Date(2026, 10, 19, 10, 30, 0, 0, time.UTC)
	good, bad := true, false
	first, second := int64(11), int64(22)
	return &domain.Attempt{TestTitle: "Бухгалтерский учёт: основные средства и налоги", Status: "finished", Percent: 67, Score: 2, MaxScore: 3, DurationSeconds: 725, FinishedAt: &finished,
		Questions: []domain.Question{
			{ID: 1, Question: "Какой документ подтверждает принятие основного средства к учёту?", Answers: []domain.Answer{{ID: 11, Answer: "Акт о приёме-передаче основного средства", IsCorrect: true}, {ID: 12, Answer: "Платёжное поручение"}}, Explanation: "Принятие объекта к учёту оформляется первичным учётным документом."},
			{ID: 2, Question: "Что относится к первоначальной стоимости основного средства?", Answers: []domain.Answer{{ID: 21, Answer: "Затраты на доставку и монтаж", IsCorrect: true}, {ID: 22, Answer: "Расходы на рекламу"}}},
			{ID: 3, Type: domain.QuestionText, Question: "Объясните порядок начисления амортизации.", Explanation: "Учитывайте срок полезного использования и выбранный способ амортизации."},
		},
		Answers: []domain.AttemptAnswer{{QuestionID: 1, SelectedAnswerID: &first, IsCorrect: &good}, {QuestionID: 2, SelectedAnswerID: &second, IsCorrect: &bad}, {QuestionID: 3, TextAnswer: "Амортизация начисляется в течение срока полезного использования.", IsCorrect: &good}},
	}
}

func TestEmployeePDFRequiresLogin(t *testing.T) {
	w := httptest.NewRecorder()
	employeeTestingInvitationAction(w, httptest.NewRequest(http.MethodGet, "/api/employee-testing/invitations/1/results.pdf", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private results must not be cached")
	}
}

func TestEmployeePDFMaterialHTML(t *testing.T) {
	imageURL, err := employeePDFImage(materialPNG())
	if err != nil {
		t.Fatal(err)
	}
	fixture := employeePDFFixture()
	html, err := buildEmployeePDFHTML("Сотрудник", "employee@example.invalid", fixture, map[int64][]employeePDFMaterial{
		1: {{Name: "Диаграмма.png", Kind: "Изображение", Image: imageURL}, {Name: "Данные <script>.pdf", Kind: "Документ PDF"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"data:image/jpeg;base64,", "Диаграмма.png", "Данные &lt;script&gt;.pdf", "Документ PDF"} {
		if !bytes.Contains(html, []byte(want)) {
			t.Fatalf("missing %s", want)
		}
	}
	if bytes.Contains(html, []byte("application/pdf;base64")) || bytes.Contains(html, []byte("<iframe")) || bytes.Contains(html, []byte("Данные <script>")) {
		t.Fatal("PDF contents embedded or unescaped name")
	}
	if _, err = employeePDFImage([]byte("not an image")); err == nil {
		t.Fatal("invalid image accepted")
	}
}

func TestEmployeePDFWindowsDiscovery(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows browser locations")
	}
	root := t.TempDir()
	browser := filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe")
	if err := os.MkdirAll(filepath.Dir(browser), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(browser, []byte("MZ"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PDF_CHROMIUM_PATH", "")
	t.Setenv("PATH", "")
	t.Setenv("ProgramFiles", root)
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("LOCALAPPDATA", "")
	found, err := employeePDFBrowser()
	if err != nil || found != browser {
		t.Fatalf("browser discovery: %q, %v", found, err)
	}
}

func TestEmployeePDFIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_EMPLOYEE_PDF_DB_TESTS") != "1" {
		t.Skip("requires opt-in isolated cloud PostgreSQL test")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := fmt.Sprintf("employee_pdf_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "employee_pdf_test_") {
			t.Fatal("unsafe cleanup target")
		}
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	original := db
	db = isolated
	defer func() { db = original; isolated.Close() }()
	var actual string
	if err := isolated.QueryRowContext(ctx, "SELECT current_schema()").Scan(&actual); err != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	_, err = isolated.ExecContext(ctx, `CREATE TABLE company_test_employees(id BIGINT PRIMARY KEY,full_name TEXT,email TEXT);
		CREATE TABLE test_attempts(id BIGINT PRIMARY KEY,status TEXT);
		CREATE TABLE company_test_invitations(id BIGINT PRIMARY KEY,owner_user_id BIGINT,employee_id BIGINT,attempt_id BIGINT,status TEXT);
		INSERT INTO company_test_employees VALUES(1,'PDF fixture','pdf@example.invalid');
		INSERT INTO test_attempts VALUES(1,'finished'),(2,'started');
		INSERT INTO company_test_invitations VALUES(1,100,1,1,'finished'),(2,100,1,1,'sent'),(3,100,1,2,'finished');`)
	if err != nil {
		t.Fatal(err)
	}
	name, email, attempt, err := loadEmployeePDFInvitation(ctx, 100, 1)
	if err != nil || name != "PDF fixture" || email != "pdf@example.invalid" || attempt != 1 {
		t.Fatalf("owner result: %q %q %d %v", name, email, attempt, err)
	}
	for _, target := range []struct{ owner, invitation int64 }{{200, 1}, {100, 2}, {100, 3}, {100, 99}} {
		if _, _, _, err := loadEmployeePDFInvitation(ctx, target.owner, target.invitation); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("must reject %+v: %v", target, err)
		}
	}
}

func TestEmployeePDFHTML(t *testing.T) {
	fixture := employeePDFFixture()
	fixture.Questions[0].Question = `<img src="http://127.0.0.1/private"><script>alert(1)</script>`
	fixture.Answers[2].TextAnswer = `<iframe src="file:///etc/passwd"></iframe>`
	// Multi-select answers must retain every selected option.
	id := int64(12)
	fixture.Answers = append(fixture.Answers, domain.AttemptAnswer{QuestionID: 1, SelectedAnswerID: &id})
	html, err := buildEmployeePDFHTML(`<script>name</script>`, "anna@example.test", fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<script>", "<iframe", `<img src="http:`, "<canvas"} {
		if bytes.Contains(html, []byte(forbidden)) {
			t.Fatalf("unsafe/raster HTML: %s", forbidden)
		}
	}
	for _, expected := range []string{"&lt;script&gt;name&lt;/script&gt;", "data:image/png;base64,", "default-src", "Проверьте свои ответы", "right-answer", "wrong-answer", "Дата завершения: 19.10.2026 10:30 UTC"} {
		if !bytes.Contains(html, []byte(expected)) {
			t.Fatalf("missing: %s", expected)
		}
	}
	if strings.Count(string(html), "Выбран сотрудником") != 3 {
		t.Fatal("selected answers not preserved")
	}
}

func TestEmployeePDFChromium(t *testing.T) {
	if os.Getenv("PDF_CHROMIUM_PATH") == "" {
		t.Skip("set PDF_CHROMIUM_PATH to test actual HTML to PDF conversion")
	}
	fixture := employeePDFFixture()
	data, err := os.ReadFile("static/profimarket-ai-publish.png")
	if err != nil {
		t.Fatal(err)
	}
	imageURL, err := employeePDFImage(data)
	if err != nil {
		t.Fatal(err)
	}
	html, err := buildEmployeePDFHTML("Анна Смирнова", "anna@example.test", fixture, map[int64][]employeePDFMaterial{1: {{Name: "Изображение к вопросу.png", Kind: "Изображение", Image: imageURL}, {Name: "Материалы.pdf", Kind: "Документ PDF"}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if preview := os.Getenv("PDF_PREVIEW_DIR"); preview != "" {
		if err := os.MkdirAll(preview, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(preview, "employee-result.html"), html, 0600); err != nil {
			t.Fatal(err)
		}
	}
	pdf, err := renderEmployeeHTMLPDF(ctx, html)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("/Font")) || !bytes.Contains(pdf, []byte("/ToUnicode")) {
		t.Fatal("PDF must contain searchable Unicode fonts")
	}
	preview := os.Getenv("PDF_PREVIEW_DIR")
	if preview != "" {
		if err := os.MkdirAll(preview, 0700); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"employee-result.html": html, "FinTalent-result-example.pdf": pdf} {
			if err := os.WriteFile(filepath.Join(preview, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A card taller than A4 must flow across pages without losing its final line.
	fixture.Questions[2].Explanation = strings.Repeat("Длинное пояснение с кириллицей. ", 500) + "КОНЕЦ ДЛИННОГО ПОЯСНЕНИЯ"
	longHTML, err := buildEmployeePDFHTML("Анна Смирнова", "anna@example.test", fixture)
	if err != nil {
		t.Fatal(err)
	}
	longPDF, err := renderEmployeeHTMLPDF(ctx, longHTML)
	if err != nil {
		t.Fatal(err)
	}
	if preview != "" {
		if err := os.WriteFile(filepath.Join(preview, "FinTalent-result-long.pdf"), longPDF, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmployeePDFMissingChromium(t *testing.T) {
	t.Setenv("PDF_CHROMIUM_PATH", filepath.Join(t.TempDir(), "missing-chromium"))
	if _, err := renderEmployeeHTMLPDF(context.Background(), []byte("<p>test</p>")); err == nil {
		t.Fatal("missing Chromium must return an error")
	}
}
