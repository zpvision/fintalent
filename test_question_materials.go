package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"FinTalent/internal/testmodule/repository"
)

const questionMaterialLimit = 6 << 20

func questionMaterialType(data []byte) string {
	typ := http.DetectContentType(data)
	if typ == "application/pdf" && bytes.HasPrefix(data, []byte("%PDF-")) {
		return typ
	}
	if typ != "image/jpeg" && typ != "image/png" {
		return ""
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40000000 {
		return ""
	}
	return typ
}

func uploadQuestionMaterial(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, 405, "Метод не поддерживается")
		return
	}
	user, err := userFromRequest(r)
	if err != nil {
		jsonError(w, 401, "Войдите в аккаунт")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, questionMaterialLimit+(1<<20))
	if err = r.ParseMultipartForm(1 << 20); err != nil {
		jsonError(w, 400, "Файл должен быть не больше 6 МБ")
		return
	}
	defer r.MultipartForm.RemoveAll()
	testID, _ := strconv.ParseInt(r.FormValue("test_id"), 10, 64)
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, 400, "Выберите изображение или PDF")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, questionMaterialLimit+1))
	if err != nil || len(data) == 0 || len(data) > questionMaterialLimit {
		jsonError(w, 400, "Файл должен быть не больше 6 МБ")
		return
	}
	typ := questionMaterialType(data)
	if typ == "" {
		jsonError(w, 400, "Разрешены изображения JPG, PNG и документы PDF")
		return
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(name))
	if (typ == "image/jpeg" && ext != ".jpg" && ext != ".jpeg") || (typ == "image/png" && ext != ".png") || (typ == "application/pdf" && ext != ".pdf") {
		jsonError(w, 400, "Расширение файла должно соответствовать содержимому: JPG, PNG или PDF")
		return
	}
	name = strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return -1
		}
		return c
	}, name)
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) > 180 {
		stem := []rune(strings.TrimSuffix(name, filepath.Ext(name)))
		name = string(stem[:180-len(ext)]) + ext
	}
	if name == "" || name == "." {
		name = "Материал"
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 500, "Не удалось загрузить файл")
		return
	}
	defer tx.Rollback()
	var locked int64
	err = tx.QueryRowContext(r.Context(), `SELECT t.id FROM tests t JOIN test_versions v ON v.test_id=t.id AND v.version=t.current_version JOIN users u ON u.id=t.author_id WHERE t.id=$1 AND t.author_id=$2 AND NOT u.is_blocked AND t.status='draft' AND t.deleted_at IS NULL AND v.published_at IS NULL AND NOT EXISTS(SELECT 1 FROM test_attempts a WHERE a.test_version_id=v.id) FOR UPDATE OF t`, testID, user.ID).Scan(&locked)
	if err != nil {
		jsonError(w, 403, "Материалы можно добавлять только в свой редактируемый черновик теста")
		return
	}
	var used int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(size),0) FROM test_question_materials WHERE test_id=$1`, testID).Scan(&used); err != nil {
		jsonError(w, 500, "Не удалось загрузить файл")
		return
	}
	if used+int64(len(data)) > 256<<20 {
		jsonError(w, 400, "Общий размер материалов этого теста превысит 256 МБ")
		return
	}
	asset := repository.Material{Name: name, MIME: typ, Size: len(data)}
	err = tx.QueryRowContext(r.Context(), `INSERT INTO test_question_materials(test_id,owner_id,name,mime,size,content) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, testID, user.ID, name, typ, len(data), data).Scan(&asset.ID)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		jsonError(w, 500, "Не удалось сохранить файл")
		return
	}
	jsonResponse(w, 201, asset)
}

func serveQuestionMaterial(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != "GET" && r.Method != "HEAD" {
		jsonError(w, 405, "Метод не поддерживается")
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/test-materials/"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	question, _ := strconv.ParseInt(r.URL.Query().Get("question"), 10, 64)
	var uid int64
	if u, e := userFromRequest(r); e == nil {
		uid = u.ID
	}
	invitation := r.Header.Get("X-Test-Invitation")
	resultToken := r.Header.Get("X-Test-Result")
	if len(invitation) > 128 || len(resultToken) > 128 {
		http.NotFound(w, r)
		return
	}
	resultHash := ""
	if decoded, e := hex.DecodeString(resultToken); e == nil && len(decoded) == 32 {
		sum := sha256.Sum256([]byte(resultToken))
		resultHash = hex.EncodeToString(sum[:])
	}
	var data []byte
	var name, typ string
	err = db.QueryRowContext(r.Context(), questionMaterialAccessSQL, id, uid, isAdmin(r), question, invitation, resultHash).Scan(&name, &typ, &data)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			jsonError(w, 500, "Не удалось открыть материал")
		}
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == "GET" {
		_, _ = w.Write(data)
	}
}

const questionMaterialAccessSQL = `SELECT m.name,m.mime,m.content FROM test_question_materials m JOIN tests t ON t.id=m.test_id JOIN users author ON author.id=t.author_id
 WHERE m.id=$1 AND (NOT author.is_blocked OR author.is_system) AND (
 $3 OR (m.owner_id=$2 AND EXISTS(SELECT 1 FROM users u WHERE u.id=$2 AND NOT u.is_blocked)) OR EXISTS(
 SELECT 1 FROM test_questions q JOIN test_versions v ON v.id=q.test_version_id WHERE q.id=$4 AND v.test_id=m.test_id AND COALESCE(q.settings->'materials','[]'::jsonb) @> jsonb_build_array(jsonb_build_object('id',m.id)) AND (
 ($2>0 AND t.status='published' AND t.visibility IN('public','marketplace') AND t.deleted_at IS NULL AND v.version=t.current_version AND EXISTS(SELECT 1 FROM users u WHERE u.id=$2 AND NOT u.is_blocked)) OR
 EXISTS(SELECT 1 FROM test_attempts a JOIN users u ON u.id=a.user_id WHERE a.test_version_id=v.id AND a.user_id=$2 AND NOT u.is_blocked) OR
 EXISTS(SELECT 1 FROM company_test_invitations i JOIN company_test_employees e ON e.id=i.employee_id JOIN users owner ON owner.id=i.owner_user_id LEFT JOIN users recipient ON recipient.id=$2 LEFT JOIN test_attempts a ON a.id=i.attempt_id WHERE i.test_version_id=v.id AND NOT owner.is_blocked AND NOT owner.is_system AND (
 (i.token=$5 AND $5<>'' AND i.status IN('started','finished') AND i.expires_at>NOW()) OR
 (i.result_token_hash=$6 AND $6<>'' AND i.result_token_expires_at>NOW() AND i.status='finished' AND a.status='finished') OR
 (recipient.id>0 AND NOT recipient.is_blocked AND lower(recipient.email)=lower(e.email) AND i.status='finished' AND a.status='finished')
 )))))`
