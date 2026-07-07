package address

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"go.mau.fi/whatsmeow/types"
)

type parsedAddress struct {
	number string
	jid    types.JID
	direct bool
}

func NormalizeAddress(value string) (string, error) {
	parsed, err := parseAddress(value)
	if err != nil {
		return "", err
	}
	if parsed.direct {
		return parsed.jid.String(), nil
	}
	return parsed.number, nil
}

func LegacyBrazilianNumberWithoutNinthDigit(number string) (string, bool) {
	if len(number) != 13 || !allDigits(number) || number[:2] != "55" || number[4] != '9' {
		return number, false
	}
	ddd, err := strconv.Atoi(number[2:4])
	if err != nil || ddd < 31 {
		return number, false
	}
	oldNumber := number[5:]
	if oldNumber[0] < '7' {
		return number, false
	}
	return number[:4] + oldNumber, true
}

func BuildCandidates(number string) []string {
	candidates := []string{number}
	if withoutNinth, ok := LegacyBrazilianNumberWithoutNinthDigit(number); ok {
		candidates = appendUnique(candidates, withoutNinth)
		return candidates
	}
	if withNinth, ok := brazilianMobileWithNinthDigit(number); ok {
		candidates = appendUnique(candidates, withNinth)
	}
	return candidates
}

func MaskAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	user, server, hasServer := strings.Cut(trimmed, "@")
	masked := maskDigits(user)
	if hasServer {
		return masked + "@" + server
	}
	return masked
}

func parseAddress(value string) (parsedAddress, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return parsedAddress{}, fmt.Errorf("%w: empty address", ErrInvalidAddress)
	}
	if strings.Contains(raw, "@") {
		jid, err := types.ParseJID(raw)
		if err != nil {
			return parsedAddress{}, fmt.Errorf("%w: %w", ErrInvalidAddress, err)
		}
		if isDirectJID(jid) {
			return parsedAddress{jid: jid, direct: true}, nil
		}
		if jid.Server != types.DefaultUserServer && jid.Server != types.LegacyUserServer {
			return parsedAddress{}, fmt.Errorf("%w: unsupported jid server", ErrInvalidAddress)
		}
		number, err := normalizePhonePart(jid.User)
		if err != nil {
			return parsedAddress{}, err
		}
		return parsedAddress{number: number}, nil
	}
	number, err := normalizePhonePart(raw)
	if err != nil {
		return parsedAddress{}, err
	}
	return parsedAddress{number: number}, nil
}

func isDirectJID(jid types.JID) bool {
	switch jid.Server {
	case types.GroupServer,
		types.HiddenUserServer,
		types.NewsletterServer,
		types.BroadcastServer,
		types.MessengerServer,
		types.InteropServer,
		types.HostedServer,
		types.HostedLIDServer,
		types.BotServer:
		return true
	default:
		return jid.RawAgent != 0 || jid.Device != 0
	}
}

func normalizePhonePart(value string) (string, error) {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
			continue
		case unicode.IsLetter(r):
			return "", fmt.Errorf("%w: unexpected letter in phone", ErrInvalidAddress)
		default:
			return "", fmt.Errorf("%w: unexpected phone character", ErrInvalidAddress)
		}
	}
	number := builder.String()
	if number == "" {
		return "", fmt.Errorf("%w: empty phone", ErrInvalidAddress)
	}
	if len(number) < 8 || len(number) > 20 {
		return "", fmt.Errorf("%w: invalid phone length", ErrInvalidAddress)
	}
	return number, nil
}

func brazilianMobileWithNinthDigit(number string) (string, bool) {
	if len(number) != 12 || !allDigits(number) || number[:2] != "55" {
		return number, false
	}
	ddd, err := strconv.Atoi(number[2:4])
	if err != nil || ddd < 31 {
		return number, false
	}
	subscriber := number[4:]
	if subscriber[0] < '7' {
		return number, false
	}
	return number[:4] + "9" + subscriber, true
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func maskDigits(value string) string {
	runes := []rune(value)
	digitPositions := make([]int, 0, len(runes))
	for i, r := range runes {
		if r >= '0' && r <= '9' {
			digitPositions = append(digitPositions, i)
		}
	}
	if len(digitPositions) <= 8 {
		return value
	}
	for _, pos := range digitPositions[4 : len(digitPositions)-4] {
		runes[pos] = '*'
	}
	return string(runes)
}
