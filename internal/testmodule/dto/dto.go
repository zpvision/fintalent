package dto

type CreateTest struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Category         string  `json:"category"`
	Difficulty       string  `json:"difficulty"`
	Visibility       string  `json:"visibility"`
	IsFree           bool    `json:"is_free"`
	Price            float64 `json:"price"`
	PassingPercent   float64 `json:"passing_percent"`
	TimeLimitSeconds *int    `json:"time_limit_seconds"`
}
type UpdateTest = CreateTest
type AnswerInput struct {
	ID        int64  `json:"id,omitempty"`
	Answer    string `json:"answer"`
	IsCorrect bool   `json:"is_correct"`
	SortOrder int    `json:"sort_order"`
}
type CreateQuestion struct {
	Question     string         `json:"question"`
	QuestionType string         `json:"question_type"`
	Explanation  string         `json:"explanation"`
	Points       float64        `json:"points"`
	SortOrder    int            `json:"sort_order"`
	Settings     map[string]any `json:"settings"`
	Answers      []AnswerInput  `json:"answers"`
}
type SubmitAnswer struct {
	QuestionID        int64   `json:"question_id"`
	SelectedAnswerIDs []int64 `json:"selected_answer_ids"`
	TextAnswer        string  `json:"text_answer"`
}
type ModerateTest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}
type ListFilter struct {
	Scope    string
	Status   string
	Author   string
	Category string
	Price    string
	Search   string
	Limit    int
	Offset   int
}
