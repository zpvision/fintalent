package objectstorage

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveS3IsolatedPrefix(t *testing.T) {
	if os.Getenv("RUN_S3_LIVE_TESTS") != "1" {
		t.Skip("explicit live-test opt-in required")
	}
	cfg, err := ConfigFrom(os.Getenv, false)
	if err != nil || cfg.Driver != "s3" {
		t.Fatal("S3 configuration incomplete")
	}
	store := NewS3(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err = store.Check(ctx); err != nil {
		t.Fatal("bucket connection/access failed")
	}
	token, err := UserKey(1, "files")
	if err != nil {
		t.Fatal(err)
	}
	key := "integration-tests/" + strings.TrimPrefix(token, "users/1/files/") + "/sample.txt"
	defer func() {
		clean, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if store.Delete(clean, key) != nil {
			t.Error("test object cleanup failed")
		}
	}()
	const content = "FinTalent isolated S3 integration test\n"
	if store.Put(ctx, key, strings.NewReader(content), int64(len(content)), "text/plain") != nil {
		t.Fatal("test upload failed")
	}
	info, err := store.Stat(ctx, key)
	if err != nil || info.Size != int64(len(content)) {
		t.Fatal("metadata mismatch")
	}
	body, _, err := store.Open(ctx, key, "")
	if err != nil {
		t.Fatal("download failed")
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(data) != content {
		t.Fatal("download content mismatch")
	}
	signed, err := store.SignGet(ctx, key, "text/plain", "attachment", time.Minute)
	if err != nil {
		t.Fatal("presign failed")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", signed.URL, nil)
	req.Header.Set("Range", "bytes=0-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("signed request failed")
	}
	data, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 206 || string(data) != content[:9] {
		t.Fatal("signed Range failed")
	}
	if store.Delete(ctx, key) != nil {
		t.Fatal("delete failed")
	}
	// Only objects in this run's private test prefix are touched.
	multiKey := strings.TrimSuffix(key, "sample.txt") + "multipart.bin"
	id, err := store.Begin(ctx, multiKey, "application/octet-stream")
	if err != nil {
		t.Fatal("multipart begin failed")
	}
	defer func() {
		clean, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		_ = store.Abort(clean, multiKey, id)
		if store.Delete(clean, multiKey) != nil {
			t.Error("multipart cleanup failed")
		}
	}()
	partsData := [][]byte{bytes.Repeat([]byte("a"), 5<<20), []byte("last part")}
	for _, i := range []int{1, 0} {
		data := partsData[i]
		signed, e := store.SignPart(ctx, multiKey, id, i+1, int64(len(data)), 15*time.Minute)
		if e != nil {
			t.Fatal("part signing failed")
		}
		origin := "https://fintalent.ru"
		if i == 1 {
			origin = "http://localhost:8080"
		}
		preflight, _ := http.NewRequestWithContext(ctx, "OPTIONS", signed.URL, nil)
		preflight.Header.Set("Origin", origin)
		preflight.Header.Set("Access-Control-Request-Method", "PUT")
		preflight.Header.Set("Access-Control-Request-Headers", "content-type")
		preflightResponse, preflightError := http.DefaultClient.Do(preflight)
		if preflightError != nil {
			t.Fatal("CORS preflight request failed")
		}
		io.Copy(io.Discard, preflightResponse.Body)
		preflightResponse.Body.Close()
		if preflightResponse.StatusCode != 200 || preflightResponse.Header.Get("Access-Control-Allow-Origin") != origin || !strings.Contains(preflightResponse.Header.Get("Access-Control-Allow-Methods"), "PUT") {
			t.Errorf("CORS preflight rejected for %s (status=%d)", origin, preflightResponse.StatusCode)
		}
		req, _ := http.NewRequestWithContext(ctx, "PUT", signed.URL, bytes.NewReader(data))
		req.Header.Set("Origin", origin)
		for k, v := range signed.Headers {
			req.Header.Set(k, v)
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatalf("direct part PUT failed (part=%d, context=%v, type=%T)", i+1, ctx.Err(), e)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal("direct part PUT rejected", resp.StatusCode)
		}
		if resp.Header.Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("CORS does not allow %s", origin)
		}
		t.Logf("CORS checked: %s", origin)
	}
	parts, err := store.Parts(ctx, multiKey, id)
	if err != nil || len(parts) != 2 {
		t.Fatal("multipart list failed")
	}
	if store.Complete(ctx, multiKey, id, parts) != nil {
		t.Fatal("multipart complete failed")
	}
	body, _, err = store.Open(ctx, multiKey, "")
	if err != nil {
		t.Fatal("multipart download failed")
	}
	data, err = io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(data, bytes.Join(partsData, nil)) {
		t.Fatal("multipart content mismatch")
	}
	t.Log("multipart: signed PUT, ListParts, Complete and content verification passed")
	privateURL, err := store.SignGet(ctx, multiKey, "application/octet-stream", "attachment", time.Minute)
	if err != nil {
		t.Fatal("private access probe signing failed")
	}
	unsigned, err := url.Parse(privateURL.URL)
	if err != nil {
		t.Fatal("private access probe URL invalid")
	}
	unsigned.RawQuery = ""
	probe, _ := http.NewRequestWithContext(ctx, "GET", unsigned.String(), nil)
	anonymous, err := http.DefaultClient.Do(probe)
	if err != nil {
		t.Fatal("anonymous access probe failed")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(anonymous.Body, 1024))
	anonymous.Body.Close()
	if anonymous.StatusCode != 401 && anonymous.StatusCode != 403 && anonymous.StatusCode != 404 {
		t.Fatal("private bucket allowed anonymous read or access could not be verified", anonymous.StatusCode)
	}
	t.Log("unsigned access denied:", anonymous.StatusCode)
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jN0sAAAAASUVORK5CYII=")
	for _, sample := range []struct {
		name, mime string
		data       []byte
	}{
		{"image.png", "image/png", image},
		{"document.pdf", "application/pdf", []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF\n")},
	} {
		k := strings.TrimSuffix(key, "sample.txt") + sample.name
		func() {
			defer func() {
				clean, done := context.WithTimeout(context.Background(), 30*time.Second)
				defer done()
				if store.Delete(clean, k) != nil {
					t.Error("sample cleanup failed")
				}
			}()
			if store.Put(ctx, k, bytes.NewReader(sample.data), int64(len(sample.data)), sample.mime) != nil {
				t.Fatal("sample upload failed")
			}
			b, info, e := store.Open(ctx, k, "")
			if e != nil {
				t.Fatal("sample download failed")
			}
			got, e := io.ReadAll(b)
			b.Close()
			if e != nil || !bytes.Equal(got, sample.data) || info.ContentType != sample.mime {
				t.Fatal("sample verification failed")
			}
		}()
	}
	expiring, err := store.SignGet(ctx, multiKey, "application/octet-stream", "attachment", time.Second)
	if err != nil {
		t.Fatal("expiry signing failed")
	}
	time.Sleep(2 * time.Second)
	req, _ = http.NewRequestWithContext(ctx, "GET", expiring.URL, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("expiry check request failed")
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Error("expired URL was not denied", resp.StatusCode)
	}
	if _, err = store.Stat(ctx, key); err != ErrNotFound {
		t.Fatal("object remained after deletion")
	}
}
