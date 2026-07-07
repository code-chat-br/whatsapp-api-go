package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/config"
)

func NewLogger(cfg config.LogConfig) (zerolog.Logger, error) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return zerolog.Logger{}, err
	}

	zerolog.SetGlobalLevel(level)

	var writer io.Writer = os.Stdout
	if level == zerolog.TraceLevel || level == zerolog.DebugLevel {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Logger()

	return logger, nil
}
