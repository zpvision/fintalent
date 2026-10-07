package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var feedbackChallengeKey = func() [32]byte {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		panic(err)
	}
	return key
}()

func registerHomeFeedbackRoutes() {
	http.HandleFunc("/api/public/feedback/challenge", homeFeedbackChallenge)
	http.HandleFunc("/api/public/feedback", homeFeedback)
}

func feedbackNumbers(nonce []byte) (int, int) {
	mac := hmac.New(sha256.New, feedbackChallengeKey[:])
	mac.Write(nonce)
	sum := mac.Sum(nil)
	return int(sum[0]%9) + 1, int(sum[1]%9) + 1
}

func newFeedbackChallenge(now time.Time) (string, int, int, error) {
	var payload [24]byte
	binary.BigEndian.PutUint64(payload[:8], uint64(now.Unix()))
	if _, err := rand.Read(payload[8:]); err != nil {
		return "", 0, 0, err
	}
	mac := hmac.New(sha256.New, feedbackChallengeKey[:])
	mac.Write(payload[:])
	token := append(payload[:], mac.Sum(nil)...)
	a, b := feedbackNumbers(payload[8:])
	return base64.RawURLEncoding.EncodeToString(token), a, b, nil
}

func validFeedbackAnswer(token string, answer int, now time.Time) bool {
	if len(token) > 100 {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(data) != 56 {
		return false
	}
	issued := time.Unix(int64(binary.BigEndian.Uint64(data[:8])), 0)
	if issued.After(now) || now.Sub(issued) > 10*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, feedbackChallengeKey[:])
	mac.Write(data[:24])
	if !hmac.Equal(data[24:], mac.Sum(nil)) {
		return false
	}
	a, b := feedbackNumbers(data[8:24])
	return answer == a+b
}

func homeFeedbackChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		jsonError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	token, a, b, err := newFeedbackChallenge(time.Now())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось загрузить проверку")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"token": token, "question": "Сколько будет " + strconv.Itoa(a) + " + " + strconv.Itoa(b) + "?"})
}

func homeFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		jsonError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<10)
	defer r.Body.Close()
	var in struct {
		Email     string `json:"email"`
		Message   string `json:"message"`
		Website   string `json:"website"`
		Challenge string `json:"challenge"`
		Answer    int    `json:"answer"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&in); err != nil {
		jsonError(w, http.StatusBadRequest, "Некорректные данные")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		jsonError(w, http.StatusBadRequest, "Некорректные данные")
		return
	}
	if in.Website != "" {
		jsonResponse(w, http.StatusAccepted, map[string]string{"message": "Вопрос принят"})
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Message = strings.TrimSpace(in.Message)
	address, err := mail.ParseAddress(in.Email)
	if err != nil || address.Address != in.Email || len(in.Email) > 254 {
		jsonError(w, http.StatusBadRequest, "Укажите корректный адрес почты")
		return
	}
	if length := utf8.RuneCountInString(in.Message); length < 10 || length > 4000 || strings.ContainsRune(in.Message, 0) {
		jsonError(w, http.StatusBadRequest, "Сообщение должно содержать от 10 до 4000 символов")
		return
	}
	if !validFeedbackAnswer(in.Challenge, in.Answer, time.Now()) {
		jsonError(w, http.StatusBadRequest, "Проверьте ответ на вопрос")
		return
	}
	data := eventNotificationEmailData{
		Badge: "ОБРАТНАЯ СВЯЗЬ", Title: "Новый вопрос с сайта",
		Intro:     "Посетитель сайта отправил вопрос. Адрес для ответа указан ниже.",
		CardLabel: "Адрес для ответа", CardTitle: in.Email, Details: in.Message,
		ButtonText: "Открыть FinTalent", ButtonURL: applicationBaseURL(),
		Footer: "Ответьте отправителю по указанному адресу почты.",
	}
	if err := enqueueNotification(r.Context(), db, "event", "Команда FinTalent", "info@fintalent.ru", "Вопрос с сайта — FinTalent", data, ""); err != nil {
		jsonError(w, http.StatusServiceUnavailable, "Не удалось отправить вопрос. Попробуйте позже")
		return
	}
	jsonResponse(w, http.StatusAccepted, map[string]string{"message": "Вопрос принят"})
}
