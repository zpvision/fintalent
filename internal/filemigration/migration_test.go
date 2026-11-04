package filemigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourcesRemainInsideUploadDirectories(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "static", "uploads", "avatars")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(dir, "file.png")
	if e := os.WriteFile(p, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	if got, e := ResolveSource(root, "static/uploads/avatars/file.png"); e != nil || got != p {
		t.Fatal(got, e)
	}
	for _, s := range []string{".env", "static/uploads/../../.env", "static/uploads/../file.png", "/etc/passwd", "uploads/resume-certificates/../.env", "static\\uploads\\avatars\\file.png"} {
		if _, e := ResolveSource(root, s); e == nil {
			t.Fatal("unsafe path accepted", s)
		}
	}
}
func TestManifestRejectsAmbiguousInput(t *testing.T) {
	for _, s := range []string{`[] {}`, `[{"file":{"legacy_ref":"same"}},{"file":{"legacy_ref":"same"}}]`, `[{"unknown":1}]`, `[{"file":{"legacy_ref":""}}]`} {
		if _, e := ReadManifest(strings.NewReader(s)); e == nil {
			t.Fatal("ambiguous manifest accepted")
		}
	}
	if _, e := ReadManifest(strings.NewReader(`[]`)); e != nil {
		t.Fatal(e)
	}
}

func TestApplicationTargetValidation(t *testing.T) {
	for _, base := range []string{"https://fintalent.ru", "http://localhost:8080", "http://127.0.0.1:12345/"} {
		if e := ValidateApplicationBase(base); e != nil {
			t.Fatal(base, e)
		}
	}
	for _, base := range []string{"https:", "http://untrusted.invalid", "https://user:password@example.invalid", "https://example.invalid/?", "https://example.invalid/#", "https://example.invalid/private", "https://example.invalid/?token=fixture"} {
		if e := ValidateApplicationBase(base); e == nil {
			t.Fatal("unsafe base accepted")
		}
	}
}
