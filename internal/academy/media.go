package academy

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxAssetSize = 20 << 20

func fileType(name string, body []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	detected := http.DetectContentType(body)
	images := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp"}
	if typ := images[ext]; typ != "" && typ == detected {
		return typ, nil
	}
	if ext == ".pdf" && bytes.HasPrefix(body, []byte("%PDF-")) {
		return "application/pdf", nil
	}
	if (ext == ".txt" || ext == ".csv") && utf8.Valid(body) && !bytes.ContainsRune(body, 0) && (strings.HasPrefix(detected, "text/plain") || strings.HasPrefix(detected, "text/csv")) {
		return "text/plain; charset=utf-8", nil
	}
	if oneOf(ext, ".zip", ".docx", ".xlsx", ".pptx") {
		zr, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if e != nil {
			return "", fmt.Errorf("Повреждённый архив или документ")
		}
		if len(zr.File) > 10000 {
			return "", fmt.Errorf("Слишком много файлов в архиве")
		}
		if ext == ".zip" {
			return "application/zip", nil
		}
		prefix := map[string]string{".docx": "word/", ".xlsx": "xl/", ".pptx": "ppt/"}[ext]
		content, part := false, false
		for _, f := range zr.File {
			if f.Name == "[Content_Types].xml" {
				content = true
			}
			if strings.HasPrefix(f.Name, prefix) {
				part = true
			}
			if strings.HasSuffix(strings.ToLower(f.Name), "vbaproject.bin") {
				return "", fmt.Errorf("Документы с макросами не поддерживаются")
			}
		}
		if content && part {
			return map[string]string{".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation"}[ext], nil
		}
	}
	return "", fmt.Errorf("Разрешены JPG, PNG, GIF, WebP, PDF, DOCX, XLSX, PPTX, ZIP, TXT, CSV; до 20 МБ. Видео — по ссылке")
}
func (h *Handler) assets(w http.ResponseWriter, r *http.Request, course, user int64, parts []string) {
	var program int64
	if e := h.db.QueryRowContext(r.Context(), `SELECT id FROM academy_course_programs WHERE course_id=$1`, course).Scan(&program); e != nil {
		problem(w, e)
		return
	}
	if len(parts) == 0 || parts[0] == "" {
		if r.Method == "GET" {
			var raw []byte
			e := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'entity_key',entity_key,'name',name,'mime',mime,'size',size) ORDER BY id),'[]') FROM academy_media_assets WHERE program_id=$1 AND submission_id IS NULL AND deleted_at IS NULL`, program).Scan(&raw)
			if e != nil {
				problem(w, e)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(raw)
			return
		}
		if r.Method != "POST" {
			fail(w, 405, "Метод не поддерживается")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAssetSize+(1<<20))
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			fail(w, 400, "Не удалось загрузить файл. Максимум 20 МБ")
			return
		}
		defer r.MultipartForm.RemoveAll()
		key := r.FormValue("entity_key")
		var exists bool
		e := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_lessons WHERE program_id=$1 AND key=$2 UNION ALL SELECT 1 FROM academy_modules WHERE program_id=$1 AND key=$2)`, program, key).Scan(&exists)
		if e != nil {
			problem(w, e)
			return
		}
		if !exists {
			fail(w, 400, "Сначала сохраните модуль или урок")
			return
		}
		upload, e := h.storeUpload(r)
		if e != nil {
			problem(w, e)
			return
		}
		storage, name, typ := upload.storage, upload.name, upload.mime
		file := filepath.Join(h.root, storage)
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			_ = os.Remove(file)
			problem(w, e)
			return
		}
		defer tx.Rollback()
		var id int64
		e = tx.QueryRowContext(r.Context(), `INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, program, key, storage, name, typ, upload.size, user).Scan(&id)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_asset_links(asset_id,program_id,entity_key) VALUES($1,$2,$3)`, id, program, key)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			_ = os.Remove(file)
			problem(w, e)
			return
		}
		respond(w, 201, map[string]any{"id": id, "entity_key": key, "name": name, "mime": typ, "size": upload.size})
		return
	}
	id, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || id <= 0 {
		fail(w, 404, "Файл не найден")
		return
	}
	var storage, name, typ string
	e = h.db.QueryRowContext(r.Context(), `SELECT storage_key,name,mime FROM academy_media_assets WHERE id=$1 AND program_id=$2 AND submission_id IS NULL AND deleted_at IS NULL`, id, program).Scan(&storage, &name, &typ)
	if e != nil {
		problem(w, e)
		return
	}
	if r.Method == "DELETE" {
		var inUse bool
		e = h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_program_versions WHERE program_id=$1) OR EXISTS(SELECT 1 FROM academy_modules WHERE program_id=$1 AND cover_asset_id=$2) OR EXISTS(SELECT 1 FROM academy_lessons WHERE program_id=$1 AND cover_asset_id=$2) OR EXISTS(SELECT 1 FROM academy_lesson_blocks WHERE program_id=$1 AND (data->>'asset_id'=$2::text OR data->>'poster_id'=$2::text OR (data->'material_ids') @> to_jsonb($2::bigint)))`, program, id).Scan(&inUse)
		if e != nil {
			problem(w, e)
			return
		}
		if inUse {
			fail(w, 409, "Файл используется программой или сохранённой версией. Сначала уберите его из материалов; файлы версий сохраняются")
			return
		}
		_, e = h.db.ExecContext(r.Context(), `UPDATE academy_media_assets SET deleted_at=NOW() WHERE id=$1 AND program_id=$2`, id, program)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, map[string]bool{"deleted": true})
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	h.sendAssetFile(w, r, storage, name, typ)
}
func (h *Handler) serveAsset(w http.ResponseWriter, r *http.Request, program, id int64) {
	var storage, name, typ string
	err := h.db.QueryRowContext(r.Context(), `SELECT storage_key,name,mime FROM academy_media_assets WHERE program_id=$1 AND id=$2 AND deleted_at IS NULL`, program, id).Scan(&storage, &name, &typ)
	if err != nil {
		problem(w, err)
		return
	}
	h.sendAssetFile(w, r, storage, name, typ)
}
func (h *Handler) sendAssetFile(w http.ResponseWriter, r *http.Request, storage, name, typ string) {
	if !regexpStorage(storage) {
		fail(w, 404, "Файл не найден")
		return
	}
	f, e := os.Open(filepath.Join(h.root, storage))
	if e != nil {
		fail(w, 404, "Файл временно недоступен")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		problem(w, e)
		return
	}
	disposition := "attachment"
	if strings.HasPrefix(typ, "image/") && r.URL.Query().Get("download") != "1" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
func regexpStorage(s string) bool {
	if len(s) != 48 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}

type storedUpload struct {
	storage, name, mime, path string
	size                      int
}

func (u storedUpload) remove() { _ = os.Remove(u.path) }
func parseUpload(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAssetSize+(1<<20))
	if r.ParseMultipartForm(1<<20) != nil {
		return fmt.Errorf("Не удалось загрузить файл. Максимум 20 МБ")
	}
	return nil
}
func (h *Handler) storeUpload(r *http.Request) (storedUpload, error) {
	f, header, e := r.FormFile("file")
	if e != nil {
		return storedUpload{}, inputError{"Выберите файл"}
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, maxAssetSize+1))
	if e != nil || len(body) == 0 || len(body) > maxAssetSize {
		return storedUpload{}, inputError{"Файл пустой или больше 20 МБ"}
	}
	typ, e := fileType(header.Filename, body)
	if e != nil {
		return storedUpload{}, inputError{e.Error()}
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	runes := []rune(name)
	if len(runes) > 240 {
		name = string(runes[:240])
	}
	token := make([]byte, 24)
	if _, e = rand.Read(token); e != nil {
		return storedUpload{}, e
	}
	storage := hex.EncodeToString(token)
	if e = os.MkdirAll(h.root, 0700); e != nil {
		return storedUpload{}, e
	}
	file := filepath.Join(h.root, storage)
	if e = os.WriteFile(file, body, 0600); e != nil {
		return storedUpload{}, e
	}
	return storedUpload{storage, name, typ, file, len(body)}, nil
}
