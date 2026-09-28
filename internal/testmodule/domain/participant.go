package domain

// Hide grading material in every participant-facing question response.
func HideQuestionSolutions(questions []Question) {
	for i := range questions {
		q := &questions[i]
		q.Explanation = ""
		if q.Type == QuestionText {
			q.Answers = []Answer{}
		}
		for j := range q.Answers {
			q.Answers[j].IsCorrect = false
		}
	}
}
