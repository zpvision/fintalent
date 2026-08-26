package main

import "testing"

func TestEmbeddedOKVEDData(t *testing.T) {
	data, err := okvedDataFS.ReadFile("data/okved.csv")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseOKVEDCSV(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3090 {
		t.Fatalf("expected 3090 OKVED entries, got %d", len(entries))
	}
	want := map[string]string{
		"A":        "СЕЛЬСКОЕ, ЛЕСНОЕ ХОЗЯЙСТВО, ОХОТА, РЫБОЛОВСТВО И РЫБОВОДСТВО",
		"01.11.11": "Выращивание пшеницы",
		"99.00":    "Деятельность экстерриториальных организаций и органов",
	}
	for _, entry := range entries {
		if expected, ok := want[entry.Code]; ok {
			if entry.Name != expected {
				t.Errorf("code %s: expected %q, got %q", entry.Code, expected, entry.Name)
			}
			delete(want, entry.Code)
		}
	}
	for code := range want {
		t.Errorf("missing OKVED code %s", code)
	}
}

func TestFormatOKVEDCode(t *testing.T) {
	tests := map[string]string{"01": "01", "011": "01.1", "0111": "01.11", "01111": "01.11.1", "011111": "01.11.11"}
	for input, expected := range tests {
		if actual := formatOKVEDCode(input); actual != expected {
			t.Errorf("formatOKVEDCode(%q)=%q, expected %q", input, actual, expected)
		}
	}
}
