package jwt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/config"
)

var (
	ErrInstanceNameRequired = errors.New("instance name is required")
	ErrJWTGeneration        = errors.New("failed to generate JWT")
)

type Generator interface {
	Generate(instanceName string) (string, error)
}

type JWTGenerator struct {
	secret           []byte
	expiresInSeconds int64
	logger           zerolog.Logger
}

func NewJWTGenerator(authConfig config.AuthenticationConfig, logger zerolog.Logger) (*JWTGenerator, error) {
	if strings.TrimSpace(authConfig.JWTSecret) == "" {
		return nil, config.ErrMissingJWTSecret
	}

	return &JWTGenerator{
		secret:           []byte(authConfig.JWTSecret),
		expiresInSeconds: authConfig.JWTExpiresInSeconds,
		logger:           logger,
	}, nil
}

func (g *JWTGenerator) Generate(instanceName string) (string, error) {
	if strings.TrimSpace(instanceName) == "" {
		return "", ErrInstanceNameRequired
	}

	claims := InstanceClaims{
		InstanceName: instanceName,
	}

	if g.expiresInSeconds > 0 {
		expiresAt := time.Now().UTC().Add(time.Duration(g.expiresInSeconds) * time.Second)
		claims.ExpiresAt = gojwt.NewNumericDate(expiresAt)
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(g.secret)
	if err != nil {
		wrapped := fmt.Errorf("%w: sign token: %w", ErrJWTGeneration, err)
		g.logger.Error().Err(wrapped).Msg("failed to sign JWT")
		return "", wrapped
	}

	return signedToken, nil
}
