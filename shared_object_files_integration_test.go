package main

import (
	"FinTalent/internal/academy"
	"FinTalent/internal/filemigration"
	"FinTalent/internal/objectstorage"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	imagepng "image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Legacy suites must never silently upload fixtures to a real bucket merely
// because the developer has S3 credentials in their local environment.
func useLocalSharedStorage(t *testing.T) {
	t.Helper()
	previous := fileStorageOptions
	fileStorageOptions = func() academy.StorageOptions { return academy.StorageOptions{Driver: "local"} }
	t.Cleanup(func() { fileStorageOptions = previous })
}

func TestSharedFilesIsolatedPostgres(t *testing.T) {
	f := newAcademyFocusedFixture(t)
	remote := &storageFixture{objects: map[string]storageFixtureObject{}}
	fileStorageOptions = func() academy.StorageOptions { return academy.StorageOptions{Driver: "s3", Primary: remote} }
	ctx := context.Background()
	registry := objectstorage.Registry{DB: f.DB, Store: remote}
	png := materialPNG()
	ref := "/static/uploads/avatars/fixture.png"
	meta := objectstorage.RegisteredFile{Ref: ref, Owner: fileUser(f.Owner), Category: "files", Name: "fixture.png", MIME: "image/png", Size: int64(len(png))}
	if e := registry.CopyVerified(ctx, meta, bytes.NewReader(png)); e != nil {
		t.Fatal(e)
	}
	saved, e := registry.Lookup(ctx, ref)
	if e != nil || saved.Status != "ready" || !strings.HasPrefix(saved.Key, fmt.Sprintf("users/%d/files/", f.Owner)) {
		t.Fatal("registry not ready", e)
	}
	if e = registry.CopyVerified(ctx, meta, bytes.NewReader(png)); e != nil {
		t.Fatal("repeat verification", e)
	}
	wrong := meta
	wrong.Owner = fileUser(f.Student)
	if e = registry.CopyVerified(ctx, wrong, bytes.NewReader(png)); e == nil {
		t.Fatal("owner changed")
	}
	if _, e = f.DB.Exec(`UPDATE object_files SET owner_id=$2 WHERE legacy_ref=$1`, ref, f.Student); e == nil {
		t.Fatal("database allowed changing permanent storage ownership")
	}
	public := sharedPublicFiles(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	w := httptest.NewRecorder()
	public.ServeHTTP(w, httptest.NewRequest("GET", ref, nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), png) || w.Header().Get("X-FinTalent-Storage") != "s3" {
		t.Fatal("historical URL did not read S3", w.Code)
	}
	// Even a fake backend outage cannot silently switch a ready row to local.
	delete(remote.objects, saved.Key)
	w = httptest.NewRecorder()
	public.ServeHTTP(w, httptest.NewRequest("GET", ref, nil))
	if w.Code != 503 {
		t.Fatal("missing S3 object did not fail closed")
	}
	if e = registry.CopyVerified(ctx, meta, bytes.NewReader(png)); e == nil {
		t.Fatal("missing ready object was not detected")
	}
	remote.objects[saved.Key] = storageFixtureObject{png, "image/png"}
	// Original files survive copying and remain available for explicit rollback.
	original := filepath.Join(t.TempDir(), "original.png")
	if e = os.WriteFile(original, png, 0600); e != nil {
		t.Fatal(e)
	}
	input, e := os.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	if e = registry.CopyVerified(ctx, meta, input); e != nil {
		t.Fatal(e)
	}
	input.Close()
	if _, e = os.Stat(original); e != nil {
		t.Fatal("source removed")
	}
	var testID, material int64
	if e = f.DB.QueryRow(`INSERT INTO tests(author_id,slug) VALUES($1,'shared-s3-fixture') RETURNING id`, f.Owner).Scan(&testID); e != nil {
		t.Fatal(e)
	}
	if e = f.DB.QueryRow(`INSERT INTO test_question_materials(test_id,owner_id,name,mime,size,content) VALUES($1,$2,'fixture.png','image/png',$3,$4) RETURNING id`, testID, f.Owner, len(png), png).Scan(&material); e != nil {
		t.Fatal(e)
	}
	materialMeta := meta
	materialMeta.Ref = "test-material:" + strconv.FormatInt(material, 10)
	materialMeta.Category = "tests"
	if e = registry.CopyVerified(ctx, materialMeta, bytes.NewReader(png)); e != nil {
		t.Fatal(e)
	}
	cookie := func(user int64) *http.Cookie {
		token := fmt.Sprintf("shared-storage-fixture-%d", user)
		if _, e := f.DB.Exec(`INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,now()+interval '1 day')`, user, hashSessionToken(token)); e != nil {
			t.Fatal(e)
		}
		return &http.Cookie{Name: sessionCookie, Value: token}
	}
	ownerCookie, otherCookie := cookie(f.Owner), cookie(f.Student)
	for _, tc := range []struct {
		cookie *http.Cookie
		code   int
	}{{ownerCookie, 200}, {otherCookie, 404}, {nil, 404}} {
		r := httptest.NewRequest("GET", "/api/test-materials/"+strconv.FormatInt(material, 10), nil)
		if tc.cookie != nil {
			r.AddCookie(tc.cookie)
		}
		w = httptest.NewRecorder()
		serveQuestionMaterial(w, r)
		if w.Code != tc.code {
			t.Fatal("material access changed", w.Code, tc.code)
		}
		if tc.code == 200 && (!bytes.Equal(w.Body.Bytes(), png) || w.Header().Get("Location") != "") {
			t.Fatal("private response redirected or corrupted")
		}
	}
	var unchanged []byte
	if e = f.DB.QueryRow(`SELECT content FROM test_question_materials WHERE id=$1`, material).Scan(&unchanged); e != nil || !bytes.Equal(unchanged, png) {
		t.Fatal("BYTEA source changed")
	}
	var certificate int64
	if e = f.DB.QueryRow(`INSERT INTO resume_education_certificates(user_id,storage_name,original_name,content_type,file_size) VALUES($1,'fixture-key.png','certificate.png','image/png',$2) RETURNING id`, f.Owner, len(png)).Scan(&certificate); e != nil {
		t.Fatal(e)
	}
	certMeta := meta
	certMeta.Ref = "certificate:fixture-key.png"
	if e = registry.CopyVerified(ctx, certMeta, bytes.NewReader(png)); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		cookie *http.Cookie
		code   int
	}{{ownerCookie, 200}, {otherCookie, 404}, {nil, 401}} {
		r := httptest.NewRequest("GET", "/api/v1/resumes/education-certificate?id="+strconv.FormatInt(certificate, 10), nil)
		if tc.cookie != nil {
			r.AddCookie(tc.cookie)
		}
		w = httptest.NewRecorder()
		serveResumeCertificate(w, r)
		if w.Code != tc.code {
			t.Fatal("certificate access changed", w.Code, tc.code)
		}
	}
	root, sources := exerciseSharedUploads(t, f, ownerCookie, registry, public)
	mux := http.NewServeMux()
	mux.Handle("/static/", public)
	mux.HandleFunc("/api/test-materials/", serveQuestionMaterial)
	mux.HandleFunc("/api/v1/resumes/education-certificate", resumeEducationCertificateHandler)
	application := httptest.NewServer(mux)
	t.Cleanup(application.Close)
	var rollbackSource *filemigration.Source
	for _, source := range sources {
		if source.Problem != "" {
			continue
		}
		if e = filemigration.Apply(ctx, registry, root, source); e != nil {
			t.Fatal("idempotent migration", e)
		}
		wrongHash := source
		wrongHash.File.SHA256 = strings.Repeat("0", 64)
		if e = filemigration.Apply(ctx, registry, root, wrongHash); e == nil {
			t.Fatal("inventory SHA-256 was ignored")
		}
		if e = filemigration.VerifyApplication(ctx, registry, application.URL, ownerCookie.String(), source); e != nil {
			t.Fatal("application URL verification", source.Type, e)
		}
		if strings.HasPrefix(source.File.Ref, "certificate:") {
			forged := source
			forged.Path = "static/uploads/wrong-original.png"
			if e = filemigration.Rollback(ctx, registry, root, forged); e == nil {
				t.Fatal("certificate rollback accepted mismatched source path")
			}
			if e = filemigration.Rollback(ctx, registry, root, source); e != nil {
				t.Fatal("certificate rollback", e)
			}
			if e = filemigration.Apply(ctx, registry, root, source); e != nil {
				t.Fatal("certificate reapply", e)
			}
		}
		if strings.HasPrefix(source.File.Ref, "/static/uploads/") {
			copy := source
			rollbackSource = &copy
		}
	}
	if rollbackSource == nil {
		t.Fatal("rollback source missing")
	}
	legacyOnly := httptest.NewServer(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	t.Cleanup(legacyOnly.Close)
	if e = filemigration.VerifyApplication(ctx, registry, legacyOnly.URL, ownerCookie.String(), *rollbackSource); e == nil {
		t.Fatal("Nginx bypass of the S3 resolver was not detected")
	}
	if e = filemigration.Rollback(ctx, registry, root, *rollbackSource); e != nil {
		t.Fatal("rollback", e)
	}
	w = httptest.NewRecorder()
	public.ServeHTTP(w, httptest.NewRequest("GET", rollbackSource.File.Ref, nil))
	if w.Code != 200 || w.Header().Get("X-FinTalent-Storage") != "" {
		t.Fatal("legacy reads not restored")
	}
	if e = filemigration.Apply(ctx, registry, root, *rollbackSource); e != nil {
		t.Fatal("reapply", e)
	}
	p, e := filemigration.ResolveSource(root, rollbackSource.Path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, []byte("changed original"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = filemigration.Rollback(ctx, registry, root, *rollbackSource); e == nil {
		t.Fatal("corrupt original was allowed as rollback source")
	}
	if e = filemigration.Apply(ctx, registry, root, *rollbackSource); e == nil {
		t.Fatal("manifest source change was not detected")
	}
}

func exerciseSharedUploads(t *testing.T, f academyFocusedFixture, cookie *http.Cookie, registry objectstorage.Registry, public http.Handler) (string, []filemigration.Source) {
	t.Helper()
	ctx := context.Background()
	var e error
	var w *httptest.ResponseRecorder
	// Exercise real upload handlers in an isolated directory, not only registry APIs.
	t.Chdir(t.TempDir())
	var largePNG bytes.Buffer
	if e = imagepng.Encode(&largePNG, image.NewRGBA(image.Rect(0, 0, 100, 100))); e != nil {
		t.Fatal(e)
	}
	var company int64
	if e = f.DB.QueryRow(`INSERT INTO accounting_companies(owner_user_id,name,slug) VALUES($1,'S3 company','shared-s3-company') RETURNING id`, f.Owner).Scan(&company); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, field string
		handler     http.HandlerFunc
		extra       map[string]string
		code        int
	}{
		{"avatar", "avatar", profileAvatar, nil, 201},
		{"publication", "image", publicationUpload, nil, 201},
		{"profimarket", "image", profiMarketUploadAPI, nil, 201},
		{"company", "file", accountingCompanyUpload, map[string]string{"company_id": strconv.FormatInt(company, 10), "kind": "logo"}, 200},
		{"certificate", "certificate", resumeEducationCertificateHandler, nil, 201},
	} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile(tc.field, "original.png")
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write(largePNG.Bytes())
		for k, v := range tc.extra {
			_ = writer.WriteField(k, v)
		}
		_ = writer.Close()
		r := httptest.NewRequest("POST", "http://example.com/upload", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("Origin", "http://example.com")
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		tc.handler(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s upload status=%d body=%s", tc.name, w.Code, w.Body.String())
		}
		var response struct {
			URL    string `json:"url"`
			Avatar string `json:"avatar"`
			ID     int64  `json:"id"`
		}
		if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		ref := response.URL
		if ref == "" {
			ref = response.Avatar
		}
		if tc.name == "certificate" {
			var key string
			if e = f.DB.QueryRow(`SELECT storage_name FROM resume_education_certificates WHERE id=$1`, response.ID).Scan(&key); e != nil {
				t.Fatal(e)
			}
			ref = "certificate:" + key
		}
		row, e := registry.Lookup(ctx, ref)
		if e != nil || row.Status != "ready" || row.Name != "original.png" {
			t.Fatal(tc.name, "did not register original metadata", e)
		}
		expected := fileUser(f.Owner)
		if tc.name == "company" {
			expected = fileCompany(company)
		}
		if row.Owner != expected {
			t.Fatal(tc.name, "wrong business owner")
		}
		if row.CreatedBy != f.Owner {
			t.Fatal(tc.name, "uploader audit identity missing")
		}
		if tc.name != "certificate" {
			r = httptest.NewRequest("GET", ref, nil)
			w = httptest.NewRecorder()
			public.ServeHTTP(w, r)
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), largePNG.Bytes()) {
				t.Fatal(tc.name, "historical public URL failed")
			}
		}
	}
	var companyBytes int64
	if e = f.DB.QueryRow(`SELECT bytes FROM file_storage_usage WHERE owner_kind='accounting_company' AND owner_id=$1 AND category='files'`, company).Scan(&companyBytes); e != nil || companyBytes != int64(largePNG.Len()) {
		t.Fatal("company storage usage attribution failed", e)
	}
	root, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	sources, e := filemigration.Inventory(ctx, f.DB, root)
	if e != nil {
		t.Fatal("inventory", e)
	}

	return root, sources
}
