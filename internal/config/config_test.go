package config

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestLoadDockerEnvironmentDoesNotLoadEnvFile(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "process-secret")
	t.Setenv(envGlobalAuthToken, "process-global-token")
	setValidWhatsAppEnv(t)
	chdirTemp(t)
	writeEnvFile(t, `DATABASE_URL="postgres://file"
AUTHENTICATION_JWT_EXPIRES_IN="0"
AUTHENTICATION_JWT_SECRET="file-secret"
AUTHENTICATION_GLOBAL_AUTH_TOKEN="file-global-token"
`)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Environment.Docker {
		t.Fatal("expected docker environment")
	}
	if cfg.Database.URL != "postgres://process" {
		t.Fatalf("expected process DATABASE_URL, got %q", cfg.Database.URL)
	}
	if cfg.Authentication.GlobalAuthToken != "process-global-token" {
		t.Fatal("expected process global auth token")
	}
}

func TestLoadNonDockerLoadsEnvFile(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "false")
	chdirTemp(t)
	writeValidEnvFile(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment.Docker {
		t.Fatal("expected non-docker environment")
	}
	if cfg.Database.URL != "postgres://file" {
		t.Fatalf("expected file DATABASE_URL, got %q", cfg.Database.URL)
	}
}

func TestLoadMissingDockerEnvironmentLoadsEnvFile(t *testing.T) {
	preserveEnv(t)
	os.Unsetenv(envDocker)
	chdirTemp(t)
	writeValidEnvFile(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment.Docker {
		t.Fatal("expected default non-docker environment")
	}
}

func TestLoadUppercaseDockerEnvironmentAccepted(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "TRUE")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "process-secret")
	t.Setenv(envGlobalAuthToken, "process-global-token")
	setValidWhatsAppEnv(t)
	chdirTemp(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Environment.Docker {
		t.Fatal("expected docker environment")
	}
}

func TestLoadInvalidDockerEnvironmentReturnsError(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "invalid")

	_, err := Load(zerolog.Nop())
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
	}
}

func TestLoadMissingEnvFileOutsideDockerReturnsError(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "false")
	chdirTemp(t)

	_, err := Load(zerolog.Nop())
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
	}
}

func TestLoadProcessVariablesHavePriorityOverEnvFile(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "false")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "7200")
	t.Setenv(envJWTSecret, "process-secret")
	t.Setenv(envGlobalAuthToken, "process-global-token")
	setValidWhatsAppEnv(t)
	chdirTemp(t)
	writeEnvFile(t, `DATABASE_URL="postgres://file"
AUTHENTICATION_JWT_EXPIRES_IN="3600"
AUTHENTICATION_JWT_SECRET="file-secret"
AUTHENTICATION_GLOBAL_AUTH_TOKEN="file-global-token"
`)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.URL != "postgres://process" {
		t.Fatalf("expected process DATABASE_URL, got %q", cfg.Database.URL)
	}
	if cfg.Authentication.JWTExpiresInSeconds != 7200 {
		t.Fatalf("expected process expiration, got %d", cfg.Authentication.JWTExpiresInSeconds)
	}
	if cfg.Authentication.GlobalAuthToken != "process-global-token" {
		t.Fatal("expected process global auth token")
	}
}

func TestLoadRequiredVariables(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr error
	}{
		{
			name: "empty database URL",
			env: map[string]string{
				envDatabaseURL:     "",
				envJWTExpiresIn:    "3600",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrMissingDatabaseURL,
		},
		{
			name: "empty JWT secret",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "3600",
				envJWTSecret:       "",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrMissingJWTSecret,
		},
		{
			name: "spaces JWT secret",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "3600",
				envJWTSecret:       "   ",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrMissingJWTSecret,
		},
		{
			name: "empty global auth token",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "3600",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "",
			},
			wantErr: ErrMissingGlobalAuth,
		},
		{
			name: "spaces global auth token",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "3600",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "   ",
			},
			wantErr: ErrMissingGlobalAuth,
		},
		{
			name: "empty JWT expiration",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrInvalidJWTExpiration,
		},
		{
			name: "non numeric JWT expiration",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "abc",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrInvalidJWTExpiration,
		},
		{
			name: "negative JWT expiration",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    "-1",
				envJWTSecret:       "secret",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrInvalidJWTExpiration,
		},
		{
			name: "overflow JWT expiration",
			env: map[string]string{
				envDatabaseURL:     "postgres://process",
				envJWTExpiresIn:    strconvFormatInt(math.MaxInt64/int64(time.Second) + 1),
				envJWTSecret:       "secret",
				envGlobalAuthToken: "global-token",
			},
			wantErr: ErrInvalidJWTExpiration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			setValidWhatsAppEnv(t)
			if errors.Is(tt.wantErr, ErrMissingDatabaseURL) {
				t.Setenv(envWhatsAppSessionPostgresURL, "postgres://sessions")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := Load(zerolog.Nop())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoadJWTExpirationZeroAndPositiveAccepted(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn string
		want      int64
	}{
		{name: "zero", expiresIn: "0", want: 0},
		{name: "positive", expiresIn: "3600", want: 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, tt.expiresIn)
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)

			cfg, err := Load(zerolog.Nop())
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Authentication.JWTExpiresInSeconds != tt.want {
				t.Fatalf("expected expiration %d, got %d", tt.want, cfg.Authentication.JWTExpiresInSeconds)
			}
		})
	}
}

func TestLoadDatabasePersistenceFlags(t *testing.T) {
	tests := []struct {
		name              string
		env               map[string]string
		wantNewMessage    bool
		wantMessageUpdate bool
		wantContacts      bool
	}{
		{
			name:              "defaults",
			wantNewMessage:    true,
			wantMessageUpdate: false,
			wantContacts:      false,
		},
		{
			name: "explicit true and false",
			env: map[string]string{
				envDatabaseSaveDataNewMessage: "false",
				envDatabaseSaveMessageUpdate:  "true",
				envDatabaseSaveDataContacts:   "true",
			},
			wantNewMessage:    false,
			wantMessageUpdate: true,
			wantContacts:      true,
		},
		{
			name: "trimmed uppercase values",
			env: map[string]string{
				envDatabaseSaveDataNewMessage: " TRUE ",
				envDatabaseSaveMessageUpdate:  " FALSE ",
				envDatabaseSaveDataContacts:   " TRUE ",
			},
			wantNewMessage:    true,
			wantMessageUpdate: false,
			wantContacts:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cfg, err := Load(zerolog.Nop())
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Database.SaveDataNewMessage != tt.wantNewMessage {
				t.Fatalf("expected SaveDataNewMessage %v, got %v", tt.wantNewMessage, cfg.Database.SaveDataNewMessage)
			}
			if cfg.Database.SaveMessageUpdate != tt.wantMessageUpdate {
				t.Fatalf("expected SaveMessageUpdate %v, got %v", tt.wantMessageUpdate, cfg.Database.SaveMessageUpdate)
			}
			if cfg.Database.SaveDataContacts != tt.wantContacts {
				t.Fatalf("expected SaveDataContacts %v, got %v", tt.wantContacts, cfg.Database.SaveDataContacts)
			}
		})
	}
}

func TestLoadWhatsAppSessionConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		storeSet         bool
		store            string
		sqliteDSNSet     bool
		sqliteDSN        string
		postgresURL      string
		mainDatabaseURL  string
		wantStore        WhatsAppSessionStore
		wantSQLiteDSN    string
		wantPostgresURL  string
		wantResolvedDSN  string
		wantErr          string
		forbiddenInError string
	}{
		{
			name:            "unset store defaults to postgres using database URL",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStorePostgres,
			wantSQLiteDSN:   defaultWhatsAppSessionSQLiteDSN,
			wantResolvedDSN: "postgres://api:secret@localhost:5432/app",
		},
		{
			name:            "sqlite with valid DSN",
			storeSet:        true,
			store:           "sqlite",
			sqliteDSNSet:    true,
			sqliteDSN:       "file:./data/custom.db?_foreign_keys=true&cache=shared",
			postgresURL:     "postgres://sessions:secret@localhost:5432/sessions",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStoreSQLite,
			wantSQLiteDSN:   "file:./data/custom.db?_foreign_keys=true&cache=shared",
		},
		{
			name:            "sqlite without DSN",
			storeSet:        true,
			store:           "sqlite",
			sqliteDSNSet:    true,
			sqliteDSN:       "   ",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantErr:         "WHATSAPP_SESSION_SQLITE_DSN is required when WHATSAPP_SESSION_STORE=sqlite",
		},
		{
			name:            "postgres with dedicated URL",
			storeSet:        true,
			store:           "postgres",
			postgresURL:     "postgres://sessions:session-secret@localhost:5432/sessions",
			mainDatabaseURL: "postgres://api:api-secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStorePostgres,
			wantSQLiteDSN:   defaultWhatsAppSessionSQLiteDSN,
			wantPostgresURL: "postgres://sessions:session-secret@localhost:5432/sessions",
			wantResolvedDSN: "postgres://sessions:session-secret@localhost:5432/sessions",
		},
		{
			name:            "postgres falls back to database URL",
			storeSet:        true,
			store:           "postgres",
			mainDatabaseURL: "postgres://api:api-secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStorePostgres,
			wantSQLiteDSN:   defaultWhatsAppSessionSQLiteDSN,
			wantResolvedDSN: "postgres://api:api-secret@localhost:5432/app",
		},
		{
			name:     "postgres without any URL",
			storeSet: true,
			store:    "postgres",
			wantErr:  "WHATSAPP_SESSION_POSTGRES_URL or DATABASE_URL is required when WHATSAPP_SESSION_STORE=postgres",
		},
		{
			name:            "empty store fails",
			storeSet:        true,
			store:           "   ",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantErr:         "invalid WHATSAPP_SESSION_STORE: expected sqlite or postgres",
		},
		{
			name:             "invalid store fails without leaking value",
			storeSet:         true,
			store:            "postgres://user:password@localhost:5432/secret",
			mainDatabaseURL:  "postgres://api:secret@localhost:5432/app",
			wantErr:          "invalid WHATSAPP_SESSION_STORE: expected sqlite or postgres",
			forbiddenInError: "password",
		},
		{
			name:            "uppercase store is normalized",
			storeSet:        true,
			store:           "POSTGRES",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStorePostgres,
			wantSQLiteDSN:   defaultWhatsAppSessionSQLiteDSN,
			wantResolvedDSN: "postgres://api:secret@localhost:5432/app",
		},
		{
			name:            "store spaces are trimmed",
			storeSet:        true,
			store:           " sqlite ",
			sqliteDSNSet:    true,
			sqliteDSN:       " file:./data/trimmed.db?_foreign_keys=on ",
			mainDatabaseURL: "postgres://api:secret@localhost:5432/app",
			wantStore:       WhatsAppSessionStoreSQLite,
			wantSQLiteDSN:   "file:./data/trimmed.db?_foreign_keys=on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, tt.mainDatabaseURL)
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)
			if tt.storeSet {
				t.Setenv(envWhatsAppSessionStore, tt.store)
			}
			if tt.sqliteDSNSet {
				t.Setenv(envWhatsAppSessionSQLiteDSN, tt.sqliteDSN)
			}
			if tt.postgresURL != "" {
				t.Setenv(envWhatsAppSessionPostgresURL, tt.postgresURL)
			}

			cfg, err := Load(zerolog.Nop())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				if tt.forbiddenInError != "" && strings.Contains(err.Error(), tt.forbiddenInError) {
					t.Fatalf("error leaked forbidden value %q: %v", tt.forbiddenInError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.WhatsAppSession.Store != tt.wantStore {
				t.Fatalf("expected store %q, got %q", tt.wantStore, cfg.WhatsAppSession.Store)
			}
			if cfg.WhatsAppSession.SQLiteDSN != tt.wantSQLiteDSN {
				t.Fatalf("expected SQLite DSN %q, got %q", tt.wantSQLiteDSN, cfg.WhatsAppSession.SQLiteDSN)
			}
			if cfg.WhatsAppSession.PostgresURL != tt.wantPostgresURL {
				t.Fatalf("expected Postgres URL %q, got %q", tt.wantPostgresURL, cfg.WhatsAppSession.PostgresURL)
			}
			if tt.wantStore == WhatsAppSessionStorePostgres && cfg.WhatsAppSession.PostgresDSN(cfg.Database.URL) != tt.wantResolvedDSN {
				t.Fatalf("expected resolved Postgres DSN %q, got %q", tt.wantResolvedDSN, cfg.WhatsAppSession.PostgresDSN(cfg.Database.URL))
			}
		})
	}
}

func TestLoadWebhookGlobalConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		enabled     string
		url         string
		wantEnabled bool
		wantURL     string
		wantErr     bool
	}{
		{name: "defaults disabled"},
		{name: "enabled with http URL", enabled: "true", url: "http://internal.local/webhook", wantEnabled: true, wantURL: "http://internal.local/webhook"},
		{name: "enabled with https URL", enabled: "true", url: "https://example.com/hook", wantEnabled: true, wantURL: "https://example.com/hook"},
		{name: "disabled with empty URL", enabled: "false", wantEnabled: false},
		{name: "enabled without URL", enabled: "true", wantErr: true},
		{name: "invalid bool", enabled: "yes", url: "https://example.com/hook", wantErr: true},
		{name: "invalid scheme", enabled: "false", url: "ftp://example.com/hook", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)
			if tt.enabled != "" {
				t.Setenv(envWebhookGlobalEnabled, tt.enabled)
			}
			if tt.url != "" {
				t.Setenv(envWebhookGlobalURL, tt.url)
			}

			cfg, err := Load(zerolog.Nop())
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Webhook.GlobalEnabled != tt.wantEnabled {
				t.Fatalf("expected enabled %v, got %v", tt.wantEnabled, cfg.Webhook.GlobalEnabled)
			}
			if cfg.Webhook.GlobalURL != tt.wantURL {
				t.Fatalf("expected URL %q, got %q", tt.wantURL, cfg.Webhook.GlobalURL)
			}
		})
	}
}

func TestLoadMessageProcessingConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    MessageProcessingConfig
		wantErr bool
	}{
		{
			name: "defaults",
			want: MessageProcessingConfig{
				Workers:           4,
				QueueSize:         100,
				ProcessingTimeout: 60 * time.Second,
				GroupInfoTimeout:  30 * time.Second,
				SendTimeout:       30 * time.Second,
			},
		},
		{
			name: "explicit values",
			env: map[string]string{
				envMessageProcessingWorkers:   "2",
				envMessageProcessingQueueSize: "10",
				envMessageProcessingTimeout:   "45s",
				envMessageGroupInfoTimeout:    "15s",
				envMessageSendTimeout:         "20s",
			},
			want: MessageProcessingConfig{
				Workers:           2,
				QueueSize:         10,
				ProcessingTimeout: 45 * time.Second,
				GroupInfoTimeout:  15 * time.Second,
				SendTimeout:       20 * time.Second,
			},
		},
		{name: "invalid workers", env: map[string]string{envMessageProcessingWorkers: "0"}, wantErr: true},
		{name: "invalid queue", env: map[string]string{envMessageProcessingQueueSize: "-1"}, wantErr: true},
		{name: "invalid timeout", env: map[string]string{envMessageProcessingTimeout: "0s"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cfg, err := Load(zerolog.Nop())
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.MessageProcessing != tt.want {
				t.Fatalf("message processing config mismatch: want %#v got %#v", tt.want, cfg.MessageProcessing)
			}
		})
	}
}

func TestLoadInvalidDatabasePersistenceFlag(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envGlobalAuthToken, "global-token")
	t.Setenv(envDatabaseSaveDataNewMessage, "yes")
	setValidWhatsAppEnv(t)

	_, err := Load(zerolog.Nop())
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
	}
}

func TestLoadWhatsAppConfiguration(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envGlobalAuthToken, "global-token")
	setValidWhatsAppEnv(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.WhatsApp.QRCodeLimit != 5 {
		t.Fatalf("expected QR limit 5, got %d", cfg.WhatsApp.QRCodeLimit)
	}
	if cfg.WhatsApp.MaximumPairingTime() != 150*time.Second {
		t.Fatalf("expected max pairing time 150s, got %s", cfg.WhatsApp.MaximumPairingTime())
	}
	if cfg.WhatsApp.PairingTimeout != 150*time.Second {
		t.Fatalf("expected pairing timeout 150s, got %s", cfg.WhatsApp.PairingTimeout)
	}
	if cfg.WhatsApp.SessionPhoneClient != DefaultSessionPhoneClient {
		t.Fatalf("expected default session phone client %q, got %q", DefaultSessionPhoneClient, cfg.WhatsApp.SessionPhoneClient)
	}
	if cfg.WhatsApp.SessionPhoneName != DefaultSessionPhoneName {
		t.Fatalf("expected default session phone name %q, got %q", DefaultSessionPhoneName, cfg.WhatsApp.SessionPhoneName)
	}
	if cfg.WhatsApp.AddressCacheTTL != 168*time.Hour {
		t.Fatalf("expected default address cache TTL 168h, got %s", cfg.WhatsApp.AddressCacheTTL)
	}
}

func TestLoadWhatsAppSessionDeviceConfiguration(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envGlobalAuthToken, "global-token")
	t.Setenv(envSessionPhoneClient, " chrome ")
	t.Setenv(envSessionPhoneName, " Linux ")
	setValidWhatsAppEnv(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.WhatsApp.SessionPhoneClient != "chrome" {
		t.Fatalf("expected trimmed session phone client, got %q", cfg.WhatsApp.SessionPhoneClient)
	}
	if cfg.WhatsApp.SessionPhoneName != "Linux" {
		t.Fatalf("expected trimmed session phone name, got %q", cfg.WhatsApp.SessionPhoneName)
	}
}

func TestLoadServerAndLogConfiguration(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envGlobalAuthToken, "global-token")
	t.Setenv(envServerPort, "8084")
	t.Setenv(envLogLevel, "trace")
	setValidWhatsAppEnv(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "8084" {
		t.Fatalf("expected server port 8084, got %q", cfg.Server.Port)
	}
	if cfg.Log.Level != "trace" {
		t.Fatalf("expected log level trace, got %q", cfg.Log.Level)
	}
}

func TestLoadServerPortDefault(t *testing.T) {
	preserveEnv(t)
	t.Setenv(envDocker, "true")
	t.Setenv(envDatabaseURL, "postgres://process")
	t.Setenv(envJWTExpiresIn, "3600")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envGlobalAuthToken, "global-token")
	t.Setenv(envLogLevel, "info")
	setValidWhatsAppEnv(t)

	cfg, err := Load(zerolog.Nop())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != "8084" {
		t.Fatalf("expected default server port 8084, got %q", cfg.Server.Port)
	}
}

func TestLoadInvalidServerAndLogConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "non numeric port", key: envServerPort, value: "abc"},
		{name: "port with host", key: envServerPort, value: "localhost:8084"},
		{name: "port out of range", key: envServerPort, value: "70000"},
		{name: "invalid log level", key: envLogLevel, value: "verbose"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			t.Setenv(envServerPort, "8084")
			t.Setenv(envLogLevel, "trace")
			setValidWhatsAppEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load(zerolog.Nop())
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
			}
		})
	}
}

func TestLoadInvalidWhatsAppConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "zero QR limit", key: envQRCodeLimit, value: "0"},
		{name: "zero QR expiration", key: envQRCodeExpiration, value: "0"},
		{name: "invalid light color", key: envQRCodeLightColor, value: "ffffff"},
		{name: "invalid connect timeout", key: envWhatsAppConnectTimeout, value: "0"},
		{name: "invalid pairing timeout", key: envWhatsAppPairingTimeout, value: "0s"},
		{name: "invalid address cache ttl", key: envWhatsAppAddressCacheTTL, value: "0s"},
		{name: "invalid concurrency", key: envWhatsAppStartupConcurrency, value: "-1"},
		{name: "invalid bool", key: envWhatsAppAutoReconnect, value: "yes"},
		{name: "max delay below initial", key: envWhatsAppReconnectMax, value: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preserveEnv(t)
			t.Setenv(envDocker, "true")
			t.Setenv(envDatabaseURL, "postgres://process")
			t.Setenv(envJWTExpiresIn, "3600")
			t.Setenv(envJWTSecret, "secret")
			t.Setenv(envGlobalAuthToken, "global-token")
			setValidWhatsAppEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load(zerolog.Nop())
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("expected ErrInvalidConfiguration, got %v", err)
			}
		})
	}
}

func preserveEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		envDocker,
		envServerPort,
		envLogLevel,
		envDatabaseURL,
		envDatabaseSaveDataNewMessage,
		envDatabaseSaveMessageUpdate,
		envDatabaseSaveDataContacts,
		envJWTExpiresIn,
		envJWTSecret,
		envGlobalAuthToken,
		envQRCodeLimit,
		envQRCodeExpiration,
		envQRCodeLightColor,
		envQRCodeDarkColor,
		envSessionPhoneClient,
		envSessionPhoneName,
		envWhatsAppPairingTimeout,
		envWhatsAppAutoReconnect,
		envWhatsAppStartupConcurrency,
		envWhatsAppConnectTimeout,
		envWhatsAppReconnectInitial,
		envWhatsAppReconnectMax,
		envWhatsAppProfilePictureTimeout,
		envWhatsAppAddressCacheTTL,
		envWhatsAppSessionStore,
		envWhatsAppSessionSQLiteDSN,
		envWhatsAppSessionPostgresURL,
		envWebhookGlobalURL,
		envWebhookGlobalEnabled,
		envMessageProcessingWorkers,
		envMessageProcessingQueueSize,
		envMessageProcessingTimeout,
		envMessageGroupInfoTimeout,
		envMessageSendTimeout,
	}
	original := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			copyValue := value
			original[key] = &copyValue
		} else {
			original[key] = nil
		}
		os.Unsetenv(key)
	}

	t.Cleanup(func() {
		for key, value := range original {
			if value == nil {
				os.Unsetenv(key)
				continue
			}
			os.Setenv(key, *value)
		}
	})
}

func chdirTemp(t *testing.T) string {
	t.Helper()

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	return dir
}

func writeValidEnvFile(t *testing.T) {
	t.Helper()
	writeEnvFile(t, `DATABASE_URL="postgres://file"
AUTHENTICATION_JWT_EXPIRES_IN="3600"
AUTHENTICATION_JWT_SECRET="file-secret"
AUTHENTICATION_GLOBAL_AUTH_TOKEN="file-global-token"
SERVER_PORT="8084"
LOG_LEVEL="trace"
QRCODE_LIMIT="5"
QRCODE_EXPIRATION_TIME="30"
QRCODE_LIGHT_COLOR="#ffffff"
QRCODE_DARK_COLOR="#198754"
WHATSAPP_AUTO_RECONNECT="true"
WHATSAPP_STARTUP_RECONNECT_CONCURRENCY="5"
WHATSAPP_CONNECT_TIMEOUT="30"
WHATSAPP_RECONNECT_INITIAL_DELAY="2"
WHATSAPP_RECONNECT_MAX_DELAY="60"
WHATSAPP_PROFILE_PICTURE_TIMEOUT="15"
WHATSAPP_PAIRING_TIMEOUT="150s"
WHATSAPP_SESSION_STORE="postgres"
WHATSAPP_SESSION_SQLITE_DSN="file:./data/whatsmeow.db?_foreign_keys=on"
WHATSAPP_SESSION_POSTGRES_URL=""
DATABASE_SAVE_DATA_NEW_MESSAGE="true"
DATABASE_SAVE_MESSAGE_UPDATE="false"
DATABASE_SAVE_DATA_CONTACTS="false"
WEBHOOK_GLOBAL_URL=""
WEBHOOK_GLOBAL_ENABLED="false"
MESSAGE_PROCESSING_WORKERS="4"
MESSAGE_PROCESSING_QUEUE_SIZE="100"
MESSAGE_PROCESSING_TIMEOUT="60s"
MESSAGE_GROUP_INFO_TIMEOUT="30s"
MESSAGE_SEND_TIMEOUT="30s"
`)
}

func writeEnvFile(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(".", environmentFile), []byte(content), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func setValidWhatsAppEnv(t *testing.T) {
	t.Helper()
	t.Setenv(envQRCodeLimit, "5")
	t.Setenv(envQRCodeExpiration, "30")
	t.Setenv(envQRCodeLightColor, "#ffffff")
	t.Setenv(envQRCodeDarkColor, "#198754")
	t.Setenv(envWhatsAppPairingTimeout, "150s")
	t.Setenv(envWhatsAppAutoReconnect, "true")
	t.Setenv(envWhatsAppStartupConcurrency, "5")
	t.Setenv(envWhatsAppConnectTimeout, "30")
	t.Setenv(envWhatsAppReconnectInitial, "2")
	t.Setenv(envWhatsAppReconnectMax, "60")
	t.Setenv(envWhatsAppProfilePictureTimeout, "15")
}
