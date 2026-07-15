package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
)

type UserResolver func(*http.Request) (int64, error)
type AdminResolver func(*http.Request) bool
type Handler struct {
	service *service.Service
	user    UserResolver
	admin   AdminResolver
}

func New(s *service.Service, u UserResolver, a AdminResolver) *Handler {
	return &Handler{service: s, user: u, admin: a}
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/tests", h.tests)
	mux.HandleFunc("/api/tests/", h.testRoutes)
	mux.HandleFunc("/api/questions/", h.questionRoutes)
	mux.HandleFunc("/api/answers/", h.answerRoutes)
	mux.HandleFunc("/api/attempts/", h.attemptRoutes)
	mux.HandleFunc("/api/me/test-results", h.myResults)
	mux.HandleFunc("/api/admin/tests", h.adminTests)
	mux.HandleFunc("/api/admin/tests/", h.adminTestRoutes)
	mux.HandleFunc("/api/admin/attempts/", h.adminAttemptRoutes)
}

func (h *Handler) adminAttemptRoutes(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		writeError(w, http.StatusUnauthorized, "требуется вход в админку")
		return
	}
	id, _, err := idPart(r.URL.Path, "/api/admin/attempts/")
	if err != nil || r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "некорректный запрос")
		return
	}
	v, err := h.service.Attempt(r.Context(), id, 0, true)
	if err != nil {
		handleErr(w, err)
		return
	}
	respond(w, http.StatusOK, v)
}
func userID(h *Handler, w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := h.user(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "требуется авторизация")
		return 0, false
	}
	return id, true
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func handleErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, 404, "объект не найден")
	case errors.Is(err, repository.ErrForbidden):
		writeError(w, 403, "недостаточно прав")
	default:
		writeError(w, 400, err.Error())
	}
}
func idPart(path, prefix string) (int64, string, error) {
	tail := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(tail, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	rest := ""
	if len(parts) > 1 {
		rest = strings.Join(parts[1:], "/")
	}
	return id, rest, err
}
func filter(r *http.Request) dto.ListFilter {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return dto.ListFilter{Scope: r.URL.Query().Get("scope"), Status: r.URL.Query().Get("status"), Author: r.URL.Query().Get("author"), Category: r.URL.Query().Get("category"), Price: r.URL.Query().Get("price"), Search: r.URL.Query().Get("q"), Limit: limit, Offset: offset}
}

func (h *Handler) tests(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		v, e := h.service.List(r.Context(), filter(r), uid, false)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
	case http.MethodPost:
		var in dto.CreateTest
		if !decode(w, r, &in) {
			return
		}
		v, e := h.service.Create(r.Context(), uid, in)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 201, v)
	default:
		writeError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) testRoutes(w http.ResponseWriter, r *http.Request) {
	id, action, err := idPart(r.URL.Path, "/api/tests/")
	if err != nil {
		writeError(w, 400, "некорректный id")
		return
	}
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	if action == "questions" && r.Method == http.MethodPost {
		var in dto.CreateQuestion
		if !decode(w, r, &in) {
			return
		}
		qid, e := h.service.AddQuestion(r.Context(), id, uid, in)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 201, map[string]int64{"id": qid})
		return
	}
	if action == "publish" && r.Method == http.MethodPost {
		if e := h.service.Publish(r.Context(), id, uid); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "тест опубликован"})
		return
	}
	if action == "draft-version" && r.Method == http.MethodPost {
		if e := h.service.ForkDraft(r.Context(), id, uid); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, http.StatusOK, map[string]string{"message": "создана новая черновая версия"})
		return
	}
	if action == "attempts" {
		if r.Method == http.MethodPost {
			v, e := h.service.Start(r.Context(), id, uid)
			if e != nil {
				handleErr(w, e)
				return
			}
			respond(w, 201, v)
		} else {
			v, e := h.service.Attempts(r.Context(), id, uid, false)
			if e != nil {
				handleErr(w, e)
				return
			}
			respond(w, 200, v)
		}
		return
	}
	if action == "statistics" && r.Method == http.MethodGet {
		v, e := h.service.Statistics(r.Context(), id)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	switch r.Method {
	case http.MethodGet:
		v, e := h.service.Get(r.Context(), id, uid, false)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
	case http.MethodPut:
		var in dto.UpdateTest
		if !decode(w, r, &in) {
			return
		}
		if e := h.service.Update(r.Context(), id, uid, in); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "сохранено"})
	case http.MethodDelete:
		if e := h.service.Delete(r.Context(), id, uid, false); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "удалено"})
	default:
		writeError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) questionRoutes(w http.ResponseWriter, r *http.Request) {
	id, action, err := idPart(r.URL.Path, "/api/questions/")
	if err != nil {
		writeError(w, 400, "некорректный id")
		return
	}
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	if action == "answers" && r.Method == http.MethodPost {
		var in dto.AnswerInput
		if !decode(w, r, &in) {
			return
		}
		aid, e := h.service.AddAnswer(r.Context(), id, uid, in)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 201, map[string]int64{"id": aid})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var in dto.CreateQuestion
		if !decode(w, r, &in) {
			return
		}
		if e := h.service.UpdateQuestion(r.Context(), id, uid, in); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "сохранено"})
	case http.MethodDelete:
		if e := h.service.DeleteQuestion(r.Context(), id, uid); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "удалено"})
	default:
		writeError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) answerRoutes(w http.ResponseWriter, r *http.Request) {
	id, _, err := idPart(r.URL.Path, "/api/answers/")
	if err != nil {
		writeError(w, 400, "некорректный id")
		return
	}
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var in dto.AnswerInput
		if !decode(w, r, &in) {
			return
		}
		if e := h.service.UpdateAnswer(r.Context(), id, uid, in); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "сохранено"})
	case http.MethodDelete:
		if e := h.service.DeleteAnswer(r.Context(), id, uid); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "удалено"})
	default:
		writeError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) attemptRoutes(w http.ResponseWriter, r *http.Request) {
	id, action, err := idPart(r.URL.Path, "/api/attempts/")
	if err != nil {
		writeError(w, 400, "некорректный id")
		return
	}
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	if action == "answers" && r.Method == http.MethodPost {
		var in dto.SubmitAnswer
		if !decode(w, r, &in) {
			return
		}
		if e := h.service.SaveAnswer(r.Context(), id, uid, in); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "ответ сохранён"})
		return
	}
	if action == "finish" && r.Method == http.MethodPost {
		v, e := h.service.Finish(r.Context(), id, uid)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	if r.Method == http.MethodGet {
		v, e := h.service.Attempt(r.Context(), id, uid, false)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	writeError(w, 405, "метод не поддерживается")
}
func (h *Handler) myResults(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	v, e := h.service.Attempts(r.Context(), 0, uid, false)
	if e != nil {
		handleErr(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Handler) adminTests(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		writeError(w, 401, "требуется вход в админку")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, 405, "метод не поддерживается")
		return
	}
	v, e := h.service.List(r.Context(), filter(r), 0, true)
	if e != nil {
		handleErr(w, e)
		return
	}
	respond(w, 200, v)
}
func (h *Handler) adminTestRoutes(w http.ResponseWriter, r *http.Request) {
	if !h.admin(r) {
		writeError(w, 401, "требуется вход в админку")
		return
	}
	id, action, err := idPart(r.URL.Path, "/api/admin/tests/")
	if err != nil {
		writeError(w, 400, "некорректный id")
		return
	}
	if action == "attempts" {
		v, e := h.service.Attempts(r.Context(), id, 0, true)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	if action == "statistics" {
		v, e := h.service.Statistics(r.Context(), id)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	if action == "moderate" && r.Method == http.MethodPost {
		var in dto.ModerateTest
		if !decode(w, r, &in) {
			return
		}
		if e := h.service.Moderate(r.Context(), id, in); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "статус изменён"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		v, e := h.service.Get(r.Context(), id, 0, true)
		if e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, v)
	case http.MethodDelete:
		if e := h.service.Delete(r.Context(), id, 0, true); e != nil {
			handleErr(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "удалено"})
	default:
		writeError(w, 405, "метод не поддерживается")
	}
}
