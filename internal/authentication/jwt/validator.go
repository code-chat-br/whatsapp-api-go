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

var ErrJWTInvalid = errors.New("invalid JWT")

type Validator interface {
	Validate(tokenString string) (InstanceClaims, error)
}

type JWTValidator struct {
	secret           []byte
	ignoreExpiration bool
	logger           zerolog.Logger
}

func NewJWTValidator(authConfig config.AuthenticationConfig, logger zerolog.Logger) (*JWTValidator, error) {
	if strings.TrimSpace(authConfig.JWTSecret) == "" {
		return nil, config.ErrMissingJWTSecret
	}

	return &JWTValidator{
		secret:           []byte(authConfig.JWTSecret),
		ignoreExpiration: authConfig.JWTExpiresInSeconds == 0,
		logger:           logger,
	}, nil
}

func (v *JWTValidator) Validate(tokenString string) (InstanceClaims, error) {
	if strings.TrimSpace(tokenString) == "" {
		return InstanceClaims{}, ErrJWTInvalid
	}

	claims := InstanceClaims{}
	parserOptions := []gojwt.ParserOption{
		gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}),
	}
	if v.ignoreExpiration {
		parserOptions = append(parserOptions, gojwt.WithoutClaimsValidation())
	}
	parser := gojwt.NewParser(parserOptions...)
	token, err := parser.ParseWithClaims(tokenString, &claims, func(token *gojwt.Token) (any, error) {
		if token.Method != gojwt.SigningMethodHS256 {
			return nil, ErrJWTInvalid
		}
		return v.secret, nil
	})

	if err != nil {
		v.logger.Debug().Err(err).Msg("JWT validation failed")
		return InstanceClaims{}, fmt.Errorf("%w", ErrJWTInvalid)
	}

	if token == nil || !token.Valid || strings.TrimSpace(claims.InstanceName) == "" {
		return InstanceClaims{}, ErrJWTInvalid
	}

	if v.ignoreExpiration && claims.NotBefore != nil && time.Now().UTC().Before(claims.NotBefore.Time) {
		return InstanceClaims{}, ErrJWTInvalid
	}

	return claims, nil
}
