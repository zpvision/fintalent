package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"FinTalent/internal/vacancymodule/dto"
	"FinTalent/internal/vacancymodule/repository"
	"FinTalent/internal/vacancymodule/service"
)

type UserResolver func(*http.Request) (int64, error)
type Handler struct {
	service *service.Service
	user    UserResolver
}

func New(s *service.Service, u UserResolver) *Handler { return &Handler{service: s, user: u} }
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vacancy-builder", h.builder)
	mux.HandleFunc("/api/v1/vacancies", h.vacancies)
	mux.HandleFunc("/api/v1/vacancies/match-preview", h.preview)
	mux.HandleFunc("/api/v1/vacancies/", h.vacancy)
	mux.HandleFunc("/api/v1/resumes/draft", h.resumeDraft)
}

func userID(h *Handler, w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := h.user(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "требуется авторизация")
		return 0, false
	}
	return id, true
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		respondError(w, http.StatusBadRequest, "некорректные данные")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func respondError(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		respondError(w, http.StatusNotFound, "вакансия не найдена")
	case errors.Is(err, repository.ErrForbidden):
		respondError(w, http.StatusForbidden, "недостаточно прав")
	default:
		respondError(w, http.StatusBadRequest, err.Error())
	}
}

func (h *Handler) builder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, 405, "метод не поддерживается")
		return
	}
	if _, ok := userID(h, w, r); !ok {
		return
	}
	blocks, err := h.service.Builder(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"blocks": blocks, "importance_options": []map[string]string{{"value": "required", "label": "Обязательно"}, {"value": "preferred", "label": "Желательно"}, {"value": "bonus", "label": "Будет преимуществом"}}})
}
func (h *Handler) vacancies(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.service.List(r.Context(), uid)
		if err != nil {
			handleError(w, err)
			return
		}
		respond(w, 200, items)
	case http.MethodPost:
		v, err := h.service.Create(r.Context(), uid)
		if err != nil {
			handleError(w, err)
			return
		}
		respond(w, 201, v)
	default:
		respondError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) vacancy(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/vacancies/"), "/")
	parts := strings.Split(tail, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		respondError(w, 400, "некорректный id")
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if action != "" {
		if action == "duties" {
			switch r.Method {
			case http.MethodGet:
				ids, dutyErr := h.service.VacancyDuties(r.Context(), id, uid)
				if dutyErr != nil {
					handleError(w, dutyErr)
					return
				}
				respond(w, 200, map[string][]int64{"duty_ids": ids})
			case http.MethodPut:
				var input struct {
					DutyIDs []int64 `json:"duty_ids"`
				}
				if !decode(w, r, &input) {
					return
				}
				if dutyErr := h.service.SaveVacancyDuties(r.Context(), id, uid, input.DutyIDs); dutyErr != nil {
					handleError(w, dutyErr)
					return
				}
				respond(w, 200, map[string]string{"message": "обязанности сохранены"})
			default:
				respondError(w, 405, "метод не поддерживается")
			}
			return
		}
		if r.Method != http.MethodPost {
			respondError(w, 405, "метод не поддерживается")
			return
		}
		switch action {
		case "publish":
			err = h.service.Publish(r.Context(), id, uid)
		case "unpublish":
			err = h.service.Unpublish(r.Context(), id, uid)
		case "archive":
			err = h.service.Archive(r.Context(), id, uid)
		default:
			respondError(w, 404, "действие не найдено")
			return
		}
		if err != nil {
			handleError(w, err)
			return
		}
		respond(w, 200, map[string]string{"message": "статус изменён"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		v, e := h.service.Get(r.Context(), id, uid)
		if e != nil {
			handleError(w, e)
			return
		}
		respond(w, 200, v)
	case http.MethodPut:
		var input dto.VacancyDraft
		if !decode(w, r, &input) {
			return
		}
		v, e := h.service.Update(r.Context(), id, uid, input)
		if e != nil {
			handleError(w, e)
			return
		}
		respond(w, 200, v)
	case http.MethodDelete:
		if e := h.service.Delete(r.Context(), id, uid); e != nil {
			handleError(w, e)
			return
		}
		respond(w, 200, map[string]string{"message": "вакансия удалена"})
	default:
		respondError(w, 405, "метод не поддерживается")
	}
}
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, 405, "метод не поддерживается")
		return
	}
	if _, ok := userID(h, w, r); !ok {
		return
	}
	var input dto.MatchPreview
	if !decode(w, r, &input) {
		return
	}
	result, err := h.service.Preview(r.Context(), input)
	if err != nil {
		handleError(w, err)
		return
	}
	respond(w, 200, result)
}
func (h *Handler) resumeDraft(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(h, w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		draft, err := h.service.LoadResume(r.Context(), uid)
		if err != nil {
			handleError(w, err)
			return
		}
		respond(w, 200, draft)
	case http.MethodPut:
		var input dto.ResumeDraft
		if !decode(w, r, &input) {
			return
		}
		if err := h.service.SaveResume(r.Context(), uid, input); err != nil {
			handleError(w, err)
			return
		}
		respond(w, 200, map[string]string{"message": "резюме сохранено"})
	default:
		respondError(w, 405, "метод не поддерживается")
	}
}
