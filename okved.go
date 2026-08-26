package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	okvedSourceVersion = "89/2026"
	okvedSourceDate    = "2026-08-01"
	okvedSourceURL     = "https://rosstat.gov.ru/opendata/7708234640-okvedva/data-20260801T1408-structure-20180402T1704.csv"
)

//go:embed data/okved.csv
var okvedDataFS embed.FS

type okvedEntry struct {
	Code       string `json:"code"`
	Section    string `json:"section"`
	Name       string `json:"name"`
	ParentCode string `json:"parent_code,omitempty"`
	Level      int    `json:"level"`
	IsSection  bool   `json:"is_section"`
}

func prepareOKVEDDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS okved_entries (
		code VARCHAR(12) PRIMARY KEY,
		section CHAR(1) NOT NULL,
		name TEXT NOT NULL,
		parent_code VARCHAR(12),
		level SMALLINT NOT NULL,
		is_section BOOLEAN NOT NULL DEFAULT FALSE
	);
	CREATE INDEX IF NOT EXISTS okved_entries_section_idx ON okved_entries(section,code);
	CREATE INDEX IF NOT EXISTS okved_entries_parent_idx ON okved_entries(parent_code);
	CREATE TABLE IF NOT EXISTS okved_metadata (
		id SMALLINT PRIMARY KEY DEFAULT 1 CHECK(id=1),
		version VARCHAR(30) NOT NULL,
		source_date DATE NOT NULL,
		source_url TEXT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`)
	if err != nil {
		return err
	}
	var version string
	err = db.QueryRowContext(ctx, `SELECT version FROM okved_metadata WHERE id=1`).Scan(&version)
	if err == nil && version == okvedSourceVersion {
		return nil
	}
	data, err := okvedDataFS.ReadFile("data/okved.csv")
	if err != nil {
		return err
	}
	entries, err := parseOKVEDCSV(string(data))
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM okved_entries`); err != nil {
		return err
	}
	if err = insertOKVEDEntries(ctx, tx, entries); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO okved_metadata(id,version,source_date,source_url,updated_at) VALUES(1,$1,$2,$3,NOW()) ON CONFLICT(id) DO UPDATE SET version=EXCLUDED.version,source_date=EXCLUDED.source_date,source_url=EXCLUDED.source_url,updated_at=NOW()`, okvedSourceVersion, okvedSourceDate, okvedSourceURL); err != nil {
		return err
	}
	return tx.Commit()
}

func insertOKVEDEntries(ctx context.Context, tx *sql.Tx, entries []okvedEntry) error {
	const batchSize = 250
	for start := 0; start < len(entries); start += batchSize {
		end := min(start+batchSize, len(entries))
		values := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*6)
		for _, entry := range entries[start:end] {
			base := len(args)
			values = append(values, "($"+strconv.Itoa(base+1)+",$"+strconv.Itoa(base+2)+",$"+strconv.Itoa(base+3)+",NULLIF($"+strconv.Itoa(base+4)+",''),$"+strconv.Itoa(base+5)+",$"+strconv.Itoa(base+6)+")")
			args = append(args, entry.Code, entry.Section, entry.Name, entry.ParentCode, entry.Level, entry.IsSection)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO okved_entries(code,section,name,parent_code,level,is_section) VALUES `+strings.Join(values, ","), args...); err != nil {
			return err
		}
	}
	return nil
}

func parseOKVEDCSV(data string) ([]okvedEntry, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(data, "\ufeff")))
	reader.Comma = ';'
	reader.FieldsPerRecord = 3
	entries := make([]okvedEntry, 0, 3100)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		section := strings.TrimSpace(record[0])
		code := strings.TrimSpace(record[1])
		name := strings.TrimSpace(record[2])
		if section == "" || name == "" {
			continue
		}
		if code == "" {
			entries = append(entries, okvedEntry{Code: section, Section: section, Name: name, Level: 0, IsSection: true})
			continue
		}
		digits := strings.ReplaceAll(code, ".", "")
		if len(digits) < 2 || len(digits) > 6 {
			return nil, errors.New("invalid OKVED code: " + code)
		}
		parent := section
		if len(digits) > 2 {
			parent = formatOKVEDCode(digits[:len(digits)-1])
		}
		entries = append(entries, okvedEntry{Code: code, Section: section, Name: name, ParentCode: parent, Level: len(digits) - 1})
	}
	if len(entries) < 3000 {
		return nil, errors.New("incomplete OKVED data")
	}
	return entries, nil
}

func formatOKVEDCode(digits string) string {
	if len(digits) <= 2 {
		return digits
	}
	if len(digits) <= 4 {
		return digits[:2] + "." + digits[2:]
	}
	return digits[:2] + "." + digits[2:4] + "." + digits[4:]
}

func adminOKVED(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	section := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("section")))
	levelValue := strings.TrimSpace(r.URL.Query().Get("level"))
	level, _ := strconv.Atoi(levelValue)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	clauses := []string{"1=1"}
	args := []any{}
	if query != "" {
		args = append(args, "%"+query+"%")
		clauses = append(clauses, "(code ILIKE $"+strconv.Itoa(len(args))+" OR name ILIKE $"+strconv.Itoa(len(args))+")")
	}
	if len(section) == 1 {
		args = append(args, section)
		clauses = append(clauses, "section=$"+strconv.Itoa(len(args)))
	}
	if levelValue != "" && level >= 0 {
		args = append(args, level)
		clauses = append(clauses, "level=$"+strconv.Itoa(len(args)))
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM okved_entries WHERE `+where, args...).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить ОКВЭД")
		return
	}
	queryArgs := append(append([]any{}, args...), limit, (page-1)*limit)
	rows, err := db.QueryContext(r.Context(), `SELECT code,section,name,COALESCE(parent_code,''),level,is_section FROM okved_entries WHERE `+where+` ORDER BY section,is_section DESC,code LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить ОКВЭД")
		return
	}
	defer rows.Close()
	items := []okvedEntry{}
	for rows.Next() {
		var item okvedEntry
		if err = rows.Scan(&item.Code, &item.Section, &item.Name, &item.ParentCode, &item.Level, &item.IsSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить ОКВЭД")
			return
		}
		items = append(items, item)
	}
	sections, err := loadOKVEDSections(r.Context())
	if err != nil || rows.Err() != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось загрузить ОКВЭД")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]any{
		"items": items, "sections": sections, "total": total, "page": page, "limit": limit,
		"version": okvedSourceVersion, "source_date": okvedSourceDate, "source_url": okvedSourceURL, "generated_at": time.Now(),
	})
}

func loadOKVEDSections(ctx context.Context) ([]okvedEntry, error) {
	rows, err := db.QueryContext(ctx, `SELECT code,section,name,'',level,is_section FROM okved_entries WHERE is_section=TRUE ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []okvedEntry{}
	for rows.Next() {
		var item okvedEntry
		if err = rows.Scan(&item.Code, &item.Section, &item.Name, &item.ParentCode, &item.Level, &item.IsSection); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
