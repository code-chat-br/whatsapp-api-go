package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewConfiguresSessionDeviceBeforeClients(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	sourcePath := filepath.Join(filepath.Dir(filename), "app.go")
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read app.go: %v", err)
	}
	source := string(content)

	configureIndex := strings.Index(source, "whatsapp.ConfigureSessionDevice")
	databaseIndex := strings.Index(source, "postgres.NewPostgresPool")
	factoryIndex := strings.Index(source, "whatsapp.NewSQLStoreClientFactory")
	serviceIndex := strings.Index(source, "whatsapp.NewService")

	if configureIndex < 0 {
		t.Fatal("ConfigureSessionDevice call not found")
	}
	for name, index := range map[string]int{
		"postgres.NewPostgresPool":          databaseIndex,
		"whatsapp.NewSQLStoreClientFactory": factoryIndex,
		"whatsapp.NewService":               serviceIndex,
	} {
		if index < 0 {
			t.Fatalf("%s call not found", name)
		}
		if configureIndex > index {
			t.Fatalf("ConfigureSessionDevice must run before %s", name)
		}
	}
}
