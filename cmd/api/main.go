package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"whatsapp-go-api/internal/app"
	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadConfig()
	if err != nil {
		logger, _ := logging.NewLogger(config.LogConfig{Level: "error"})
		logger.Fatal().Err(err).Msg("failed to load configuration")
	}

	logger, err := logging.NewLogger(cfg.Log)
	if err != nil {
		fallback, _ := logging.NewLogger(config.LogConfig{Level: "error"})
		fallback.Fatal().Err(err).Msg("failed to initialize logger")
	}
	logger.Info().Msg("configuration_loaded")
	logger.Info().Msg("logger_initialized")

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize application")
	}

	if err := application.Run(ctx); err != nil {
		logger.Fatal().Err(err).Msg("application stopped with error")
	}
}
