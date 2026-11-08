package academy

import (
	"FinTalent/internal/objectstorage"
	"bytes"
	"context"
	"database/sql"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const uploadPartSize int64 = 32 << 20

type uploadSession struct {
	ErrorCode      string    `json:"error_code"`
	ChecksumOffset int64     `json:"checksum_offset"`
	ChecksumState  string    `json:"checksum_state"`
	ObjectETag     string    `json:"object_etag"`
	ID             string    `json:"id"`
	ProgramID      int64     `json:"program_id"`
	CourseID       int64     `json:"course_id"`
	UserID         int64     `json:"user_id"`
	EntityKey      string    `json:"entity_key"`
	Key            string    `json:"storage_key"`
	Backend        string    `json:"backend_id"`
	Name           string    `json:"name"`
	MIME           string    `json:"mime"`
	Size           int64     `json:"size"`
	PartSize       int64     `json:"part_size"`
	UploadID       string    `json:"upload_id"`
	Status         string    `json:"status"`
	AssetID        *int64    `json:"asset_id"`
	Expires        time.Time `json:"expires_at"`
}

func (h *Handler) getUpload(ctx context.Context, id string) (uploadSession, error) {
	var s uploadSession
	var raw []byte
	e := h.db.QueryRowContext(ctx, `SELECT to_jsonb(u) FROM academy_upload_sessions u WHERE id=$1`, id).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &s)
	}
	return s, e
}
func cleanAssetName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	r := []rune(name)
	if len(r) > 240 {
		ext := []rune(filepath.Ext(name))
		if len(ext) > 16 {
			ext = nil
		}
		name = string(r[:240-len(ext)]) + string(ext)
	}
	return name
}
func directType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	return map[string]string{".mp4": "video/mp4", ".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation", ".zip": "application/zip", ".txt": "text/plain; charset=utf-8", ".csv": "text/plain; charset=utf-8"}[ext]
}
func (h *Handler) uploadSessions(w http.ResponseWriter, r *http.Request, course, user, program int64, path []string) {
	if len(path) == 0 && r.Method == "GET" {
		settings, e := academySettings(r.Context(), h.db)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, map[string]any{"enabled": h.storage.Multipart != nil, "video_max_bytes": h.storage.VideoMax, "document_max_bytes": int64(settings.UploadMB) << 20})
		return
	}
	remote := h.storage.Multipart
	if remote == nil {
		fail(w, 409, "Прямая загрузка доступна после подключения S3")
		return
	}
	if len(path) == 0 && r.Method == "POST" {
		var in struct {
			ID     string `json:"request_key"`
			Entity string `json:"entity_key"`
			Name   string `json:"name"`
			Size   int64  `json:"size"`
		}
		if !decode(w, r, &in) {
			return
		}
		in.Name = cleanAssetName(in.Name)
		typ := directType(in.Name)
		limit := int64(maxAssetSize)
		if typ == "video/mp4" {
			limit = h.storage.VideoMax
		} else {
			settings, e := academySettings(r.Context(), h.db)
			if e != nil {
				problem(w, e)
				return
			}
			limit = int64(settings.UploadMB) << 20
		}
		if !keyPattern.MatchString(in.ID) || !keyPattern.MatchString(in.Entity) || in.Size <= 0 || in.Size > limit || typ == "" || in.Name == "." {
			fail(w, 400, "Проверьте имя, тип и размер файла")
			return
		}
		// Lock only while reserving quota; no S3 calls in this transaction.
		tx, e := h.db.BeginTx(r.Context(), nil)
		if e != nil {
			problem(w, e)
			return
		}
		defer tx.Rollback()
		var found int64
		if e = tx.QueryRowContext(r.Context(), `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&found); e != nil {
			problem(w, e)
			return
		}
		e = tx.QueryRowContext(r.Context(), `SELECT id FROM academy_course_programs WHERE id=$1 FOR UPDATE`, program).Scan(&found)
		if e != nil {
			problem(w, e)
			return
		}
		var valid bool
		e = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academy_editable_entities WHERE program_id=$1 AND key=$2)`, program, in.Entity).Scan(&valid)
		if e != nil {
			problem(w, e)
			return
		}
		if !valid {
			fail(w, 400, "Сначала сохраните модуль или урок")
			return
		}
		existing, e := h.getUpload(r.Context(), in.ID)
		if e == nil {
			if existing.UserID != user || existing.CourseID != course || existing.EntityKey != in.Entity || existing.Name != in.Name || existing.Size != in.Size {
				problem(w, ErrConflict)
				return
			}
			if existing.Status != "ready" && existing.Backend != remote.Identity() {
				fail(w, 409, "Конфигурация хранилища изменилась. Начните загрузку заново")
				return
			}
			_ = tx.Rollback()
			h.uploadState(w, r, existing)
			return
		}
		if !errors.Is(e, sql.ErrNoRows) {
			problem(w, e)
			return
		}
		var count int
		e = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM academy_upload_sessions WHERE user_id=$1 AND status IN('initiating','uploading','finalizing') AND expires_at>now()`, user).Scan(&count)
		if e != nil {
			problem(w, e)
			return
		}
		if count >= 8 {
			fail(w, 429, "Завершите или отмените предыдущие загрузки")
			return
		}
		key, e := objectstorage.UserKey(user, "academy")
		if e != nil {
			problem(w, e)
			return
		}
		_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_upload_sessions(id,program_id,course_id,user_id,entity_key,storage_key,backend_id,name,mime,size,part_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, in.ID, program, course, user, in.Entity, key, remote.Identity(), in.Name, typ, in.Size, uploadPartSize)
		if e == nil {
			_, e = tx.ExecContext(r.Context(), `INSERT INTO academy_storage_gc(storage_key,storage_driver,next_at) VALUES($1,'s3',now()+INTERVAL '26 hours')`, key)
		}
		if e == nil {
			e = tx.Commit()
		}
		if e != nil {
			problem(w, e)
			return
		}
		upload, e := remote.Begin(r.Context(), objectKey("s3", key), typ)
		if e != nil {
			_, _ = h.db.ExecContext(r.Context(), `UPDATE academy_upload_sessions SET status='failed',error_code='init_failed' WHERE id=$1`, in.ID)
			h.storageFailure(w)
			return
		}
		_, e = h.db.ExecContext(r.Context(), `UPDATE academy_upload_sessions SET upload_id=$2,status='uploading',updated_at=now() WHERE id=$1`, in.ID, upload)
		if e != nil {
			problem(w, e)
			return
		}
		s, e := h.getUpload(r.Context(), in.ID)
		if e != nil {
			problem(w, e)
			return
		}
		h.uploadState(w, r, s)
		return
	}
	if len(path) == 0 {
		fail(w, 405, "Метод не поддерживается")
		return
	}
	s, e := h.getUpload(r.Context(), path[0])
	if e != nil {
		problem(w, e)
		return
	}
	if s.UserID != user || s.ProgramID != program || s.CourseID != course {
		problem(w, sql.ErrNoRows)
		return
	}
	if s.Status != "ready" && s.Backend != remote.Identity() {
		fail(w, 409, "Конфигурация хранилища изменилась. Начните загрузку заново")
		return
	}
	if len(path) == 1 && r.Method == "GET" {
		h.uploadState(w, r, s)
		return
	}
	if len(path) == 1 && r.Method == "DELETE" {
		if s.Status == "ready" {
			fail(w, 409, "Файл уже сохранён. Удалите материал обычным способом")
			return
		}
		if s.Status == "finalizing" {
			fail(w, 409, "Дождитесь завершения проверки файла")
			return
		}
		_, e = h.db.ExecContext(r.Context(), `UPDATE academy_upload_sessions SET status='cancelled',expires_at=now(),updated_at=now() WHERE id=$1 AND status IN('uploading','initiating','failed')`, s.ID)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 200, map[string]bool{"cancelled": true})
		return
	}
	if !s.Expires.After(time.Now()) || !oneOf(s.Status, "uploading", "finalizing", "ready") {
		fail(w, 409, "Сессия загрузки закончилась. Выберите файл заново")
		return
	}
	if len(path) == 2 && path[1] == "complete" && r.Method == "POST" {
		_, e = h.db.ExecContext(r.Context(), `UPDATE academy_upload_sessions SET status='finalizing',updated_at=now() WHERE id=$1 AND status='uploading'`, s.ID)
		if e != nil {
			problem(w, e)
			return
		}
		respond(w, 202, map[string]string{"status": "finalizing"})
		return
	}
	if len(path) == 3 && path[1] == "parts" && r.Method == "POST" && s.Status == "uploading" {
		n, _ := strconv.Atoi(path[2])
		count := int((s.Size + s.PartSize - 1) / s.PartSize)
		if n < 1 || n > count {
			problem(w, ErrInvalid)
			return
		}
		size := s.PartSize
		if n == count {
			size = s.Size - int64(n-1)*s.PartSize
		}
		url, e := remote.SignPart(r.Context(), objectKey("s3", s.Key), s.UploadID, n, size, h.storage.TTL)
		if e != nil {
			h.storageFailure(w)
			return
		}
		respond(w, 200, url)
		return
	}
	fail(w, 405, "Метод не поддерживается")
}
func (h *Handler) uploadState(w http.ResponseWriter, r *http.Request, s uploadSession) {
	parts := []objectstorage.Part{}
	if s.Status == "uploading" && s.Expires.After(time.Now()) {
		var e error
		parts, e = h.storage.Multipart.Parts(r.Context(), objectKey("s3", s.Key), s.UploadID)
		if e != nil {
			h.storageFailure(w)
			return
		}
	}
	var asset json.RawMessage
	if s.AssetID != nil {
		e := h.db.QueryRowContext(r.Context(), `SELECT jsonb_build_object('id',id,'entity_key',entity_key,'name',name,'mime',mime,'size',size) FROM academy_media_assets WHERE id=$1 AND deleted_at IS NULL`, s.AssetID).Scan(&asset)
		if errors.Is(e, sql.ErrNoRows) {
			fail(w, 410, "Материал удалён. Начните новую загрузку")
			return
		}
		if e != nil {
			problem(w, e)
			return
		}
	}
	respond(w, 200, map[string]any{"id": s.ID, "status": s.Status, "error_code": s.ErrorCode, "part_size": s.PartSize, "size": s.Size, "verified_bytes": s.ChecksumOffset, "expires_at": s.Expires, "parts": parts, "asset": asset})
}

func (h *Handler) ProcessUploads(ctx context.Context) error {
	if h.storage.Multipart == nil {
		return nil
	}
	var id string
	e := h.db.QueryRowContext(ctx, `WITH picked AS(SELECT id FROM academy_upload_sessions WHERE status='finalizing' AND expires_at>now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY updated_at,created_at LIMIT 1 FOR UPDATE SKIP LOCKED) UPDATE academy_upload_sessions u SET lease_until=now()+INTERVAL '10 minutes' FROM picked p WHERE u.id=p.id RETURNING u.id`).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	s, e := h.getUpload(ctx, id)
	if e != nil {
		return e
	}
	if s.Backend != h.storage.Multipart.Identity() {
		return nil
	}
	e = h.finalizeUpload(ctx, s)
	if e != nil {
		code := "storage_or_database_unavailable"
		terminal := false
		if errors.Is(e, ErrInvalid) {
			code = "invalid_state"
			terminal = true
		}
		if errors.Is(e, objectstorage.ErrNotFound) {
			code = "object_missing"
			terminal = true
		}
		_, _ = h.db.ExecContext(ctx, `UPDATE academy_upload_sessions SET error_code=$2,lease_until=now()+INTERVAL '30 seconds',status=CASE WHEN $3 THEN 'failed' ELSE status END,expires_at=CASE WHEN $3 THEN now() ELSE expires_at END WHERE id=$1 AND status='finalizing'`, id, code, terminal)
	}
	return e
}
func validateParts(s uploadSession, p []objectstorage.Part) bool {
	count := int((s.Size + s.PartSize - 1) / s.PartSize)
	if len(p) != count {
		return false
	}
	for i, v := range p {
		expected := s.PartSize
		if i == count-1 {
			expected = s.Size - int64(i)*s.PartSize
		}
		if v.Number != i+1 || v.Size != expected || v.ETag == "" {
			return false
		}
	}
	return true
}
func (h *Handler) finalizeUpload(ctx context.Context, s uploadSession) error {
	remote := h.storage.Multipart
	key := objectKey("s3", s.Key)
	// A previous Complete may have succeeded despite a lost response. Head recovers it.
	info, e := remote.Stat(ctx, key)
	if errors.Is(e, objectstorage.ErrNotFound) {
		var parts []objectstorage.Part
		parts, e = remote.Parts(ctx, key, s.UploadID)
		if e != nil {
			return e
		}
		if !validateParts(s, parts) {
			_, e = h.db.ExecContext(ctx, `UPDATE academy_upload_sessions SET status='failed',expires_at=now(),error_code='parts_mismatch' WHERE id=$1`, s.ID)
			return e
		}
		if e = remote.Complete(ctx, key, s.UploadID, parts); e != nil {
			return e
		}
		info, e = remote.Stat(ctx, key)
	}
	if e != nil {
		return e
	}
	valid := info.Size == s.Size && info.ContentType == s.MIME
	if valid {
		body, _, e := remote.Open(ctx, key, "bytes=0-511")
		if e != nil {
			return e
		}
		prefix, e := io.ReadAll(io.LimitReader(body, 513))
		body.Close()
		if e != nil {
			return e
		}
		if s.MIME == "video/mp4" {
			valid = len(prefix) >= 24 && bytes.Equal(prefix[4:8], []byte("ftyp"))
		} else if s.Size <= maxAssetSize {
			body, _, e := remote.Open(ctx, key, "")
			if e != nil {
				return e
			}
			tmp, e := newValidationFile(body, s.Size)
			body.Close()
			if e != nil {
				return e
			}
			typ, err := streamFileType(s.Name, tmp, s.Size)
			removeValidationFile(tmp)
			valid = err == nil && typ == s.MIME
		} else {
			valid = false
		}
	}
	if !valid {
		_, e = h.db.ExecContext(ctx, `UPDATE academy_upload_sessions SET status='failed',expires_at=now(),error_code='invalid_content' WHERE id=$1`, s.ID)
		return e
	}
	checksum, done, e := h.verifyUploadChunk(ctx, s, info)
	if e != nil {
		return e
	}
	if !done {
		return nil
	}
	confirmed, e := remote.Stat(ctx, key)
	if e != nil {
		return e
	}
	if confirmed.Size != info.Size || confirmed.ETag != info.ETag {
		return objectstorage.ErrUnavailable
	}
	tx, e := h.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var status string
	var lockedProgram int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM academy_course_programs WHERE id=$1 FOR UPDATE`, s.ProgramID).Scan(&lockedProgram); e != nil {
		return e
	}
	e = tx.QueryRowContext(ctx, `SELECT status FROM academy_upload_sessions WHERE id=$1 FOR UPDATE`, s.ID).Scan(&status)
	if e != nil {
		return e
	}
	if status != "finalizing" {
		return nil
	}
	// Revalidate owner and entity after the network operations (revocation or deletion may race).
	var allowed bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM courses c JOIN users u ON u.id=c.owner_id JOIN course_access_settings a ON a.id=true WHERE c.id=$1 AND c.owner_id=$2 AND NOT u.is_blocked AND NOT u.is_system AND (a.public_access OR u.courses_access)) AND EXISTS(SELECT 1 FROM academy_lessons WHERE program_id=$3 AND key=$4 UNION ALL SELECT 1 FROM academy_modules WHERE program_id=$3 AND key=$4)`, s.CourseID, s.UserID, s.ProgramID, s.EntityKey).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		_, e = tx.ExecContext(ctx, `UPDATE academy_upload_sessions SET status='failed',expires_at=now(),error_code='permission_changed' WHERE id=$1`, s.ID)
		if e != nil {
			return e
		}
		return tx.Commit()
	}
	var asset int64
	e = tx.QueryRowContext(ctx, `INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by,storage_driver,checksum,checksum_kind) VALUES($1,$2,$3,$4,$5,$6,$7,'s3',$8,'sha256') RETURNING id`, s.ProgramID, s.EntityKey, s.Key, s.Name, s.MIME, s.Size, s.UserID, checksum).Scan(&asset)
	if e == nil {
		_, e = tx.ExecContext(ctx, `INSERT INTO academy_asset_links(asset_id,program_id,entity_key) VALUES($1,$2,$3)`, asset, s.ProgramID, s.EntityKey)
	}
	if e == nil {
		_, e = tx.ExecContext(ctx, `UPDATE academy_upload_sessions SET status='ready',asset_id=$2,checksum_state='',checksum_offset=size,updated_at=now(),lease_until=NULL WHERE id=$1`, s.ID, asset)
	}
	if e != nil {
		return e
	}
	return tx.Commit()
}

// Incremental SHA-256 uses bounded streaming RAM and durable hash state. A large
// upload survives worker restarts without rereading every preceding byte.
func (h *Handler) verifyUploadChunk(ctx context.Context, s uploadSession, info objectstorage.Info) (string, bool, error) {
	digest := newDigest()
	offset := s.ChecksumOffset
	if s.ObjectETag != info.ETag {
		offset = 0
		s.ChecksumState = ""
	}
	if offset < 0 || offset > s.Size {
		return "", false, ErrInvalid
	}
	if offset > 0 {
		state, e := hex.DecodeString(s.ChecksumState)
		if e != nil {
			return "", false, ErrInvalid
		}
		if e = digest.(encoding.BinaryUnmarshaler).UnmarshalBinary(state); e != nil {
			return "", false, ErrInvalid
		}
	}
	if offset < s.Size {
		end := min(s.Size, offset+(32<<20))
		body, meta, e := h.storage.Primary.Open(ctx, objectKey("s3", s.Key), "bytes="+fmtSize(offset)+"-"+fmtSize(end-1))
		if e != nil {
			return "", false, e
		}
		n, e := io.Copy(digest, io.LimitReader(body, end-offset+1))
		body.Close()
		if e != nil || n != end-offset || meta.Size != n || meta.ETag != "" && meta.ETag != info.ETag {
			return "", false, objectstorage.ErrUnavailable
		}
		offset = end
	}
	// Persist only full SHA blocks. The last partial block may contain original
	// file bytes; never put it in PostgreSQL. A restart may reread the last range.
	if offset == s.Size {
		return digestHex(digest), true, nil
	}
	state, e := digest.(encoding.BinaryMarshaler).MarshalBinary()
	if e != nil {
		return "", false, e
	}
	_, e = h.db.ExecContext(ctx, `UPDATE academy_upload_sessions SET checksum_offset=$2,checksum_state=$3,object_etag=$4,updated_at=now(),lease_until=CASE WHEN $2<size THEN NULL ELSE lease_until END WHERE id=$1 AND status='finalizing'`, s.ID, offset, hex.EncodeToString(state), info.ETag)
	if e != nil {
		return "", false, e
	}
	return digestHex(digest), offset == s.Size, nil
}
func (h *Handler) cleanupUploads(ctx context.Context) error {
	if h.storage.Multipart == nil {
		return nil
	}
	if _, e := h.db.ExecContext(ctx, `UPDATE academy_upload_sessions SET status='failed',expires_at=now(),error_code='init_timeout' WHERE status='initiating' AND updated_at<now()-INTERVAL '5 minutes'`); e != nil {
		return e
	}
	rows, e := h.db.QueryContext(ctx, `SELECT id FROM academy_upload_sessions WHERE status<>'ready' AND expires_at<=now() ORDER BY expires_at LIMIT 20`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		s, e := h.getUpload(ctx, id)
		if e != nil {
			return e
		}
		if s.Backend != h.storage.Multipart.Identity() {
			continue
		}
		if s.UploadID != "" {
			if e = h.storage.Multipart.Abort(ctx, objectKey("s3", s.Key), s.UploadID); e != nil {
				return e
			}
		}
		h.queueDelete(s.Key, "s3")
		_, e = h.db.ExecContext(ctx, `DELETE FROM academy_upload_sessions WHERE id=$1 AND status<>'ready'`, id)
		if e != nil {
			return e
		}
	}
	return nil
}
