package academy

import (
	test "FinTalent/internal/testmodule/domain"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type Criterion struct {
	Key      string  `json:"key"`
	Title    string  `json:"title"`
	MaxScore float64 `json:"max_score"`
}
type AssessmentPolicy struct {
	Required         *bool       `json:"required,omitempty"`
	PassingPercent   *float64    `json:"passing_percent,omitempty"`
	MaxAttempts      int         `json:"max_attempts"`
	TimeLimitSeconds int         `json:"time_limit_seconds"`
	ShowCorrect      bool        `json:"show_correct"`
	ShowExplanations bool        `json:"show_explanations"`
	AllowRetry       *bool       `json:"allow_retry,omitempty"`
	UseBest          *bool       `json:"use_best,omitempty"`
	MaxScore         float64     `json:"max_score"`
	PassingScore     *float64    `json:"passing_score,omitempty"`
	Formats          []string    `json:"formats"`
	ReviewMode       string      `json:"review_mode"`
	AllowRevision    *bool       `json:"allow_revision,omitempty"`
	Criteria         []Criterion `json:"criteria"`
}

func yes(v *bool) bool { return v == nil || *v }
func remainingTestAttempts(p AssessmentPolicy, used int) int {
	limit := p.MaxAttempts
	if !yes(p.AllowRetry) {
		limit = 1
	}
	return max(0, limit-used)
}
func canRequestRevision(p AssessmentPolicy, number int) bool {
	return yes(p.AllowRevision) && number < p.MaxAttempts
}
func policy(b Block) AssessmentPolicy {
	p := AssessmentPolicy{}
	if b.Data.Assessment != nil {
		p = *b.Data.Assessment
	}
	if p.MaxAttempts == 0 {
		p.MaxAttempts = 3
	}
	if p.PassingPercent == nil {
		v := 70.0
		p.PassingPercent = &v
	}
	if p.MaxScore == 0 {
		p.MaxScore = 100
	}
	if p.ReviewMode == "" {
		p.ReviewMode = "manual"
	}
	if len(p.Formats) == 0 {
		p.Formats = []string{b.Data.AnswerFormat}
		if p.Formats[0] == "" {
			p.Formats[0] = "text"
		}
	}
	if p.Criteria == nil {
		p.Criteria = []Criterion{}
	}
	return p
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validateAssessment(b *Block) error {
	p := policy(*b)
	if p.MaxAttempts < 1 || p.MaxAttempts > 100 || p.TimeLimitSeconds < 0 || p.TimeLimitSeconds > 86400 || !finite(*p.PassingPercent) || *p.PassingPercent < 0 || *p.PassingPercent > 100 {
		return fmt.Errorf("Проверьте лимит попыток, время и проходной процент")
	}
	if !finite(p.MaxScore) || p.MaxScore <= 0 || p.MaxScore > 100000 || p.PassingScore != nil && (!finite(*p.PassingScore) || *p.PassingScore < 0 || *p.PassingScore > p.MaxScore) {
		return fmt.Errorf("Проходной балл должен быть от 0 до максимального балла")
	}
	if len(p.Formats) < 1 || len(p.Formats) > 3 {
		return fmt.Errorf("Выберите форматы ответа")
	}
	seen := map[string]bool{}
	for _, v := range p.Formats {
		if !oneOf(v, "text", "file", "link") || seen[v] {
			return fmt.Errorf("Некорректные форматы ответа")
		}
		seen[v] = true
	}
	if !oneOf(p.ReviewMode, "manual", "automatic") {
		return fmt.Errorf("Доступны ручная проверка и автоматическая проверка формата")
	}
	if len(p.Criteria) > 20 {
		return fmt.Errorf("До 20 критериев")
	}
	total := 0.0
	seen = map[string]bool{}
	for i := range p.Criteria {
		c := &p.Criteria[i]
		c.Title = strings.TrimSpace(c.Title)
		if !keyPattern.MatchString(c.Key) || seen[c.Key] || c.Title == "" || utf8.RuneCountInString(c.Title) > 180 || !finite(c.MaxScore) || c.MaxScore <= 0 {
			return fmt.Errorf("Укажите название и положительный балл каждого критерия")
		}
		seen[c.Key] = true
		total += c.MaxScore
	}
	if len(p.Criteria) > 0 && math.Abs(total-p.MaxScore) > .001 {
		return fmt.Errorf("Сумма баллов критериев должна совпадать с максимальным баллом")
	}
	if b.Type == "assignment" && p.ReviewMode == "automatic" && (p.PassingScore != nil || len(p.Criteria) > 0) {
		return fmt.Errorf("Автоматическое принятие проверяет только формат: уберите проходной балл и критерии либо включите ручную проверку")
	}
	b.Data.Assessment = &p
	return nil
}

type Assessment struct {
	ID        int64            `json:"id"`
	VersionID int64            `json:"program_version_id"`
	LessonKey string           `json:"lesson_key"`
	BlockKey  string           `json:"block_key"`
	Kind      string           `json:"kind"`
	Title     string           `json:"title"`
	Policy    AssessmentPolicy `json:"policy"`
	Questions []test.Question  `json:"-"`
}
type TestAnswer struct {
	QuestionID  int64   `json:"question_id"`
	SelectedIDs []int64 `json:"selected_answer_ids"`
	Text        string  `json:"text_answer"`
}

func validateTestAnswer(a TestAnswer, questions []test.Question) error {
	for _, q := range questions {
		if q.ID != a.QuestionID {
			continue
		}
		if utf8.RuneCountInString(a.Text) > 20000 {
			return inputError{"Ответ — до 20 000 символов"}
		}
		if q.Type == test.QuestionText {
			if len(a.SelectedIDs) > 0 {
				return ErrInvalid
			}
			return nil
		}
		if a.Text != "" || q.Type != test.QuestionMultiple && len(a.SelectedIDs) > 1 {
			return ErrInvalid
		}
		seen := map[int64]bool{}
		for _, id := range a.SelectedIDs {
			found := false
			for _, v := range q.Answers {
				found = found || v.ID == id
			}
			if !found || seen[id] {
				return ErrInvalid
			}
			seen[id] = true
		}
		return nil
	}
	return ErrInvalid
}
func testAnswers(in []TestAnswer) []test.AttemptAnswer {
	out := []test.AttemptAnswer{}
	for _, a := range in {
		if len(a.SelectedIDs) == 0 {
			out = append(out, test.AttemptAnswer{QuestionID: a.QuestionID, TextAnswer: a.Text})
		}
		for _, id := range a.SelectedIDs {
			v := id
			out = append(out, test.AttemptAnswer{QuestionID: a.QuestionID, SelectedAnswerID: &v})
		}
	}
	return out
}

type SubmissionAnswer struct {
	Text     string   `json:"text"`
	Links    []string `json:"links"`
	AssetIDs []int64  `json:"asset_ids"`
}

func validateSubmission(a *SubmissionAnswer, p AssessmentPolicy, final bool) error {
	if a.Links == nil {
		a.Links = []string{}
	}
	if a.AssetIDs == nil {
		a.AssetIDs = []int64{}
	}
	a.Text = strings.TrimSpace(a.Text)
	if utf8.RuneCountInString(a.Text) > 40000 || len(a.Links) > 10 || len(a.AssetIDs) > 10 {
		return inputError{"Ответ: до 40 000 символов, 10 ссылок и 10 файлов"}
	}
	for _, url := range a.Links {
		if !validURL(url) {
			return inputError{"Используйте безопасные HTTPS-ссылки"}
		}
	}
	has := map[string]bool{}
	for _, f := range p.Formats {
		has[f] = true
	}
	if a.Text != "" && !has["text"] || len(a.Links) > 0 && !has["link"] || len(a.AssetIDs) > 0 && !has["file"] {
		return inputError{"Этот формат ответа не разрешён заданием"}
	}
	if final && (has["text"] && a.Text == "" || has["link"] && len(a.Links) == 0 || has["file"] && len(a.AssetIDs) == 0) {
		return inputError{"Заполните все требуемые форматы ответа"}
	}
	return nil
}
func reviewScore(p AssessmentPolicy, score *float64, criteria map[string]float64, action string) (*float64, error) {
	if len(p.Criteria) > 0 && (action == "accepted" || len(criteria) > 0) {
		sum := 0.0
		for _, c := range p.Criteria {
			v, ok := criteria[c.Key]
			if !ok || !finite(v) || v < 0 || v > c.MaxScore {
				return nil, inputError{"Оцените каждый критерий в пределах его максимума"}
			}
			sum += v
		}
		if len(criteria) != len(p.Criteria) {
			return nil, ErrInvalid
		}
		score = &sum
	}
	if score != nil && (!finite(*score) || *score < 0 || *score > p.MaxScore) {
		return nil, inputError{"Оценка должна быть от 0 до максимального балла"}
	}
	if action == "accepted" && (score == nil || p.PassingScore != nil && *score < *p.PassingScore) {
		return nil, inputError{"Для принятия укажите оценку не ниже проходного балла"}
	}
	return score, nil
}
