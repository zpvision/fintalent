package main

import "testing"

func TestValidProfileMode(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{name: profileModeJobSearch, valid: true},
		{name: profileModeProfessional, valid: true},
		{name: "", valid: false},
		{name: "employer", valid: false},
	}
	for _, test := range tests {
		if got := validProfileMode(test.name); got != test.valid {
			t.Fatalf("validProfileMode(%q) = %v, want %v", test.name, got, test.valid)
		}
	}
}

func TestApplyProfessionalResumePresentationHidesCareerOnlyData(t *testing.T) {
	view := &publicResumeView{
		ProfileMode:          profileModeProfessional,
		DesiredSalary:        180000,
		AvailableImmediately: true,
		SearchStatus:         "Готов к предложениям",
		WorkPreferences:      "Удалённая работа",
		Blocks: []publicResumeBlock{
			{Name: "Желаемая должность"},
			{Name: "Профессиональные навыки"},
			{Name: "Общая информация"},
			{Name: "Программы"},
			{Name: "CRM"},
		},
		Duties:      []publicResumeDutyGroup{{Name: "Отчётность"}},
		Cities:      []resumeFinanceCity{{ID: 1, Name: "Москва"}},
		WorkFormats: []resumeFinanceOption{{ID: 1, Value: "Удалённо"}},
	}

	applyProfessionalResumePresentation(view)

	if view.DesiredSalary != 0 || view.AvailableImmediately || view.WorkPreferences != "" {
		t.Fatal("career preferences remained in professional presentation")
	}
	if len(view.Duties) != 0 || len(view.Cities) != 0 || len(view.WorkFormats) != 0 {
		t.Fatal("career-only collections remained in professional presentation")
	}
	if len(view.Blocks) != 3 || view.Blocks[0].Name != "Профессиональные навыки" || view.Blocks[1].Name != "Программы" || view.Blocks[2].Name != "CRM" {
		t.Fatalf("unexpected professional blocks: %#v", view.Blocks)
	}
	if view.SearchStatus != "Профессиональный профиль" {
		t.Fatalf("unexpected professional status: %q", view.SearchStatus)
	}
}
