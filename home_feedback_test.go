package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedbackChallengeRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Now()
	token, a, b, err := newFeedbackChallenge(now)
	if err != nil {
		t.Fatal(err)
	}
	if !validFeedbackAnswer(token, a+b, now.Add(time.Minute)) {
		t.Fatal("fresh correct answer rejected")
	}
	if validFeedbackAnswer(token, a+b+1, now) {
		t.Fatal("wrong answer accepted")
	}
	if validFeedbackAnswer(token, a+b, now.Add(11*time.Minute)) {
		t.Fatal("expired challenge accepted")
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	data[10] ^= 1
	if validFeedbackAnswer(base64.RawURLEncoding.EncodeToString(data), a+b, now) {
		t.Fatal("modified challenge accepted")
	}
}

func TestFeedbackRejectsInvalidChallengeWithoutQueuing(t *testing.T) {
	body, err := json.Marshal(map[string]any{
		"email": "visitor@example.com", "message": "Нужна помощь с регистрацией",
		"challenge": "invalid", "answer": 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/public/feedback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	homeFeedback(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
}

func TestFeedbackHoneypotDoesNotQueue(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/public/feedback", strings.NewReader(`{"website":"spam.example"}`))
	w := httptest.NewRecorder()
	homeFeedback(w, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
	}
}

func TestFeedbackRateLimit(t *testing.T) {
	handler := requestSecurity(http.HandlerFunc(homeFeedback))
	for i := 0; i < 4; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/public/feedback", strings.NewReader(`{"challenge":"invalid"}`))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		want := http.StatusBadRequest
		if i == 3 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("request %d: got %d, want %d", i+1, w.Code, want)
		}
	}
}
