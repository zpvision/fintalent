package courses

import (
	"net/mail"
	"strings"
)

func normalizeInterestContacts(email, phone string) (string, string, bool) {
	email = strings.TrimSpace(email)
	if len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		return "", "", false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(email, "@") {
		return "", "", false
	}
	phone = strings.TrimSpace(phone)
	if len(phone) > 60 {
		return "", "", false
	}
	var digits strings.Builder
	for i, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '(' || r == ')' || r == '-':
		default:
			return "", "", false
		}
	}
	if digits.Len() < 7 || digits.Len() > 15 {
		return "", "", false
	}
	normalized := digits.String()
	if strings.HasPrefix(phone, "+") {
		normalized = "+" + normalized
	}
	return email, normalized, true
}
