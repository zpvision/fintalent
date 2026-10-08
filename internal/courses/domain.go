package courses

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Benefit struct {
	ID          int64  `json:"id"`
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description string `json:"description"`
}
type Bonus struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}
type Lesson struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Duration    int    `json:"duration_minutes"`
	Type        string `json:"type"`
}
type Module struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Lessons     []Lesson `json:"lessons"`
}
type FAQ struct {
	ID       int64  `json:"id"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
type Input struct {
	Revision     int64     `json:"revision"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CategoryID   *int64    `json:"category_id"`
	Level        string    `json:"level"`
	CoverImage   string    `json:"cover_image"`
	PromoURL     string    `json:"promo_url"`
	Price        int64     `json:"price"` // RUB minor units (kopecks).
	OldPrice     *int64    `json:"old_price"`
	Hours        float64   `json:"hours"`
	AccessMonths int       `json:"access_months"`
	Certificate  bool      `json:"certificate"`
	Chat         bool      `json:"chat"`
	Updates      bool      `json:"updates"`
	AnyDevice    bool      `json:"any_device"`
	Corporate    bool      `json:"corporate"`
	Benefits     []Benefit `json:"benefits"`
	Bonuses      []Bonus   `json:"bonuses"`
	Modules      []Module  `json:"modules"`
	FAQ          []FAQ     `json:"faq"`
}

var imagePath = regexp.MustCompile(`^/static/uploads/profimarket/[a-f0-9]{32}\.(jpg|png|webp|gif)$`)
var icons = map[string]bool{"sparkles": true, "clock": true, "folder": true, "check": true, "users": true, "calculator": true, "workflow": true, "bot": true, "list": true, "message": true}
var lessonTypes = map[string]bool{"video": true, "text": true, "practice": true, "test": true, "materials": true}

func Validate(v *Input, publishing bool) error {
	v.Title = strings.TrimSpace(v.Title)
	v.Description = strings.TrimSpace(v.Description)
	if v.Level != "beginner" && v.Level != "intermediate" && v.Level != "advanced" {
		return fmt.Errorf("Выберите уровень курса")
	}
	if utf8.RuneCountInString(v.Title) > 180 || utf8.RuneCountInString(v.Description) > 1200 {
		return fmt.Errorf("Название — до 180, описание — до 1200 символов")
	}
	if v.Price < 0 || v.Price > 10000000000 || v.OldPrice != nil && (*v.OldPrice <= v.Price || *v.OldPrice > 10000000000) {
		return fmt.Errorf("Старая цена должна быть больше текущей; цена — до 100 млн ₽")
	}
	if math.IsNaN(v.Hours) || math.IsInf(v.Hours, 0) || v.Hours < 0 || v.Hours > 100000 || v.AccessMonths < 1 || v.AccessMonths > 1200 {
		return fmt.Errorf("Проверьте длительность и срок доступа")
	}
	if v.CoverImage != "" && !imagePath.MatchString(v.CoverImage) {
		return fmt.Errorf("Загрузите обложку через форму")
	}
	if v.PromoURL != "" {
		u, e := url.Parse(v.PromoURL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(v.PromoURL) > 2000 {
			return fmt.Errorf("Промо-видео: укажите HTTPS-ссылку")
		}
	}
	if len(v.Benefits) > 4 || len(v.Bonuses) > 30 || len(v.FAQ) > 50 {
		return fmt.Errorf("Допустимо до 4 преимуществ, 30 бонусов и 50 вопросов")
	}
	if publishing && (utf8.RuneCountInString(v.Title) < 5 || utf8.RuneCountInString(v.Description) < 20 || v.CategoryID == nil || v.CoverImage == "" || v.Hours <= 0 || len(v.Benefits) < 3 || len(v.Modules) == 0) {
		return fmt.Errorf("Для публикации заполните название, описание, категорию, обложку, часы обучения, не менее 3 преимуществ и программу")
	}
	check := func(text string, max int, required bool) bool {
		return utf8.RuneCountInString(text) <= max && (!required || strings.TrimSpace(text) != "")
	}
	for _, b := range v.Benefits {
		if !icons[b.Icon] || !check(b.Title, 120, publishing) || !check(b.Description, 500, publishing) {
			return fmt.Errorf("Заполните заголовок и описание преимуществ и выберите иконку")
		}
	}
	for _, b := range v.Bonuses {
		if !check(b.Title, 180, publishing) {
			return fmt.Errorf("Проверьте названия бонусов")
		}
	}
	lessons := 0
	for _, m := range v.Modules {
		if !check(m.Title, 180, publishing) || !check(m.Description, 1200, false) || publishing && len(m.Lessons) == 0 {
			return fmt.Errorf("У каждого модуля должно быть название и хотя бы один урок")
		}
		for _, l := range m.Lessons {
			lessons++
			if !check(l.Title, 180, publishing) || !check(l.Description, 1200, false) || !lessonTypes[l.Type] || l.Duration < 0 || l.Duration > 100000 {
				return fmt.Errorf("Проверьте название, тип и продолжительность уроков")
			}
		}
	}
	for _, q := range v.FAQ {
		if !check(q.Question, 300, publishing) || !check(q.Answer, 3000, publishing) {
			return fmt.Errorf("Заполните вопросы (до 300 символов) и ответы (до 3000)")
		}
	}
	if publishing && lessons == 0 {
		return fmt.Errorf("Добавьте уроки")
	}
	return nil
}

func Slug(title string) string {
	const russian = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"
	latin := []string{"a", "b", "v", "g", "d", "e", "e", "zh", "z", "i", "y", "k", "l", "m", "n", "o", "p", "r", "s", "t", "u", "f", "h", "ts", "ch", "sh", "sch", "", "y", "", "e", "yu", "ya"}
	tr := map[rune]string{}
	for i, r := range []rune(russian) {
		tr[r] = latin[i]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if s, ok := tr[r]; ok {
			b.WriteString(s)
		} else if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := regexp.MustCompile(`-+`).ReplaceAllString(b.String(), "-")
	if len(s) > 160 {
		s = s[:160]
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "kurs"
	}
	return s
}
