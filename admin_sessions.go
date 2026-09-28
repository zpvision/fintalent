package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed migrations/065_admin_sessions.sql
var adminSessionsSchema string

func prepareAdminSessionsDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, adminSessionsSchema)
	return err
}

func hashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Rotating the configured credentials also invalidates previously issued sessions.
func adminCredentialHash() string {
	return hashSessionToken(strings.TrimSpace(os.Getenv("ADMIN_LOGIN")) + "\x00" + os.Getenv("ADMIN_PASSWORD"))
}

func createAdminSession(w http.ResponseWriter, r *http.Request) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(12 * time.Hour)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `INSERT INTO admin_sessions(token_hash,credential_hash,expires_at) VALUES($1,$2,$3)`, hashSessionToken(token), adminCredentialHash(), expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: token, Path: "/", HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 12 * 60 * 60})
	return nil
}

func isAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookie)
	if err != nil || len(cookie.Value) != 64 || db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	var valid bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=$1 AND credential_hash=$2 AND expires_at>NOW())`, hashSessionToken(cookie.Value), adminCredentialHash()).Scan(&valid)
	return err == nil && valid
}
