package main

import (
	"bytes"
	"testing"
)

func TestInjectYandexMetrikaAddsCounterOnce(t *testing.T) {
	page := []byte(`<!doctype html><html><head><title>FinTalent</title></head><body class="page"><main></main></body></html>`)
	injected := injectYandexMetrika(page)
	if bytes.Count(injected, []byte("tag.js?id=113008460")) != 1 {
		t.Fatal("counter script must be injected exactly once")
	}
	if bytes.Count(injected, []byte("watch/113008460")) != 1 {
		t.Fatal("noscript fallback must be injected exactly once")
	}
	if !bytes.Contains(injected, []byte(`ym(113008460, 'init'`)) {
		t.Fatal("counter initialization is missing")
	}
	if second := injectYandexMetrika(injected); !bytes.Equal(second, injected) {
		t.Fatal("repeated injection must not change the page")
	}
}
