package filemigration

import (
	"FinTalent/internal/objectstorage"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateAnonymousVerification(t *testing.T) {
	s := Source{Type: "test_material", ID: 3, File: objectstorage.RegisteredFile{Ref: "test-material:3", Owner: objectstorage.Owner{Kind: "user", ID: 7}}}
	for _, code := range []int{200, 204, 302, 401, 403, 404, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/test-materials/3" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Error("unexpected path or credentials")
				}
				w.Header().Set("Location", "https://example.invalid/never-follow")
				w.WriteHeader(code)
			}))
			defer server.Close()
			err := VerifyPrivateAnonymous(context.Background(), server.URL, s)
			denied := code == 401 || code == 403 || code == 404
			if denied && err != nil || !denied && !errors.Is(err, ErrPrivateExposure) {
				t.Fatalf("status %d: unexpected verification result", code)
			}
		})
	}
	s.Type = "file"
	s.File.Ref = "certificate:private-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/resumes/education-certificate" || r.URL.Query().Get("id") != "3" {
			t.Error("incorrect certificate route")
		}
		w.WriteHeader(401)
	}))
	defer server.Close()
	if VerifyPrivateAnonymous(context.Background(), server.URL, s) != nil {
		t.Fatal("certificate denial verification failed")
	}
}

func TestPrivateVerificationRejectsUnsupportedSources(t *testing.T) {
	for _, source := range []Source{
		{Type: "file", ID: 1, File: objectstorage.RegisteredFile{Ref: "/static/uploads/public.png", Owner: objectstorage.Owner{Kind: "user", ID: 7}}},
		{Type: "test_material", ID: 1, File: objectstorage.RegisteredFile{Ref: "test-material:2", Owner: objectstorage.Owner{Kind: "user", ID: 7}}},
		{Type: "test_material", ID: 1, File: objectstorage.RegisteredFile{Ref: "test-material:1"}},
	} {
		if IsPrivateSource(source) || !errors.Is(VerifyPrivateAnonymous(context.Background(), "http://127.0.0.1", source), objectstorage.ErrInvalid) {
			t.Fatal("unsupported source accepted")
		}
	}
}
