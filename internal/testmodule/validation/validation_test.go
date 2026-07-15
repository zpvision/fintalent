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
