package whatsapp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	watypes "go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"whatsapp-go-api/internal/config"
)

type WhatsAppClientFactory interface {
	NewDeviceClient() (*whatsmeow.Client, error)
	ClientForDevice(ctx context.Context, deviceJID string) (*whatsmeow.Client, error)
	Store() *sqlstore.Container
}

type SQLStoreClientFactory struct {
	container *sqlstore.Container
	logger    waLog.Logger
}

func NewSQLStoreClientFactory(ctx context.Context, sessionConfig config.WhatsAppSessionConfig, mainDatabaseURL string, logger waLog.Logger) (*SQLStoreClientFactory, error) {
	container, err := NewWhatsAppSessionContainer(ctx, sessionConfig, mainDatabaseURL, logger)
	if err != nil {
		return nil, err
	}
	return &SQLStoreClientFactory{container: container, logger: logger}, nil
}

func NewWhatsAppSessionContainer(ctx context.Context, sessionConfig config.WhatsAppSessionConfig, mainDatabaseURL string, logger waLog.Logger) (*sqlstore.Container, error) {
	sqlstore.PostgresArrayWrapper = pq.Array

	var (
		dialect string
		address string
	)

	switch sessionConfig.Store {
	case config.WhatsAppSessionStoreSQLite:
		if err := ensureSQLiteParentDirectory(sessionConfig.SQLiteDSN); err != nil {
			return nil, err
		}
		dialect = "sqlite3"
		address = sessionConfig.SQLiteDSN
	case config.WhatsAppSessionStorePostgres:
		dialect = "postgres"
		address = postgresURLForSQLStore(sessionConfig.PostgresDSN(mainDatabaseURL))
	default:
		return nil, fmt.Errorf("invalid WhatsApp session store")
	}

	container, err := sqlstore.New(ctx, dialect, address, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize whatsmeow sqlstore: %w", err)
	}
	return container, nil
}

func postgresURLForSQLStore(databaseURL string) string {
	normalized := strings.TrimSpace(databaseURL)
	if normalized == "" {
		return normalized
	}

	parsed, err := url.Parse(normalized)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		if query.Get("sslmode") == "" {
			query.Set("sslmode", "disable")
			parsed.RawQuery = query.Encode()
		}
		return parsed.String()
	}

	if strings.Contains(normalized, "sslmode=") {
		return normalized
	}
	return normalized + " sslmode=disable"
}

func ensureSQLiteParentDirectory(dsn string) error {
	dbPath := sqliteDatabasePath(dsn)
	if dbPath == "" {
		return nil
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create whatsmeow sqlite directory: %w", err)
	}
	return nil
}

func sqliteDatabasePath(dsn string) string {
	normalized := strings.TrimSpace(dsn)
	if normalized == "" || strings.Contains(normalized, ":memory:") {
		return ""
	}

	beforeQuery, _, _ := strings.Cut(normalized, "?")
	if strings.HasPrefix(beforeQuery, "file:") {
		rawPath := strings.TrimPrefix(beforeQuery, "file:")
		if strings.HasPrefix(rawPath, "//") {
			parsed, err := url.Parse(beforeQuery)
			if err == nil && parsed.Path != "" {
				rawPath = parsed.Path
			}
		}
		if rawPath == "" || strings.HasPrefix(rawPath, "mode=") {
			return ""
		}
		return filepath.FromSlash(rawPath)
	}

	return filepath.FromSlash(beforeQuery)
}

func (f *SQLStoreClientFactory) NewDeviceClient() (*whatsmeow.Client, error) {
	return f.clientForDevice(f.container.NewDevice()), nil
}

func (f *SQLStoreClientFactory) ClientForDevice(ctx context.Context, deviceJID string) (*whatsmeow.Client, error) {
	jid, err := watypes.ParseJID(deviceJID)
	if err != nil {
		return nil, fmt.Errorf("%w: parse device jid: %w", ErrSessionMissing, err)
	}
	device, err := f.container.GetDevice(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("get whatsmeow device: %w", err)
	}
	if device == nil || device.ID == nil {
		return nil, ErrSessionMissing
	}
	return f.clientForDevice(device), nil
}

func (f *SQLStoreClientFactory) Store() *sqlstore.Container {
	return f.container
}

func (f *SQLStoreClientFactory) clientForDevice(device *store.Device) *whatsmeow.Client {
	return whatsmeow.NewClient(device, f.logger.Sub("client"))
}
