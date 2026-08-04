package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSlugifyRussianTitle(t *testing.T) {
	got := slugify("5 ошибок при переходе на УСН в 2025 году")
	if got != "5-oshibok-pri-perehode-na-usn-v-2025-godu" {
		t.Fatalf("unexpected slug: %s", got)
	}
}

func TestPublicationDatabaseIntegration(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run PostgreSQL integration test")
	}
	loadLocalEnv(".env")
	var err error
	db, err = sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = preparePublicationDatabase(ctx); err != nil {
		t.Fatal(err)
	}
	if err = preparePublicationDemo(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	var content string
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(content_html),'') FROM publications WHERE slug='5-oshibok-pri-perehode-na-usn-v-2025-godu' AND status='published'`).Scan(&count, &content); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(content, "Неправильная оценка доходов") {
		t.Fatalf("demo publication invalid: count=%d content=%d", count, len(content))
	}
}

func TestRenderPublicationBlocksEscapesUserContent(t *testing.T) {
	html := renderPublicationBlocks([]publicationBlock{{Type: "paragraph", Text: `<script>alert(1)</script>`}, {Type: "image", URL: `javascript:alert(1)`, Caption: `<img onerror=x>`}})
	if strings.Contains(html, "<script") || strings.Contains(html, "javascript:") || strings.Contains(html, "onerror=") {
		t.Fatalf("unsafe content rendered: %s", html)
	}
}

func TestRenderPublicationBlocksKeepsSafeInlineFormatting(t *testing.T) {
	html := renderPublicationBlocks([]publicationBlock{{Type: "paragraph", Text: `<strong>Важное</strong> и <a href="https://example.com">ссылка</a><img src=x onerror=alert(1)>`}})
	if !strings.Contains(html, "<strong>Важное</strong>") || !strings.Contains(html, `href="https://example.com"`) {
		t.Fatalf("safe formatting was removed: %s", html)
	}
	if strings.Contains(html, "<img") || strings.Contains(html, "onerror") {
		t.Fatalf("unsafe formatting survived: %s", html)
	}
}

func TestValidatePublicationInput(t *testing.T) {
	in := publicationInput{Title: "Полезный материал", Difficulty: "unknown", Visibility: "wrong", Content: []publicationBlock{{Type: "h2", Text: "Раздел"}}}
	if err := validatePublicationInput(&in); err != nil {
		t.Fatal(err)
	}
	if in.Difficulty != "medium" || in.Visibility != "draft" || in.Slug == "" {
		t.Fatalf("defaults not applied: %+v", in)
	}
}

func TestGeneratePublicationSummaryUsesOnlySourceText(t *testing.T) {
	blocks := []publicationBlock{
		{Type: "h2", Text: "Проверьте применимость налогового режима до подачи уведомления"},
		{Type: "paragraph", Text: "Сравните ограничения по доходам и численности сотрудников. Остальные детали не должны попасть в первый тезис."},
		{Type: "checklist", Items: []string{"Зафиксируйте контрольные даты и ответственных сотрудников"}},
	}
	points := generatePublicationSummary("Переход на УСН", "Пошаговый разбор перехода на упрощённую систему налогообложения", blocks)
	if len(points) < 3 {
		t.Fatalf("expected at least three points, got %d", len(points))
	}
	joined := strings.Join(points, " ")
	if strings.Contains(joined, "несуществующий факт") || !strings.Contains(joined, "ограничения") {
		t.Fatalf("summary is not extractive: %q", joined)
	}
}

func TestValidatePublicationRejectsUnknownBlock(t *testing.T) {
	in := publicationInput{Title: "Полезный материал", Content: []publicationBlock{{Type: "raw_html", Text: "<b>x</b>"}}}
	if err := validatePublicationInput(&in); err == nil {
		t.Fatal("unknown block accepted")
	}
}

func TestPublicationExpertiseContributionIsCapped(t *testing.T) {
	if got := publicationExpertiseContribution(10000, 100000, 100000, 100000, 10000); got != 30 {
		t.Fatalf("contribution must be capped, got %v", got)
	}
	if low, high := publicationExpertiseContribution(1, 2, 1, 0, 1), publicationExpertiseContribution(3, 20, 8, 4, 3); high <= low {
		t.Fatalf("better quality should increase contribution: %v >= %v", low, high)
	}
}
