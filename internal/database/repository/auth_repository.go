package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type AuthRepository interface {
	Create(ctx context.Context, input types.CreateAuthInput) (types.Auth, error)
	CreateTx(ctx context.Context, q *db.Queries, input types.CreateAuthInput) (types.Auth, error)
	FindByInstanceIDTx(ctx context.Context, q *db.Queries, instanceID int32) (types.Auth, error)
	LockByInstanceIDTx(ctx context.Context, q *db.Queries, instanceID int32) (types.Auth, error)
	UpdateTokenTx(ctx context.Context, q *db.Queries, input types.UpdateAuthTokenInput) (types.Auth, error)
	UpdateTokenByInstanceAndOldTokenTx(ctx context.Context, q *db.Queries, input types.UpdateAuthTokenConditionInput) (types.Auth, error)
}

type authRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewAuthRepository(pool *pgxpool.Pool, logger zerolog.Logger) AuthRepository {
	return &authRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "auth_repository").Logger(),
	}
}

func (r *authRepository) Create(ctx context.Context, input types.CreateAuthInput) (types.Auth, error) {
	return r.create(ctx, r.q, input)
}

func (r *authRepository) CreateTx(ctx context.Context, q *db.Queries, input types.CreateAuthInput) (types.Auth, error) {
	return r.create(ctx, q, input)
}

func (r *authRepository) create(ctx context.Context, q *db.Queries, input types.CreateAuthInput) (types.Auth, error) {
	exists, err := q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Auth{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Auth{}, ErrInstanceNotFound
	}

	auth, err := q.CreateAuth(ctx, db.CreateAuthParams{
		Token:      input.Token,
		Instanceid: input.InstanceID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrAuthAlreadyExists, err)
		}
		if isForeignKeyViolation(err) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "auth.create").Int32("instanceId", input.InstanceID).Msg("failed to create auth")
		return types.Auth{}, fmt.Errorf("create auth: %w", err)
	}
	return mapAuth(auth), nil
}

func (r *authRepository) FindByInstanceIDTx(ctx context.Context, q *db.Queries, instanceID int32) (types.Auth, error) {
	auth, err := q.FindAuthByInstanceID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrAuthNotFound, err)
		}
		return types.Auth{}, fmt.Errorf("find auth by instance: %w", err)
	}
	return mapAuth(auth), nil
}

func (r *authRepository) LockByInstanceIDTx(ctx context.Context, q *db.Queries, instanceID int32) (types.Auth, error) {
	auth, err := q.LockAuthByInstanceID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrAuthNotFound, err)
		}
		return types.Auth{}, fmt.Errorf("lock auth by instance: %w", err)
	}
	return mapAuth(auth), nil
}

func (r *authRepository) UpdateTokenTx(ctx context.Context, q *db.Queries, input types.UpdateAuthTokenInput) (types.Auth, error) {
	auth, err := q.UpdateAuthToken(ctx, db.UpdateAuthTokenParams{
		ID:    input.AuthID,
		Token: input.NewToken,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrAuthNotFound, err)
		}
		return types.Auth{}, fmt.Errorf("update auth token: %w", err)
	}
	return mapAuth(auth), nil
}

func (r *authRepository) UpdateTokenByInstanceAndOldTokenTx(ctx context.Context, q *db.Queries, input types.UpdateAuthTokenConditionInput) (types.Auth, error) {
	auth, err := q.UpdateAuthTokenByInstanceAndOldToken(ctx, db.UpdateAuthTokenByInstanceAndOldTokenParams{
		Instanceid: input.InstanceID,
		Oldtoken:   input.OldToken,
		Newtoken:   input.NewToken,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Auth{}, fmt.Errorf("%w: %w", ErrInvalidOldToken, err)
		}
		return types.Auth{}, fmt.Errorf("update auth token conditionally: %w", err)
	}
	return mapAuth(auth), nil
}
