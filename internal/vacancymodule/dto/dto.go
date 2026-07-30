package dto

type RequirementInput struct {
	CategoryID   int64  `json:"category_id"`
	BlockID      int64  `json:"block_id"`
	DictionaryID int64  `json:"dictionary_id"`
	Importance   string `json:"importance"`
	SortOrder    int    `json:"sort_order"`
	CategoryName string `json:"category_name,omitempty"`
}

type VacancyDraft struct {
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	SalaryFrom      *float64           `json:"salary_from"`
	SalaryTo        *float64           `json:"salary_to"`
	SalaryTaxMode   string             `json:"salary_tax_mode"`
	Currency        string             `json:"currency"`
	EmploymentType  string             `json:"employment_type"`
	WorkFormat      string             `json:"work_format"`
	City            string             `json:"city"`
	Address         string             `json:"address"`
	ExperienceFrom  *int               `json:"experience_from"`
	ExperienceTo    *int               `json:"experience_to"`
	CurrentStep     int                `json:"current_step"`
	SelectedTestID  *int64             `json:"selected_test_id"`
	SelectedTestIDs []int64            `json:"selected_test_ids"`
	Requirements    []RequirementInput `json:"requirements"`
}

type MatchPreview struct {
	VacancyID    *int64             `json:"vacancy_id"`
	Requirements []RequirementInput `json:"requirements"`
	SalaryFrom   *float64           `json:"salary_from"`
	SalaryTo     *float64           `json:"salary_to"`
	WorkFormat   string             `json:"work_format"`
}

type ResumeDraft struct {
	CurrentStep int     `json:"current_step"`
	CategoryIDs []int64 `json:"category_ids"`
}
