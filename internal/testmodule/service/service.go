package service

import (
	"context"
	"errors"
	"fmt"
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
	if !admin && base.AuthorBlocked {
		return nil, repository.ErrNotFound
	}
	showCorrect := admin || base.AuthorID == user
	if !showCorrect && (base.Status != domain.StatusPublished || base.Visibility == domain.VisibilityPrivate) {
		return nil, repository.ErrForbidden
	}
	test, err := s.repo.Get(ctx, id, showCorrect)
	if err == nil && !showCorrect {
		domain.HideQuestionSolutions(test.Questions)
	}
	return test, err
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
func (s *Service) Start(ctx context.Context, test, user, vacancy int64) (*domain.Attempt, error) {
	a, err := s.repo.StartAttempt(ctx, test, user, vacancy)
	if err != nil {
		return nil, err
	}
	return s.Attempt(ctx, a.ID, user, false)
}
func (s *Service) SaveAnswer(ctx context.Context, attempt, user int64, in dto.SubmitAnswer) error {
	a, err := s.repo.GetAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	if a.UserID != user || a.Status != "started" || a.EmployeeAttempt || (a.TimeLimitSeconds > 0 && a.RemainingSeconds <= 0) {
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
		domain.HideQuestionSolutions(a.Questions)
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
	if attempt.UserID != user || attempt.Status != "started" || attempt.EmployeeAttempt {
		return nil, repository.ErrForbidden
	}
	test, err := s.repo.GetVersion(ctx, attempt.TestID, attempt.TestVersionID, true)
	if err != nil {
		return nil, err
	}
	passing := test.PassingPercent
	if attempt.PassingPercent != nil {
		passing = *attempt.PassingPercent
	}
	result := domain.Grade(test.Questions, attempt.Answers, passing)
	if err = s.repo.FinishAttempt(ctx, id, attempt.AnswerRevision, result.Score, result.MaxScore, result.Percent, result.Passed, result.Answers); err != nil {
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
func (s *Service) SetAttemptResumeVisibility(ctx context.Context, attempt, user int64, visible bool) error {
	return s.repo.SetAttemptResumeVisibility(ctx, attempt, user, visible)
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
