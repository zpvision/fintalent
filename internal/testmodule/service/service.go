package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/validation"
)

type Service struct{ repo repository.Repository }

func New(repo repository.Repository) *Service { return &Service{repo: repo} }
func (s *Service) List(ctx context.Context, f dto.ListFilter, user int64, admin bool) ([]domain.Test, error) {
	return s.repo.List(ctx, f, user, admin)
}
func (s *Service) Get(ctx context.Context, id, user int64, admin bool) (*domain.Test, error) {
	base, err := s.repo.Get(ctx, id, false)
	if err != nil {
		return nil, err
	}
	showCorrect := admin || base.AuthorID == user
	if base.Status != domain.StatusPublished && !showCorrect {
		return nil, repository.ErrForbidden
	}
	return s.repo.Get(ctx, id, showCorrect)
}
func (s *Service) Create(ctx context.Context, user int64, in dto.CreateTest) (*domain.Test, error) {
	defaults(&in)
	if err := validation.Test(in); err != nil {
		return nil, err
	}
	slug := fmt.Sprintf("test-%d-%d", user, time.Now().UnixNano())
	return s.repo.Create(ctx, user, in, slug)
}
func (s *Service) Update(ctx context.Context, id, user int64, in dto.UpdateTest) error {
	defaults(&in)
	if err := validation.Test(in); err != nil {
		return err
	}
	return s.repo.Update(ctx, id, user, in)
}
func (s *Service) Delete(ctx context.Context, id, user int64, admin bool) error {
	return s.repo.SoftDelete(ctx, id, user, admin)
}
func (s *Service) Moderate(ctx context.Context, id int64, in dto.ModerateTest) error {
	return s.repo.Moderate(ctx, id, in.Action, strings.TrimSpace(in.Reason))
}
func (s *Service) AddQuestion(ctx context.Context, test, user int64, in dto.CreateQuestion) (int64, error) {
	if err := validation.Question(in); err != nil {
		return 0, err
	}
	return s.repo.AddQuestion(ctx, test, user, in)
}
func (s *Service) UpdateQuestion(ctx context.Context, id, user int64, in dto.CreateQuestion) error {
	if err := validation.Question(in); err != nil {
		return err
	}
	return s.repo.UpdateQuestion(ctx, id, user, in)
}
func (s *Service) DeleteQuestion(ctx context.Context, id, user int64) error {
	return s.repo.DeleteQuestion(ctx, id, user)
}
func (s *Service) AddAnswer(ctx context.Context, q, user int64, in dto.AnswerInput) (int64, error) {
	if strings.TrimSpace(in.Answer) == "" {
		return 0, errors.New("ответ не может быть пустым")
	}
	return s.repo.AddAnswer(ctx, q, user, in)
}
func (s *Service) UpdateAnswer(ctx context.Context, id, user int64, in dto.AnswerInput) error {
	if strings.TrimSpace(in.Answer) == "" {
		return errors.New("ответ не может быть пустым")
	}
	return s.repo.UpdateAnswer(ctx, id, user, in)
}
func (s *Service) DeleteAnswer(ctx context.Context, id, user int64) error {
	return s.repo.DeleteAnswer(ctx, id, user)
}
func (s *Service) Publish(ctx context.Context, id, user int64) error {
	return s.repo.Publish(ctx, id, user)
}
func (s *Service) ForkDraft(ctx context.Context, id, user int64) error {
	return s.repo.ForkDraft(ctx, id, user)
}
func (s *Service) Start(ctx context.Context, test, user int64) (*domain.Attempt, error) {
	return s.repo.StartAttempt(ctx, test, user)
}
func (s *Service) SaveAnswer(ctx context.Context, attempt, user int64, in dto.SubmitAnswer) error {
	a, err := s.repo.GetAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	if a.UserID != user || a.Status != "started" {
		return repository.ErrForbidden
	}
	return s.repo.SaveAttemptAnswer(ctx, attempt, in)
}
func (s *Service) Attempt(ctx context.Context, id, user int64, admin bool) (*domain.Attempt, error) {
	a, err := s.repo.GetAttempt(ctx, id)
	if err != nil {
		return nil, err
	}
	if !admin && a.UserID != user {
		return nil, repository.ErrForbidden
	}
	if !admin && a.Status != "finished" {
		for i := range a.Answers {
			a.Answers[i].CorrectAnswer = ""
			a.Answers[i].IsCorrect = nil
			a.Answers[i].EarnedPoints = 0
		}
		for i := range a.Questions {
			for j := range a.Questions[i].Answers {
				a.Questions[i].Answers[j].IsCorrect = false
			}
		}
	}
	return a, nil
}
func (s *Service) Finish(ctx context.Context, id, user int64) (*domain.Attempt, error) {
	attempt, err := s.repo.GetAttempt(ctx, id)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != user || attempt.Status != "started" {
		return nil, repository.ErrForbidden
	}
	test, err := s.repo.GetVersion(ctx, attempt.TestID, attempt.TestVersionID, true)
	if err != nil {
		return nil, err
	}
	byQuestion := map[int64][]domain.AttemptAnswer{}
	for _, a := range attempt.Answers {
		byQuestion[a.QuestionID] = append(byQuestion[a.QuestionID], a)
	}
	score, max := 0.0, 0.0
	grades := []domain.AttemptAnswer{}
	for _, q := range test.Questions {
		max += q.Points
		submitted := byQuestion[q.ID]
		correct := false
		switch q.Type {
		case domain.QuestionText:
			expected := ""
			for _, a := range q.Answers {
				if a.IsCorrect {
					expected = strings.TrimSpace(strings.ToLower(a.Answer))
					break
				}
			}
			actual := ""
			if len(submitted) > 0 {
				actual = strings.TrimSpace(strings.ToLower(submitted[0].TextAnswer))
			}
			correct = expected != "" && actual == expected
		default:
			expected := map[int64]bool{}
			for _, a := range q.Answers {
				if a.IsCorrect {
					expected[a.ID] = true
				}
			}
			selected := map[int64]bool{}
			for _, a := range submitted {
				if a.SelectedAnswerID != nil {
					selected[*a.SelectedAnswerID] = true
				}
			}
			correct = len(expected) == len(selected) && len(expected) > 0
			for x := range selected {
				if !expected[x] {
					correct = false
				}
			}
		}
		earned := 0.0
		if correct {
			earned = q.Points
			score += earned
		}
		c := correct
		grades = append(grades, domain.AttemptAnswer{QuestionID: q.ID, IsCorrect: &c, EarnedPoints: earned})
	}
	percent := 0.0
	if max > 0 {
		percent = math.Round(score/max*10000) / 100
	}
	passed := percent >= test.PassingPercent
	if err = s.repo.FinishAttempt(ctx, id, score, max, percent, passed, grades); err != nil {
		return nil, err
	}
	return s.repo.GetAttempt(ctx, id)
}
func (s *Service) Attempts(ctx context.Context, test, user int64, admin bool) ([]domain.Attempt, error) {
	if test > 0 && !admin {
		t, err := s.repo.Get(ctx, test, false)
		if err != nil {
			return nil, err
		}
		if t.AuthorID != user {
			return nil, repository.ErrForbidden
		}
		return s.repo.ListAttempts(ctx, test, 0, true)
	}
	return s.repo.ListAttempts(ctx, test, user, admin)
}
func (s *Service) Statistics(ctx context.Context, test int64) (*domain.Statistics, error) {
	return s.repo.Statistics(ctx, test)
}
func defaults(in *dto.CreateTest) {
	if in.Difficulty == "" {
		in.Difficulty = "medium"
	}
	if in.Visibility == "" {
		in.Visibility = "private"
	}
	if in.PassingPercent == 0 {
		in.PassingPercent = 60
	}
	if in.IsFree {
		in.Price = 0
	}
}
