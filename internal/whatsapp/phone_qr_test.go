package whatsapp

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizePhoneNumber(t *testing.T) {
	tests := map[string]string{
		"5531999999999":     "5531999999999",
		"+5531999999999":    "5531999999999",
		"+55 31 99999-9999": "5531999999999",
	}

	for input, want := range tests {
		got, err := NormalizePhoneNumber(input)
		if err != nil {
			t.Fatalf("NormalizePhoneNumber(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizePhoneNumber(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePhoneNumberRejectsInvalid(t *testing.T) {
	for _, input := range []string{"", "123", "031999999999"} {
		if _, err := NormalizePhoneNumber(input); err == nil {
			t.Fatalf("expected invalid phone error for %q", input)
		}
	}
}

func TestQRGeneratorReturnsPNGDataURL(t *testing.T) {
	generator, err := NewQRGenerator("#ffffff", "#198754")
	if err != nil {
		t.Fatalf("NewQRGenerator() error = %v", err)
	}

	dataURL, err := generator.GenerateDataURL("qr-content")
	if err != nil {
		t.Fatalf("GenerateDataURL() error = %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL prefix: %s", dataURL[:20])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatal("expected PNG signature")
	}
}
