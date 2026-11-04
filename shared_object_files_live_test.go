package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/filemigration"
	"FinTalent/internal/objectstorage"
	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test-only namespace wrapper. Production object keys remain users/...; the
// physical live-test objects can only be written below integration-tests/run/.
type isolatedLiveStore struct {
	base   objectstorage.Store
	prefix string
	mu     sync.Mutex
	keys   map[string]bool
}

func (s *isolatedLiveStore) Put(c context.Context, k string, r io.Reader, n int64, m string) error {
	if !objectstorage.ValidKey(k) {
		return objectstorage.ErrInvalid
	}
	s.mu.Lock()
	s.keys[k] = true
	s.mu.Unlock()
	return s.base.Put(c, s.prefix+k, r, n, m)
}
func (s *isolatedLiveStore) Open(c context.Context, k, b string) (io.ReadCloser, objectstorage.Info, error) {
	return s.base.Open(c, s.prefix+k, b)
}
func (s *isolatedLiveStore) Stat(c context.Context, k string) (objectstorage.Info, error) {
	return s.base.Stat(c, s.prefix+k)
}
func (s *isolatedLiveStore) Delete(c context.Context, k string) error {
	return s.base.Delete(c, s.prefix+k)
}

func TestSharedFilesLiveS3IsolatedPostgres(t *testing.T) {
	if os.Getenv("RUN_S3_LIVE_TESTS") != "1" {
		t.Skip("explicit live S3 opt-in required")
	}
	f := newAcademyFocusedFixture(t)
	cfg, e := objectstorage.ConfigFrom(os.Getenv, false)
	if e != nil || cfg.Driver != "s3" {
		t.Fatal("valid S3 configuration required")
	}
	remote := objectstorage.NewS3(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if remote.Check(ctx) != nil {
		t.Fatal("live bucket access failed")
	}
	token, e := objectstorage.UserKey(1, "files")
	if e != nil {
		t.Fatal(e)
	}
	store := &isolatedLiveStore{base: remote, prefix: "integration-tests/" + strings.TrimPrefix(token, "users/1/files/") + "/", keys: map[string]bool{}}
	t.Log("isolated S3 namespace:", store.prefix)
	t.Cleanup(func() {
		store.mu.Lock()
		keys := make([]string, 0, len(store.keys))
		for key := range store.keys {
			keys = append(keys, key)
		}
		store.mu.Unlock()
		c, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		for _, key := range keys {
			err := store.Delete(c, key)
			if err != nil {
				t.Error("test object cleanup failed")
			} else if _, err = store.Stat(c, key); err != objectstorage.ErrNotFound {
				t.Error("test object remained after cleanup")
			}
		}
	})
	fileStorageOptions = func() academy.StorageOptions { return academy.StorageOptions{Driver: "s3", Primary: store} }
	registry := objectstorage.Registry{DB: f.DB, Store: store}
	session := httptest.NewRecorder()
	if e = createSession(session, f.Owner); e != nil {
		t.Fatal("fixture session failed", e)
	}
	var cookie *http.Cookie
	for _, c := range session.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = &http.Cookie{Name: c.Name, Value: c.Value}
		}
	}
	if cookie == nil {
		t.Fatal("fixture session cookie missing")
	}
	svc := service.New(repository.New(f.DB))
	test, e := svc.Create(ctx, f.Owner, dto.CreateTest{Title: "S3 integration fixture", Visibility: "public", IsFree: true, PassingPercent: 60})
	if e != nil {
		t.Fatal(e)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, e := form.CreateFormFile("file", "question.png")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write(materialPNG())
	_ = form.WriteField("test_id", strconv.FormatInt(test.ID, 10))
	_ = form.Close()
	req := httptest.NewRequest("POST", "http://example.com/api/test-materials", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(cookie)
	response := httptest.NewRecorder()
	uploadQuestionMaterial(response, req)
	if response.Code != 201 {
		t.Fatal("live question upload failed", response.Code)
	}
	var asset struct {
		ID int64 `json:"id"`
	}
	if e = json.Unmarshal(response.Body.Bytes(), &asset); e != nil || asset.ID <= 0 {
		t.Fatal("question upload ID missing")
	}
	public := sharedPublicFiles(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	root, sources := exerciseSharedUploads(t, f, cookie, registry, public)
	mux := http.NewServeMux()
	mux.Handle("/static/", public)
	mux.HandleFunc("/api/test-materials/", serveQuestionMaterial)
	mux.HandleFunc("/api/v1/resumes/education-certificate", resumeEducationCertificateHandler)
	application := httptest.NewServer(mux)
	t.Cleanup(application.Close)
	checked := 0
	for _, source := range sources {
		if source.Problem != "" {
			t.Fatal("unexpected unresolved fixture")
		}
		if e = filemigration.Apply(ctx, registry, root, source); e != nil {
			t.Fatal("live migration verification failed")
		}
		if e = filemigration.VerifyApplication(ctx, registry, application.URL, cookie.String(), source); e != nil {
			t.Fatal("live application URL failed", source.Type)
		}
		checked++
	}
	denied := httptest.NewRecorder()
	serveQuestionMaterial(denied, httptest.NewRequest("GET", "/api/test-materials/"+strconv.FormatInt(asset.ID, 10), nil))
	if denied.Code != 404 {
		t.Fatal("anonymous test material access was allowed")
	}
	var original []byte
	if e = f.DB.QueryRow(`SELECT content FROM test_question_materials WHERE id=$1`, asset.ID).Scan(&original); e != nil || !bytes.Equal(original, materialPNG()) {
		t.Fatal("BYTEA original changed")
	}
	t.Logf("Verified %d real S3-backed application file URLs in an isolated test prefix; originals retained", checked)
}
