package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"FinTalent/internal/clientexchange"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Only synthetic records in a fresh cloud schema; no production tables or workers.
func TestClientExchangeAdminIsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_CLIENT_EXCHANGE_ADMIN_DB_TESTS") != "1" {
		t.Skip("requires opt-in isolated cloud PostgreSQL test")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || os.Getenv("DATABASE_URL") == "" {
		t.Fatal("cloud DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	adminDB := stdlib.OpenDB(*config)
	defer adminDB.Close()
	schema := fmt.Sprintf("ce_admin_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = adminDB.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "ce_admin_test_") {
			t.Fatal("unsafe cleanup target")
		}
		if _, err := adminDB.ExecContext(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config.RuntimeParams["search_path"] = schema
	isolate := stdlib.OpenDB(*config)
	defer isolate.Close()
	var actual string
	if err := isolate.QueryRowContext(ctx, "SELECT current_schema()").Scan(&actual); err != nil || actual != schema {
		t.Fatal("schema isolation failed")
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := isolate.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE users(id bigserial PRIMARY KEY,full_name text NOT NULL,email text NOT NULL,password_hash text,is_blocked boolean NOT NULL DEFAULT false,is_system boolean NOT NULL DEFAULT false,avatar_url text);
	CREATE TABLE accounting_companies(owner_user_id bigint,verified boolean,status text,deleted_at timestamptz);
	INSERT INTO users(full_name,email) VALUES('Synthetic Seller','seller@example.invalid'),('Synthetic Buyer','buyer@example.invalid')`)
	migration, err := clientExchangeMigrationFS.ReadFile("migrations/039_client_exchange.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(string(migration))
	mux := http.NewServeMux()
	clientexchange.New(isolate, func(r *http.Request) (clientexchange.UserIdentity, error) {
		return clientexchange.UserIdentity{ID: 2}, nil
	}, func(r *http.Request) bool { return r.Header.Get("X-Test-Admin") == "yes" }).Register(mux)
	handler := requestSecurity(mux)
	request := func(method, path string, body any, admin bool) *httptest.ResponseRecorder {
		t.Helper()
		var raw []byte
		var err error
		if body != nil {
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
		if admin {
			r.Header.Set("X-Test-Admin", "yes")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("got %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	newCard := func(status string) int64 {
		t.Helper()
		var id int64
		if err := isolate.QueryRowContext(ctx, `INSERT INTO client_exchange_listings(seller_user_id,title,status,client_inn,client_legal_name) VALUES(1,'Synthetic card',$1,'7707083893','Synthetic legal name') RETURNING id`, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	base := "/api/admin/client-exchange/listings"
	id := newCard("draft")
	path := fmt.Sprintf("%s/%d", base, id)
	for _, route := range []string{base, path} {
		for _, method := range []string{"GET", "PUT", "POST", "DELETE"} {
			check(request(method, route, nil, false), 401)
		}
	}
	csrfRequest := httptest.NewRequest("DELETE", path, strings.NewReader(`{}`))
	csrfRequest.Header.Set("X-Test-Admin", "yes")
	csrfRequest.Header.Set("Origin", "https://evil.invalid")
	csrfResult := httptest.NewRecorder()
	handler.ServeHTTP(csrfResult, csrfRequest)
	check(csrfResult, 403)
	check(request("POST", base, nil, true), 405)
	check(request("GET", base+"?status=invalid", nil, true), 400)
	check(request("GET", base+"?page=-1", nil, true), 400)
	check(request("GET", base+"/invalid", nil, true), 400)
	check(request("GET", path+"/archive/extra", nil, true), 400)
	check(request("GET", base+"/99999999", nil, true), 404)
	for _, q := range []string{"seller@example.invalid", "Synthetic legal name", "7707083893", "1"} {
		w := request("GET", base+"?q="+strings.ReplaceAll(q, " ", "%20"), nil, true)
		check(w, 200)
		if !strings.Contains(w.Body.String(), `"total":1`) {
			t.Fatalf("search failed: %s", w.Body.String())
		}
	}
	var option int64
	if err := isolate.QueryRowContext(ctx, `SELECT id FROM client_exchange_dictionary_items WHERE kind='marketplace' LIMIT 1`).Scan(&option); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO client_exchange_listing_options(listing_id,item_id,kind) VALUES($1,$2,'marketplace')`, id, option)
	type detail struct {
		Listing struct {
			UpdatedAt time.Time `json:"updated_at"`
			Status    string    `json:"status"`
		} `json:"listing"`
		Input clientexchange.ListingInput `json:"input"`
	}
	get := func(p string) detail {
		t.Helper()
		w := request("GET", p, nil, true)
		check(w, 200)
		var d detail
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	d := get(path)
	check(request("PUT", path, map[string]any{"updated_at": d.Listing.UpdatedAt}, true), 400)
	if len(d.Input.MarketplaceIDs) != 1 || d.Input.MarketplaceIDs[0] != option || d.Input.ClientINN != "7707083893" {
		t.Fatalf("incomplete detail: %+v", d)
	}
	d.Input.Title = "Edited by admin"
	check(request("PUT", path, map[string]any{"input": d.Input, "updated_at": d.Listing.UpdatedAt}, true), 200)
	check(request("PUT", path, map[string]any{"input": d.Input, "updated_at": d.Listing.UpdatedAt}, true), 409)
	d = get(path)
	if d.Input.Title != "Edited by admin" || len(d.Input.MarketplaceIDs) != 1 {
		t.Fatal("edit lost data")
	}
	d.Input.MarketplaceIDs = []int64{99999999}
	d.Input.Title = "Must roll back"
	check(request("PUT", path, map[string]any{"input": d.Input, "updated_at": d.Listing.UpdatedAt}, true), 400)
	d = get(path)
	if d.Input.Title != "Edited by admin" || len(d.Input.MarketplaceIDs) != 1 {
		t.Fatal("failed edit was not atomic")
	}
	// Validate a live card strictly and retain all its multi-select options.
	for _, pair := range []struct{ column, kind string }{{"industry_id", "industry"}, {"employee_range_id", "employee_range"}, {"tax_system_id", "tax_system"}, {"revenue_range_id", "revenue_range"}, {"accounting_state_id", "accounting_state"}, {"transfer_reason_id", "transfer_reason"}, {"transfer_type_id", "transfer_type"}} {
		exec(`UPDATE client_exchange_listings SET `+pair.column+`=(SELECT id FROM client_exchange_dictionary_items WHERE kind=$2 ORDER BY id LIMIT 1) WHERE id=$1`, id, pair.kind)
	}
	exec(`UPDATE client_exchange_listings SET status='has_responses',city='Москва',updated_at=clock_timestamp() WHERE id=$1`, id)
	exec(`INSERT INTO client_exchange_responses(listing_id,buyer_user_id) VALUES($1,2)`, id)
	d = get(path)
	d.Input.City = ""
	check(request("PUT", path, map[string]any{"input": d.Input, "updated_at": d.Listing.UpdatedAt}, true), 400)
	d = get(path)
	check(request("POST", path+"/archive", map[string]any{"updated_at": d.Listing.UpdatedAt}, true), 200)
	if get(path).Listing.Status != "archived" {
		t.Fatal("archive did not change status")
	}
	check(request("GET", fmt.Sprintf("/api/client-exchange/listings/%d", id), nil, false), 404)
	var responses int
	if err := isolate.QueryRowContext(ctx, `SELECT count(*) FROM client_exchange_responses WHERE listing_id=$1`, id).Scan(&responses); err != nil || responses != 1 {
		t.Fatal("archive lost responses")
	}
	d = get(path)
	check(request("DELETE", path, map[string]any{"updated_at": d.Listing.UpdatedAt}, true), 200)
	check(request("GET", path, nil, true), 404)
	check(request("DELETE", path, map[string]any{"updated_at": d.Listing.UpdatedAt}, true), 404)
	for _, status := range []string{"buyer_selected", "transfer_in_progress", "transferred"} {
		locked := newCard(status)
		p := fmt.Sprintf("%s/%d", base, locked)
		d := get(p)
		for _, op := range []struct{ method, suffix string }{{"PUT", ""}, {"DELETE", ""}, {"POST", "/archive"}} {
			check(request(op.method, p+op.suffix, map[string]any{"input": d.Input, "updated_at": d.Listing.UpdatedAt}, true), 409)
		}
	}
	for i := 0; i < 31; i++ {
		newCard("draft")
	}
	w := request("GET", base+"?status=draft&page=1", nil, true)
	check(w, 200)
	var page struct {
		Items []any `json:"items"`
		Total int   `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 30 || page.Total != 31 {
		t.Fatalf("pagination: %s", w.Body.String())
	}
	w = request("GET", base+"?status=draft&page=2", nil, true)
	check(w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("page 2: %s", w.Body.String())
	}
	// Two administrators saving the same version must not overwrite each other.
	raceID := newCard("draft")
	racePath := fmt.Sprintf("%s/%d", base, raceID)
	raceDetail := get(racePath)
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			in := raceDetail.Input
			in.Title = fmt.Sprintf("Concurrent edit %d", i)
			codes[i] = request("PUT", racePath, map[string]any{"input": in, "updated_at": raceDetail.Listing.UpdatedAt}, true).Code
		}(i)
	}
	close(start)
	wg.Wait()
	if !((codes[0] == 200 && codes[1] == 409) || (codes[1] == 200 && codes[0] == 409)) {
		t.Fatalf("concurrent admin edits: %v", codes)
	}
	t.Run("existing seller and buyer workflows", func(t *testing.T) {
		original := db
		db = isolate
		defer func() { db = original }()
		testReleaseClientExchange(t, ctx)
	})
}
