package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestReactFrontendDisabled(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		disabled bool
	}{
		{name: "default enabled", value: "", disabled: false},
		{name: "explicit enabled", value: "true", disabled: false},
		{name: "false", value: "false", disabled: true},
		{name: "zero", value: "0", disabled: true},
		{name: "off case insensitive", value: " OFF ", disabled: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("REACT_FRONTEND", test.value)
			if got := reactFrontendDisabled(); got != test.disabled {
				t.Fatalf("reactFrontendDisabled() = %v, want %v", got, test.disabled)
			}
		})
	}
}

func TestServeFrontendPageFallsBackToLegacyWhenDisabled(t *testing.T) {
	t.Setenv("REACT_FRONTEND", "false")
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()

	serveFrontendPage("static/login.html")(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-FinTalent-Frontend") != "" {
		t.Fatal("legacy response must not have the React marker header")
	}
	if !strings.Contains(response.Body.String(), `id="login-form"`) {
		t.Fatal("legacy login page was not returned")
	}
}

func TestServeFrontendPageRejectsUnsupportedMethod(t *testing.T) {
	t.Setenv("REACT_FRONTEND", "true")
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	response := httptest.NewRecorder()

	serveFrontendPage("static/login.html")(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeFrontendPageSupportsHead(t *testing.T) {
	if _, err := os.Stat(reactFrontendIndex); err != nil {
		t.Skip("React build is not present; run npm run build first")
	}
	t.Setenv("REACT_FRONTEND", "true")
	request := httptest.NewRequest(http.MethodHead, "/login", nil)
	response := httptest.NewRecorder()

	serveFrontendPage("static/login.html")(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("X-FinTalent-Frontend"); got != "react" {
		t.Fatalf("frontend marker = %q, want react", got)
	}
	if response.Body.Len() != 0 {
		t.Fatal("HEAD response must not contain a body")
	}
}
