package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"FinTalent/internal/diagnostics"
)

var errorJournal *diagnostics.Store
var frontendDiagnosticLimiter requestLimiter

func applicationRelease() string {
	if v := diagnostics.Token(os.Getenv("APP_RELEASE"), 80); v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" {
				return diagnostics.Token(v.Value, 80)
			}
		}
	}
	return "development"
}

func startErrorJournal() func() {
	dir := strings.TrimSpace(os.Getenv("ERROR_LOG_DIR"))
	if dir == "" {
		dir = filepath.Join(".local-data", "diagnostics")
	}
	// A configuration mistake must never expose the private journal via FileServer.
	abs, err := filepath.Abs(dir)
	if err == nil {
		var rel string
		static, _ := filepath.Abs("static")
		rel, err = filepath.Rel(static, abs)
		if err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			err = fmt.Errorf("journal must be outside static")
		}
	}
	if err == nil {
		errorJournal, err = diagnostics.Open(diagnostics.Config{Dir: dir, Release: applicationRelease()})
	}
	if err != nil {
		// One startup warning only; a full or read-only disk must not stop the site.
		log.Print("Журнал ошибок недоступен; проверьте ERROR_LOG_DIR, свободное место и права каталога")
		return func() {}
	}
	diagnostics.SetDefault(errorJournal)
	return func() { diagnostics.SetDefault(nil); errorJournal.Close() }
}

func registerDiagnosticsRoutes() {
	http.HandleFunc("/api/diagnostics/events", frontendErrorEvent)
	http.HandleFunc("/api/admin/diagnostics", adminErrorReport)
}

func frontendErrorEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if errorJournal == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !frontendDiagnosticLimiter.allow("all", 300, time.Now()) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	var in struct {
		Code      string `json:"code"`
		Route     string `json:"route"`
		Method    string `json:"method"`
		Status    int    `json:"status"`
		RequestID string `json:"request_id"`
		Stack     string `json:"stack"`
		Release   string `json:"release"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, "Некорректное событие")
		return
	}
	allowed := map[string]bool{"javascript_error": true, "unhandled_rejection": true, "react_error": true, "network_error": true, "api_error": true, "resource_error": true}
	if !allowed[in.Code] || in.Status < 0 || in.Status > 599 {
		writeJSON(w, 400, "Некорректный тип события")
		return
	}
	// Client events are untrusted. Identity comes only from the existing DB session.
	var userID int64
	if cookie, e := r.Cookie(sessionCookie); e == nil && len(cookie.Value) == 64 && db != nil {
		if u, err := userFromRequest(r); err == nil {
			userID = u.ID
		}
	}
	errorJournal.Submit(diagnostics.Event{Source: "frontend", Code: in.Code, Operation: "browser", UserID: userID, Route: in.Route, Method: in.Method, Status: in.Status, RequestID: in.RequestID, Stack: in.Stack, ClientRelease: in.Release})
	w.WriteHeader(http.StatusNoContent)
}

var errorReportSlots = make(chan struct{}, 1)

func adminErrorReport(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, "Метод не поддерживается")
		return
	}
	period, err := diagnostics.ParseHours(r.URL.Query().Get("hours"))
	if err != nil {
		writeJSON(w, 400, "Период от 1 до 720 часов")
		return
	}
	if errorJournal == nil {
		writeJSON(w, 503, "Журнал недоступен")
		return
	}
	select {
	case errorReportSlots <- struct{}{}:
		defer func() { <-errorReportSlots }()
	default:
		writeJSON(w, 429, "Отчёт уже формируется")
		return
	}
	report, err := errorJournal.Report(time.Now().Add(-period))
	if err != nil {
		writeJSON(w, 503, "Не удалось прочитать журнал")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="fintalent-errors.json"`)
	}
	writeAdminJSON(w, 200, report)
}

func injectErrorCollector(content []byte) []byte {
	tag := `<meta name="fintalent-release" content="` + applicationRelease() + `">`
	if !strings.Contains(string(content), "/static/error-telemetry.js") {
		tag += `<script src="/static/error-telemetry.js?v=1"></script>`
	}
	return []byte(strings.Replace(string(content), "<head>", "<head>"+tag, 1))
}
