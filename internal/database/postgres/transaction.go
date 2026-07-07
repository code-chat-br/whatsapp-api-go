package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
)

func WithTransaction(
	ctx context.Context,
	pool *pgxpool.Pool,
	logger zerolog.Logger,
	fn func(q *db.Queries) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		logger.Error().Err(err).Str("operation", "transaction.begin").Msg("failed to begin transaction")
		return fmt.Errorf("begin transaction: %w", err)
	}

	queries := db.New(pool).WithTx(tx)
	if err := fn(queries); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			logger.Error().Err(rollbackErr).Str("operation", "transaction.rollback").Msg("failed to rollback transaction")
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error().Err(err).Str("operation", "transaction.commit").Msg("failed to commit transaction")
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
