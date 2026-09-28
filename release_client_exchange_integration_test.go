package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"FinTalent/internal/clientexchange"
)

// Invoked only inside the verified isolated schema fixture.
func testReleaseClientExchange(t *testing.T, ctx context.Context) {
	var seller, buyer, stranger int64
	for i, dest := range []*int64{&seller, &buyer, &stranger} {
		if err := db.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash) VALUES('Exchange fixture',$1,repeat('*',60)) RETURNING id`, fmt.Sprintf("exchange%d@example.invalid", i)).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	clientexchange.New(db, func(r *http.Request) (clientexchange.UserIdentity, error) {
		id, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		return clientexchange.UserIdentity{ID: id}, err
	}, func(*http.Request) bool { return false }).Register(mux)
	request := func(user int64, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	newListing := func(status string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRowContext(ctx, `INSERT INTO client_exchange_listings(seller_user_id,title,status,client_inn,client_legal_name) VALUES($1,'Exchange fixture',$2,'1234567890','PRIVATE COMPANY') RETURNING id`, seller, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	id := newListing("active")
	path := fmt.Sprintf("/api/client-exchange/listings/%d", id)
	for i := 0; i < 2; i++ {
		if w := request(buyer, "POST", path+"/favorite", ""); w.Code != 200 {
			t.Fatalf("favorite: %d %s", w.Code, w.Body.String())
		}
	}
	for _, status := range []string{"draft", "archived", "cancelled"} {
		exec(`UPDATE client_exchange_listings SET status=$2 WHERE id=$1`, id, status)
		if w := request(buyer, "POST", path+"/favorite", ""); w.Code != 404 {
			t.Fatalf("hidden favorite %s: %d", status, w.Code)
		}
		if w := request(buyer, "GET", path, ""); w.Code != 404 {
			t.Fatalf("hidden detail %s: %d", status, w.Code)
		}
		if w := request(buyer, "GET", "/api/client-exchange/my/favorites", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
			t.Fatalf("hidden favorite list: %d %s", w.Code, w.Body.String())
		}
	}
	assertCount(t, `SELECT count(*) FROM client_exchange_views WHERE listing_id=$1`, 0, id)
	exec(`UPDATE client_exchange_listings SET status='active' WHERE id=$1`, id)
	exec(`UPDATE users SET is_blocked=true WHERE id=$1`, seller)
	if w := request(buyer, "POST", path+"/favorite", ""); w.Code != 404 {
		t.Fatalf("blocked favorite: %d", w.Code)
	}
	if w := request(buyer, "POST", path+"/responses", `{"accept_original_price":true}`); w.Code < 400 {
		t.Fatal("blocked response accepted")
	}
	exec(`UPDATE users SET is_blocked=false WHERE id=$1`, seller)
	db.SetMaxOpenConns(1)
	w := request(buyer, "GET", "/api/client-exchange/my/favorites", "")
	db.SetMaxOpenConns(16)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Exchange fixture") || strings.Contains(w.Body.String(), "PRIVATE COMPANY") {
		t.Fatalf("favorite visibility: %d %s", w.Code, w.Body.String())
	}
	if w := request(seller, "PUT", path, `{"title":"must roll back","marketplace_ids":[99999999]}`); w.Code < 400 {
		t.Fatal("invalid option accepted")
	}
	assertCount(t, `SELECT count(*) FROM client_exchange_listings WHERE id=$1 AND title='Exchange fixture'`, 1, id)
	if w := request(seller, "POST", "/api/client-exchange/listings", `{"title":"must roll back","marketplace_ids":[99999999]}`); w.Code < 400 {
		t.Fatal("invalid create accepted")
	}
	assertCount(t, `SELECT count(*) FROM client_exchange_listings WHERE seller_user_id=$1`, 1, seller)
	for iteration := 0; iteration < 3; iteration++ {
		listing := newListing("has_responses")
		var response int64
		if err := db.QueryRowContext(ctx, `INSERT INTO client_exchange_responses(listing_id,buyer_user_id) VALUES($1,$2) RETURNING id`, listing, buyer).Scan(&response); err != nil {
			t.Fatal(err)
		}
		lp := fmt.Sprintf("/api/client-exchange/listings/%d", listing)
		rp := fmt.Sprintf("/api/client-exchange/responses/%d/accept", response)
		var wg sync.WaitGroup
		start := make(chan struct{})
		codes := make([]int, 2)
		for i, p := range []string{lp + "/archive", rp} {
			wg.Add(1)
			go func(i int, p string) { defer wg.Done(); <-start; codes[i] = request(seller, "POST", p, "").Code }(i, p)
		}
		close(start)
		wg.Wait()
		if (codes[0] == 200) == (codes[1] == 200) {
			t.Fatalf("archive/accept race: %v", codes)
		}
		assertCount(t, `SELECT count(*) FROM client_exchange_listings l JOIN client_exchange_responses r ON r.listing_id=l.id WHERE l.id=$1 AND ((l.status='buyer_selected' AND r.status='accepted' AND l.selected_buyer_user_id=r.buyer_user_id) OR (l.status='archived' AND r.status='pending' AND l.selected_buyer_user_id IS NULL))`, 1, listing)
	}
	var response int64
	if err := db.QueryRowContext(ctx, `INSERT INTO client_exchange_responses(listing_id,buyer_user_id) VALUES($1,$2) RETURNING id`, id, buyer).Scan(&response); err != nil {
		t.Fatal(err)
	}
	rp := fmt.Sprintf("/api/client-exchange/responses/%d/accept", response)
	if w := request(seller, "POST", rp, ""); w.Code != 200 {
		t.Fatalf("accept: %d %s", w.Code, w.Body.String())
	}
	for _, user := range []int64{seller, buyer, stranger} {
		w := request(user, "GET", path, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE COMPANY") != (user != stranger) {
			t.Fatalf("private fields for %d: %d %s", user, w.Code, w.Body.String())
		}
	}
	if w := request(seller, "POST", path+"/complete", ""); w.Code != 200 {
		t.Fatalf("complete: %d", w.Code)
	}
	if w := request(seller, "DELETE", path, ""); w.Code != 409 {
		t.Fatalf("delete transferred: %d", w.Code)
	}
	deleted := newListing("has_responses")
	if err := db.QueryRowContext(ctx, `INSERT INTO client_exchange_responses(listing_id,buyer_user_id) VALUES($1,$2) RETURNING id`, deleted, buyer).Scan(&response); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE client_exchange_listings SET deleted_at=NOW() WHERE id=$1`, deleted)
	if w := request(seller, "POST", fmt.Sprintf("/api/client-exchange/responses/%d/accept", response), ""); w.Code < 400 {
		t.Fatal("accepted deleted parent")
	}
}
