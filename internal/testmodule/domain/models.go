package domain

import "time"

const (
	StatusDraft           = "draft"
	StatusPublished       = "published"
	StatusArchived        = "archived"
	StatusDeleted         = "deleted"
	StatusBlocked         = "blocked"
	VisibilityPrivate     = "private"
	VisibilityPublic      = "public"
	VisibilityMarketplace = "marketplace"
	QuestionSingle        = "single_choice"
	QuestionMultiple      = "multiple_choice"
	QuestionBoolean       = "boolean"
	QuestionText          = "text"
)

type Test struct {
	ID               int64      `json:"id"`
	AuthorID         int64      `json:"author_id"`
	AuthorName       string     `json:"author_name"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Slug             string     `json:"slug"`
	Category         string     `json:"category"`
	Difficulty       string     `json:"difficulty"`
	Status           string     `json:"status"`
	Visibility       string     `json:"visibility"`
	Price            float64    `json:"price"`
	Currency         string     `json:"currency"`
	IsFree           bool       `json:"is_free"`
	Version          int        `json:"version"`
	PassingPercent   float64    `json:"passing_percent"`
	TimeLimitSeconds *int       `json:"time_limit_seconds,omitempty"`
	QuestionCount    int        `json:"question_count"`
	AttemptsCount    int64      `json:"attempts_count"`
	AveragePercent   float64    `json:"average_percent"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Questions        []Question `json:"questions,omitempty"`
}

type Question struct {
	ID            int64          `json:"id"`
	TestVersionID int64          `json:"test_version_id"`
	SortOrder     int            `json:"sort_order"`
	Question      string         `json:"question"`
	Type          string         `json:"question_type"`
	Explanation   string         `json:"explanation,omitempty"`
	Points        float64        `json:"points"`
	Settings      map[string]any `json:"settings,omitempty"`
	Answers       []Answer       `json:"answers"`
}

type Answer struct {
	ID         int64  `json:"id"`
	QuestionID int64  `json:"question_id"`
	Answer     string `json:"answer"`
	IsCorrect  bool   `json:"is_correct,omitempty"`
	SortOrder  int    `json:"sort_order"`
}

type Statistics struct {
	TestID                 int64   `json:"test_id"`
	AttemptsCount          int64   `json:"attempts_count"`
	CompletedCount         int64   `json:"completed_count"`
	PassedCount            int64   `json:"passed_count"`
	FailedCount            int64   `json:"failed_count"`
	AveragePercent         float64 `json:"average_percent"`
	AverageDurationSeconds float64 `json:"average_duration_seconds"`
}

type Attempt struct {
	ID              int64           `json:"id"`
	TestID          int64           `json:"test_id"`
	TestVersionID   int64           `json:"test_version_id"`
	UserID          int64           `json:"user_id"`
	UserName        string          `json:"user_name,omitempty"`
	TestTitle       string          `json:"test_title,omitempty"`
	Score           float64         `json:"score"`
	MaxScore        float64         `json:"max_score"`
	Percent         float64         `json:"percent"`
	Passed          *bool           `json:"passed,omitempty"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	DurationSeconds int             `json:"duration_seconds"`
	Status          string          `json:"status"`
	CorrectAnswers  int             `json:"correct_answers"`
	TotalQuestions  int             `json:"total_questions"`
	Answers         []AttemptAnswer `json:"answers,omitempty"`
}

type AttemptAnswer struct {
	QuestionID       int64      `json:"question_id"`
	Question         string     `json:"question,omitempty"`
	SelectedAnswerID *int64     `json:"selected_answer_id,omitempty"`
	SelectedAnswer   string     `json:"selected_answer,omitempty"`
	TextAnswer       string     `json:"text_answer,omitempty"`
	CorrectAnswer    string     `json:"correct_answer,omitempty"`
	IsCorrect        *bool      `json:"is_correct,omitempty"`
	EarnedPoints     float64    `json:"earned_points"`
	AnsweredAt       *time.Time `json:"answered_at,omitempty"`
	ResponseSeconds  int        `json:"response_seconds"`
}
