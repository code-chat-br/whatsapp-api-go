package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waAdv"
	watypes "go.mau.fi/whatsmeow/types"

	"whatsapp-go-api/internal/config"
)

func TestPostgresURLForSQLStoreAddsSSLModeDisableToURL(t *testing.T) {
	got := postgresURLForSQLStore("postgresql://user:pass@localhost:5432/app")
	want := "postgresql://user:pass@localhost:5432/app?sslmode=disable"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPostgresURLForSQLStorePreservesExplicitSSLMode(t *testing.T) {
	input := "postgres://user:pass@localhost:5432/app?sslmode=require"
	if got := postgresURLForSQLStore(input); got != input {
		t.Fatalf("expected explicit sslmode to be preserved, got %q", got)
	}
}

func TestPostgresURLForSQLStoreAddsSSLModeDisableToKeywordDSN(t *testing.T) {
	got := postgresURLForSQLStore("host=localhost user=postgres dbname=app")
	want := "host=localhost user=postgres dbname=app sslmode=disable"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWhatsAppSessionConfigPostgresDSNResolution(t *testing.T) {
	mainDatabaseURL := "postgres://api:secret@localhost:5432/app"
	dedicatedDatabaseURL := "postgres://sessions:secret@localhost:5432/sessions"

	dedicated := config.WhatsAppSessionConfig{
		Store:       config.WhatsAppSessionStorePostgres,
		PostgresURL: dedicatedDatabaseURL,
	}
	if got := dedicated.PostgresDSN(mainDatabaseURL); got != dedicatedDatabaseURL {
		t.Fatalf("expected dedicated URL, got %q", got)
	}

	shared := config.WhatsAppSessionConfig{Store: config.WhatsAppSessionStorePostgres}
	if got := shared.PostgresDSN(mainDatabaseURL); got != mainDatabaseURL {
		t.Fatalf("expected main database URL fallback, got %q", got)
	}
}

func TestEnsureSQLiteParentDirectoryCreatesDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions", "whatsmeow.db")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_foreign_keys=on&cache=shared"

	if err := ensureSQLiteParentDirectory(dsn); err != nil {
		t.Fatalf("ensureSQLiteParentDirectory() error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("expected SQLite parent directory to exist: %v", err)
	}
}

func TestNewWhatsAppSessionContainerSQLitePersistsDevice(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "nested", "whatsmeow.db")
	cfg := config.WhatsAppSessionConfig{
		Store:     config.WhatsAppSessionStoreSQLite,
		SQLiteDSN: "file:" + filepath.ToSlash(dbPath) + "?_foreign_keys=on",
	}

	container, err := NewWhatsAppSessionContainer(ctx, cfg, "postgres://api:secret@localhost:5432/app", nil)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite integration requires CGO-enabled go-sqlite3: %v", err)
		}
		t.Fatalf("NewWhatsAppSessionContainer() error = %v", err)
	}
	jid := watypes.NewJID("5511999999999", watypes.DefaultUserServer)
	device := container.NewDevice()
	device.ID = &jid
	device.Account = &waAdv.ADVSignedDeviceIdentity{}
	device.Platform = "test"
	if err := device.Save(ctx); err != nil {
		_ = container.Close()
		t.Fatalf("save device: %v", err)
	}
	if err := container.Close(); err != nil {
		t.Fatalf("close first container: %v", err)
	}

	reopened, err := NewWhatsAppSessionContainer(ctx, cfg, "postgres://api:secret@localhost:5432/app", nil)
	if err != nil {
		t.Fatalf("reopen container: %v", err)
	}
	defer reopened.Close()
	persisted, err := reopened.GetDevice(ctx, jid)
	if err != nil {
		t.Fatalf("get persisted device: %v", err)
	}
	if persisted == nil || persisted.ID == nil || persisted.ID.String() != jid.String() {
		t.Fatalf("expected persisted device %q, got %#v", jid.String(), persisted)
	}
}
