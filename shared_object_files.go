package main

import (
	"FinTalent/internal/objectstorage"
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/090_shared_object_files.sql
var sharedObjectFilesSchema string

var fileStorageOptions = academyStorageOptions

func prepareSharedObjectFilesDatabase(ctx context.Context) error {
	_, e := db.ExecContext(ctx, sharedObjectFilesSchema)
	return e
}
func sharedFiles() objectstorage.Registry {
	return objectstorage.Registry{DB: db}
}
func sharedFileWriter() objectstorage.Registry {
	return objectstorage.Registry{DB: db, Store: fileStorageOptions().Primary}
}

func mirrorUserFile(r *http.Request, ref, path, name string, owner objectstorage.Owner, category string, actors ...int64) error {
	if fileStorageOptions().Driver != "s3" {
		return nil
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." {
		name = filepath.Base(path)
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return e
	}
	prefix := make([]byte, 512)
	n, _ := f.Read(prefix)
	typ := http.DetectContentType(prefix[:n])
	if strings.HasSuffix(strings.ToLower(path), ".svg") {
		typ = "image/svg+xml"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	actor := int64(0)
	if owner.Kind == "user" {
		actor = owner.ID
	}
	if len(actors) > 0 {
		actor = actors[0]
	}
	return sharedFileWriter().CopyVerified(ctx, objectstorage.RegisteredFile{Ref: ref, Owner: owner, Category: category, Name: name, MIME: typ, Size: st.Size(), CreatedBy: actor}, f)
}
func mirrorTestMaterial(r *http.Request, id, owner int64, name, typ string, data []byte) error {
	if fileStorageOptions().Driver != "s3" {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	return sharedFileWriter().CopyVerified(ctx, objectstorage.RegisteredFile{Ref: "test-material:" + strconv.FormatInt(id, 10), Owner: objectstorage.Owner{Kind: "user", ID: owner}, Category: "tests", Name: name, MIME: typ, Size: int64(len(data)), CreatedBy: owner}, bytes.NewReader(data))
}

// Called only after the module has performed its original authorization.
// Public /static URLs preserve their existing public semantics; private files
// never receive a publicly registered route.
func serveRegisteredFile(w http.ResponseWriter, r *http.Request, ref string) bool {
	if db == nil {
		return false
	}
	f, e := sharedFiles().Lookup(r.Context(), ref)
	if e == sql.ErrNoRows {
		return false
	}
	if e != nil {
		http.Error(w, "File metadata unavailable", 503)
		return true
	}
	if f.Status != "ready" {
		return false
	}
	options := fileStorageOptions()
	if options.Primary == nil {
		http.Error(w, "File storage unavailable", 503)
		return true
	}
	disposition := "attachment"
	if !strings.HasPrefix(ref, "certificate:") && (strings.HasPrefix(f.MIME, "image/") || f.MIME == "application/pdf") {
		disposition = "inline"
	}
	// Preserve same-origin API responses. Invitation headers must never be
	// forwarded by a browser through a cross-origin redirect to S3.
	var body io.ReadCloser
	if r.Method == "HEAD" {
		_, e = options.Primary.Stat(r.Context(), f.Key)
	} else {
		var info objectstorage.Info
		body, info, e = options.Primary.Open(r.Context(), f.Key, "")
		if e == nil && info.Size != f.Size {
			e = objectstorage.ErrUnavailable
		}
	}
	if body != nil {
		defer body.Close()
	}
	if e != nil {
		http.Error(w, "File storage unavailable", 503)
		return true
	}
	w.Header().Set("Content-Type", f.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	// Public legacy images previously exposed only their random URL name.
	// Preserve that privacy boundary; original upload names remain in metadata.
	if !strings.HasPrefix(ref, "/static/uploads/") {
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-FinTalent-Storage", "s3")
	if f.MIME == "application/pdf" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}
	if f.MIME == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if body != nil {
		_, _ = io.CopyN(w, body, f.Size)
	}
	return true
}
func sharedPublicFiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/uploads/") && !objectstorage.PublicUploadRef(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if (r.Method == "GET" || r.Method == "HEAD") && strings.HasPrefix(r.URL.Path, "/static/uploads/") && serveRegisteredFile(w, r, r.URL.Path) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveResumeCertificate(w http.ResponseWriter, r *http.Request) {
	u, e := userFromRequest(r)
	if e != nil {
		http.Error(w, "Unauthorized", 401)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	var key, name, typ string
	if db.QueryRowContext(r.Context(), `SELECT storage_name,original_name,content_type FROM resume_education_certificates WHERE id=$1 AND user_id=$2`, id, u.ID).Scan(&key, &name, &typ) != nil {
		http.NotFound(w, r)
		return
	}
	if serveRegisteredFile(w, r, "certificate:"+key) {
		return
	}
	if filepath.Base(key) != key {
		http.NotFound(w, r)
		return
	}
	f, e := os.Open(filepath.Join("uploads", "resume-certificates", key))
	if e != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func fileUser(id int64) objectstorage.Owner { return objectstorage.Owner{Kind: "user", ID: id} }
func fileCompany(id int64) objectstorage.Owner {
	return objectstorage.Owner{Kind: "accounting_company", ID: id}
}
func filePlatform() objectstorage.Owner { return objectstorage.Owner{Kind: "platform"} }
