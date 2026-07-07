package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

func NewPostgresPool(ctx context.Context, databaseURL string, logger zerolog.Logger) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		logger.Error().Err(err).Str("operation", "postgres.parse_config").Msg("failed to parse postgres config")
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logger.Error().Err(err).Str("operation", "postgres.new_pool").Msg("failed to create postgres pool")
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		logger.Error().Err(err).Str("operation", "postgres.ping").Msg("failed to ping postgres")
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
