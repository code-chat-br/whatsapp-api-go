package whatsapp

import (
	"strings"
	"unicode"
)

func NormalizePhoneNumber(value string) (string, error) {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	phone := builder.String()
	if len(phone) < 8 || strings.HasPrefix(phone, "0") {
		return "", ErrInvalidPhoneNumber
	}
	return phone, nil
}

func MaskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}
