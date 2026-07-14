package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "fintalent_session"

var db *sql.DB

type user struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/fintalent?sslmode=disable"
	}
	var err error
	db, err = sql.Open("pgx", databaseURL)
	if err != nil {
		log.Printf("Ошибка настройки PostgreSQL: %v", err)
	} else if err = prepareDatabase(); err != nil {
		log.Printf("PostgreSQL пока недоступен: %v", err)
	} else {
		log.Println("PostgreSQL подключен")
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/", servePage("static/index.html"))
	http.HandleFunc("/register", servePage("static/register.html"))
	http.HandleFunc("/login", servePage("static/login.html"))
	http.HandleFunc("/profile", servePage("static/profile.html"))
	http.HandleFunc("/api/register", registerUser)
	http.HandleFunc("/api/login", loginUser)
	http.HandleFunc("/api/logout", logoutUser)
	http.HandleFunc("/api/me", currentUser)
	registerAdminRoutes()
	registerResumeRoutes()

	log.Println("FinTalent запущен: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func prepareDatabase() error {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, full_name VARCHAR(200) NOT NULL,
		email VARCHAR(254) NOT NULL UNIQUE, password_hash VARCHAR(60) NOT NULL,
		agreed_to_terms BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash CHAR(64) NOT NULL UNIQUE, expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions(expires_at);`)
	if err != nil {
		return err
	}
	return prepareAdminDatabase(ctx)
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func servePage(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filename)
	}
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) || !parseForm(w, r) {
		return
	}
	fullName := strings.Join(strings.Fields(r.FormValue("full_name")), " ")
	emailAddress := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	if len([]rune(fullName)) < 3 || len([]rune(fullName)) > 200 {
		writeJSON(w, http.StatusBadRequest, "Укажите корректное ФИО")
		return
	}
	if !validEmail(emailAddress) {
		writeJSON(w, http.StatusBadRequest, "Укажите корректный email")
		return
	}
	if len(password) < 8 || len(password) > 72 {
		writeJSON(w, http.StatusBadRequest, "Пароль должен содержать от 8 до 72 символов")
		return
	}
	if r.FormValue("agreement") != "on" {
		writeJSON(w, http.StatusBadRequest, "Необходимо принять условия")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, "Не удалось обработать пароль")
		return
	}
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var userID int64
	err = db.QueryRowContext(ctx, `INSERT INTO users (full_name,email,password_hash,agreed_to_terms) VALUES ($1,$2,$3,TRUE) RETURNING id`, fullName, emailAddress, string(hash)).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			writeJSON(w, http.StatusConflict, "Пользователь с таким email уже зарегистрирован")
		} else {
			log.Printf("Ошибка регистрации: %v", err)
			writeJSON(w, http.StatusServiceUnavailable, "База данных временно недоступна")
		}
		return
	}
	if err := createSession(w, userID); err != nil {
		log.Printf("Ошибка создания сессии: %v", err)
		writeJSON(w, http.StatusInternalServerError, "Аккаунт создан, но не удалось выполнить вход")
		return
	}
	writeJSON(w, http.StatusCreated, "Аккаунт успешно создан")
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) || !parseForm(w, r) {
		return
	}
	emailAddress := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var userID int64
	var passwordHash string
	err := db.QueryRowContext(ctx, `SELECT id,password_hash FROM users WHERE email=$1`, emailAddress).Scan(&userID, &passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		writeJSON(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if err := createSession(w, userID); err != nil {
		log.Printf("Ошибка входа: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, "Не удалось выполнить вход")
		return
	}
	writeJSON(w, http.StatusOK, "Вход выполнен")
}

func createSession(w http.ResponseWriter, userID int64) error {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().Add(30 * 24 * time.Hour)
	ctx, cancel := contextWithTimeout()
	defer cancel()
	_, err := db.ExecContext(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)`, userID, hex.EncodeToString(hash[:]), expires)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: 30 * 24 * 60 * 60})
	return nil
}

func userFromRequest(r *http.Request) (*user, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	ctx, cancel := contextWithTimeout()
	defer cancel()
	u := &user{}
	err = db.QueryRowContext(ctx, `SELECT u.id,u.full_name,u.email FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>NOW()`, hex.EncodeToString(hash[:])).Scan(&u.ID, &u.FullName, &u.Email)
	return u, err
}

func currentUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(u)
}

func logoutUser(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		hash := sha256.Sum256([]byte(cookie.Value))
		ctx, cancel := contextWithTimeout()
		_, _ = db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hex.EncodeToString(hash[:]))
		cancel()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, "Выход выполнен")
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return false
	}
	return true
}

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseMultipartForm(64 << 10); err != nil && r.Form == nil {
		writeJSON(w, http.StatusBadRequest, "Некорректные данные формы")
		return false
	}
	return true
}

func validEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value && len(value) <= 254
}

func writeJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	key := "message"
	if status >= 400 {
		key = "error"
	}
	_ = json.NewEncoder(w).Encode(map[string]string{key: message})
}
