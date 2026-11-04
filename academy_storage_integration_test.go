package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/objectstorage"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type storageFixtureObject struct {
	data []byte
	typ  string
}
type storageFixture struct {
	objects                        map[string]storageFixtureObject
	data                           []byte
	typ                            string
	lostComplete, corrupt, failPut bool
	completed, aborted             int
}

func (s *storageFixture) Identity() string { return "fixture-bucket" }
func (s *storageFixture) Put(_ context.Context, k string, r io.Reader, n int64, typ string) error {
	if s.failPut {
		return objectstorage.ErrUnavailable
	}
	b, _ := io.ReadAll(r)
	if s.corrupt {
		b = append([]byte{}, b...)
		b[0] ^= 1
	}
	s.objects[k] = storageFixtureObject{b, typ}
	return nil
}
func (s *storageFixture) Open(_ context.Context, k, ran string) (io.ReadCloser, objectstorage.Info, error) {
	v, ok := s.objects[k]
	if !ok {
		return nil, objectstorage.Info{}, objectstorage.ErrNotFound
	}
	b := v.data
	if ran != "" {
		var start, end int64
		if _, err := fmt.Sscanf(ran, "bytes=%d-%d", &start, &end); err != nil {
			return nil, objectstorage.Info{}, objectstorage.ErrInvalid
		}
		if end >= int64(len(b)) {
			end = int64(len(b)) - 1
		}
		b = b[start : end+1]
	}
	return io.NopCloser(bytes.NewReader(b)), objectstorage.Info{Size: int64(len(b)), ContentType: v.typ, ETag: "fixture-etag"}, nil
}
func (s *storageFixture) Stat(ctx context.Context, k string) (objectstorage.Info, error) {
	r, i, e := s.Open(ctx, k, "")
	if r != nil {
		r.Close()
	}
	return i, e
}
func (s *storageFixture) Delete(_ context.Context, k string) error { delete(s.objects, k); return nil }
func (s *storageFixture) Begin(_ context.Context, k, typ string) (string, error) {
	s.typ = typ
	return "fixture-upload", nil
}
func (s *storageFixture) SignPart(_ context.Context, k, id string, n int, size int64, ttl time.Duration) (objectstorage.Signed, error) {
	return objectstorage.Signed{URL: "https://storage.invalid/part", ExpiresAt: time.Now().Add(ttl)}, nil
}
func (s *storageFixture) Parts(_ context.Context, k, id string) ([]objectstorage.Part, error) {
	parts := []objectstorage.Part{}
	for start := 0; start < len(s.data); start += 32 << 20 {
		parts = append(parts, objectstorage.Part{Number: len(parts) + 1, Size: int64(min(32<<20, len(s.data)-start)), ETag: "part"})
	}
	return parts, nil
}
func (s *storageFixture) Complete(_ context.Context, k, id string, p []objectstorage.Part) error {
	s.completed++
	s.objects[k] = storageFixtureObject{append([]byte{}, s.data...), s.typ}
	if s.lostComplete {
		s.lostComplete = false
		return objectstorage.ErrUnavailable
	}
	return nil
}
func (s *storageFixture) Abort(context.Context, string, string) error { s.aborted++; return nil }
func (s *storageFixture) SignGet(_ context.Context, k, typ, disp string, ttl time.Duration) (objectstorage.Signed, error) {
	return objectstorage.Signed{URL: "https://storage.invalid/" + k + "?temporary=true", ExpiresAt: time.Now().Add(ttl)}, nil
}

func TestAcademyObjectStorageIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	ctx := context.Background()
	remote := &storageFixture{objects: map[string]storageFixtureObject{}}
	f.Handler.SetStorage(academy.StorageOptions{Primary: remote, Multipart: remote, Driver: "s3", TTL: time.Minute, VideoMax: 1 << 30})
	if _, e := f.DB.Exec("UPDATE users SET courses_access=true WHERE id=$1", f.Owner); e != nil {
		t.Fatal(e)
	}
	var module int64
	entity := academy.Key()
	if e := f.DB.QueryRow(`INSERT INTO academy_modules(program_id,key,title) VALUES($1,$2,'Module') RETURNING id`, f.Program, academy.Key()).Scan(&module); e != nil {
		t.Fatal(e)
	}
	if _, e := f.DB.Exec(`INSERT INTO academy_lessons(program_id,module_id,key,title) VALUES($1,$2,$3,'Lesson')`, f.Program, module, entity); e != nil {
		t.Fatal(e)
	}
	request := func(method, path string, actor int64, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-Test-User", strconv.FormatInt(actor, 10))
		w := httptest.NewRecorder()
		f.Mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s = %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	base := fmt.Sprintf("/api/academy/courses/%d/assets", f.Course)
	remote.data = append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 32)...)
	create := func(name string, size int) string {
		t.Helper()
		id := academy.Key()
		request("POST", base+"/uploads", f.Owner, map[string]any{"request_key": id, "name": name, "size": size, "entity_key": entity}, 200)
		return id
	}
	id := create("lesson.mp4", len(remote.data))
	request("GET", base+"/uploads/"+id, f.Student, nil, 404)
	request("POST", base+"/uploads/"+id+"/parts/2", f.Owner, nil, 400)
	request("POST", base+"/uploads/"+id+"/parts/1", f.Owner, nil, 200)
	remote.lostComplete = true
	request("POST", base+"/uploads/"+id+"/complete", f.Owner, nil, 202)
	if e := f.Handler.ProcessUploads(ctx); e == nil {
		t.Fatal("expected simulated lost complete response")
	}
	if _, e := f.DB.Exec("UPDATE academy_upload_sessions SET lease_until=now()-INTERVAL '1 minute' WHERE id=$1", id); e != nil {
		t.Fatal(e)
	}
	if e := f.Handler.ProcessUploads(ctx); e != nil {
		t.Fatal(e)
	}
	w := request("GET", base+"/uploads/"+id, f.Owner, nil, 200)
	var state struct {
		Status string `json:"status"`
		Asset  struct {
			ID int64 `json:"id"`
		} `json:"asset"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &state); e != nil || state.Status != "ready" || state.Asset.ID == 0 || remote.completed != 1 {
		t.Fatal("complete recovery failed", w.Body.String(), e)
	}
	request("POST", base+"/uploads/"+id+"/complete", f.Owner, nil, 202)
	if e := f.Handler.ProcessUploads(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := f.DB.QueryRow("SELECT count(*) FROM academy_media_assets WHERE id=$1", state.Asset.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate material")
	}
	w = request("GET", fmt.Sprintf("%s/%d", base, state.Asset.ID), f.Owner, nil, 307)
	if !strings.HasPrefix(w.Header().Get("Location"), "https://storage.invalid/") {
		t.Fatal("no signed redirect")
	}
	// A copied upload URL must not allow a different account to obtain content.
	request("GET", fmt.Sprintf("%s/%d", base, state.Asset.ID), f.Student, nil, 404)
	invalid := create("bad.mp4", len(remote.data)+1)
	request("POST", base+"/uploads/"+invalid+"/complete", f.Owner, nil, 202)
	if e := f.Handler.ProcessUploads(ctx); e != nil {
		t.Fatal(e)
	}
	w = request("GET", base+"/uploads/"+invalid, f.Owner, nil, 200)
	if !strings.Contains(w.Body.String(), `"status":"failed"`) {
		t.Fatal("wrong size accepted")
	}
	cancelled := create("cancel.mp4", len(remote.data))
	request("DELETE", base+"/uploads/"+cancelled, f.Owner, nil, 200)
	if e := f.Handler.CleanupStorage(ctx); e != nil {
		t.Fatal(e)
	}
	if remote.aborted == 0 {
		t.Fatal("abandoned multipart was not aborted")
	}
	// Migrations retain the original bytes even after garbage collection.
	key := strings.Repeat("a", 48)
	source := []byte("%PDF-1.7\nprivate local material")
	if e := os.WriteFile(filepath.Join(f.Root, key), source, 0600); e != nil {
		t.Fatal(e)
	}
	var local int64
	if e := f.DB.QueryRow(`INSERT INTO academy_media_assets(program_id,entity_key,storage_key,name,mime,size,created_by) VALUES($1,$2,$3,'old.pdf','application/pdf',$4,$5) RETURNING id`, f.Program, entity, key, len(source), f.Owner).Scan(&local); e != nil {
		t.Fatal(e)
	}
	remote.failPut = true
	if e := f.Handler.MigrateLocalAsset(ctx, local); e == nil {
		t.Fatal("S3 outage did not fail migration")
	}
	remote.failPut = false
	remote.corrupt = true
	if e := f.Handler.MigrateLocalAsset(ctx, local); e == nil {
		t.Fatal("corrupt copy switched metadata")
	}
	remote.corrupt = false
	if e := f.Handler.MigrateLocalAsset(ctx, local); e != nil {
		t.Fatal(e)
	}
	if e := f.Handler.MigrateLocalAsset(ctx, local); e != nil {
		t.Fatal("migration not idempotent", e)
	}
	if _, e := f.DB.Exec(`INSERT INTO academy_storage_gc(storage_key,storage_driver,next_at) VALUES($1,'local',now())`, key); e != nil {
		t.Fatal(e)
	}
	if e := f.Handler.CleanupStorage(ctx); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(f.Root, key)); e != nil || !bytes.Equal(b, source) {
		t.Fatal("migration source removed")
	}
	var driver, kind, checksum string
	if e := f.DB.QueryRow("SELECT storage_driver,checksum_kind,checksum FROM academy_media_assets WHERE id=$1", local).Scan(&driver, &kind, &checksum); e != nil || driver != "s3" || kind != "sha256" || len(checksum) != 64 {
		t.Fatal("verified metadata missing", e)
	}
	// A restarted worker continues a hash checkpoint rather than publishing an
	// unverified large object. The first range is a full SHA block boundary.
	remote.data = append(append([]byte{}, remote.data...), make([]byte, (32<<20)+127)...)
	large := create("large.mp4", len(remote.data))
	request("POST", base+"/uploads/"+large+"/complete", f.Owner, nil, 202)
	if e := f.Handler.ProcessUploads(ctx); e != nil {
		t.Fatal(e)
	}
	var offset int64
	if e := f.DB.QueryRow("SELECT checksum_offset FROM academy_upload_sessions WHERE id=$1", large).Scan(&offset); e != nil || offset != (32<<20) {
		t.Fatal("hash checkpoint missing", offset, e)
	}
	restarted := academy.New(f.DB, nil, nil, f.Root)
	options := academy.StorageOptions{Primary: remote, Multipart: remote, Driver: "s3", TTL: time.Minute, VideoMax: 1 << 30}
	restarted.SetStorage(options)
	if e := restarted.ProcessUploads(ctx); e != nil {
		t.Fatal(e)
	}
	var actual string
	if e := f.DB.QueryRow(`SELECT a.checksum FROM academy_media_assets a JOIN academy_upload_sessions u ON u.asset_id=a.id WHERE u.id=$1 AND u.status='ready'`, large).Scan(&actual); e != nil {
		t.Fatal(e)
	}
	expected := sha256.Sum256(remote.data)
	if actual != hex.EncodeToString(expected[:]) {
		t.Fatal("resumed SHA-256 incorrect")
	}
	// Immutable versions protect only the objects they reference; an unrelated
	// snapshot must not make every future unused upload undeletable.
	versionContent, _ := json.Marshal(academy.Program{Modules: []academy.Module{{Key: academy.Key(), Lessons: []academy.Lesson{{Key: academy.Key(), Blocks: []academy.Block{{Key: academy.Key(), Type: "file", Data: academy.BlockData{AssetID: state.Asset.ID}}}}}}}})
	if _, e := f.DB.Exec(`INSERT INTO academy_program_versions(program_id,number,source_revision,content,created_by) VALUES($1,2,2,$2,$3)`, f.Program, versionContent, f.Owner); e != nil {
		t.Fatal(e)
	}
	request("DELETE", fmt.Sprintf("%s/%d", base, state.Asset.ID), f.Owner, nil, 409)
	var largeAsset int64
	var largeKey string
	if e := f.DB.QueryRow(`SELECT asset_id,storage_key FROM academy_upload_sessions WHERE id=$1`, large).Scan(&largeAsset, &largeKey); e != nil {
		t.Fatal(e)
	}
	var storageOwner int64
	if !strings.HasPrefix(largeKey, fmt.Sprintf("users/%d/academy/", f.Owner)) {
		t.Fatal("new object key lacks permanent owner")
	}
	if err := f.DB.QueryRow("SELECT storage_owner_id FROM academy_media_assets WHERE id=$1", largeAsset).Scan(&storageOwner); err != nil || storageOwner != f.Owner {
		t.Fatal("storage owner missing", err)
	}
	if _, err := f.DB.Exec("UPDATE academy_media_assets SET storage_owner_id=$2 WHERE id=$1", largeAsset, f.Student); err == nil {
		t.Fatal("storage owner is mutable")
	}
	if _, err := f.DB.Exec("UPDATE courses SET owner_id=$2 WHERE id=$1", f.Course, f.Student); err != nil {
		t.Fatal(err)
	}
	var preserved string
	if err := f.DB.QueryRow("SELECT storage_key FROM academy_media_assets WHERE id=$1 AND storage_owner_id=$2", largeAsset, f.Owner).Scan(&preserved); err != nil || preserved != largeKey {
		t.Fatal("course transfer changed storage attribution")
	}
	if _, err := f.DB.Exec("UPDATE courses SET owner_id=$2 WHERE id=$1", f.Course, f.Owner); err != nil {
		t.Fatal(err)
	}
	request("DELETE", fmt.Sprintf("%s/%d", base, largeAsset), f.Owner, nil, 200)
	if e := restarted.CleanupStorage(ctx); e != nil {
		t.Fatal(e)
	}
	if _, ok := remote.objects[largeKey]; ok {
		t.Fatal("unreferenced deleted material retained")
	}
	// Both report formats are stored remotely, authorize before redirect, and
	// expire through the same retryable storage garbage collector.
	for _, format := range []string{"csv", "xlsx"} {
		reports := fmt.Sprintf("/api/academy/courses/%d/analytics/reports", f.Course)
		w = request("POST", reports, f.Owner, map[string]any{"kind": "summary", "format": format, "filters": map[string]any{"period": "all", "timezone": "UTC"}, "request_key": academy.Key()}, 202)
		var job struct {
			ID int64 `json:"id"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &job); e != nil {
			t.Fatal(e)
		}
		if e := academy.ProcessAnalyticsReports(ctx, f.DB, f.Root, options); e != nil {
			t.Fatal(e)
		}
		var status, key, reportDriver string
		if e := f.DB.QueryRow("SELECT status,storage_key,storage_driver FROM academy_report_jobs WHERE id=$1", job.ID).Scan(&status, &key, &reportDriver); e != nil || status != "ready" || reportDriver != "s3" {
			t.Fatal("remote report failed", status, e)
		}
		if _, ok := remote.objects[key]; !ok {
			t.Fatal("report object missing")
		}
		request("GET", fmt.Sprintf("%s/%d/file", reports, job.ID), f.Owner, nil, 307)
		request("GET", fmt.Sprintf("%s/%d/file", reports, job.ID), f.Student, nil, 404)
		if _, e := f.DB.Exec("UPDATE academy_report_jobs SET expires_at=now()-INTERVAL '1 minute' WHERE id=$1", job.ID); e != nil {
			t.Fatal(e)
		}
		if e := academy.ProcessAnalyticsReports(ctx, f.DB, f.Root, options); e != nil {
			t.Fatal(e)
		}
		if e := restarted.CleanupStorage(ctx); e != nil {
			t.Fatal(e)
		}
		if _, ok := remote.objects[key]; ok {
			t.Fatal("expired report not deleted")
		}
	}
}
