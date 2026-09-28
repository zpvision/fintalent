package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
	"FinTalent/internal/testmodule/repository"
	"FinTalent/internal/testmodule/service"
)

func testReleaseQuestionTypes(t *testing.T, ctx context.Context) {
	var owner, reader int64
	for i, dest := range []*int64{&owner, &reader} {
		email := []string{"types-owner@example.invalid", "types-reader@example.invalid"}[i]
		if err := db.QueryRowContext(ctx, `INSERT INTO users(full_name,email,password_hash) VALUES('Types fixture',$1,repeat('*',60)) RETURNING id`, email).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.New(db)
	svc := service.New(repo)
	test, err := svc.Create(ctx, owner, dto.CreateTest{Title: "All question types", Visibility: "public", IsFree: true, PassingPercent: 60})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{domain.QuestionSingle, domain.QuestionMultiple, domain.QuestionBoolean, domain.QuestionText} {
		answers := []dto.AnswerInput{{Answer: "Yes", IsCorrect: true}, {Answer: "No"}}
		if kind == domain.QuestionMultiple {
			answers = append(answers, dto.AnswerInput{Answer: "Also yes", IsCorrect: true})
		}
		if kind == domain.QuestionText {
			answers = []dto.AnswerInput{{Answer: "SECRET_REFERENCE", IsCorrect: true}}
		}
		if _, err := svc.AddQuestion(ctx, test.ID, owner, dto.CreateQuestion{Question: kind, QuestionType: kind, Explanation: "SECRET_EXPLANATION", Points: 1, Answers: answers}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Publish(ctx, test.ID, owner); err != nil {
		t.Fatal(err)
	}
	authorView, err := svc.Get(ctx, test.ID, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	assertSafe := func(v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"SECRET_REFERENCE", "SECRET_EXPLANATION", `"is_correct":true`} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("participant secret: %s", secret)
			}
		}
	}
	public, err := svc.Get(ctx, test.ID, reader, false)
	if err != nil {
		t.Fatal(err)
	}
	assertSafe(public)
	attempt, err := svc.Start(ctx, test.ID, reader, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertSafe(attempt)
	for _, q := range authorView.Questions {
		good := dto.SubmitAnswer{QuestionID: q.ID}
		for _, a := range q.Answers {
			if a.IsCorrect {
				if q.Type == domain.QuestionText {
					good.TextAnswer = a.Answer
				} else {
					good.SelectedAnswerIDs = append(good.SelectedAnswerIDs, a.ID)
				}
			}
		}
		if err := svc.SaveAnswer(ctx, attempt.ID, reader, good); err != nil {
			t.Fatalf("valid %s: %v", q.Type, err)
		}
		bad := dto.SubmitAnswer{QuestionID: q.ID, SelectedAnswerIDs: []int64{999999999}}
		if err := svc.SaveAnswer(ctx, attempt.ID, reader, bad); err == nil {
			t.Fatalf("foreign answer accepted: %s", q.Type)
		}
		if q.Type != domain.QuestionText {
			bad.SelectedAnswerIDs = []int64{q.Answers[0].ID, q.Answers[0].ID}
			if err := svc.SaveAnswer(ctx, attempt.ID, reader, bad); err == nil {
				t.Fatalf("duplicate/cardinality accepted: %s", q.Type)
			}
		}
		if err := svc.SaveAnswer(ctx, attempt.ID, owner, good); err == nil {
			t.Fatal("author changed another participant's answer")
		}
	}
	reloaded, err := svc.Attempt(ctx, attempt.ID, reader, false)
	if err != nil {
		t.Fatal(err)
	}
	// The participant's own submitted text may equal the reference; it remains
	// visible. Only grading material must be withheld on reload.
	assertSafe(reloaded.Questions)
	for _, answer := range reloaded.Answers {
		if answer.CorrectAnswer != "" || answer.IsCorrect != nil || answer.EarnedPoints != 0 {
			t.Fatal("grading disclosed in saved participant answers")
		}
	}
	if reloaded.ID != attempt.ID || !reloaded.StartedAt.Equal(attempt.StartedAt) || len(reloaded.Answers) != 5 {
		t.Fatalf("resume contract: answers=%d", len(reloaded.Answers))
	}
	if _, err := svc.Attempt(ctx, attempt.ID, owner, false); err == nil {
		t.Fatal("foreign attempt disclosed")
	}
	admin, err := svc.Attempt(ctx, attempt.ID, owner, true)
	if err != nil || admin.Questions[0].Explanation == "" {
		t.Fatal("admin grading access lost")
	}
	finished, err := svc.Finish(ctx, attempt.ID, reader)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Percent != 100 {
		t.Fatalf("invalid submissions destroyed prior answers: %v", finished.Percent)
	}
	if _, err := svc.Finish(ctx, attempt.ID, reader); err == nil {
		t.Fatal("attempt finished twice")
	}
	t.Run("employee races and passport counts", func(t *testing.T) {
		testReleaseEmployeeRaces(t, ctx, owner, authorView, attempt.TestVersionID)
	})
}
