package logging

import (
	"testing"

	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/config"
)

func TestNewLoggerAcceptsAllowedLevels(t *testing.T) {
	levels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic", "disabled"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			logger, err := NewLogger(config.LogConfig{Level: level})
			if err != nil {
				t.Fatalf("NewLogger() error = %v", err)
			}
			parsed, _ := zerolog.ParseLevel(level)
			if logger.GetLevel() != parsed {
				t.Fatalf("expected logger level %s, got %s", parsed, logger.GetLevel())
			}
		})
	}
}

func TestNewLoggerRejectsInvalidLevel(t *testing.T) {
	if _, err := NewLogger(config.LogConfig{Level: "verbose"}); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}
