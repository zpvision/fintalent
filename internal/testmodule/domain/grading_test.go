package domain

import "testing"

func TestSharedGradeAllQuestionTypes(t *testing.T) {
	yes, no := int64(1), int64(2)
	questions := []Question{{ID: 1, Type: QuestionSingle, Points: 2, Answers: []Answer{{ID: yes, IsCorrect: true}, {ID: no}}}, {ID: 2, Type: QuestionMultiple, Points: 3, Answers: []Answer{{ID: 3, IsCorrect: true}, {ID: 4, IsCorrect: true}, {ID: 5}}}, {ID: 3, Type: QuestionText, Points: 5, Answers: []Answer{{Answer: " Точный Ответ ", IsCorrect: true}}}, {ID: 4, Type: QuestionBoolean, Points: 2, Answers: []Answer{{ID: 6, IsCorrect: true}, {ID: 7}}}}
	three, four, six := int64(3), int64(4), int64(6)
	answers := []AttemptAnswer{{QuestionID: 1, SelectedAnswerID: &yes}, {QuestionID: 2, SelectedAnswerID: &three}, {QuestionID: 2, SelectedAnswerID: &four}, {QuestionID: 3, TextAnswer: "точный ответ"}, {QuestionID: 4, SelectedAnswerID: &six}}
	r := Grade(questions, answers, 70)
	if r.Score != 12 || r.Percent != 100 || !r.Passed || len(r.Answers) != 4 {
		t.Fatal(r)
	}
	r = Grade(questions, answers[:2], 70)
	if r.Score != 2 || r.Passed {
		t.Fatal("partial multiple choice incorrectly awarded", r)
	}
}
