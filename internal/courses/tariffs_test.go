package courses

import (
	"math"
	"strings"
	"testing"
)

func TestTariffValidation(t *testing.T) {
	v := validInput()
	v.PricingMode = "tariffs"
	v.Tariffs = []Tariff{{Title: "  Базовый ", Description: "Материалы", Price: 100, Hours: 2}, {Title: "С поддержкой", Description: "Проверка заданий", Price: 200, Hours: 4, Recommended: true}}
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

func TestTariffDetailsValidation(t *testing.T) {
	v := validInput()
	v.PricingMode = "tariffs"
	v.Hours = 0
	old := int64(200)
	v.Tariffs = []Tariff{{Title: "Базовый", Description: strings.Repeat("я", 300), Price: 100, OldPrice: &old, Hours: 2.5}, {Title: "Полный", Description: "Практика", Price: 200, Hours: 4}}
	if err := Validate(&v, true); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Tariff){
		func(t *Tariff) { t.Description += "я" },
		func(t *Tariff) { n := t.Price; t.OldPrice = &n },
		func(t *Tariff) { t.Hours = -1 },
		func(t *Tariff) { t.Hours = math.Inf(1) },
		func(t *Tariff) { t.Hours = 0 },
	} {
		bad := v
		bad.Tariffs = append([]Tariff{}, v.Tariffs...)
		change(&bad.Tariffs[0])
		if Validate(&bad, true) == nil {
			t.Fatal("invalid details accepted")
		}
	}
}
