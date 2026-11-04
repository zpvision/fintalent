package filemigration

import (
	"FinTalent/internal/objectstorage"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrApplicationAccess = errors.New("application authentication or access denied")
var ErrApplicationResolver = errors.New("application did not use S3 resolver")

// VerifyApplication checks the stable application URL, rather than only S3.
// It refuses redirects so a private session cookie cannot be sent to a provider.
func ValidateApplicationBase(base string) error {
	u, e := url.Parse(base)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(base, "#") || u.Path != "" && u.Path != "/" {
		return objectstorage.ErrInvalid
	}
	if u.Scheme != "https" && (u.Scheme != "http" || u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		return objectstorage.ErrInvalid
	}
	return nil
}

func VerifyApplication(ctx context.Context, registry objectstorage.Registry, base, cookie string, s Source) error {
	if e := ValidateApplicationBase(base); e != nil {
		return e
	}
	path := s.File.Ref
	if s.Type == "test_material" {
		path = "/api/test-materials/" + strconv.FormatInt(s.ID, 10)
	} else if strings.HasPrefix(path, "certificate:") {
		path = "/api/v1/resumes/education-certificate?id=" + strconv.FormatInt(s.ID, 10)
	} else if !strings.HasPrefix(path, "/static/uploads/") {
		return objectstorage.ErrInvalid
	}
	f, e := registry.Lookup(ctx, s.File.Ref)
	if e != nil {
		return e
	}
	if f.Status != "ready" {
		return objectstorage.ErrInvalid
	}
	req, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(base, "/")+path, nil)
	if e != nil {
		return objectstorage.ErrInvalid
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Cache-Control", "no-cache")
	client := http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(req)
	if e != nil {
		return objectstorage.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode == 404 {
		return ErrApplicationAccess
	}
	if response.StatusCode != 200 {
		return objectstorage.ErrUnavailable
	}
	if response.Header.Get("X-FinTalent-Storage") != "s3" {
		return ErrApplicationResolver
	}
	digest := sha256.New()
	n, e := io.Copy(digest, io.LimitReader(response.Body, f.Size+1))
	if e != nil || n != f.Size || hex.EncodeToString(digest.Sum(nil)) != f.SHA256 {
		return objectstorage.ErrUnavailable
	}
	return nil
}

// Rollback enables legacy reads only after verifying the retained original.
// It never removes a remote object, BYTEA value, or local file.
func Rollback(ctx context.Context, registry objectstorage.Registry, root string, s Source) error {
	saved, e := registry.Lookup(ctx, s.File.Ref)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if saved.Status != "ready" {
		return nil
	}
	if saved.Owner != s.File.Owner {
		return objectstorage.ErrInvalid
	}
	digest := sha256.New()
	var size int64
	if s.Type == "test_material" {
		if s.File.Ref != "test-material:"+strconv.FormatInt(s.ID, 10) {
			return objectstorage.ErrInvalid
		}
		var data []byte
		if e = registry.DB.QueryRowContext(ctx, `SELECT content FROM test_question_materials WHERE id=$1`, s.ID).Scan(&data); e != nil {
			return e
		}
		size = int64(len(data))
		_, _ = digest.Write(data)
	} else if s.Type == "file" {
		if e = validateFileSource(ctx, registry.DB, s); e != nil {
			return e
		}
		p, e := ResolveSource(root, s.Path)
		if e != nil {
			return e
		}
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		size, e = io.Copy(digest, f)
		f.Close()
		if e != nil {
			return e
		}
	} else {
		return objectstorage.ErrInvalid
	}
	if size != saved.Size || hex.EncodeToString(digest.Sum(nil)) != saved.SHA256 {
		return objectstorage.ErrInvalid
	}
	_, e = registry.DB.ExecContext(ctx, `UPDATE object_files SET status='pending' WHERE legacy_ref=$1 AND object_key=$2 AND status='ready'`, s.File.Ref, saved.Key)
	return e
}
