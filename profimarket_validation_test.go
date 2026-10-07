package main

import "testing"

func TestProfiResourceValidation(t *testing.T) {
	for _, value := range []string{"", "/static/uploads/image.png", "https://example.com/image.png?q=%22"} {
		if !safeContentURL(value) {
			t.Errorf("rejected safe URL %q", value)
		}
	}
	for _, value := range []string{`invalid" onerror="alert(1)`, "javascript:alert(1)", "data:text/html,test", "//evil.test/x", "/\\evil.test/x", "https://example.com/\nx", "https://user:pass@example.com/x"} {
		if safeContentURL(value) {
			t.Errorf("accepted unsafe URL %q", value)
		}
	}
	for _, in := range []profiSolutionInput{
		{CoverImage: `x" onerror="alert(1)`},
		{Sections: []profiSection{{NumberingColor: "red;position:fixed;inset:0"}}},
		{Media: []profiMedia{{Type: "VIDEO", URL: "https://evil.test/embed"}}},
		{ProductData: map[string]any{"video_url": "javascript:alert(1)"}},
	} {
		if validateProfiResources(&in) == nil {
			t.Fatal("accepted malicious resource")
		}
	}
}
