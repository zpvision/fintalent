package courses

import "testing"

func TestTariffValidation(t *testing.T) {
	v := validInput()
	v.PricingMode = "tariffs"
	v.Tariffs = []Tariff{{Title: "  Базовый ", Description: "Материалы", Price: 100}, {Title: "С поддержкой", Description: "Проверка заданий", Price: 200, Recommended: true}}
	if err := Validate(&v, true); err != nil {
		t.Fatal(err)
	}
	if v.Tariffs[0].Title != "Базовый" {
		t.Fatal("title not trimmed")
	}
	for _, test := range []struct {
		name   string
		change func(*Input)
	}{
		{"mode", func(v *Input) { v.PricingMode = "invalid" }},
		{"one", func(v *Input) { v.Tariffs = v.Tariffs[:1] }},
		{"five", func(v *Input) { v.Tariffs = make([]Tariff, 5) }},
		{"duplicate", func(v *Input) { v.Tariffs[1].Title = "БАЗОВЫЙ" }},
		{"negative", func(v *Input) { v.Tariffs[1].Price = -1 }},
		{"two recommended", func(v *Input) { v.Tariffs[0].Recommended = true }},
		{"description", func(v *Input) { v.Tariffs[1].Description = "" }},
		{"foreign input", func(v *Input) { v.Tariffs[0].ID = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := v
			bad.Tariffs = append([]Tariff{}, v.Tariffs...)
			test.change(&bad)
			if Validate(&bad, true) == nil {
				t.Fatal("invalid tariffs accepted")
			}
		})
	}
	v.Tariffs[0].Description = ""
	if err := Validate(&v, false); err != nil {
		t.Fatal("draft rejected", err)
	}
	v.PricingMode = "single"
	if err := Validate(&v, true); err != nil {
		t.Fatal("inactive draft rejected", err)
	}
}
