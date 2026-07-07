package whatsapp

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
)

func TestConfigureSessionDeviceValidValues(t *testing.T) {
	restore := preserveDeviceProps(t)

	if err := ConfigureSessionDevice(SessionDeviceConfig{
		Client: "CHROME",
		Name:   "Linux",
	}, zerolog.Nop()); err != nil {
		t.Fatalf("ConfigureSessionDevice() error = %v", err)
	}

	if got := store.DeviceProps.GetPlatformType(); got != waCompanionReg.DeviceProps_CHROME {
		t.Fatalf("expected platform CHROME, got %s", got.String())
	}
	if got := store.DeviceProps.GetOs(); got != "Linux" {
		t.Fatalf("expected OS Linux, got %q", got)
	}
	restore()
}

func TestConfigureSessionDeviceNormalizesValues(t *testing.T) {
	restore := preserveDeviceProps(t)

	if err := ConfigureSessionDevice(SessionDeviceConfig{
		Client: " chrome ",
		Name:   " Linux ",
	}, zerolog.Nop()); err != nil {
		t.Fatalf("ConfigureSessionDevice() error = %v", err)
	}

	if got := store.DeviceProps.GetPlatformType(); got != waCompanionReg.DeviceProps_CHROME {
		t.Fatalf("expected platform CHROME, got %s", got.String())
	}
	if got := store.DeviceProps.GetOs(); got != "Linux" {
		t.Fatalf("expected OS Linux, got %q", got)
	}
	restore()
}

func TestConfigureSessionDeviceDefaults(t *testing.T) {
	restore := preserveDeviceProps(t)

	if err := ConfigureSessionDevice(SessionDeviceConfig{}, zerolog.Nop()); err != nil {
		t.Fatalf("ConfigureSessionDevice() error = %v", err)
	}

	if got := store.DeviceProps.GetPlatformType(); got != waCompanionReg.DeviceProps_DESKTOP {
		t.Fatalf("expected platform DESKTOP, got %s", got.String())
	}
	if got := store.DeviceProps.GetOs(); got != "CodeChat" {
		t.Fatalf("expected OS CodeChat, got %q", got)
	}
	restore()
}

func TestConfigureSessionDeviceInvalidPlatformDoesNotMutateDeviceProps(t *testing.T) {
	restore := preserveDeviceProps(t)
	originalOS := store.DeviceProps.GetOs()
	originalPlatform := store.DeviceProps.GetPlatformType()

	err := ConfigureSessionDevice(SessionDeviceConfig{
		Client: "CHROMIUM_INVALID",
		Name:   "Valid Name",
	}, zerolog.Nop())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `CONFIG_SESSION_PHONE_CLIENT "CHROMIUM_INVALID"`) {
		t.Fatalf("expected invalid value in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "CHROME") || !strings.Contains(err.Error(), "DESKTOP") {
		t.Fatalf("expected supported values in error, got %v", err)
	}
	if got := store.DeviceProps.GetOs(); got != originalOS {
		t.Fatalf("expected OS to stay %q, got %q", originalOS, got)
	}
	if got := store.DeviceProps.GetPlatformType(); got != originalPlatform {
		t.Fatalf("expected platform to stay %s, got %s", originalPlatform.String(), got.String())
	}
	restore()
}

func TestConfigureSessionDeviceInvalidNameDoesNotMutateDeviceProps(t *testing.T) {
	restore := preserveDeviceProps(t)
	originalOS := store.DeviceProps.GetOs()
	originalPlatform := store.DeviceProps.GetPlatformType()

	err := ConfigureSessionDevice(SessionDeviceConfig{
		Client: "CHROME",
		Name:   "CodeChat\nInvalid",
	}, zerolog.Nop())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "CONFIG_SESSION_PHONE_NAME") {
		t.Fatalf("expected env name in error, got %v", err)
	}
	if got := store.DeviceProps.GetOs(); got != originalOS {
		t.Fatalf("expected OS to stay %q, got %q", originalOS, got)
	}
	if got := store.DeviceProps.GetPlatformType(); got != originalPlatform {
		t.Fatalf("expected platform to stay %s, got %s", originalPlatform.String(), got.String())
	}
	restore()
}

func TestParseSessionPhonePlatformSupportsInstalledEnumValues(t *testing.T) {
	for _, value := range []string{
		"CHROME",
		"FIREFOX",
		"IE",
		"OPERA",
		"SAFARI",
		"EDGE",
		"DESKTOP",
		"IPAD",
		"ANDROID_TABLET",
		"OHANA",
		"ALOHA",
		"CATALINA",
		"TCL_TV",
		"IOS_PHONE",
		"IOS_CATALYST",
		"ANDROID_PHONE",
		"ANDROID_AMBIGUOUS",
		"WEAR_OS",
		"AR_WRIST",
		"AR_DEVICE",
		"UWP",
		"VR",
		"CLOUD_API",
		"SMARTGLASSES",
	} {
		t.Run(value, func(t *testing.T) {
			if _, err := parseSessionPhonePlatform(value); err != nil {
				t.Fatalf("parseSessionPhonePlatform(%q) error = %v", value, err)
			}
		})
	}
}

func preserveDeviceProps(t *testing.T) func() {
	t.Helper()
	originalOS := store.DeviceProps.Os
	originalPlatform := store.DeviceProps.PlatformType

	restore := func() {
		store.DeviceProps.Os = originalOS
		store.DeviceProps.PlatformType = originalPlatform
	}
	t.Cleanup(restore)
	return restore
}
