package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	testhandler "FinTalent/internal/testmodule/handler"
	testrepository "FinTalent/internal/testmodule/repository"
	testservice "FinTalent/internal/testmodule/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "fintalent_session"

var db *sql.DB

type user struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

func loadLocalEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if name != "" && os.Getenv(name) == "" {
			_ = os.Setenv(name, value)
		}
	}
}

func main() {
	loadLocalEnv(".env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			log.Fatal("DATABASE_URL обязателен в production")
		}
		databaseURL = "postgres://postgres:postgres@localhost:5432/fintalent?sslmode=disable"
	}
	var err error
	db, err = sql.Open("pgx", databaseURL)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			log.Fatalf("Ошибка настройки PostgreSQL: %v", err)
		}
		log.Printf("Ошибка настройки PostgreSQL: %v", err)
	} else if err = prepareDatabase(); err != nil {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			log.Fatalf("Не удалось подготовить PostgreSQL: %v", err)
		}
		log.Printf("PostgreSQL пока недоступен: %v", err)
	} else {
		log.Println("PostgreSQL подключен")
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fs.ServeHTTP(w, r)
	})))
	http.HandleFunc("/", servePage("static/index.html"))
	http.HandleFunc("/register", servePage("static/register.html"))
	http.HandleFunc("/login", servePage("static/login.html"))
	http.HandleFunc("/profile", servePage("static/profile.html"))
	http.HandleFunc("/tests", servePage("static/tests.html"))
	http.HandleFunc("/tests/create", servePage("static/test-create.html"))
	http.HandleFunc("/tests/take", servePage("static/test-take.html"))
	http.HandleFunc("/vacancies/create", servePage("static/vacancy-create.html"))
	http.HandleFunc("/vacancies/view", servePage("static/vacancy-view.html"))
	http.HandleFunc("/vacancies", servePage("static/vacancies.html"))
	http.HandleFunc("/resumes", servePage("static/resumes.html"))
	http.HandleFunc("/docs/openapi.yaml", servePage("docs/openapi.yaml"))
	http.HandleFunc("/api/register", registerUser)
	http.HandleFunc("/api/login", loginUser)
	http.HandleFunc("/api/logout", logoutUser)
	http.HandleFunc("/api/me", currentUser)
	http.HandleFunc("/api/profile/avatar", profileAvatar)
	registerAdminRoutes()
	registerResumeRoutes()
	registerVacancyModuleRoutes()
	registerPublicVacancyRoutes()
	registerDemoContentRoutes()
	registerGeographyRoutes()
	registerMarketplaceRoutes()
	registerProfiMarketRoutes()
	registerPublicationRoutes()
	registerEmployeeTestingRoutes()
	testRepo := testrepository.New(db)
	testService := testservice.New(testRepo)
	testHandler := testhandler.New(testService, func(r *http.Request) (int64, error) {
		u, err := userFromRequest(r)
		if err != nil {
			return 0, err
		}
		return u.ID, nil
	}, isAdmin)
	testHandler.Register(http.DefaultServeMux)

	listenAddress := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if listenAddress == "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			port = "8080"
		}
		listenAddress = ":" + port
	}
	server := &http.Server{
		Addr:              listenAddress,
		Handler:           securityHeaders(http.DefaultServeMux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	log.Printf("FinTalent запущен на %s", listenAddress)
	log.Fatal(server.ListenAndServe())
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")
		if secureCookies() {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func prepareDatabase() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	if _, err = db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_blocked BOOLEAN NOT NULL DEFAULT FALSE`); err != nil {
		return err
	}
	if err := prepareAdminDatabase(ctx); err != nil {
		return err
	}
	if err := prepareVacancyModuleDatabase(ctx); err != nil {
		return err
	}
	if err := prepareTestingDatabase(ctx); err != nil {
		return err
	}
	if err := prepareTestCategories(ctx); err != nil {
		return err
	}
	if err := prepareMarketplaceDatabase(ctx); err != nil {
		return err
	}
	if err := prepareProfiMarketDatabase(ctx); err != nil {
		return err
	}
	if err := prepareGeographyDatabase(ctx); err != nil {
		return err
	}
	if err := preparePublicationDatabase(ctx); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SEED_DEMO_DATA")), "false") {
		return nil
	}
	if err := prepareDemoContent(ctx); err != nil {
		return err
	}
	if err := prepareProfiMarketDemo(ctx); err != nil {
		return err
	}
	return preparePublicationDemo(ctx)
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func secureCookies() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("COOKIE_SECURE")), "true")
}

func servePage(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if strings.HasSuffix(strings.ToLower(filename), ".html") {
			content, err := os.ReadFile(filename)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			content = []byte(strings.Replace(string(content), "</head>", `<link rel="stylesheet" href="/static/layout-safety.css"><link rel="stylesheet" href="/static/site-header.css?v=1"><link rel="stylesheet" href="/static/site-background.css?v=1"><script src="/static/site-errors.js?v=1"></script></head>`, 1))
			if filepath.Base(filename) == "profile.html" {
				content = []byte(strings.Replace(string(content), "</body>", `<script src="/static/profile-avatar.js?v=1"></script></body>`, 1))
			}
			content = []byte(strings.Replace(string(content), "</body>", `<script src="/static/site-header.js?v=1"></script></body>`, 1))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
			return
		}
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
	var isBlocked bool
	err := db.QueryRowContext(ctx, `SELECT id,password_hash,is_blocked FROM users WHERE email=$1`, emailAddress).Scan(&userID, &passwordHash, &isBlocked)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		writeJSON(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if isBlocked {
		writeJSON(w, http.StatusForbidden, "Ваш аккаунт заблокирован")
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
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: 30 * 24 * 60 * 60})
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
	err = db.QueryRowContext(ctx, `SELECT u.id,u.full_name,u.email,COALESCE(u.avatar_url,'') FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>NOW() AND NOT u.is_blocked`, hex.EncodeToString(hash[:])).Scan(&u.ID, &u.FullName, &u.Email, &u.Avatar)
	return u, err
}

func profileAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err = r.ParseMultipartForm(5 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, "Фотография должна быть не больше 5 МБ")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "Выберите фотографию")
		return
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[http.DetectContentType(head[:n])]
	if ext == "" {
		writeJSON(w, http.StatusBadRequest, "Поддерживаются JPG, PNG и WebP")
		return
	}
	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		writeJSON(w, 500, "Не удалось сохранить фотографию")
		return
	}
	dir := filepath.Join("static", "uploads", "avatars")
	if err = os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, 500, "Не удалось сохранить фотографию")
		return
	}
	name := hex.EncodeToString(token) + ext
	path := filepath.Join(dir, name)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		writeJSON(w, 500, "Не удалось сохранить фотографию")
		return
	}
	if _, err = out.Write(head[:n]); err == nil {
		_, err = io.Copy(out, io.LimitReader(file, 5<<20))
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		writeJSON(w, 500, "Не удалось сохранить фотографию")
		return
	}
	url := "/static/uploads/avatars/" + name
	if _, err = db.ExecContext(r.Context(), `UPDATE users SET avatar_url=$1 WHERE id=$2`, url, u.ID); err != nil {
		_ = os.Remove(path)
		writeJSON(w, 500, "Не удалось обновить профиль")
		return
	}
	profiRespond(w, http.StatusCreated, map[string]string{"avatar": url})
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
