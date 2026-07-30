package domain

import "time"

const (
	ImportanceRequired  = "required"
	ImportancePreferred = "preferred"
	ImportanceBonus     = "bonus"
)

var ImportanceCoefficients = map[string]int{
	ImportanceRequired:  100,
	ImportancePreferred: 60,
	ImportanceBonus:     30,
}

type Requirement struct {
	CategoryID   int64  `json:"category_id"`
	BlockID      int64  `json:"block_id"`
	DictionaryID int64  `json:"dictionary_id"`
	Importance   string `json:"importance"`
	SortOrder    int    `json:"sort_order"`
	CategoryName string `json:"category_name,omitempty"`
}

type Vacancy struct {
	ID              int64         `json:"id"`
	UserID          int64         `json:"user_id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Status          string        `json:"status"`
	SalaryFrom      *float64      `json:"salary_from"`
	SalaryTo        *float64      `json:"salary_to"`
	SalaryTaxMode   string        `json:"salary_tax_mode"`
	Currency        string        `json:"currency"`
	EmploymentType  string        `json:"employment_type"`
	WorkFormat      string        `json:"work_format"`
	City            string        `json:"city"`
	Address         string        `json:"address"`
	ExperienceFrom  *int          `json:"experience_from"`
	ExperienceTo    *int          `json:"experience_to"`
	CurrentStep     int           `json:"current_step"`
	SelectedTestID  *int64        `json:"selected_test_id,omitempty"`
	SelectedTestIDs []int64       `json:"selected_test_ids"`
	Requirements    []Requirement `json:"requirements"`
	DutyIDs         []int64       `json:"duty_ids"`
	PublishedAt     *time.Time    `json:"published_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type CategoryMetadata struct {
	CategoryID             int64
	BlockID                int64
	DictionaryID           int64
	Name                   string
	UseImportanceInVacancy bool
	SingleChoice           bool
	Active                 bool
}

type BuilderItem struct {
	ID      int64  `json:"id"`
	Value   string `json:"value"`
	Comment string `json:"comment"`
	Icon    string `json:"icon"`
}

type BuilderDictionary struct {
	ID                     int64         `json:"id"`
	Name                   string        `json:"name"`
	Alias                  string        `json:"alias"`
	Icon                   string        `json:"icon"`
	UseImportanceInVacancy bool          `json:"use_importance_in_vacancy"`
	SingleChoice           bool          `json:"single_choice"`
	SelectionColor         string        `json:"selection_color"`
	Items                  []BuilderItem `json:"items"`
}

type BuilderBlock struct {
	ID                       int64               `json:"id"`
	Name                     string              `json:"name"`
	SortOrder                int                 `json:"sort_order"`
	ShowDictionariesTogether bool                `json:"show_dictionaries_together"`
	ShowDictionaryIcon       bool                `json:"show_dictionary_icon"`
	PlainAnswerText          bool                `json:"plain_answer_text"`
	ColumnsPerRow            int                 `json:"columns_per_row"`
	Dictionaries             []BuilderDictionary `json:"dictionaries"`
}

type MatchResult struct {
	MandatoryMatch             bool     `json:"mandatory_match"`
	MissingRequiredCategoryIDs []int64  `json:"missing_required_category_ids"`
	Score                      *float64 `json:"score"`
}

type PreviewResult struct {
	TotalResumes     int64            `json:"total_resumes"`
	MandatoryMatched int64            `json:"mandatory_matched"`
	PartiallyMatched int64            `json:"partially_matched"`
	AverageScore     *float64         `json:"average_score"`
	ScoreRanges      map[string]int64 `json:"score_ranges"`
}
