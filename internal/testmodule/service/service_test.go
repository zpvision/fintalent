package service

import (
	"context"
	"testing"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/repository"
)

type startRepository struct {
	repository.Repository
	attempt domain.Attempt
}

func (r *startRepository) StartAttempt(context.Context, int64, int64) (*domain.Attempt, error) {
	return &domain.Attempt{ID: r.attempt.ID}, nil
}

func (r *startRepository) GetAttempt(context.Context, int64) (*domain.Attempt, error) {
	return &r.attempt, nil
}

func TestStartReturnsAttemptQuestionsWithoutCorrectAnswers(t *testing.T) {
	r := &startRepository{attempt: domain.Attempt{ID: 42, UserID: 7, TestVersionID: 3, Status: "started", ShuffleAnswers: true,
		Questions: []domain.Question{{ID: 11, Answers: []domain.Answer{{ID: 2}, {ID: 1, IsCorrect: true}}}},
	}}
	a, err := New(r).Start(context.Background(), 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !a.ShuffleAnswers || a.TestVersionID != 3 || len(a.Questions) != 1 || a.Questions[0].Answers[0].ID != 2 {
		t.Fatal("attempt version or answer order was not preserved")
	}
	for _, answer := range a.Questions[0].Answers {
		if answer.IsCorrect {
			t.Fatal("start response exposes correct answers")
		}
	}
}
