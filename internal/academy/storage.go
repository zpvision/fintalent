package academy

import (
	"FinTalent/internal/objectstorage"
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type StorageOptions struct {
	Primary   objectstorage.Store
	Multipart objectstorage.Multipart
	Driver    string
	TTL       time.Duration
	VideoMax  int64
}

func (h *Handler) SetStorage(o StorageOptions) { h.storage = o }
func (h *Handler) store(driver string) (objectstorage.Store, error) {
	if driver == "local" {
		return objectstorage.Local{Root: h.root}, nil
	}
	if driver == "s3" && h.storage.Driver == "s3" && h.storage.Primary != nil {
		return h.storage.Primary, nil
	}
	return nil, objectstorage.ErrUnavailable
}
func objectKey(driver, key string) string {
	if driver == "s3" {
		if strings.HasPrefix(key, "users/") {
			return key
		}
		if strings.HasPrefix(key, "reports/users/") {
			return strings.TrimPrefix(key, "reports/")
		}
		if strings.HasPrefix(key, "reports/") {
			return "academy/" + key
		}
		return "academy/media/" + key
	}
	return key
}
func (h *Handler) uploadDriver() string {
	if h.storage.Driver == "s3" {
		return "s3"
	}
	return "local"
}
func (h *Handler) reserveObject(ctx context.Context, key, driver string) error {
	_, e := h.db.ExecContext(ctx, `INSERT INTO academy_storage_gc(storage_key,storage_driver) VALUES($1,$2) ON CONFLICT DO NOTHING`, key, driver)
	return e
}
func (h *Handler) queueDelete(key, driver string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, e := h.db.ExecContext(ctx, `INSERT INTO academy_storage_gc(storage_key,storage_driver,next_at) VALUES($1,$2,now()) ON CONFLICT(storage_key,storage_driver) DO UPDATE SET next_at=now()`, key, driver)
	return e
}
func (h *Handler) storageFailure(w http.ResponseWriter) {
	fail(w, 503, "Хранилище временно недоступно. Повторите позже")
}

// All callers have already checked course/enrollment/submission/admin authorization.
func (h *Handler) sendStoredAsset(w http.ResponseWriter, r *http.Request, key, name, typ string) {
	var driver string
	if e := h.db.QueryRowContext(r.Context(), `SELECT storage_driver FROM academy_media_assets WHERE storage_key=$1 AND deleted_at IS NULL`, key).Scan(&driver); e != nil {
		problem(w, e)
		return
	}
	store, e := h.store(driver)
	if e != nil {
		h.storageFailure(w)
		return
	}
	if driver == "local" {
		if r.URL.Query().Get("resolve") == "1" {
			u := *r.URL
			q := u.Query()
			q.Del("resolve")
			u.RawQuery = q.Encode()
			respond(w, 200, map[string]any{"url": u.RequestURI(), "expires_at": nil})
			return
		}
		f, e := os.Open(filepath.Join(h.root, key))
		if e != nil {
			fail(w, 404, "Файл временно недоступен")
			return
		}
		defer f.Close()
		st, e := f.Stat()
		if e != nil {
			h.storageFailure(w)
			return
		}
		assetHeaders(w, r, name, typ)
		http.ServeContent(w, r, name, st.ModTime(), f)
		return
	}
	disposition := "attachment"
	if (strings.HasPrefix(typ, "image/") || typ == "video/mp4") && r.URL.Query().Get("download") != "1" {
		disposition = "inline"
	}
	disp := mime.FormatMediaType(disposition, map[string]string{"filename": name})
	if r.Method == "HEAD" {
		info, e := store.Stat(r.Context(), objectKey(driver, key))
		if e != nil {
			h.storageFailure(w)
			return
		}
		assetHeaders(w, r, name, typ)
		w.Header().Set("Content-Length", fmtSize(info.Size))
		w.WriteHeader(200)
		return
	}
	signed, e := h.storage.Multipart.SignGet(r.Context(), objectKey(driver, key), typ, disp, h.storage.TTL)
	if e != nil {
		h.storageFailure(w)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.URL.Query().Get("resolve") == "1" {
		respond(w, 200, signed)
		return
	}
	http.Redirect(w, r, signed.URL, http.StatusTemporaryRedirect)
}
func assetHeaders(w http.ResponseWriter, r *http.Request, name, typ string) {
	disposition := "attachment"
	if (strings.HasPrefix(typ, "image/") || typ == "video/mp4") && r.URL.Query().Get("download") != "1" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
}

func (h *Handler) CleanupStorage(ctx context.Context) error {
	// Claims survive restarts; no transaction is held across object storage calls.
	rows, e := h.db.QueryContext(ctx, `WITH picked AS(SELECT storage_key,storage_driver FROM academy_storage_gc WHERE next_at<=now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY next_at LIMIT 20 FOR UPDATE SKIP LOCKED) UPDATE academy_storage_gc g SET lease_until=now()+INTERVAL '5 minutes' FROM picked p WHERE g.storage_key=p.storage_key AND g.storage_driver=p.storage_driver RETURNING g.storage_key,g.storage_driver`)
	if e != nil {
		return e
	}
	type item struct{ k, d string }
	items := []item{}
	for rows.Next() {
		var x item
		if e = rows.Scan(&x.k, &x.d); e != nil {
			rows.Close()
			return e
		}
		items = append(items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, x := range items {
		var live bool
		e = h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM academy_media_assets WHERE storage_key=$1 AND (storage_driver=$2 AND deleted_at IS NULL OR $2='local' AND storage_driver='s3')) OR EXISTS(SELECT 1 FROM academy_report_jobs WHERE $1='reports/'||storage_key AND storage_driver=$2 AND deleted_at IS NULL AND expires_at>now() AND status IN('ready','running'))`, x.k, x.d).Scan(&live)
		if e != nil {
			return e
		}
		if live {
			_, e = h.db.ExecContext(ctx, `DELETE FROM academy_storage_gc WHERE storage_key=$1 AND storage_driver=$2`, x.k, x.d)
		} else {
			var s objectstorage.Store
			s, e = h.store(x.d)
			if e == nil {
				e = s.Delete(ctx, objectKey(x.d, x.k))
			}
			if e == nil {
				_, e = h.db.ExecContext(ctx, `DELETE FROM academy_storage_gc WHERE storage_key=$1 AND storage_driver=$2`, x.k, x.d)
			} else {
				_, e = h.db.ExecContext(ctx, `UPDATE academy_storage_gc SET attempts=attempts+1,next_at=now()+LEAST(3600,30*power(2,LEAST(attempts,6))) * INTERVAL '1 second',lease_until=NULL WHERE storage_key=$1 AND storage_driver=$2`, x.k, x.d)
			}
		}
		if e != nil {
			return e
		}
	}
	return h.cleanupUploads(ctx)
}

// MigrateLocalAsset copies and reads back SHA-256 before conditionally changing the row.
// The source remains intact. Concurrent/repeated invocations are safe because keys are immutable.
func (h *Handler) MigrateLocalAsset(ctx context.Context, id int64) error {
	if h.storage.Driver != "s3" {
		return objectstorage.ErrInvalid
	}
	var key, driver string
	var size int64
	var typ, previousChecksum, checksumKind string
	e := h.db.QueryRowContext(ctx, `SELECT storage_key,storage_driver,size,mime,checksum,checksum_kind FROM academy_media_assets WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&key, &driver, &size, &typ, &previousChecksum, &checksumKind)
	if e != nil {
		return e
	}
	if driver == "s3" {
		return nil
	}
	if !regexpStorage(key) {
		return objectstorage.ErrInvalid
	}
	local := objectstorage.Local{Root: h.root}
	r, info, e := local.Open(ctx, key, "")
	if e != nil {
		return e
	}
	defer r.Close()
	if info.Size != size {
		return errors.New("local size mismatch")
	}
	digest := newDigest()
	if _, e = io.Copy(digest, r); e != nil {
		return e
	}
	if checksumKind == "sha256" && previousChecksum != "" && digestHex(digest) != previousChecksum {
		return errors.New("local checksum mismatch")
	}
	if e = h.reserveObject(ctx, key, "s3"); e != nil {
		return e
	}
	seeker, ok := r.(io.Seeker)
	if !ok {
		return objectstorage.ErrInvalid
	}
	if _, e = seeker.Seek(0, 0); e != nil {
		return e
	}
	if e = h.storage.Primary.Put(ctx, objectKey("s3", key), r, size, typ); e != nil {
		return e
	}
	expected := digestHex(digest)
	body, remote, e := h.storage.Primary.Open(ctx, objectKey("s3", key), "")
	if e != nil {
		return e
	}
	defer body.Close()
	check := newDigest()
	n, e := io.Copy(check, body)
	if e != nil || n != size || remote.Size != size || digestHex(check) != expected {
		return errors.New("copy verification failed")
	}
	_, e = h.db.ExecContext(ctx, `UPDATE academy_media_assets SET storage_driver='s3',checksum=$2,checksum_kind='sha256',storage_updated_at=now() WHERE id=$1 AND storage_driver='local' AND deleted_at IS NULL`, id, expected)
	return e
}

func newDigest() hash.Hash         { return sha256.New() }
func digestHex(h hash.Hash) string { return hex.EncodeToString(h.Sum(nil)) }
func fmtSize(n int64) string       { return strconv.FormatInt(n, 10) }
func newValidationFile(r io.Reader, size int64) (*os.File, error) {
	f, e := os.CreateTemp("", "academy-validate-*")
	if e != nil {
		return nil, e
	}
	n, e := io.Copy(f, io.LimitReader(r, size+1))
	if e != nil || n != size {
		removeValidationFile(f)
		return nil, objectstorage.ErrInvalid
	}
	return f, nil
}
func removeValidationFile(f *os.File) { name := f.Name(); f.Close(); _ = os.Remove(name) }
func streamFileType(name string, f *os.File, size int64) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	prefix := make([]byte, 512)
	n, e := f.ReadAt(prefix, 0)
	if e != nil && e != io.EOF {
		return "", e
	}
	if ext == ".txt" || ext == ".csv" {
		if _, e = f.Seek(0, 0); e != nil {
			return "", e
		}
		reader := bufio.NewReader(f)
		for {
			r, width, err := reader.ReadRune()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			if r == 0 || r == utf8.RuneError && width == 1 {
				return "", inputError{"Текстовый файл должен быть корректным UTF-8"}
			}
		}
		for n > 0 && !utf8.Valid(prefix[:n]) {
			n--
		}
	}
	if !oneOf(ext, ".zip", ".docx", ".xlsx", ".pptx") {
		return fileType(name, prefix[:n])
	}
	zr, e := zip.NewReader(f, size)
	if e != nil || len(zr.File) > 10000 {
		return "", inputError{"Повреждённый или слишком большой архив"}
	}
	if ext == ".zip" {
		return "application/zip", nil
	}
	expected := map[string]string{".docx": "word/", ".xlsx": "xl/", ".pptx": "ppt/"}[ext]
	content, part := false, false
	for _, v := range zr.File {
		if strings.HasSuffix(strings.ToLower(v.Name), "vbaproject.bin") {
			return "", inputError{"Документы с макросами не поддерживаются"}
		}
		content = content || v.Name == "[Content_Types].xml"
		part = part || strings.HasPrefix(v.Name, expected)
	}
	if !content || !part {
		return "", inputError{"Повреждённый документ"}
	}
	return map[string]string{".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation"}[ext], nil
}
func (h *Handler) writeStoredReport(ctx context.Context, key, format string, rows [][]string) (int64, error) {
	if h.uploadDriver() == "local" {
		return writeReportFile(h.root, key, format, rows)
	}
	if !reportKeyPattern.MatchString(key) {
		return 0, ErrInvalid
	}
	tmp, e := os.CreateTemp("", "academy-report-*")
	if e != nil {
		return 0, e
	}
	defer removeValidationFile(tmp)
	if format == "xlsx" {
		e = writeAnalyticsXLSX(tmp, rows)
	} else {
		e = writeAnalyticsCSV(tmp, rows)
	}
	if e != nil {
		return 0, e
	}
	st, e := tmp.Stat()
	if e != nil {
		return 0, e
	}
	if _, e = tmp.Seek(0, 0); e != nil {
		return 0, e
	}
	if e = h.reserveObject(ctx, "reports/"+key, "s3"); e != nil {
		return 0, e
	}
	typ := "text/csv; charset=utf-8"
	if format == "xlsx" {
		typ = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	e = h.storage.Primary.Put(ctx, objectKey("s3", "reports/"+key), tmp, st.Size(), typ)
	return st.Size(), e
}
