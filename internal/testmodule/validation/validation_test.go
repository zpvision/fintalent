package validation

import (
	"testing"

	"FinTalent/internal/testmodule/dto"
)

func TestQuestionTypes(t *testing.T) {
	cases := []dto.CreateQuestion{
		{Question: "Один ответ", QuestionType: "single_choice", Points: 1, Answers: []dto.AnswerInput{{Answer: "Да", IsCorrect: true}, {Answer: "Нет"}}},
		{Question: "Несколько ответов", QuestionType: "multiple_choice", Points: 2, Answers: []dto.AnswerInput{{Answer: "А", IsCorrect: true}, {Answer: "Б", IsCorrect: true}}},
		{Question: "Верно ли?", QuestionType: "boolean", Points: 1, Answers: []dto.AnswerInput{{Answer: "Да", IsCorrect: true}, {Answer: "Нет"}}},
		{Question: "Открытый ответ", QuestionType: "text", Points: 3, Answers: []dto.AnswerInput{{Answer: "Эталон", IsCorrect: true}}},
	}
	for _, tc := range cases {
		if err := Question(tc); err != nil {
			t.Errorf("valid %s question rejected: %v", tc.QuestionType, err)
		}
		tc.Points = 0
		if err := Question(tc); err != nil {
			t.Errorf("zero-point %s question rejected: %v", tc.QuestionType, err)
		}
		tc.Points = -0.1
		if err := Question(tc); err == nil {
			t.Errorf("negative-point %s question accepted", tc.QuestionType)
		}
	}
}

func TestRejectsInvalidQuestion(t *testing.T) {
	invalid := dto.CreateQuestion{Question: "Вопрос", QuestionType: "single_choice", Points: 1, Answers: []dto.AnswerInput{{Answer: "А", IsCorrect: true}, {Answer: "Б", IsCorrect: true}}}
	if err := Question(invalid); err == nil {
		t.Fatal("single choice with two correct answers must be rejected")
	}
}

func TestPaidTestNeedsPrice(t *testing.T) {
	in := dto.CreateTest{Title: "Платный тест", Difficulty: "medium", Visibility: "marketplace", IsFree: false}
	if err := Test(in); err == nil {
		t.Fatal("paid test without price must be rejected")
	}
}

func TestTimeLimitValidation(t *testing.T) {
	for _, seconds := range []int{0, 1, 60, 900, 2147483647, -1, 2147483648} {
		in := dto.CreateTest{Title: "Valid test", Difficulty: "medium", Visibility: "public", IsFree: true, PassingPercent: 60, TimeLimitSeconds: &seconds}
		err := Test(in)
		valid := seconds >= 0 && seconds <= 2147483647
		if (err == nil) != valid {
			t.Errorf("time limit %d: %v", seconds, err)
		}
	}
}
