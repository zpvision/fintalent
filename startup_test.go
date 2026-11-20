package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func captureStartupLogs(t testing.TB) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

func TestStartupDiagnosticsContainOnlyTimingAndSafeCodes(t *testing.T) {
	buf := captureStartupLogs(t)
	secret := "postgres://private-user:private-password@private-host/db?token=private-token"
	t.Setenv("DATABASE_URL", secret)
	t.Setenv("S3_SECRET_KEY", "private-s3-secret")
	startupTimer("database.connect_ping")()
	for _, err := range []error{errors.New(secret), fmt.Errorf("%s: %w", secret, context.DeadlineExceeded), startupStateError{secret}, startupStateError{"23505"}} {
		startupFailure("database.prepare", err)
	}
	text := buf.String()
	for _, s := range []string{"private-", "postgres://", "DATABASE_URL", "S3_SECRET_KEY"} {
		if strings.Contains(text, s) {
			t.Fatalf("confidential information logged")
		}
	}
	for _, s := range []string{"event=begin", "event=end duration_ms=", "error_code=operation_failed", "error_code=deadline_exceeded", "error_code=23505"} {
		if !strings.Contains(text, s) {
			t.Fatal("missing diagnostic field", s)
		}
	}
	if lines := strings.Count(text, "\n"); lines != 6 {
		t.Fatal("unexpected logging volume", lines)
	}
}

type startupStateError struct{ state string }

func (e startupStateError) Error() string    { return "sensitive raw error" }
func (e startupStateError) SQLState() string { return e.state }

func TestStartupModeAlwaysPreparesSynchronously(t *testing.T) {
	for _, mode := range []string{"", "auto", "verify", "skip", "password-secret"} {
		t.Setenv("DB_STARTUP_MODE", mode)
		err := validateStartupMode()
		if (err == nil) != (mode == "" || mode == "auto") {
			t.Fatal("unexpected mode acceptance")
		}
		if err != nil && strings.Contains(err.Error(), "password-secret") {
			t.Fatal("mode value leaked")
		}
	}
}

// Guard the original SQL and prepare-call order without contacting PostgreSQL.
// The expected digest was compared with HEAD before adding diagnostics.
func TestStartupDatabasePreparationContract(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func prepareDatabase() error {")
	if start < 0 {
		t.Fatal("preparation entry point missing")
	}
	body := source[start:]
	end := strings.Index(body, "\n}")
	if end < 0 {
		t.Fatal("function end missing")
	}
	body = body[:end+2]
	var parts []string
	for _, match := range regexp.MustCompile("`([^`]*)`").FindAllStringSubmatch(body, -1) {
		parts = append(parts, match[1])
	}
	for _, match := range regexp.MustCompile(`\b(prepare\w+)\(`).FindAllStringSubmatch(body, -1) {
		parts = append(parts, match[1])
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	const expected = "734634f298a1a50c8b397a446c4f901ed451db5b3e8e73f7ce6ceee89571d61e"
	if got := hex.EncodeToString(digest[:]); got != expected {
		t.Fatal("SQL or preparation call order changed; diagnostic release must preserve both", got)
	}
}

func TestStartupTimerStagesAreStatic(t *testing.T) {
	files := []string{"main.go", "runtime_lifecycle.go", "academy_object_storage.go", "academy_module.go", "accounting_company_module.go", "admin.go", "admin_sessions.go", "client_exchange_module.go", "contact_messages.go", "courses_module.go", "employee_testing.go", "geography.go", "help_module.go", "marketplace.go", "notification_outbox.go", "okved.go", "password_reset.go", "profile_purpose.go", "profimarket.go", "publication_module.go", "shared_object_files.go", "survey.go", "test_categories.go", "testing_module.go", "vacancy_module.go", "vacancy_survey.go"}
	for _, name := range files {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "startupTimer" && ident.Name != "startupFailure" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Errorf("dynamic startup stage in %s", name)
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil || !regexp.MustCompile(`^[A-Za-z0-9_.]+$`).MatchString(value) {
				t.Errorf("invalid startup stage in %s", name)
			}
			return true
		})
	}
}

type startupBenchmarkWriter struct{}

func (startupBenchmarkWriter) Write(p []byte) (int, error) { return len(p), nil }
func BenchmarkStartupTimer(b *testing.B) {
	previous := log.Writer()
	log.SetOutput(startupBenchmarkWriter{})
	b.Cleanup(func() { log.SetOutput(previous) })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startupTimer("database.prepareAcademyDatabase")()
	}
}
