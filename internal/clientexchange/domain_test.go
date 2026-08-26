package clientexchange

import "testing"

func TestValidINNFormat(t *testing.T) {
	tests := map[string]bool{
		"7707083893":   true,
		"500100732259": true,
		"123":          false,
		"770708389A":   false,
	}
	for value, want := range tests {
		if got := validINN(value); got != want {
			t.Fatalf("validINN(%q)=%v, want %v", value, got, want)
		}
	}
}

func TestDictionaryKindsAreIndependentAndComplete(t *testing.T) {
	want := []string{"employee_range", "industry", "marketplace", "accounting_state", "transfer_reason", "edo_provider", "transfer_type", "tax_system", "revenue_range", "accounting_program"}
	if len(DictionaryKinds) != len(want) {
		t.Fatalf("got %d kinds, want %d", len(DictionaryKinds), len(want))
	}
	for _, kind := range want {
		if !validKind(kind) {
			t.Errorf("missing kind %s", kind)
		}
	}
}

func TestClamp(t *testing.T) {
	if clamp(-1, 1, 6) != 1 || clamp(7, 1, 6) != 6 || clamp(4, 1, 6) != 4 {
		t.Fatal("clamp boundaries are broken")
	}
}
