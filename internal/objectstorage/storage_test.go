package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalLifecycleAndIsolation(t *testing.T) {
	s := Local{Root: t.TempDir()}
	ctx := context.Background()
	for _, key := range []string{"../secret", "/absolute", "a//b", "a\\b", "a/../b"} {
		if s.Put(ctx, key, strings.NewReader("x"), 1, "text/plain") == nil {
			t.Fatal("unsafe key", key)
		}
	}
	if e := s.Put(ctx, "academy/file.txt", strings.NewReader("hello"), 5, "text/plain"); e != nil {
		t.Fatal(e)
	}
	r, i, e := s.Open(ctx, "academy/file.txt", "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "hello" || i.Size != 5 {
		t.Fatal(i)
	}
	if e = s.Put(ctx, "wrong", strings.NewReader("too long"), 1, ""); e == nil {
		t.Fatal("size mismatch accepted")
	}
	if _, e = s.Stat(ctx, "wrong"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, "academy/file.txt"); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, "academy/file.txt"); e != nil {
		t.Fatal("delete not idempotent")
	}
}

func TestConfigFailsClosed(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if _, e := ConfigFrom(get(nil), true); e == nil {
		t.Fatal("production defaulted to disk")
	}
	if c, e := ConfigFrom(get(nil), false); e != nil || c.Driver != "local" {
		t.Fatal(c, e)
	}
	m := map[string]string{"ACADEMY_STORAGE_DRIVER": "s3", "S3_ENDPOINT": "https://storage.yandexcloud.net", "S3_REGION": "ru-central1", "S3_BUCKET": "bucket", "S3_ACCESS_KEY": "fixture", "S3_SECRET_KEY": "fixture"}
	if _, e := ConfigFrom(get(m), true); e != nil {
		t.Fatal(e)
	}
	for _, endpoint := range []string{"http://host", "https://user:pass@host", "https://host?token=secret", "https://host/path"} {
		m["S3_ENDPOINT"] = endpoint
		if _, e := ConfigFrom(get(m), true); e == nil {
			t.Fatal("invalid endpoint accepted")
		}
	}
}

func TestS3SDKRequestsMultipartRangeAndErrors(t *testing.T) {
	var failures atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") && r.URL.Query().Get("X-Amz-Signature") == "" {
			t.Error("unsigned request")
		}
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.URL.Path == "/bucket" && r.Method == "HEAD":
			return
		case r.URL.Query().Has("uploads"):
			io.WriteString(w, `<InitiateMultipartUploadResult><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`)
		case r.URL.Query().Get("uploadId") != "" && r.Method == "GET":
			io.WriteString(w, `<ListPartsResult><IsTruncated>false</IsTruncated><Part><PartNumber>1</PartNumber><Size>4</Size><ETag>"part-tag"</ETag></Part></ListPartsResult>`)
		case r.URL.Query().Get("uploadId") != "" && r.Method == "POST":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "part-tag") {
				t.Error("complete omitted server ETag")
			}
			io.WriteString(w, `<CompleteMultipartUploadResult><ETag>"object-tag"</ETag></CompleteMultipartUploadResult>`)
		case r.URL.Query().Get("uploadId") != "" && r.Method == "DELETE":
			w.WriteHeader(204)
		case r.URL.Query().Get("partNumber") != "" && r.Method == "PUT":
			w.Header().Set("ETag", `"part-tag"`)
		case strings.HasSuffix(r.URL.Path, "/missing"):
			w.WriteHeader(404)
			io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
		case strings.HasSuffix(r.URL.Path, "/failure"):
			failures.Add(1)
			w.WriteHeader(503)
			io.WriteString(w, `<Error><Code>Unavailable</Code><Message>private URL secret</Message></Error>`)
		case r.Method == "HEAD":
			w.Header().Set("Content-Length", "4")
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("ETag", `"object-tag"`)
		case r.Method == "GET":
			if r.Header.Get("Range") != "bytes=0-3" {
				t.Error("range missing")
			}
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(206)
			io.WriteString(w, "data")
		case r.Method == "PUT":
			b, _ := io.ReadAll(r.Body)
			if string(b) != "data" {
				t.Error("put body lost")
			}
		case r.Method == "DELETE":
			w.WriteHeader(204)
		default:
			t.Error("unexpected operation", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	cfg := Config{Endpoint: server.URL, Region: "ru-central1", Bucket: "bucket", AccessKey: "fixture-access", SecretKey: "fixture-secret", PathStyle: true}
	s := NewS3(cfg)
	s.client.Options().HTTPClient.(*http.Client).Transport = server.Client().Transport
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if e := s.Check(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.Put(ctx, "file", strings.NewReader("data"), 4, "video/mp4"); e != nil {
		t.Fatal(e)
	}
	if e := s.Put(ctx, "file", bytes.NewBufferString("data"), 4, "video/mp4"); e != nil {
		t.Fatal("non-seekable upload failed", e)
	}
	info, e := s.Stat(ctx, "file")
	if e != nil || info.Size != 4 {
		t.Fatal(info, e)
	}
	r, _, e := s.Open(ctx, "file", "bytes=0-3")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "data" {
		t.Fatal("range body")
	}
	id, e := s.Begin(ctx, "file", "video/mp4")
	if e != nil || id != "upload-1" {
		t.Fatal(id, e)
	}
	signed, e := s.SignPart(ctx, "file", id, 1, 4, time.Minute)
	if e != nil || !strings.Contains(signed.URL, "X-Amz-SignedHeaders=content-length") {
		t.Fatal("part size not signed", e)
	}
	parts, e := s.Parts(ctx, "file", id)
	if e != nil || len(parts) != 1 || parts[0].Size != 4 {
		t.Fatal(parts, e)
	}
	if e = s.Complete(ctx, "file", id, parts); e != nil {
		t.Fatal(e)
	}
	if e = s.Abort(ctx, "file", id); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SignGet(ctx, "file", "video/mp4", "inline", time.Minute); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, "file"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Stat(ctx, "missing"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	_, e = s.Stat(ctx, "failure")
	if !errors.Is(e, ErrUnavailable) || strings.Contains(e.Error(), "secret") || failures.Load() < 2 {
		t.Fatal("retry/redaction failed", e, failures.Load())
	}
	cfg.Endpoint = "https://other-provider.invalid"
	cfg.Bucket = "replacement"
	if s.Identity() == NewS3(cfg).Identity() {
		t.Fatal("in-flight backend identity was not changed")
	}
}
