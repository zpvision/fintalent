package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"FinTalent/internal/accountingcompany"
)

//go:embed migrations/040_accounting_companies.sql migrations/041_accounting_companies_demo.sql
var accountingCompanyMigrationFS embed.FS

func prepareAccountingCompanyDatabase(ctx context.Context) error {
	schema, err := accountingCompanyMigrationFS.ReadFile("migrations/040_accounting_companies.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func prepareAccountingCompanyDemo(ctx context.Context) error {
	schema, err := accountingCompanyMigrationFS.ReadFile("migrations/041_accounting_companies_demo.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(schema))
	return err
}

func registerAccountingCompanyRoutes() {
	h := accountingcompany.New(db, func(r *http.Request) (accountingcompany.User, error) {
		u, err := userFromRequest(r)
		if err != nil {
			return accountingcompany.User{}, err
		}
		return accountingcompany.User{ID: u.ID, FullName: u.FullName, Email: u.Email}, nil
	}, isAdmin)
	h.Register(http.DefaultServeMux)
	http.HandleFunc("/api/accounting-companies/upload", accountingCompanyUpload)
}

func accountingCompanyUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "Метод не поддерживается"})
		return
	}
	u, err := userFromRequest(r)
	if err != nil {
		jsonResponse(w, 401, map[string]string{"error": "Требуется авторизация"})
		return
	}
	if err = r.ParseMultipartForm(4 << 20); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "Файл слишком большой"})
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	kind := r.FormValue("kind")
	folder := map[string]string{"logo": "logos", "manager": "managers", "header": "headers"}[kind]
	if folder == "" {
		jsonResponse(w, 400, map[string]string{"error": "Некорректный тип изображения"})
		return
	}
	var allowed bool
	if db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM accounting_companies WHERE id=$1 AND owner_user_id=$2 AND deleted_at IS NULL)`, companyID, u.ID).Scan(&allowed) != nil || !allowed {
		jsonResponse(w, 403, map[string]string{"error": "Недостаточно прав"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "Выберите изображение"})
		return
	}
	defer file.Close()
	if header.Size > 3<<20 {
		jsonResponse(w, 400, map[string]string{"error": "Изображение должно быть не больше 3 МБ"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, (3<<20)+1))
	if err != nil || len(data) > 3<<20 {
		jsonResponse(w, 400, map[string]string{"error": "Не удалось прочитать изображение"})
		return
	}
	contentType := http.DetectContentType(data)
	extension := map[string]string{"image/jpeg": "jpg", "image/png": "png"}[contentType]
	if extension == "" {
		jsonResponse(w, 400, map[string]string{"error": "Поддерживаются JPG и PNG"})
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 100 || cfg.Height < 100 || cfg.Width > 6000 || cfg.Height > 6000 {
		jsonResponse(w, 400, map[string]string{"error": "Некорректное изображение"})
		return
	}
	token := make([]byte, 12)
	_, _ = rand.Read(token)
	name := fmt.Sprintf("company-%d-%s.%s", companyID, hex.EncodeToString(token), extension)
	dir := filepath.Join("static", "uploads", "accounting-companies", folder)
	if err = os.MkdirAll(dir, 0755); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Не удалось сохранить файл"})
		return
	}
	path := filepath.Join(dir, name)
	if err = os.WriteFile(path, data, 0644); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "Не удалось сохранить файл"})
		return
	}
	url := "/static/" + filepath.ToSlash(filepath.Join("uploads", "accounting-companies", folder, name))
	jsonResponse(w, 200, map[string]string{"url": url})
}
