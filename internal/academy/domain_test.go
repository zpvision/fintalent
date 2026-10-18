package academy

import (
	"strings"
	"testing"
)

func fixture() Program {
	m := NewModule()
	l := NewLesson()
	l.Blocks = []Block{{Key: Key(), Type: "text", Data: BlockData{Rich: []RichBlock{{Type: "paragraph", Text: `<b>Текст</b><script>alert(1)</script><a href="javascript:alert(1)">x</a><img src=x onerror=alert(1)>`}}}}}
	m.Lessons = []Lesson{l}
	return Program{Modules: []Module{m}}
}

func TestPublishedCreationDefaults(t *testing.T) {
	p := fixture()
	if p.Modules[0].Status != "published" || p.Modules[0].Lessons[0].Status != "published" {
		t.Fatal("new modules and lessons must be published by default")
	}
	if err := Validate(&p); err != nil {
		t.Fatal(err)
	}
	if ready := CheckReadiness(p); len(ready.Blockers) != 0 || ready.Lessons != 1 {
		t.Fatalf("new lesson with materials requires manual publication: %+v", ready)
	}
	p.Modules[0].Lessons[0].Status = "draft"
	if err := Validate(&p); err != nil || p.Modules[0].Lessons[0].Status != "draft" {
		t.Fatalf("explicit draft status was not preserved: %v", err)
	}
	if len(CheckReadiness(p).Blockers) == 0 {
		t.Fatal("explicit draft lesson became available for publication")
	}
}

func TestValidateAndSanitize(t *testing.T) {
	p := fixture()
	if e := Validate(&p); e != nil {
		t.Fatal(e)
	}
	html := p.Modules[0].Lessons[0].Blocks[0].Data.Rich[0].Text
	if strings.Contains(html, "script") || strings.Contains(html, "onerror") || strings.Contains(html, "javascript:") || !strings.Contains(html, "<b>") {
		t.Fatal("unsafe or lost formatting", html)
	}
	for _, change := range []func(*Program){func(p *Program) { p.Modules[0].Key = "../x" }, func(p *Program) { p.Modules[0].Lessons[0].Key = p.Modules[0].Key }, func(p *Program) { p.Modules[0].Lessons[0].CompletionRule = "now" }, func(p *Program) { p.Modules[0].Lessons[0].Blocks[0].Type = "html" }, func(p *Program) { p.Modules[0].Lessons[0].Blocks[0].Data.URL = "javascript:alert(1)" }, func(p *Program) { p.Modules[0].Lessons[0].Duration = -1 }} {
		p := fixture()
		change(&p)
		if Validate(&p) == nil {
			t.Fatal("invalid program accepted")
		}
	}
}
func TestVideoSources(t *testing.T) {
	for _, url := range []string{"https://youtu.be/abc_123", "https://www.youtube.com/watch?v=abc-123", "https://vimeo.com/123456", "https://rutube.ru/video/abc123/", "https://cdn.example.invalid/a.mp4"} {
		if typ, _ := VideoSource(url); typ == "" {
			t.Fatal(url)
		}
	}
	for _, url := range []string{"javascript:alert(1)", "http://youtube.com/watch?v=a", "https://youtube.com.evil.invalid/watch?v=a", "https://evil.invalid/watch?a=b", "https://user:pass@youtu.be/a"} {
		if typ, _ := VideoSource(url); typ != "" {
			t.Fatal(url)
		}
	}
}
func TestFileValidation(t *testing.T) {
	for _, v := range []struct{ name, body string }{{"bad.png", "<script>x</script>"}, {"a.svg", "<svg/>"}, {"a.docx", "not a zip"}, {"x.exe", "MZ"}} {
		if _, e := fileType(v.name, []byte(v.body)); e == nil {
			t.Fatal("unsafe file accepted", v.name)
		}
	}
	if _, e := fileType("guide.pdf", []byte("%PDF-1.7\nfixture")); e != nil {
		t.Fatal(e)
	}
	if _, e := fileType("guide.txt", []byte("Учебные материалы")); e != nil {
		t.Fatal(e)
	}
}
