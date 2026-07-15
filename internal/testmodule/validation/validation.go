package validation

import (
	"errors"
	"strings"

	"FinTalent/internal/testmodule/domain"
	"FinTalent/internal/testmodule/dto"
)

func Test(input dto.CreateTest) error {
	if len([]rune(strings.TrimSpace(input.Title))) < 3 || len([]rune(input.Title)) > 240 {
		return errors.New("название должно содержать от 3 до 240 символов")
	}
	if input.Difficulty != "easy" && input.Difficulty != "medium" && input.Difficulty != "hard" {
		return errors.New("некорректная сложность")
	}
	if input.Visibility != domain.VisibilityPrivate && input.Visibility != domain.VisibilityPublic && input.Visibility != domain.VisibilityMarketplace {
		return errors.New("некорректная видимость")
	}
	if input.Price < 0 || (!input.IsFree && input.Price <= 0) {
		return errors.New("для платного теста укажите стоимость")
	}
	if input.PassingPercent < 0 || input.PassingPercent > 100 {
		return errors.New("проходной процент должен быть от 0 до 100")
	}
	return nil
}

func Question(input dto.CreateQuestion) error {
	if len([]rune(strings.TrimSpace(input.Question))) < 3 {
		return errors.New("укажите текст вопроса")
	}
	if input.Points <= 0 {
		return errors.New("баллы должны быть больше нуля")
	}
	switch input.QuestionType {
	case domain.QuestionSingle, domain.QuestionMultiple, domain.QuestionBoolean:
		if len(input.Answers) < 2 {
			return errors.New("добавьте минимум два варианта ответа")
		}
		correct := 0
		for _, a := range input.Answers {
			if strings.TrimSpace(a.Answer) == "" {
				return errors.New("вариант ответа не может быть пустым")
			}
			if a.IsCorrect {
				correct++
			}
		}
		if correct == 0 || input.QuestionType == domain.QuestionSingle && correct != 1 || input.QuestionType == domain.QuestionBoolean && (len(input.Answers) != 2 || correct != 1) {
			return errors.New("отметьте корректное количество правильных ответов")
		}
	case domain.QuestionText:
		if len(input.Answers) != 1 || strings.TrimSpace(input.Answers[0].Answer) == "" {
			return errors.New("для текстового вопроса нужен один эталонный ответ")
		}
	default:
		return errors.New("тип вопроса пока не поддерживается")
	}
	return nil
}
