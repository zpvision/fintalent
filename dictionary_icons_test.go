package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

// Run with RUN_DB_TESTS=1 and DATABASE_URL pointing to a separate cloud test DB.
func TestDictionaryIconDefaultsIntegration(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 and DATABASE_URL to a separate cloud test database")
	}
	loadLocalEnv(".env")
	conn, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal("cannot configure test database")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("cannot connect to test database")
	}
	defer tx.Rollback()
	// Temporary tables shadow application tables; no persistent data is changed.
	_, err = tx.ExecContext(ctx, `
		CREATE TEMP TABLE dictionaries (id BIGINT PRIMARY KEY, alias TEXT, icon TEXT);
		CREATE TEMP TABLE dictionary_items (
			id BIGINT PRIMARY KEY, dictionary_id BIGINT, value TEXT, icon TEXT,
			sort_order INTEGER, deleted_at TIMESTAMPTZ
		);`)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := vacancyMigrationFS.ReadFile("migrations/046_dictionary_icon_defaults.sql")
	if err != nil {
		t.Fatal(err)
	}
	// An empty installation must also be safe.
	if _, err = tx.ExecContext(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO dictionaries VALUES
			(901, 'position', ''), (902, 'experience', NULL),
			(903, 'software', '/static/uploads/custom-title.svg'),
			(904, 'accounting_areas', ''), (905, 'custom', '');
		INSERT INTO dictionary_items VALUES
			(2001, 901, 'Главный бухгалтер', '', 100, NULL),
			(2002, 901, 'Бухгалтер', '/api/assets/position-icon/99.png', 99, NULL),
			(2003, 902, 'Нет опыта', '/static/icons/dictionaries/experience-2003.svg', 50, NULL),
			(2004, 903, 'Excel', ' ', 40, NULL),
			(2005, 904, 'НДС', '/api/assets/accounting-area-icon/99.png', 30, NULL),
			(2006, 901, 'Аудитор', '/static/uploads/position-icons/custom.svg', 20, NULL),
			(2007, 903, 'Диадок', 'https://example.com/custom.svg', 10, NULL),
			(2008, 902, 'До 1 года', '', 0, NOW()),
			(2009, 904, 'УСН', '/static/uploads/accounting-areas/custom-usn.svg', 5, NULL),
			(14, 905, 'Новый ответ', '', 0, NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]string{
		2001: "/static/icons/positions/position-00.svg",
		2002: "/static/icons/positions/position-02.svg",
		2003: "/static/icons/dictionaries/experience-14.svg",
		2004: "/static/icons/software/excel.svg",
		2005: "/static/icons/accounting-areas/vat.svg",
		2006: "/static/uploads/position-icons/custom.svg",
		2007: "https://example.com/custom.svg",
		2008: "",
		2009: "/static/uploads/accounting-areas/custom-usn.svg",
		14:   "",
	}
	for pass := 0; pass < 2; pass++ {
		if _, err = tx.ExecContext(ctx, string(schema)); err != nil {
			t.Fatal(err)
		}
		for id, expected := range want {
			var got string
			if err = tx.QueryRowContext(ctx, `SELECT icon FROM dictionary_items WHERE id=$1`, id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != expected {
				t.Errorf("pass %d item %d: icon = %q, want %q", pass, id, got, expected)
			}
			if strings.HasPrefix(got, "/static/icons/") {
				if _, err = os.Stat(strings.TrimPrefix(got, "/")); err != nil {
					t.Errorf("icon asset %q is unavailable: %v", got, err)
				}
			}
		}
		for id, expected := range map[int64]string{901: "/api/assets/dictionary-icon/901.svg", 902: "/api/assets/dictionary-icon/902.svg", 903: "/static/uploads/custom-title.svg"} {
			var got string
			if err = tx.QueryRowContext(ctx, `SELECT icon FROM dictionaries WHERE id=$1`, id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != expected {
				t.Errorf("pass %d dictionary %d: icon = %q, want %q", pass, id, got, expected)
			}
		}
	}
}
