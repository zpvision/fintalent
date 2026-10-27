package courses

import (
	"strings"
	"testing"
)

func validInput() Input {
	category := int64(1)
	return Input{Title: "Курс для бухгалтера", Description: "Практические знания для ежедневной работы", CategoryID: &category, Level: "beginner", CoverImage: "/static/uploads/profimarket/" + strings.Repeat("a", 32) + ".webp", Hours: 6, AccessMonths: 12, Benefits: []Benefit{{Icon: "clock", Title: "Время", Description: "Экономия времени"}, {Icon: "check", Title: "Практика", Description: "Рабочие навыки"}, {Icon: "folder", Title: "Материалы", Description: "Готовые шаблоны"}}, Modules: []Module{{Title: "Основы", Lessons: []Lesson{{Title: "Первый урок", Type: "video", Duration: 15}}}}}
}

func TestLearningOutcomes(t *testing.T) {
	for _, publishing := range []bool{false, true} {
		v := validInput()
		v.LearningOutcomes = []string{"  Составлять финансовую модель  ", " ", strings.Repeat("я", 300)}
		if err := Validate(&v, publishing); err != nil || v.LearningOutcomes[0] != "Составлять финансовую модель" || v.LearningOutcomes[1] != "" {
			t.Fatalf("valid outcomes rejected or not trimmed: %v", err)
		}
		v.LearningOutcomes = []string{strings.Repeat("я", 301)}
		if Validate(&v, publishing) == nil {
			t.Fatal("oversized outcome accepted")
		}
		v.LearningOutcomes = make([]string, 21)
		if Validate(&v, publishing) == nil {
			t.Fatal("too many outcomes accepted")
		}
	}
}
func TestCustomFeatureValidation(t *testing.T) {
	for _, publishing := range []bool{false, true} {
		v := validInput()
		for i := 0; i < 10; i++ {
			v.CustomFeatures = append(v.CustomFeatures, Bonus{Title: strings.Repeat("я", 179) + string(rune('а'+i))})
		}
		if err := Validate(&v, publishing); err != nil {
			t.Fatalf("ten 180-character features rejected: %v", err)
		}
		v.CustomFeatures = append(v.CustomFeatures, Bonus{Title: "Лишний пункт"})
		if Validate(&v, publishing) == nil {
			t.Fatal("eleven features accepted")
		}
		for _, items := range [][]Bonus{
			{{Title: " \n\t "}}, {{Title: strings.Repeat("я", 181)}}, {{ID: -1, Title: "Пункт"}},
			{{Title: "Проверка заданий"}, {Title: " проверка   ЗАДАНИЙ "}},
		} {
			v = validInput()
			v.CustomFeatures = items
			if Validate(&v, publishing) == nil {
				t.Fatalf("invalid features accepted: %+v", items)
			}
		}
		v = validInput()
		v.CustomFeatures = []Bonus{{Title: "  Разбор кейсов  "}}
		if err := Validate(&v, publishing); err != nil || v.CustomFeatures[0].Title != "Разбор кейсов" {
			t.Fatalf("trim failed: %v", err)
		}
	}
}
func TestLessonTypeKeys(t *testing.T) {
	for _, key := range []string{"video", "lesson-1", "lesson-9223372036854775807"} {
		v := validInput()
		v.Modules[0].Lessons[0].Type = key
		if err := Validate(&v, false); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
	for _, key := range []string{"", "unknown", "lesson-0", "lesson--1", "lesson-1x", "lesson-12345678901234567890"} {
		v := validInput()
		v.Modules[0].Lessons[0].Type = key
		if err := Validate(&v, false); err == nil {
			t.Errorf("accepted %q", key)
		}
	}
}
func TestPublicationValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Input)
	}{
		{"missing category", func(v *Input) { v.CategoryID = nil }},
		{"missing cover", func(v *Input) { v.CoverImage = "" }},
		{"invalid benefit color", func(v *Input) { v.Benefits[0].Color = "url(javascript:alert(1))" }},
		{"invalid bonuses color", func(v *Input) { v.BonusesColor = "unknown" }},
		{"duplicate features", func(v *Input) { v.FeatureIDs = []int64{1, 1} }},
		{"invalid feature id", func(v *Input) { v.FeatureIDs = []int64{-1} }},
		{"partial benefit", func(v *Input) { v.Benefits[0].Description = " " }},
		{"empty module", func(v *Input) { v.Modules[0].Lessons = nil }},
		{"empty lesson title", func(v *Input) { v.Modules[0].Lessons[0].Title = " " }},
		{"invalid type", func(v *Input) { v.Modules[0].Lessons[0].Type = "script" }},
		{"negative duration", func(v *Input) { v.Modules[0].Lessons[0].Duration = -1 }},
		{"unsafe url", func(v *Input) { v.PromoURL = "javascript:alert(1)" }},
		{"url credentials", func(v *Input) { v.PromoURL = "https://user:password@example.org/video" }},
		{"external cover", func(v *Input) { v.CoverImage = "https://example.org/tracker.jpg" }},
		{"cover traversal", func(v *Input) { v.CoverImage = "/static/uploads/profimarket/../secret" }},
		{"invalid discount", func(v *Input) { p := int64(1); v.Price = 2; v.OldPrice = &p }},
		{"oversized faq", func(v *Input) { v.FAQ = []FAQ{{Question: "Вопрос", Answer: strings.Repeat("я", 3001)}} }},
	}
	v := validInput()
	if e := Validate(&v, true); e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := validInput()
			c.change(&v)
			if Validate(&v, true) == nil {
				t.Fatal("accepted invalid course")
			}
		})
	}
	draft := Input{Level: "beginner", AccessMonths: 12}
	if e := Validate(&draft, false); e != nil {
		t.Fatal("empty draft rejected", e)
	}
	if Validate(&draft, true) == nil {
		t.Fatal("empty draft published")
	}
}
func TestBenefitLimit(t *testing.T) {
	for _, publishing := range []bool{false, true} {
		v := validInput()
		v.Benefits = append(v.Benefits, v.Benefits[0])
		if err := Validate(&v, publishing); err != nil {
			t.Fatalf("four benefits rejected (publishing=%v): %v", publishing, err)
		}
		v.Benefits = append(v.Benefits, v.Benefits[0])
		if Validate(&v, publishing) == nil {
			t.Fatalf("five benefits accepted (publishing=%v)", publishing)
		}
	}
}

func TestOptionalBenefitSlots(t *testing.T) {
	for _, count := range []int{0, 1, 4} {
		v := validInput()
		v.Benefits = make([]Benefit, count)
		for i := range v.Benefits {
			v.Benefits[i] = Benefit{Icon: "sparkles", Color: "mint"}
		}
		if err := Validate(&v, true); err != nil {
			t.Fatalf("empty slots rejected: %v", err)
		}
	}
	v := validInput()
	v.Benefits = v.Benefits[:1]
	v.Benefits[0].Color = "ocean"
	if err := Validate(&v, true); err != nil {
		t.Fatalf("single filled benefit rejected: %v", err)
	}
}

func TestCourseSlug(t *testing.T) {
	for in, want := range map[string]string{"ИИ для бухгалтера": "ii-dlya-buhgaltera", "  Excel / 1С  ": "excel-1s", "!!!": "kurs"} {
		if got := Slug(in); got != want {
			t.Fatalf("slug %q = %q, want %q", in, got, want)
		}
	}
}
