package main

import (
	"net/http"
	"os"
	"strings"
)

const reactFrontendIndex = "static/react/index.html"

func serveFrontendPage(legacyFilename string) http.HandlerFunc {
	legacyHandler := servePage(legacyFilename)
	return func(w http.ResponseWriter, r *http.Request) {
		if reactFrontendDisabled() {
			legacyHandler(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}
		content, err := os.ReadFile(reactFrontendIndex)
		if err != nil {
			legacyHandler(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-FinTalent-Frontend", "react")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(content)
	}
}

func redirectFrontendPath(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		destination := target
		if r.URL.RawQuery != "" {
			destination += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, destination, http.StatusPermanentRedirect)
	}
}

func redirectFrontendPrefix(sourcePrefix, targetPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		destination := targetPrefix + strings.TrimPrefix(r.URL.Path, sourcePrefix)
		if r.URL.RawQuery != "" {
			destination += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, destination, http.StatusPermanentRedirect)
	}
}

func serveFrontendRoot(legacyFilename string) http.HandlerFunc {
	legacyHandler := servePage(legacyFilename)
	reactHandler := serveFrontendPage(legacyFilename)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			legacyHandler(w, r)
			return
		}
		reactHandler(w, r)
	}
}

func reactFrontendDisabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("REACT_FRONTEND")))
	return value == "false" || value == "0" || value == "off"
}
