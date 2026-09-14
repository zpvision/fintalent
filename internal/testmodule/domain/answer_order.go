package domain

import (
	"hash/fnv"
	"sort"
	"strconv"
)

// PrepareAnswerOrder uses the same stable order for taking, resuming and reviewing
// an attempt. IDs and correctness stay attached to their answers.
func PrepareAnswerOrder(questions []Question, attemptID int64, shuffle bool) {
	if !shuffle {
		return
	}
	for i := range questions {
		q := &questions[i]
		if q.Type != QuestionSingle && q.Type != QuestionMultiple && q.Type != QuestionBoolean {
			continue
		}
		prefix := strconv.FormatInt(attemptID, 10) + ":" + strconv.FormatInt(q.ID, 10) + ":"
		keys := make(map[int64]uint32, len(q.Answers))
		for _, a := range q.Answers {
			h := fnv.New32a()
			_, _ = h.Write([]byte(prefix + strconv.FormatInt(a.ID, 10)))
			keys[a.ID] = h.Sum32()
		}
		sort.Slice(q.Answers, func(i, j int) bool {
			a, b := q.Answers[i].ID, q.Answers[j].ID
			if keys[a] == keys[b] {
				return a < b
			}
			return keys[a] < keys[b]
		})
	}
}
