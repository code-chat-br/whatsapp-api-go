package whatsapp

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"

	"whatsapp-go-api/internal/config"
)

const maxSessionPhoneNameRunes = 64

type SessionDeviceConfig struct {
	Client string
	Name   string
}

var sessionPhonePlatforms = map[string]waCompanionReg.DeviceProps_PlatformType{
	"CHROME":            waCompanionReg.DeviceProps_CHROME,
	"FIREFOX":           waCompanionReg.DeviceProps_FIREFOX,
	"IE":                waCompanionReg.DeviceProps_IE,
	"OPERA":             waCompanionReg.DeviceProps_OPERA,
	"SAFARI":            waCompanionReg.DeviceProps_SAFARI,
	"EDGE":              waCompanionReg.DeviceProps_EDGE,
	"DESKTOP":           waCompanionReg.DeviceProps_DESKTOP,
	"IPAD":              waCompanionReg.DeviceProps_IPAD,
	"ANDROID_TABLET":    waCompanionReg.DeviceProps_ANDROID_TABLET,
	"OHANA":             waCompanionReg.DeviceProps_OHANA,
	"ALOHA":             waCompanionReg.DeviceProps_ALOHA,
	"CATALINA":          waCompanionReg.DeviceProps_CATALINA,
	"TCL_TV":            waCompanionReg.DeviceProps_TCL_TV,
	"IOS_PHONE":         waCompanionReg.DeviceProps_IOS_PHONE,
	"IOS_CATALYST":      waCompanionReg.DeviceProps_IOS_CATALYST,
	"ANDROID_PHONE":     waCompanionReg.DeviceProps_ANDROID_PHONE,
	"ANDROID_AMBIGUOUS": waCompanionReg.DeviceProps_ANDROID_AMBIGUOUS,
	"WEAR_OS":           waCompanionReg.DeviceProps_WEAR_OS,
	"AR_WRIST":          waCompanionReg.DeviceProps_AR_WRIST,
	"AR_DEVICE":         waCompanionReg.DeviceProps_AR_DEVICE,
	"UWP":               waCompanionReg.DeviceProps_UWP,
	"VR":                waCompanionReg.DeviceProps_VR,
	"CLOUD_API":         waCompanionReg.DeviceProps_CLOUD_API,
	"SMARTGLASSES":      waCompanionReg.DeviceProps_SMARTGLASSES,
}

func ConfigureSessionDevice(cfg SessionDeviceConfig, logger zerolog.Logger) error {
	name, err := normalizeSessionPhoneName(cfg.Name)
	if err != nil {
		return err
	}
	platformName := strings.TrimSpace(cfg.Client)
	if platformName == "" {
		platformName = config.DefaultSessionPhoneClient
	}
	platform, err := parseSessionPhonePlatform(platformName)
	if err != nil {
		return err
	}

	store.DeviceProps.Os = &name
	store.DeviceProps.PlatformType = platform.Enum()

	logger.Info().
		Str("platform", strings.ToLower(platform.String())).
		Str("os", name).
		Msg("device")

	return nil
}

func normalizeSessionPhoneName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		name = config.DefaultSessionPhoneName
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("invalid CONFIG_SESSION_PHONE_NAME: value must be valid UTF-8")
	}
	if utf8.RuneCountInString(name) > maxSessionPhoneNameRunes {
		return "", fmt.Errorf("invalid CONFIG_SESSION_PHONE_NAME: maximum length is %d characters", maxSessionPhoneNameRunes)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("invalid CONFIG_SESSION_PHONE_NAME: control characters are not supported")
		}
	}
	return name, nil
}

func parseSessionPhonePlatform(value string) (waCompanionReg.DeviceProps_PlatformType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if platform, ok := sessionPhonePlatforms[normalized]; ok {
		return platform, nil
	}
	return waCompanionReg.DeviceProps_UNKNOWN, fmt.Errorf(
		"invalid CONFIG_SESSION_PHONE_CLIENT %q: supported values are %s",
		value,
		strings.Join(SupportedSessionPhonePlatforms(), ", "),
	)
}

func SupportedSessionPhonePlatforms() []string {
	values := make([]string, 0, len(sessionPhonePlatforms))
	for value := range sessionPhonePlatforms {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
