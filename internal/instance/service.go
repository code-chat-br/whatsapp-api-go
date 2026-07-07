package instance

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/database/postgres"
	"whatsapp-go-api/internal/database/repository"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

const maxInstanceNameLength = 255

type Service interface {
	Create(ctx context.Context, input CreateInstanceInput) (CreateInstanceResult, error)
	List(ctx context.Context, instanceName *string) ([]types.InstanceDetails, error)
	FetchByName(ctx context.Context, instanceName string) (types.InstanceDetails, error)
	RefreshToken(ctx context.Context, instanceName string, bearerToken string, oldToken string) (types.Auth, error)
}

type CreateInstanceInput struct {
	InstanceName       *string
	Description        *string
	ExternalAttributes json.RawMessage
}

type CreateInstanceResult struct {
	Instance types.Instance
	Auth     types.Auth
}

type InstanceService struct {
	pool      *pgxpool.Pool
	instances repository.InstanceRepository
	auths     repository.AuthRepository
	tokenGen  authjwt.Generator
	tokenVal  authjwt.Validator
	logger    zerolog.Logger
}

func NewService(
	pool *pgxpool.Pool,
	instances repository.InstanceRepository,
	auths repository.AuthRepository,
	tokenGen authjwt.Generator,
	tokenVal authjwt.Validator,
	logger zerolog.Logger,
) *InstanceService {
	return &InstanceService{
		pool:      pool,
		instances: instances,
		auths:     auths,
		tokenGen:  tokenGen,
		tokenVal:  tokenVal,
		logger:    logger.With().Str("component", "instance_service").Logger(),
	}
}

func (s *InstanceService) Create(ctx context.Context, input CreateInstanceInput) (CreateInstanceResult, error) {
	name, err := normalizeInstanceName(input.InstanceName)
	if err != nil {
		return CreateInstanceResult{}, err
	}
	if name == "" {
		name = "instance-" + uuid.NewString()
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		input.Description = &description
	}
	if err := validateJSONObject(input.ExternalAttributes); err != nil {
		return CreateInstanceResult{}, err
	}

	var result CreateInstanceResult
	err = postgres.WithTransaction(ctx, s.pool, s.logger, func(q *db.Queries) error {
		created, err := s.instances.CreateTx(ctx, q, types.CreateInstanceInput{
			Name:               name,
			Description:        input.Description,
			ExternalAttributes: input.ExternalAttributes,
		})
		if err != nil {
			return err
		}

		token, err := s.tokenGen.Generate(created.Instance.Name)
		if err != nil {
			return fmt.Errorf("%w: %w", authjwt.ErrJWTGeneration, err)
		}

		auth, err := s.auths.CreateTx(ctx, q, types.CreateAuthInput{
			Token:      token,
			InstanceID: created.Instance.ID,
		})
		if err != nil {
			return err
		}

		result = CreateInstanceResult{Instance: created.Instance, Auth: auth}
		return nil
	})
	if err != nil {
		return CreateInstanceResult{}, err
	}

	s.logger.Info().
		Str("operation", "instance.create").
		Str("instanceName", result.Instance.Name).
		Int32("instanceId", result.Instance.ID).
		Msg("instance created")

	return result, nil
}

func (s *InstanceService) List(ctx context.Context, instanceName *string) ([]types.InstanceDetails, error) {
	if instanceName != nil {
		normalized := strings.TrimSpace(*instanceName)
		if normalized == "" || len(normalized) > maxInstanceNameLength {
			return nil, repository.ErrInvalidInput
		}
		instanceName = &normalized
	}

	items, err := s.instances.ListDetails(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	s.logger.Info().Str("operation", "instance.list").Int("count", len(items)).Msg("instances listed")
	return items, nil
}

func (s *InstanceService) FetchByName(ctx context.Context, instanceName string) (types.InstanceDetails, error) {
	name, err := normalizeRequiredName(instanceName)
	if err != nil {
		return types.InstanceDetails{}, err
	}
	item, err := s.instances.FetchDetailsByName(ctx, name)
	if err != nil {
		return types.InstanceDetails{}, err
	}
	s.logger.Info().Str("operation", "instance.fetch").Str("instanceName", name).Msg("instance fetched")
	return item, nil
}

func (s *InstanceService) RefreshToken(ctx context.Context, instanceName string, bearerToken string, oldToken string) (types.Auth, error) {
	name, err := normalizeRequiredName(instanceName)
	if err != nil {
		return types.Auth{}, err
	}
	oldToken = strings.TrimSpace(oldToken)
	bearerToken = strings.TrimSpace(bearerToken)
	if oldToken == "" || bearerToken == "" {
		return types.Auth{}, repository.ErrInvalidOldToken
	}
	if subtle.ConstantTimeCompare([]byte(bearerToken), []byte(oldToken)) != 1 {
		return types.Auth{}, repository.ErrInvalidOldToken
	}

	claims, err := s.tokenVal.Validate(bearerToken)
	if err != nil {
		return types.Auth{}, repository.ErrInvalidOldToken
	}
	if claims.InstanceName != name {
		return types.Auth{}, repository.ErrTokenInstanceMismatch
	}

	var updated types.Auth
	err = postgres.WithTransaction(ctx, s.pool, s.logger, func(q *db.Queries) error {
		instance, err := s.instances.FindByNameTx(ctx, q, name)
		if err != nil {
			return err
		}

		auth, err := s.auths.LockByInstanceIDTx(ctx, q, instance.Instance.ID)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(auth.Token), []byte(oldToken)) != 1 {
			return repository.ErrInvalidOldToken
		}

		newToken, err := s.tokenGen.Generate(name)
		if err != nil {
			return fmt.Errorf("%w: %w", authjwt.ErrJWTGeneration, err)
		}
		updated, err = s.auths.UpdateTokenByInstanceAndOldTokenTx(ctx, q, types.UpdateAuthTokenConditionInput{
			InstanceID: instance.Instance.ID,
			OldToken:   oldToken,
			NewToken:   newToken,
		})
		return err
	})
	if err != nil {
		return types.Auth{}, err
	}

	s.logger.Info().
		Str("operation", "instance.refresh_token").
		Str("instanceName", name).
		Int32("authId", updated.ID).
		Msg("instance token refreshed")

	return updated, nil
}

func normalizeInstanceName(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	return normalizeRequiredName(*value)
}

func normalizeRequiredName(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > maxInstanceNameLength {
		return "", repository.ErrInvalidInput
	}
	return normalized, nil
}

func validateJSONObject(value json.RawMessage) error {
	if len(value) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("%w: %w", repository.ErrInvalidJSON, err)
	}
	if _, ok := decoded.(map[string]any); !ok {
		return repository.ErrInvalidJSON
	}
	return nil
}

func IsValidationError(err error) bool {
	return errors.Is(err, repository.ErrInvalidInput) || errors.Is(err, repository.ErrInvalidJSON)
}
