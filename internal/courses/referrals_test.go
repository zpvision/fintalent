package courses

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReferralValidation(t *testing.T) {
	for _, v := range []struct {
		name, code string
		valid      bool
	}{{"Telegram", "Telegram_2026", true}, {" Email ", " abc-123 ", true}, {"", "hello", false}, {"A", "ab", false}, {"A", "hello world", false}, {"A", "кириллица", false}, {"A", "<script>", false}, {"A", strings.Repeat("a", 65), false}, {strings.Repeat("Я", 121), "okay", false}} {
		_, code, valid := normalizeReferral(v.name, v.code)
		if valid != v.valid {
			t.Fatalf("validation for %q: %v", v.code, valid)
		}
		if valid && code != strings.ToLower(strings.TrimSpace(v.code)) {
			t.Fatal("code not normalized")
		}
	}
}

func TestReferralCrossOriginAndPreviewBots(t *testing.T) {
	r := httptest.NewRequest("POST", "https://fintalent.ru/api/courses/interest", nil)
	r.Header.Set("Origin", "https://evil.example")
	if referralSameOrigin(r) {
		t.Fatal("cross-origin request accepted")
	}
	r.Header.Set("Origin", "https://fintalent.ru")
	if !referralSameOrigin(r) {
		t.Fatal("same-origin denied")
	}
	r.Header.Set("User-Agent", "TelegramBot (like TwitterBot)")
	if !isPreviewBot(r) {
		t.Fatal("preview bot counted")
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 Chrome/130.0")
	if isPreviewBot(r) {
		t.Fatal("browser rejected")
	}
}

func TestReferralOpaqueCookieAndRotation(t *testing.T) {
	r := httptest.NewRequest("GET", "https://fintalent.ru", nil)
	if guestHash(r) != "" {
		t.Fatal("missing cookie accepted")
	}
	r.AddCookie(&http.Cookie{Name: referralCookie, Value: strings.Repeat("a", 64)})
	hash := guestHash(r)
	if len(hash) != 64 || hash == strings.Repeat("a", 64) {
		t.Fatal("cookie must be hashed")
	}
	w := httptest.NewRecorder()
	ClearReferralGuestCookie(w, r)
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].MaxAge != -1 || !c[0].HttpOnly || !c[0].Secure || c[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("consumed cookie must be cleared securely")
	}
	for _, token := range []string{"1", strings.Repeat("z", 64)} {
		q := httptest.NewRequest("GET", "/", nil)
		q.AddCookie(&http.Cookie{Name: referralCookie, Value: token})
		if guestHash(q) != "" {
			t.Fatal("malformed cookie accepted")
		}
	}
}

// These early exits deliberately use a nil DB: rejected requests must not read
// or allocate visitor data, create interests, or trust client identifiers.
func TestReferralEarlyDenials(t *testing.T) {
	h := New(nil, nil, nil)
	for _, path := range []string{"visit", "interest"} {
		call := h.referralVisit
		if path == "interest" {
			call = h.courseInterest
		}
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		call(w, r)
		if w.Code != 405 {
			t.Fatal("method accepted")
		}
		r = httptest.NewRequest("POST", "https://fintalent.ru", strings.NewReader(`{}`))
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w = httptest.NewRecorder()
		call(w, r)
		if w.Code != 403 {
			t.Fatal("CSRF accepted")
		}
		r = httptest.NewRequest("POST", "/", strings.NewReader(`{"slug":"course","code":"telegram"}`))
		r.Header.Set("User-Agent", "TelegramBot")
		w = httptest.NewRecorder()
		call(w, r)
		if w.Code != 204 {
			t.Fatal("bot must not write")
		}
	}
}

func TestGuestInterestRequiresLoginBeforeDatabaseAccess(t *testing.T) {
	h := New(nil, func(*http.Request) (int64, error) { return 0, nil }, nil)
	r := httptest.NewRequest("POST", "/api/courses/interest", strings.NewReader(`{"slug":"course","user_id":123,"referral_link_id":456}`))
	w := httptest.NewRecorder()
	h.courseInterest(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("anonymous interest must be rejected before any database write")
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("guest interest must not allocate a guest session")
	}
}
