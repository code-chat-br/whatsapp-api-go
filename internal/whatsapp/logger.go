package whatsapp

import (
	"github.com/rs/zerolog"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type WhatsmeowLogger struct {
	logger zerolog.Logger
}

func NewWhatsmeowLogger(logger zerolog.Logger) WhatsmeowLogger {
	return WhatsmeowLogger{logger: logger.With().Str("component", "whatsmeow").Logger()}
}

func (l WhatsmeowLogger) Warnf(msg string, args ...any) {
	l.logger.Warn().Msgf(msg, args...)
}

func (l WhatsmeowLogger) Errorf(msg string, args ...any) {
	l.logger.Error().Msgf(msg, args...)
}

func (l WhatsmeowLogger) Infof(msg string, args ...any) {
	l.logger.Info().Msgf(msg, args...)
}

func (l WhatsmeowLogger) Debugf(msg string, args ...any) {
	l.logger.Debug().Msgf(msg, args...)
}

func (l WhatsmeowLogger) Sub(module string) waLog.Logger {
	return WhatsmeowLogger{logger: l.logger.With().Str("module", module).Logger()}
}
