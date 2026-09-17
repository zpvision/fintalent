package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetCodeLifetime  = 10 * time.Minute
	passwordResetTokenLifetime = 10 * time.Minute
	passwordResetMaxAttempts   = 5
)

//go:embed migrations/048_password_reset.sql
var passwordResetSchema string

var sixDigitCode = regexp.MustCompile(`^\d{6}$`)

func preparePasswordResetDatabase(ctx context.Context) error {
	_, err := db.ExecContext(ctx, passwordResetSchema)
	return err
}

func registerPasswordResetRoutes() {
	http.HandleFunc("/api/password-reset/request", requestPasswordReset)
	http.HandleFunc("/api/password-reset/verify", verifyPasswordReset)
	http.HandleFunc("/api/password-reset/complete", completePasswordReset)
}

func passwordResetSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("PASSWORD_RESET_SECRET"))
	if len(secret) >= 32 {
		return []byte(secret), nil
	}
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	if smtpPassword == "" {
		return nil, errors.New("PASSWORD_RESET_SECRET или SMTP_PASSWORD должен быть настроен")
	}
	derived := sha256.Sum256([]byte("fintalent-password-reset:" + smtpPassword))
	return derived[:], nil
}

func resetHMAC(secret []byte, purpose, value string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(purpose + ":" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func newResetCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func newResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeResetJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !requirePost(w, r) {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, "Некорректные данные")
		return false
	}
	return true
}

func resetClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !decodeResetJSON(w, r, &input) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) {
		writeJSON(w, http.StatusBadRequest, "Введите корректный email")
		return
	}
	secret, err := passwordResetSecret()
	if err != nil {
		log.Printf("password reset config: %v", err)
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	emailHash := resetHMAC(secret, "email", email)
	ip := resetClientIP(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var recentEmail, recentIP int
	var lastRequest sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(created_at) FROM password_reset_requests WHERE email_hash=$1 AND created_at > NOW()-INTERVAL '1 hour'`, emailHash).Scan(&recentEmail, &lastRequest)
	if err == nil {
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM password_reset_requests WHERE request_ip=$1 AND created_at > NOW()-INTERVAL '1 hour'`, ip).Scan(&recentIP)
	}
	if err != nil {
		log.Printf("password reset rate query: %v", err)
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	if recentEmail >= 3 || recentIP >= 10 || (lastRequest.Valid && time.Since(lastRequest.Time) < time.Minute) {
		writeJSON(w, http.StatusOK, "Если аккаунт существует, письмо с кодом отправлено")
		return
	}
	code, err := newResetCode()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	var userID sql.NullInt64
	var fullName string
	err = db.QueryRowContext(ctx, `SELECT id,full_name FROM users WHERE email=$1 AND NOT is_blocked AND NOT is_system`, email).Scan(&userID, &fullName)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("password reset user query: %v", err)
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	tx, err := db.BeginTx(ctx, nil)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE password_reset_requests SET used_at=NOW() WHERE email_hash=$1 AND used_at IS NULL`, emailHash)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO password_reset_requests(user_id,email_hash,code_hash,request_ip,expires_at) VALUES($1,$2,$3,$4,NOW()+INTERVAL '10 minutes')`, userID, emailHash, resetHMAC(secret, "code", email+":"+code), ip)
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		log.Printf("password reset insert: %v", err)
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	writeJSON(w, http.StatusOK, "Если аккаунт существует, письмо с кодом отправлено")
	if userID.Valid {
		go func() {
			if err := sendPasswordResetEmail(fullName, email, code, 10); err != nil {
				log.Printf("password reset email to %s: %v", email, err)
			}
		}()
	}
}

func verifyPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct{ Email, Code string }
	if !decodeResetJSON(w, r, &input) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	code := strings.TrimSpace(input.Code)
	if !validEmail(email) || !sixDigitCode.MatchString(code) {
		writeJSON(w, http.StatusBadRequest, "Код недействителен или истёк")
		return
	}
	secret, err := passwordResetSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var id int64
	var storedHash string
	err = db.QueryRowContext(ctx, `SELECT id,code_hash FROM password_reset_requests WHERE email_hash=$1 AND user_id IS NOT NULL AND used_at IS NULL AND verified_at IS NULL AND expires_at>NOW() AND failed_attempts<$2 ORDER BY created_at DESC LIMIT 1`, resetHMAC(secret, "email", email), passwordResetMaxAttempts).Scan(&id, &storedHash)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Код недействителен или истёк")
		return
	}
	wanted := resetHMAC(secret, "code", email+":"+code)
	if !hmac.Equal([]byte(storedHash), []byte(wanted)) {
		_, _ = db.ExecContext(ctx, `UPDATE password_reset_requests SET failed_attempts=failed_attempts+1 WHERE id=$1`, id)
		writeJSON(w, http.StatusBadRequest, "Код недействителен или истёк")
		return
	}
	token, err := newResetToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	tokenHash := resetHMAC(secret, "token", token)
	result, err := db.ExecContext(ctx, `UPDATE password_reset_requests SET verified_at=NOW(),reset_token_hash=$1,reset_expires_at=NOW()+INTERVAL '10 minutes' WHERE id=$2 AND verified_at IS NULL`, tokenHash, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		writeJSON(w, http.StatusBadRequest, "Код недействителен или истёк")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"reset_token": token})
}

func completePasswordReset(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ResetToken           string `json:"reset_token"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if !decodeResetJSON(w, r, &input) {
		return
	}
	if input.Password != input.PasswordConfirmation {
		writeJSON(w, http.StatusBadRequest, "Пароли не совпадают")
		return
	}
	if len(input.Password) < 8 || len(input.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, "Пароль должен содержать от 8 до 72 символов")
		return
	}
	secret, err := passwordResetSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Сервис временно недоступен")
		return
	}
	defer tx.Rollback()
	var requestID, userID int64
	err = tx.QueryRowContext(ctx, `SELECT id,user_id FROM password_reset_requests WHERE reset_token_hash=$1 AND verified_at IS NOT NULL AND used_at IS NULL AND reset_expires_at>NOW() FOR UPDATE`, resetHMAC(secret, "token", strings.TrimSpace(input.ResetToken))).Scan(&requestID, &userID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Ссылка восстановления недействительна или истекла")
		return
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, string(hash), userID); err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE password_reset_requests SET used_at=NOW() WHERE user_id=$1 AND used_at IS NULL`, userID)
	}
	if err != nil || tx.Commit() != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось изменить пароль")
		return
	}
	writeJSON(w, http.StatusOK, "Пароль изменён. Теперь вы можете войти.")
}
