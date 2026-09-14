package domain

import (
	"reflect"
	"slices"
	"testing"
)

func TestPrepareAnswerOrder(t *testing.T) {
	answers := []Answer{{ID: 1, Answer: "First", IsCorrect: true}, {ID: 2, Answer: "Second"}, {ID: 3, Answer: "Third"}, {ID: 4, Answer: "Fourth"}}
	for _, kind := range []string{QuestionSingle, QuestionMultiple, QuestionBoolean, QuestionText} {
		t.Run(kind, func(t *testing.T) {
			qs := []Question{{ID: 11, Type: kind, Answers: slices.Clone(answers), Settings: map[string]any{"shuffle_answers": false}}}
			PrepareAnswerOrder(qs, 42, false)
			if !reflect.DeepEqual(qs[0].Answers, answers) {
				t.Fatal("disabled setting changed the author order")
			}
			PrepareAnswerOrder(qs, 42, true)
			ordered := slices.Clone(qs[0].Answers)
			PrepareAnswerOrder(qs, 42, true)
			if !reflect.DeepEqual(qs[0].Answers, ordered) {
				t.Fatal("order changed within the same attempt")
			}
			if kind == QuestionText {
				if !reflect.DeepEqual(ordered, answers) {
					t.Fatal("text answer was shuffled")
				}
				return
			}
			var changed bool
			for attempt := int64(1); attempt <= 100; attempt++ {
				PrepareAnswerOrder(qs, attempt, true)
				changed = changed || !reflect.DeepEqual(ordered, qs[0].Answers)
				for _, a := range qs[0].Answers {
					if a != answers[a.ID-1] {
						t.Fatal("answer identity or correctness changed")
					}
				}
			}
			if !changed {
				t.Fatal("different attempts always have the same order")
			}
		})
	}
}

func TestShuffleAppliesToEveryChoiceQuestion(t *testing.T) {
	qs := []Question{
		{ID: 11, Type: QuestionSingle, Answers: []Answer{{ID: 1}, {ID: 2}, {ID: 3}}},
		{ID: 11, Type: QuestionMultiple, Settings: map[string]any{"shuffle_answers": false}, Answers: []Answer{{ID: 1}, {ID: 2}, {ID: 3}}},
	}
	PrepareAnswerOrder(qs, 1, true)
	if !reflect.DeepEqual(qs[0].Answers, qs[1].Answers) {
		t.Fatal("legacy per-question settings overrode the test setting")
	}
	if qs[0].Answers[0].ID == 1 && qs[0].Answers[1].ID == 2 {
		t.Fatal("fixture must exercise a changed order")
	}
}
