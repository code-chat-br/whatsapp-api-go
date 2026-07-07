package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/database/migrations"
	"whatsapp-go-api/internal/database/postgres"
	"whatsapp-go-api/internal/logging"
	"whatsapp-go-api/internal/whatsapp"
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

	database, err := postgres.NewPostgresPool(ctx, cfg.Database.URL, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect database")
	}
	defer database.Close()

	if err := migrations.Run(ctx, database); err != nil {
		logger.Fatal().Err(err).Msg("failed to run application migrations")
	}
	logger.Info().Msg("database_migrations_completed")

	waLogger := whatsapp.NewWhatsmeowLogger(logger)
	clientFactory, err := whatsapp.NewSQLStoreClientFactory(ctx, cfg.WhatsAppSession, cfg.Database.URL, waLogger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize whatsmeow sqlstore")
	}
	defer func() {
		if err := clientFactory.Store().Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close whatsmeow sqlstore")
		}
	}()

	logger.Info().Msg("whatsmeow_migrations_completed")
}
