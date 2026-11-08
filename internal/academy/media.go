package academy

import (
	"FinTalent/internal/objectstorage"
	"archive/zip"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	if len(parts) > 0 && parts[0] == "uploads" {
		h.uploadSessions(w, r, course, user, program, parts[1:])
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
		settings, settingErr := academySettings(r.Context(), h.db)
		if settingErr != nil {
			problem(w, settingErr)
			return
		}
		limit := int64(settings.UploadMB) << 20
		r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			fail(w, 400, "Не удалось загрузить файл. Максимум 20 МБ")
			return
		}
		defer r.MultipartForm.RemoveAll()
		key := r.FormValue("entity_key")
		var exists bool
		e := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_editable_entities WHERE program_id=$1 AND key=$2)`, program, key).Scan(&exists)
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

		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			upload.remove()
			problem(w, e)
			return
		}
		defer tx.Rollback()
		var id int64
		e = tx.QueryRowContext(r.Context(), `INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by,storage_driver,checksum,checksum_kind) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'sha256') RETURNING id`, program, key, storage, name, typ, upload.size, user, upload.driver, upload.checksum).Scan(&id)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_asset_links(asset_id,program_id,entity_key) VALUES($1,$2,$3)`, id, program, key)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			upload.remove()
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
		tx, err := h.db.BeginTx(r.Context(), nil)
		if err != nil {
			problem(w, err)
			return
		}
		defer tx.Rollback()
		var locked int64
		if err = tx.QueryRowContext(r.Context(), "SELECT id FROM academy_course_programs WHERE id=$1 FOR UPDATE", program).Scan(&locked); err != nil {
			problem(w, err)
			return
		}
		var inUse bool
		e = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_all_program_contents v CROSS JOIN LATERAL jsonb_array_elements(v.content->'modules') m WHERE v.program_id=$1 AND (m->>'cover_asset_id'=$2::bigint::text OR EXISTS(SELECT 1 FROM jsonb_array_elements(m->'lessons') l WHERE l->>'cover_asset_id'=$2::bigint::text OR EXISTS(SELECT 1 FROM jsonb_array_elements(l->'blocks') b WHERE b->'data'->>'asset_id'=$2::bigint::text OR b->'data'->>'poster_id'=$2::bigint::text OR (b->'data'->'material_ids') @> to_jsonb($2::bigint))))) OR EXISTS(SELECT 1 FROM academy_modules WHERE program_id=$1 AND cover_asset_id=$2::bigint) OR EXISTS(SELECT 1 FROM academy_lessons WHERE program_id=$1 AND cover_asset_id=$2::bigint) OR EXISTS(SELECT 1 FROM academy_lesson_blocks WHERE program_id=$1 AND (data->>'asset_id'=$2::bigint::text OR data->>'poster_id'=$2::bigint::text OR (data->'material_ids') @> to_jsonb($2::bigint)))`, program, id).Scan(&inUse)
		if e != nil {
			problem(w, e)
			return
		}
		if inUse {
			fail(w, 409, "Файл используется программой или сохранённой версией. Сначала уберите его из материалов; файлы версий сохраняются")
			return
		}
		_, e = tx.ExecContext(r.Context(), `WITH removed AS(UPDATE academy_media_assets SET deleted_at=now() WHERE id=$1 AND program_id=$2 RETURNING storage_key,storage_driver) INSERT INTO academy_storage_gc(storage_key,storage_driver,next_at) SELECT storage_key,storage_driver,now() FROM removed ON CONFLICT(storage_key,storage_driver) DO UPDATE SET next_at=now()`, id, program)
		if e == nil {
			e = tx.Commit()
		}
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
func (h *Handler) sendAssetFile(w http.ResponseWriter, r *http.Request, key, name, typ string) {
	if !regexpStorage(key) {
		fail(w, 404, "Файл не найден")
		return
	}
	h.sendStoredAsset(w, r, key, name, typ)
}
func regexpStorage(s string) bool {
	if strings.HasPrefix(s, "users/") {
		return regexp.MustCompile(`^users/[1-9][0-9]*/academy/[a-f0-9]{48}$`).MatchString(s)
	}
	if len(s) != 48 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}

type storedUpload struct {
	storage, name, mime, path, driver, checksum string
	size                                        int
	cleanup                                     func()
}

func (u storedUpload) remove() {
	if u.cleanup != nil {
		u.cleanup()
	}
}
func parseUpload(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAssetSize+(1<<20))
	if r.ParseMultipartForm(1<<20) != nil {
		return fmt.Errorf("Не удалось загрузить файл. Максимум 20 МБ")
	}
	return nil
}
func (h *Handler) storeUpload(r *http.Request) (storedUpload, error) {
	settings, e := academySettings(r.Context(), h.db)
	if e != nil {
		return storedUpload{}, e
	}
	limit := int64(settings.UploadMB) << 20
	f, header, e := r.FormFile("file")
	if e != nil {
		return storedUpload{}, inputError{"Выберите файл"}
	}
	defer f.Close()
	tmp, e := os.CreateTemp("", "academy-upload-*")
	if e != nil {
		return storedUpload{}, e
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	digest := newDigest()
	n, e := io.Copy(io.MultiWriter(tmp, digest), io.LimitReader(f, limit+1))
	if e != nil || n == 0 || n > limit {
		return storedUpload{}, inputError{"Файл пустой или больше разрешённого размера"}
	}
	typ, e := streamFileType(header.Filename, tmp, n)
	if e != nil {
		return storedUpload{}, inputError{e.Error()}
	}
	name := cleanAssetName(header.Filename)
	user, e := h.user(r)
	if e != nil || user <= 0 {
		return storedUpload{}, ErrAccess
	}
	fullKey, e := objectstorage.UserKey(user, "academy")
	if e != nil {
		return storedUpload{}, e
	}
	key := fullKey
	if h.uploadDriver() == "local" {
		key = fullKey[strings.LastIndex(fullKey, "/")+1:]
	}
	driver := h.uploadDriver()
	store, e := h.store(driver)
	if e != nil {
		return storedUpload{}, e
	}
	if e = h.reserveObject(r.Context(), key, driver); e != nil {
		return storedUpload{}, e
	}
	if _, e = tmp.Seek(0, 0); e != nil {
		return storedUpload{}, e
	}
	if e = store.Put(r.Context(), objectKey(driver, key), tmp, n, typ); e != nil {
		return storedUpload{}, e
	}
	return storedUpload{storage: key, name: name, mime: typ, size: int(n), driver: driver, checksum: digestHex(digest), cleanup: func() { h.queueDelete(key, driver) }}, nil
}
