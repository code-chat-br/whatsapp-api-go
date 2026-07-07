package jwt

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/config"
)

func TestGenerateJWTWithInstanceName(t *testing.T) {
	secret := "test-secret"
	generator := newTestGenerator(t, secret, 3600, zerolog.Nop())

	tokenString, err := generator.Generate("instance-name")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims := parseToken(t, tokenString, secret)

	if claims.InstanceName != "instance-name" {
		t.Fatalf("expected instanceName claim, got %q", claims.InstanceName)
	}

	assertFunctionalClaims(t, tokenString, true)
}

func TestGenerateJWTWithPositiveExpiration(t *testing.T) {
	secret := "test-secret"
	generator := newTestGenerator(t, secret, 3600, zerolog.Nop())

	before := time.Now().UTC().Add(3600 * time.Second)
	tokenString, err := generator.Generate("instance-name")
	after := time.Now().UTC().Add(3600 * time.Second)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims := parseToken(t, tokenString, secret)
	if claims.ExpiresAt == nil {
		t.Fatal("expected exp claim")
	}

	expiration := claims.ExpiresAt.Time
	if expiration.Before(before.Add(-2*time.Second)) || expiration.After(after.Add(2*time.Second)) {
		t.Fatalf("expected expiration around one hour from now, got %s", expiration)
	}
}

func TestGenerateJWTWithZeroExpirationOmitsExp(t *testing.T) {
	secret := "test-secret"
	generator := newTestGenerator(t, secret, 0, zerolog.Nop())

	tokenString, err := generator.Generate("instance-name")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims := parseToken(t, tokenString, secret)
	if claims.InstanceName != "instance-name" {
		t.Fatalf("expected instanceName claim, got %q", claims.InstanceName)
	}
	if claims.ExpiresAt != nil {
		t.Fatalf("expected no exp claim, got %v", claims.ExpiresAt)
	}

	assertFunctionalClaims(t, tokenString, false)
}

func TestGenerateJWTValidatesInstanceName(t *testing.T) {
	generator := newTestGenerator(t, "test-secret", 3600, zerolog.Nop())

	for _, instanceName := range []string{"", "   "} {
		t.Run("invalid instance name", func(t *testing.T) {
			_, err := generator.Generate(instanceName)
			if !errors.Is(err, ErrInstanceNameRequired) {
				t.Fatalf("expected ErrInstanceNameRequired, got %v", err)
			}
		})
	}
}

func TestNewJWTGeneratorRejectsEmptySecret(t *testing.T) {
	_, err := NewJWTGenerator(config.AuthenticationConfig{
		JWTSecret:           "",
		JWTExpiresInSeconds: 3600,
	}, zerolog.Nop())
	if !errors.Is(err, config.ErrMissingJWTSecret) {
		t.Fatalf("expected ErrMissingJWTSecret, got %v", err)
	}
}

func TestGenerateJWTDoesNotLogToken(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	generator := newTestGenerator(t, "test-secret", 3600, logger)

	tokenString, err := generator.Generate("instance-name")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if bytes.Contains(logs.Bytes(), []byte(tokenString)) {
		t.Fatal("logger contains generated token")
	}
}

func newTestGenerator(t *testing.T, secret string, expiresInSeconds int64, logger zerolog.Logger) *JWTGenerator {
	t.Helper()

	generator, err := NewJWTGenerator(config.AuthenticationConfig{
		JWTSecret:           secret,
		JWTExpiresInSeconds: expiresInSeconds,
	}, logger)
	if err != nil {
		t.Fatalf("NewJWTGenerator() error = %v", err)
	}

	return generator
}

func parseToken(t *testing.T, tokenString string, secret string) *InstanceClaims {
	t.Helper()

	claims := &InstanceClaims{}
	token, err := gojwt.ParseWithClaims(tokenString, claims, func(token *gojwt.Token) (any, error) {
		if token.Method != gojwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected valid token")
	}

	return claims
}

func assertFunctionalClaims(t *testing.T, tokenString string, expectExpiration bool) {
	t.Helper()

	parser := gojwt.NewParser()
	claims := gojwt.MapClaims{}
	if _, _, err := parser.ParseUnverified(tokenString, claims); err != nil {
		t.Fatalf("parse token without verification: %v", err)
	}

	encoded, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	var claimMap map[string]any
	if err := json.Unmarshal(encoded, &claimMap); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}

	expectedCount := 1
	if expectExpiration {
		expectedCount = 2
		if _, ok := claimMap["exp"]; !ok {
			t.Fatal("expected exp claim")
		}
	} else if _, ok := claimMap["exp"]; ok {
		t.Fatal("expected exp claim to be absent")
	}

	if _, ok := claimMap["instanceName"]; !ok {
		t.Fatal("expected instanceName claim")
	}
	if len(claimMap) != expectedCount {
		t.Fatalf("expected %d functional claims, got %d: %v", expectedCount, len(claimMap), claimMap)
	}
}
