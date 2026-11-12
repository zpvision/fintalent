package courses

import "testing"

func TestNormalizeInterestContacts(t *testing.T) {
	for _, v := range []struct {
		email, phone, want string
		valid              bool
	}{
		{"person@example.ru", "+7 (999) 123-45-67", "+79991234567", true},
		{" person@example.ru ", "8 999 123 45 67", "89991234567", true},
		{"bad", "+79991234567", "", false},
		{"Name <person@example.ru>", "+79991234567", "", false},
		{"person@example.ru\r\nBcc:x@y.ru", "+79991234567", "", false},
		{"person@example.ru", "123", "", false},
		{"person@example.ru", "+7abc9991234567", "", false},
		{"person@example.ru", "1234567890123456", "", false},
		{"person@example.ru", "", "", false},
	} {
		t.Run(v.phone+v.email, func(t *testing.T) {
			_, phone, valid := normalizeInterestContacts(v.email, v.phone)
			if valid != v.valid || phone != v.want {
				t.Fatal("unexpected contact validation")
			}
		})
	}
}
