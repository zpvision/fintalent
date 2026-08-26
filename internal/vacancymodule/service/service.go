package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"FinTalent/internal/vacancymodule/domain"
	"FinTalent/internal/vacancymodule/dto"
	"FinTalent/internal/vacancymodule/matching"
)

type Repository interface {
	Builder(context.Context) ([]domain.BuilderBlock, error)
	Metadata(context.Context, [][2]int64) (map[[2]int64]domain.CategoryMetadata, error)
	Create(context.Context, int64) (int64, error)
	List(context.Context, int64) ([]*domain.Vacancy, error)
	Get(context.Context, int64, int64) (*domain.Vacancy, error)
	Save(context.Context, *domain.Vacancy, []domain.Requirement, string) error
	SetStatus(context.Context, int64, int64, string) error
	Delete(context.Context, int64, int64) error
	Preview(context.Context, []domain.Requirement) (*domain.PreviewResult, error)
	SaveResume(context.Context, int64, int, []int64) error
	LoadResume(context.Context, int64) (int, []int64, error)
	SaveVacancyDuties(context.Context, int64, []int64) error
	VacancyDuties(context.Context, int64) ([]int64, error)
}

type cacheEntry struct {
	result  *domain.PreviewResult
	expires time.Time
}
type Service struct {
	repo  Repository
	mu    sync.Mutex
	cache map[string]cacheEntry
}

func New(repo Repository) *Service { return &Service{repo: repo, cache: map[string]cacheEntry{}} }

func (s *Service) Builder(ctx context.Context) ([]domain.BuilderBlock, error) {
	return s.repo.Builder(ctx)
}
func (s *Service) Create(ctx context.Context, user int64) (*domain.Vacancy, error) {
	id, err := s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id, user)
}
func (s *Service) Get(ctx context.Context, id, user int64) (*domain.Vacancy, error) {
	return s.repo.Get(ctx, id, user)
}
func (s *Service) List(ctx context.Context, user int64) ([]*domain.Vacancy, error) {
	return s.repo.List(ctx, user)
}

func (s *Service) Update(ctx context.Context, id, user int64, input dto.VacancyDraft) (*domain.Vacancy, error) {
	v, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return nil, err
	}
	if v.Status != "draft" && v.Status != "published" {
		return nil, errors.New("вакансия недоступна для редактирования")
	}
	if err = validateDraft(input); err != nil {
		return nil, err
	}
	requirements, hash, err := s.NormalizeRequirements(ctx, input.Requirements)
	if err != nil {
		return nil, err
	}
	v.Title = strings.TrimSpace(input.Title)
	v.Description = strings.TrimSpace(input.Description)
	v.SalaryFrom = input.SalaryFrom
	v.SalaryTo = input.SalaryTo
	v.SalaryTaxMode = strings.TrimSpace(input.SalaryTaxMode)
	if v.SalaryTaxMode == "" {
		v.SalaryTaxMode = "net"
	}
	v.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if v.Currency == "" {
		v.Currency = "RUB"
	}
	v.EmploymentType = strings.TrimSpace(input.EmploymentType)
	v.WorkFormat = strings.TrimSpace(input.WorkFormat)
	v.City = strings.TrimSpace(input.City)
	v.Address = strings.TrimSpace(input.Address)
	v.AcceptsIndividualEntrepreneur = input.AcceptsIndividualEntrepreneur
	v.AcceptsSelfEmployed = input.AcceptsSelfEmployed
	v.ExperienceFrom = input.ExperienceFrom
	v.ExperienceTo = input.ExperienceTo
	v.CurrentStep = input.CurrentStep
	v.SelectedTestIDs = uniquePositiveIDs(input.SelectedTestIDs)
	if len(v.SelectedTestIDs) == 0 && input.SelectedTestID != nil && *input.SelectedTestID > 0 {
		v.SelectedTestIDs = []int64{*input.SelectedTestID}
	}
	if len(v.SelectedTestIDs) > 20 {
		return nil, errors.New("можно выбрать не более 20 тестов")
	}
	v.SelectedTestID = nil
	if len(v.SelectedTestIDs) > 0 {
		v.SelectedTestID = &v.SelectedTestIDs[0]
	}
	if err = s.repo.Save(ctx, v, requirements, hash); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id, user)
}

func uniquePositiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func validateDraft(input dto.VacancyDraft) error {
	if input.CurrentStep < 1 {
		return errors.New("некорректный шаг")
	}
	if len([]rune(input.Title)) > 240 {
		return errors.New("слишком длинное название")
	}
	if input.SalaryFrom != nil && *input.SalaryFrom < 0 || input.SalaryTo != nil && *input.SalaryTo < 0 {
		return errors.New("некорректная зарплата")
	}
	if input.SalaryFrom != nil && input.SalaryTo != nil && *input.SalaryFrom > *input.SalaryTo {
		return errors.New("минимальная зарплата больше максимальной")
	}
	if input.SalaryTaxMode != "" && input.SalaryTaxMode != "net" && input.SalaryTaxMode != "gross" {
		return errors.New("некорректный способ указания зарплаты")
	}
	if len([]rune(input.Address)) > 500 {
		return errors.New("слишком длинный адрес")
	}
	if input.ExperienceFrom != nil && input.ExperienceTo != nil && *input.ExperienceFrom > *input.ExperienceTo {
		return errors.New("некорректный диапазон опыта")
	}
	return nil
}

func (s *Service) NormalizeRequirements(ctx context.Context, input []dto.RequirementInput) ([]domain.Requirement, string, error) {
	if len(input) > 500 {
		return nil, "", errors.New("слишком много требований")
	}
	pairs := make([][2]int64, 0, len(input))
	filtered := make([]dto.RequirementInput, 0, len(input))
	seen := map[int64]bool{}
	for _, r := range input {
		if r.CategoryID <= 0 || r.BlockID <= 0 || seen[r.CategoryID] {
			continue
		}
		seen[r.CategoryID] = true
		pairs = append(pairs, [2]int64{r.CategoryID, r.BlockID})
		filtered = append(filtered, r)
	}
	metadata, err := s.repo.Metadata(ctx, pairs)
	if err != nil {
		return nil, "", err
	}
	out := make([]domain.Requirement, 0, len(filtered))
	singleChoiceSeen := map[int64]bool{}
	for order, r := range filtered {
		m, ok := metadata[[2]int64{r.CategoryID, r.BlockID}]
		if !ok || !m.Active || r.DictionaryID != 0 && r.DictionaryID != m.DictionaryID {
			continue
		}
		if m.SingleChoice {
			if singleChoiceSeen[m.DictionaryID] {
				continue
			}
			singleChoiceSeen[m.DictionaryID] = true
		}
		importance := r.Importance
		if !m.UseImportanceInVacancy || importance == "" {
			importance = domain.ImportanceRequired
		}
		_, ok = domain.ImportanceCoefficients[importance]
		if !ok {
			return nil, "", errors.New("некорректная важность")
		}
		sortOrder := r.SortOrder
		if sortOrder < 0 {
			sortOrder = order
		}
		out = append(out, domain.Requirement{CategoryID: r.CategoryID, BlockID: r.BlockID, DictionaryID: m.DictionaryID, Importance: importance, SortOrder: sortOrder, CategoryName: m.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BlockID != out[j].BlockID {
			return out[i].BlockID < out[j].BlockID
		}
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].CategoryID < out[j].CategoryID
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	encoded, _ := json.Marshal(out)
	sum := sha256.Sum256(encoded)
	return out, hex.EncodeToString(sum[:]), nil
}

func (s *Service) Publish(ctx context.Context, id, user int64) error {
	v, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return err
	}
	if v.Status != "draft" {
		return errors.New("вакансия уже опубликована")
	}
	if len(v.Requirements) == 0 || len(v.SelectedTestIDs) == 0 {
		return errors.New("заполните требования и выберите тест")
	}
	duties, err := s.repo.VacancyDuties(ctx, id)
	if err != nil {
		return err
	}
	if len(duties) == 0 {
		return errors.New("выберите хотя бы одну обязанность")
	}
	return s.repo.SetStatus(ctx, id, user, "published")
}

func (s *Service) VacancyDuties(ctx context.Context, id, user int64) ([]int64, error) {
	v, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return nil, err
	}
	return v.DutyIDs, nil
}

func (s *Service) SaveVacancyDuties(ctx context.Context, id, user int64, ids []int64) error {
	v, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return err
	}
	if v.Status != "draft" && v.Status != "published" {
		return errors.New("вакансия недоступна для редактирования")
	}
	if len(ids) == 0 {
		return errors.New("выберите хотя бы одну обязанность")
	}
	seen := map[int64]bool{}
	for _, dutyID := range ids {
		if dutyID <= 0 || seen[dutyID] {
			return errors.New("обязанности не должны повторяться")
		}
		seen[dutyID] = true
	}
	return s.repo.SaveVacancyDuties(ctx, id, ids)
}
func (s *Service) Archive(ctx context.Context, id, user int64) error {
	return s.repo.SetStatus(ctx, id, user, "archived")
}
func (s *Service) Unpublish(ctx context.Context, id, user int64) error {
	v, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return err
	}
	if v.Status != "published" {
		return errors.New("вакансия уже снята с публикации")
	}
	return s.repo.SetStatus(ctx, id, user, "draft")
}
func (s *Service) Delete(ctx context.Context, id, user int64) error {
	return s.repo.Delete(ctx, id, user)
}

func (s *Service) Preview(ctx context.Context, input dto.MatchPreview) (*domain.PreviewResult, error) {
	requirements, hash, err := s.NormalizeRequirements(ctx, input.Requirements)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	entry, ok := s.cache[hash]
	if ok && time.Now().Before(entry.expires) {
		s.mu.Unlock()
		copy := *entry.result
		return &copy, nil
	}
	s.mu.Unlock()
	result, err := s.repo.Preview(ctx, requirements)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[hash] = cacheEntry{result: result, expires: time.Now().Add(60 * time.Second)}
	s.mu.Unlock()
	return result, nil
}
func (s *Service) CalculateResumeMatch(requirements []domain.Requirement, resumeCategoryIDs []int64) domain.MatchResult {
	return matching.Calculate(requirements, resumeCategoryIDs)
}
func (s *Service) SaveResume(ctx context.Context, user int64, input dto.ResumeDraft) error {
	if input.CurrentStep < 1 {
		return errors.New("некорректный шаг")
	}
	return s.repo.SaveResume(ctx, user, input.CurrentStep, uniquePositiveIDs(input.CategoryIDs))
}
func (s *Service) LoadResume(ctx context.Context, user int64) (dto.ResumeDraft, error) {
	step, ids, err := s.repo.LoadResume(ctx, user)
	return dto.ResumeDraft{CurrentStep: step, CategoryIDs: ids}, err
}
