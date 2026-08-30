package main

import "testing"

func TestValidateResumeBirthdate(t *testing.T) {
	value := func(n int) *int { return &n }
	tests := []struct {
		name       string
		day, month *int
		year       *int
		valid      bool
	}{
		{"day and month are required", nil, value(4), nil, false},
		{"year is optional", value(19), value(4), nil, true},
		{"valid leap day", value(29), value(2), value(2000), true},
		{"invalid leap day", value(29), value(2), value(2001), false},
		{"invalid calendar date", value(31), value(4), nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateResumeBirthdate(test.day, test.month, test.year); (got == "") != test.valid {
				t.Fatalf("validateResumeBirthdate() = %q, valid=%v", got, test.valid)
			}
		})
	}
}
