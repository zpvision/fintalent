package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestAdminUserLoginRequiresAdmin(t *testing.T) {
	for _, handler := range []http.HandlerFunc{adminUserAction, adminEndUserSession} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("PUT", "/api/admin/users/1/login", nil))
		if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
			t.Fatalf("unauthenticated request: %d", w.Code)
		}
	}
}

func TestAdminUserLoginCloud(t *testing.T) {
	if os.Getenv("RUN_ADMIN_LOGIN_DB_TESTS") != "1" {
		t.Skip("requires RUN_ADMIN_LOGIN_DB_TESTS=1 and cloud DATABASE_URL")
	}
	loadLocalEnv(".env")
	config, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || os.Getenv("DATABASE_URL") == "" || config.Host == "localhost" || config.Host == "127.0.0.1" || config.Host == "::1" || strings.HasPrefix(config.Host, "/") {
		t.Fatal("cloud DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn := stdlib.OpenDB(*config)
	t.Cleanup(func() { conn.Close() })
	schema := fmt.Sprintf("admin_login_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = conn.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := conn.ExecContext(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	config.RuntimeParams["search_path"] = schema
	isolated := stdlib.OpenDB(*config)
	previous := db
	db = isolated
	t.Cleanup(func() { db = previous; isolated.Close() })
	_, err = db.ExecContext(ctx, `CREATE TABLE users(id BIGINT PRIMARY KEY,full_name TEXT,email TEXT,password_hash TEXT,avatar_url TEXT,is_blocked BOOLEAN DEFAULT FALSE,is_system BOOLEAN DEFAULT FALSE);
		CREATE TABLE sessions(user_id BIGINT REFERENCES users(id),token_hash TEXT UNIQUE,expires_at TIMESTAMPTZ);
		INSERT INTO users(id,full_name,email,password_hash) VALUES(1,'Test','test@example.invalid','original');
		INSERT INTO users(id,is_blocked) VALUES(2,TRUE);
		INSERT INTO users(id,is_system) VALUES(3,TRUE);`+adminSessionsSchema)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if err = createAdminSession(w, httptest.NewRequest("POST", "/", nil)); err != nil {
		t.Fatal(err)
	}
	admin := w.Result().Cookies()[0]
	w = httptest.NewRecorder()
	if err = createSession(w, 1); err != nil {
		t.Fatal(err)
	}
	original := w.Result().Cookies()[0]
	request := func(method, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.AddCookie(admin)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		var h http.HandlerFunc = adminUserAction
		if path == "/api/admin/user-session/end" {
			h = adminEndUserSession
		}
		requestSecurity(h).ServeHTTP(w, r)
		return w
	}
	for _, id := range []string{"2", "3", "999"} {
		w = request("PUT", "/api/admin/users/"+id+"/login")
		if w.Code != 404 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("unavailable user %s: %d", id, w.Code)
		}
	}
	for _, path := range []string{"/api/admin/users/1/login", "/api/admin/user-session/end"} {
		r := httptest.NewRequest("PUT", path, nil)
		r.AddCookie(admin)
		r.Header.Set("Origin", "https://untrusted.invalid")
		w := httptest.NewRecorder()
		requestSecurity(http.HandlerFunc(adminUserAction)).ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
			t.Fatalf("cross-origin request accepted: %d", w.Code)
		}
	}
	r := httptest.NewRequest("PUT", "/api/admin/users/1/login", nil)
	r.AddCookie(original)
	w = httptest.NewRecorder()
	adminUserAction(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("ordinary user session must not authorize impersonation")
	}
	w = request("GET", "/api/admin/users/1/login")
	if w.Code != 404 || len(w.Result().Cookies()) != 0 {
		t.Fatal("GET must not issue a session")
	}
	w = request("PUT", "/api/admin/users/1/login")
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	support := w.Result().Cookies()[0]
	if support.Name != sessionCookie || support.MaxAge != 3600 || !support.HttpOnly || support.SameSite != http.SameSiteLaxMode {
		t.Fatalf("incorrect session cookie: %+v", support)
	}
	assertCount(t, "SELECT count(*) FROM sessions WHERE token_hash=$1 AND expires_at BETWEEN NOW()+interval '59 minutes' AND NOW()+interval '61 minutes'", 1, hashSessionToken(support.Value))
	for _, cookie := range []*http.Cookie{support, original} {
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.AddCookie(cookie)
		u, err := userFromRequest(r)
		if err != nil || u.ID != 1 {
			t.Fatalf("session should resolve selected user: %v", err)
		}
	}
	w = request("POST", "/api/admin/user-session/end", support)
	if w.Code != 200 {
		t.Fatalf("end login: %d", w.Code)
	}
	assertCount(t, "SELECT count(*) FROM sessions WHERE token_hash=$1", 0, hashSessionToken(support.Value))
	assertCount(t, "SELECT count(*) FROM sessions WHERE token_hash=$1", 1, hashSessionToken(original.Value))
	assertCount(t, "SELECT count(*) FROM admin_sessions WHERE token_hash=$1", 1, hashSessionToken(admin.Value))
	assertCount(t, "SELECT count(*) FROM users WHERE id=1 AND password_hash='original'", 1)
}
