package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

var (
	ErrInvalidConfiguration  = errors.New("invalid configuration")
	ErrMissingDatabaseURL    = errors.New("database URL is required")
	ErrMissingJWTSecret      = errors.New("JWT secret is required")
	ErrMissingGlobalAuth     = errors.New("global auth token is required")
	ErrInvalidJWTExpiration  = errors.New("invalid JWT expiration")
	ErrInvalidWhatsAppConfig = errors.New("invalid WhatsApp configuration")
)

type Config struct {
	Server            ServerConfig
	Log               LogConfig
	Database          DatabaseConfig
	Authentication    AuthenticationConfig
	Environment       EnvironmentConfig
	WhatsApp          WhatsAppConfig
	WhatsAppSession   WhatsAppSessionConfig
	Webhook           WebhookConfig
	MessageProcessing MessageProcessingConfig
}

type ServerConfig struct {
	Port string
}

type LogConfig struct {
	Level string
}

type DatabaseConfig struct {
	URL                string
	SaveDataNewMessage bool
	SaveMessageUpdate  bool
	SaveDataContacts   bool
}

type AuthenticationConfig struct {
	JWTSecret           string
	JWTExpiresInSeconds int64
	GlobalAuthToken     string
}

type EnvironmentConfig struct {
	Docker bool
}

type WhatsAppConfig struct {
	QRCodeLimit                 int
	QRCodeExpirationTime        time.Duration
	QRCodeLightColor            string
	QRCodeDarkColor             string
	SessionPhoneClient          string
	SessionPhoneName            string
	PairingTimeout              time.Duration
	AutoReconnect               bool
	StartupReconnectConcurrency int
	ConnectTimeout              time.Duration
	ReconnectInitialDelay       time.Duration
	ReconnectMaxDelay           time.Duration
	ProfilePictureTimeout       time.Duration
	AddressCacheTTL             time.Duration
}

type WhatsAppSessionStore string

const (
	WhatsAppSessionStoreSQLite   WhatsAppSessionStore = "sqlite"
	WhatsAppSessionStorePostgres WhatsAppSessionStore = "postgres"
)

type WhatsAppSessionConfig struct {
	Store       WhatsAppSessionStore
	SQLiteDSN   string
	PostgresURL string
}

func (c WhatsAppSessionConfig) PostgresDSN(mainDatabaseURL string) string {
	if strings.TrimSpace(c.PostgresURL) != "" {
		return strings.TrimSpace(c.PostgresURL)
	}
	return strings.TrimSpace(mainDatabaseURL)
}

type WebhookConfig struct {
	GlobalURL     string
	GlobalEnabled bool
}

type MessageProcessingConfig struct {
	Workers           int
	QueueSize         int
	ProcessingTimeout time.Duration
	GroupInfoTimeout  time.Duration
	SendTimeout       time.Duration
}

func (c WhatsAppConfig) MaximumPairingTime() time.Duration {
	if c.PairingTimeout > 0 {
		return c.PairingTimeout
	}
	return time.Duration(c.QRCodeLimit) * c.QRCodeExpirationTime
}

const (
	envDocker                        = "DOCKER_ENV"
	envServerPort                    = "SERVER_PORT"
	envLogLevel                      = "LOG_LEVEL"
	envDatabaseURL                   = "DATABASE_URL"
	envDatabaseSaveDataNewMessage    = "DATABASE_SAVE_DATA_NEW_MESSAGE"
	envDatabaseSaveMessageUpdate     = "DATABASE_SAVE_MESSAGE_UPDATE"
	envDatabaseSaveDataContacts      = "DATABASE_SAVE_DATA_CONTACTS"
	envJWTExpiresIn                  = "AUTHENTICATION_JWT_EXPIRES_IN"
	envJWTSecret                     = "AUTHENTICATION_JWT_SECRET"
	envGlobalAuthToken               = "AUTHENTICATION_GLOBAL_AUTH_TOKEN"
	envQRCodeLimit                   = "QRCODE_LIMIT"
	envQRCodeExpiration              = "QRCODE_EXPIRATION_TIME"
	envQRCodeLightColor              = "QRCODE_LIGHT_COLOR"
	envQRCodeDarkColor               = "QRCODE_DARK_COLOR"
	envSessionPhoneClient            = "CONFIG_SESSION_PHONE_CLIENT"
	envSessionPhoneName              = "CONFIG_SESSION_PHONE_NAME"
	envWhatsAppPairingTimeout        = "WHATSAPP_PAIRING_TIMEOUT"
	envWhatsAppAutoReconnect         = "WHATSAPP_AUTO_RECONNECT"
	envWhatsAppStartupConcurrency    = "WHATSAPP_STARTUP_RECONNECT_CONCURRENCY"
	envWhatsAppConnectTimeout        = "WHATSAPP_CONNECT_TIMEOUT"
	envWhatsAppReconnectInitial      = "WHATSAPP_RECONNECT_INITIAL_DELAY"
	envWhatsAppReconnectMax          = "WHATSAPP_RECONNECT_MAX_DELAY"
	envWhatsAppProfilePictureTimeout = "WHATSAPP_PROFILE_PICTURE_TIMEOUT"
	envWhatsAppAddressCacheTTL       = "WHATSAPP_ADDRESS_CACHE_TTL"
	envWhatsAppSessionStore          = "WHATSAPP_SESSION_STORE"
	envWhatsAppSessionSQLiteDSN      = "WHATSAPP_SESSION_SQLITE_DSN"
	envWhatsAppSessionPostgresURL    = "WHATSAPP_SESSION_POSTGRES_URL"
	envWebhookGlobalURL              = "WEBHOOK_GLOBAL_URL"
	envWebhookGlobalEnabled          = "WEBHOOK_GLOBAL_ENABLED"
	envMessageProcessingWorkers      = "MESSAGE_PROCESSING_WORKERS"
	envMessageProcessingQueueSize    = "MESSAGE_PROCESSING_QUEUE_SIZE"
	envMessageProcessingTimeout      = "MESSAGE_PROCESSING_TIMEOUT"
	envMessageGroupInfoTimeout       = "MESSAGE_GROUP_INFO_TIMEOUT"
	envMessageSendTimeout            = "MESSAGE_SEND_TIMEOUT"
	environmentFile                  = ".env"
	maxDurationSeconds               = int64(math.MaxInt64) / int64(time.Second)
	DefaultSessionPhoneClient        = "DESKTOP"
	DefaultSessionPhoneName          = "CodeChat"
	defaultWhatsAppSessionSQLiteDSN  = "file:./data/whatsmeow.db?_foreign_keys=on"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func Load(logger zerolog.Logger) (Config, error) {
	dockerEnvironment, err := parseDockerEnvironment(os.Getenv(envDocker))
	if err != nil {
		wrapped := fmt.Errorf("%w: %w", ErrInvalidConfiguration, err)
		logger.Error().Err(wrapped).Str("variable", envDocker).Msg("failed to validate environment mode")
		return Config{}, wrapped
	}

	if !dockerEnvironment {
		if err := godotenv.Load(environmentFile); err != nil {
			wrapped := fmt.Errorf("%w: load environment file: .env file not found: %w", ErrInvalidConfiguration, err)
			logger.Error().Err(wrapped).Str("file", environmentFile).Msg("failed to load environment file")
			return Config{}, wrapped
		}
	}

	config, err := buildConfig(dockerEnvironment)
	if err != nil {
		wrapped := fmt.Errorf("%w: %w", ErrInvalidConfiguration, err)
		logger.Error().Err(wrapped).Msg("failed to validate application configuration")
		return Config{}, wrapped
	}

	logger.Info().
		Bool("dockerEnvironment", config.Environment.Docker).
		Bool("jwtExpirationEnabled", config.Authentication.JWTExpiresInSeconds > 0).
		Msg("application configuration loaded")

	return config, nil
}

func LoadConfig() (*Config, error) {
	cfg, err := Load(zerolog.Nop())
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseDockerEnvironment(value string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("invalid %s value", envDocker)
	}
}

func buildConfig(dockerEnvironment bool) (Config, error) {
	serverPort, err := parseServerPort(os.Getenv(envServerPort))
	if err != nil {
		return Config{}, err
	}
	logLevel, err := parseLogLevel(os.Getenv(envLogLevel))
	if err != nil {
		return Config{}, err
	}

	databaseURL := strings.TrimSpace(os.Getenv(envDatabaseURL))
	saveDataNewMessage, err := parseOptionalBoolEnv(envDatabaseSaveDataNewMessage, true)
	if err != nil {
		return Config{}, err
	}
	saveMessageUpdate, err := parseOptionalBoolEnv(envDatabaseSaveMessageUpdate, false)
	if err != nil {
		return Config{}, err
	}
	saveDataContacts, err := parseOptionalBoolEnv(envDatabaseSaveDataContacts, false)
	if err != nil {
		return Config{}, err
	}

	jwtSecret := os.Getenv(envJWTSecret)
	if strings.TrimSpace(jwtSecret) == "" {
		return Config{}, ErrMissingJWTSecret
	}

	globalAuthToken := os.Getenv(envGlobalAuthToken)
	if strings.TrimSpace(globalAuthToken) == "" {
		return Config{}, ErrMissingGlobalAuth
	}

	expiresInSeconds, err := parseJWTExpiration(os.Getenv(envJWTExpiresIn))
	if err != nil {
		return Config{}, err
	}
	whatsAppConfig, err := buildWhatsAppConfig()
	if err != nil {
		return Config{}, err
	}
	whatsAppSessionConfig, err := buildWhatsAppSessionConfig(databaseURL)
	if err != nil {
		return Config{}, err
	}
	if databaseURL == "" {
		return Config{}, ErrMissingDatabaseURL
	}
	webhookConfig, err := buildWebhookConfig()
	if err != nil {
		return Config{}, err
	}
	messageProcessingConfig, err := buildMessageProcessingConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: ServerConfig{
			Port: serverPort,
		},
		Log: LogConfig{
			Level: logLevel,
		},
		Database: DatabaseConfig{
			URL:                databaseURL,
			SaveDataNewMessage: saveDataNewMessage,
			SaveMessageUpdate:  saveMessageUpdate,
			SaveDataContacts:   saveDataContacts,
		},
		Authentication: AuthenticationConfig{
			JWTSecret:           jwtSecret,
			JWTExpiresInSeconds: expiresInSeconds,
			GlobalAuthToken:     globalAuthToken,
		},
		Environment: EnvironmentConfig{
			Docker: dockerEnvironment,
		},
		WhatsApp:          whatsAppConfig,
		WhatsAppSession:   whatsAppSessionConfig,
		Webhook:           webhookConfig,
		MessageProcessing: messageProcessingConfig,
	}, nil
}

func parseServerPort(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "8084", nil
	}
	if strings.Contains(normalized, ":") {
		return "", fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, envServerPort)
	}
	port, err := strconv.Atoi(normalized)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, envServerPort)
	}
	return normalized, nil
}

func parseLogLevel(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "info", nil
	}
	if _, err := zerolog.ParseLevel(normalized); err != nil {
		return "", fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, envLogLevel)
	}
	return normalized, nil
}

func parseJWTExpiration(value string) (int64, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return 0, ErrInvalidJWTExpiration
	}

	expiresInSeconds, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: parse seconds: %w", ErrInvalidJWTExpiration, err)
	}

	if expiresInSeconds < 0 {
		return 0, ErrInvalidJWTExpiration
	}

	if expiresInSeconds > maxDurationSeconds {
		return 0, fmt.Errorf("%w: seconds overflow duration", ErrInvalidJWTExpiration)
	}

	return expiresInSeconds, nil
}

func buildWhatsAppConfig() (WhatsAppConfig, error) {
	qrLimit, err := parsePositiveIntEnv(envQRCodeLimit)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	qrExpiration, err := parsePositiveDurationSecondsEnv(envQRCodeExpiration)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	lightColor := strings.TrimSpace(os.Getenv(envQRCodeLightColor))
	if !hexColorPattern.MatchString(lightColor) {
		return WhatsAppConfig{}, fmt.Errorf("%w: invalid %s", ErrInvalidWhatsAppConfig, envQRCodeLightColor)
	}
	darkColor := strings.TrimSpace(os.Getenv(envQRCodeDarkColor))
	if !hexColorPattern.MatchString(darkColor) {
		return WhatsAppConfig{}, fmt.Errorf("%w: invalid %s", ErrInvalidWhatsAppConfig, envQRCodeDarkColor)
	}
	sessionPhoneClient := strings.TrimSpace(os.Getenv(envSessionPhoneClient))
	if sessionPhoneClient == "" {
		sessionPhoneClient = DefaultSessionPhoneClient
	}
	sessionPhoneName := strings.TrimSpace(os.Getenv(envSessionPhoneName))
	if sessionPhoneName == "" {
		sessionPhoneName = DefaultSessionPhoneName
	}
	pairingTimeout, err := parseOptionalPositiveDurationEnv(envWhatsAppPairingTimeout, 3*time.Minute)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	autoReconnect, err := parseBoolEnv(envWhatsAppAutoReconnect)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	startupConcurrency, err := parsePositiveIntEnv(envWhatsAppStartupConcurrency)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	connectTimeout, err := parsePositiveDurationSecondsEnv(envWhatsAppConnectTimeout)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	reconnectInitial, err := parsePositiveDurationSecondsEnv(envWhatsAppReconnectInitial)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	reconnectMax, err := parsePositiveDurationSecondsEnv(envWhatsAppReconnectMax)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	if reconnectMax < reconnectInitial {
		return WhatsAppConfig{}, fmt.Errorf("%w: %s less than %s", ErrInvalidWhatsAppConfig, envWhatsAppReconnectMax, envWhatsAppReconnectInitial)
	}
	profileTimeout, err := parsePositiveDurationSecondsEnv(envWhatsAppProfilePictureTimeout)
	if err != nil {
		return WhatsAppConfig{}, err
	}
	addressCacheTTL, err := parseOptionalPositiveDurationEnv(envWhatsAppAddressCacheTTL, 168*time.Hour)
	if err != nil {
		return WhatsAppConfig{}, err
	}

	return WhatsAppConfig{
		QRCodeLimit:                 qrLimit,
		QRCodeExpirationTime:        qrExpiration,
		QRCodeLightColor:            lightColor,
		QRCodeDarkColor:             darkColor,
		SessionPhoneClient:          sessionPhoneClient,
		SessionPhoneName:            sessionPhoneName,
		PairingTimeout:              pairingTimeout,
		AutoReconnect:               autoReconnect,
		StartupReconnectConcurrency: startupConcurrency,
		ConnectTimeout:              connectTimeout,
		ReconnectInitialDelay:       reconnectInitial,
		ReconnectMaxDelay:           reconnectMax,
		ProfilePictureTimeout:       profileTimeout,
		AddressCacheTTL:             addressCacheTTL,
	}, nil
}

func buildWhatsAppSessionConfig(mainDatabaseURL string) (WhatsAppSessionConfig, error) {
	storeValue, storeSet := os.LookupEnv(envWhatsAppSessionStore)
	if !storeSet {
		storeValue = string(WhatsAppSessionStorePostgres)
	}
	normalizedStore := strings.ToLower(strings.TrimSpace(storeValue))
	if normalizedStore == "" {
		return WhatsAppSessionConfig{}, fmt.Errorf("%w: invalid %s: expected sqlite or postgres", ErrInvalidWhatsAppConfig, envWhatsAppSessionStore)
	}

	sqliteDSN, sqliteDSNSet := os.LookupEnv(envWhatsAppSessionSQLiteDSN)
	if !sqliteDSNSet {
		sqliteDSN = defaultWhatsAppSessionSQLiteDSN
	}
	sqliteDSN = strings.TrimSpace(sqliteDSN)
	postgresURL := strings.TrimSpace(os.Getenv(envWhatsAppSessionPostgresURL))

	cfg := WhatsAppSessionConfig{
		Store:       WhatsAppSessionStore(normalizedStore),
		SQLiteDSN:   sqliteDSN,
		PostgresURL: postgresURL,
	}

	switch cfg.Store {
	case WhatsAppSessionStoreSQLite:
		if cfg.SQLiteDSN == "" {
			return WhatsAppSessionConfig{}, fmt.Errorf("%w: %s is required when %s=sqlite", ErrInvalidWhatsAppConfig, envWhatsAppSessionSQLiteDSN, envWhatsAppSessionStore)
		}
		cfg.PostgresURL = ""
	case WhatsAppSessionStorePostgres:
		if cfg.PostgresDSN(mainDatabaseURL) == "" {
			return WhatsAppSessionConfig{}, fmt.Errorf("%w: %s or %s is required when %s=postgres", ErrInvalidWhatsAppConfig, envWhatsAppSessionPostgresURL, envDatabaseURL, envWhatsAppSessionStore)
		}
	default:
		return WhatsAppSessionConfig{}, fmt.Errorf("%w: invalid %s: expected sqlite or postgres", ErrInvalidWhatsAppConfig, envWhatsAppSessionStore)
	}

	return cfg, nil
}

func buildWebhookConfig() (WebhookConfig, error) {
	enabled, err := parseOptionalBoolEnv(envWebhookGlobalEnabled, false)
	if err != nil {
		return WebhookConfig{}, err
	}
	globalURL := strings.TrimSpace(os.Getenv(envWebhookGlobalURL))
	if globalURL != "" {
		if err := validateHTTPURL(globalURL); err != nil {
			return WebhookConfig{}, fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, envWebhookGlobalURL)
		}
	}
	if enabled && globalURL == "" {
		return WebhookConfig{}, fmt.Errorf("%w: %s required when %s is true", ErrInvalidConfiguration, envWebhookGlobalURL, envWebhookGlobalEnabled)
	}
	return WebhookConfig{GlobalURL: globalURL, GlobalEnabled: enabled}, nil
}

func buildMessageProcessingConfig() (MessageProcessingConfig, error) {
	workers, err := parseOptionalPositiveIntEnv(envMessageProcessingWorkers, 4)
	if err != nil {
		return MessageProcessingConfig{}, err
	}
	queueSize, err := parseOptionalPositiveIntEnv(envMessageProcessingQueueSize, 100)
	if err != nil {
		return MessageProcessingConfig{}, err
	}
	processingTimeout, err := parseOptionalPositiveDurationEnv(envMessageProcessingTimeout, 60*time.Second)
	if err != nil {
		return MessageProcessingConfig{}, err
	}
	groupInfoTimeout, err := parseOptionalPositiveDurationEnv(envMessageGroupInfoTimeout, 30*time.Second)
	if err != nil {
		return MessageProcessingConfig{}, err
	}
	sendTimeout, err := parseOptionalPositiveDurationEnv(envMessageSendTimeout, 30*time.Second)
	if err != nil {
		return MessageProcessingConfig{}, err
	}
	return MessageProcessingConfig{
		Workers:           workers,
		QueueSize:         queueSize,
		ProcessingTimeout: processingTimeout,
		GroupInfoTimeout:  groupInfoTimeout,
		SendTimeout:       sendTimeout,
	}, nil
}

func validateHTTPURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Host == "" {
		return ErrInvalidConfiguration
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrInvalidConfiguration
	}
	return nil
}

func parsePositiveIntEnv(key string) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, fmt.Errorf("%w: missing %s", ErrInvalidWhatsAppConfig, key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidWhatsAppConfig, key)
	}
	return parsed, nil
}

func parseOptionalPositiveIntEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, key)
	}
	return parsed, nil
}

func parsePositiveDurationSecondsEnv(key string) (time.Duration, error) {
	seconds, err := parsePositiveIntEnv(key)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseOptionalPositiveDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidWhatsAppConfig, key)
	}
	return parsed, nil
}

func parseBoolEnv(key string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%w: invalid %s", ErrInvalidWhatsAppConfig, key)
	}
}

func parseOptionalBoolEnv(key string, fallback bool) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "":
		return fallback, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%w: invalid %s", ErrInvalidConfiguration, key)
	}
}
