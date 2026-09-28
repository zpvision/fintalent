package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func testReleaseCredentialSessions(t *testing.T, ctx context.Context) {
	hash, err := bcrypt.GenerateFromPassword([]byte("original-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	const email = "credential-lifecycle@example.invalid"
	var id int64
	if err = db.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash) VALUES('Credential fixture',$1,$2) RETURNING id`, email, string(hash)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if err = createSessionForCredentials(w, id, string(hash), email); err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	if err = createSession(httptest.NewRecorder(), id); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/profile/password", strings.NewReader(`{"current_password":"original-password","new_password":"replacement-password"}`))
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	updateProfilePassword(w, r)
	if w.Code != 200 {
		t.Fatalf("password update: %d %s", w.Code, w.Body.String())
	}
	assertCount(t, "SELECT count(*) FROM sessions WHERE user_id=$1", 0, id)
	reject := func(expectedHash, expectedEmail string) {
		t.Helper()
		w := httptest.NewRecorder()
		if err := createSessionForCredentials(w, id, expectedHash, expectedEmail); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("stale credentials accepted: %v", err)
		}
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("cookie issued for stale credentials")
		}
	}
	// Reproduce a login that checked bcrypt before the password change committed.
	reject(string(hash), email)
	var currentHash string
	if err = db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1`, id).Scan(&currentHash); err != nil {
		t.Fatal(err)
	}
	// A credential update already holding the user row lock must win over a
	// concurrent issuer that verified the previous password before that update.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, string(hash), id); err != nil {
		t.Fatal(err)
	}
	issued := make(chan error, 1)
	go func() { issued <- createSessionForCredentials(httptest.NewRecorder(), id, currentHash, email) }()
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-issued; !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("concurrent stale issuer: %v", err)
	}
	currentHash = string(hash)
	if _, err = db.ExecContext(ctx, `UPDATE users SET email='changed-credential@example.invalid' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	reject(currentHash, email)
	if _, err = db.ExecContext(ctx, `UPDATE users SET is_blocked=TRUE WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	reject(currentHash, "changed-credential@example.invalid")
	if _, err = db.ExecContext(ctx, `UPDATE users SET is_blocked=FALSE,is_system=TRUE WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	reject(currentHash, "changed-credential@example.invalid")
	assertCount(t, "SELECT count(*) FROM sessions WHERE user_id=$1", 0, id)
}
