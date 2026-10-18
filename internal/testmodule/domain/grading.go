package domain

import (
	"math"
	"strings"
)

// Grade is the shared deterministic grading engine for professional and Academy contexts.
// Access, deadlines and persistence belong to the caller; client scores are never accepted.
type GradeResult struct {
	Score, MaxScore, Percent float64
	Passed                   bool
	Answers                  []AttemptAnswer
}

func Grade(questions []Question, answers []AttemptAnswer, passing float64) GradeResult {
	byQuestion := map[int64][]AttemptAnswer{}
	for _, a := range answers {
		byQuestion[a.QuestionID] = append(byQuestion[a.QuestionID], a)
	}
	score, max := 0.0, 0.0
	grades := []AttemptAnswer{}
	for _, q := range questions {
		max += q.Points
		submitted := byQuestion[q.ID]
		correct := false
		switch q.Type {
		case QuestionText:
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
		grades = append(grades, AttemptAnswer{QuestionID: q.ID, IsCorrect: &c, EarnedPoints: earned})
	}
	percent := 0.0
	if max > 0 {
		percent = math.Round(score/max*10000) / 100
	}

	return GradeResult{Score: score, MaxScore: max, Percent: percent, Passed: percent >= passing, Answers: grades}
}
