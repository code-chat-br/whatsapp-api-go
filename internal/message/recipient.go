package message

import (
	"fmt"
	"regexp"
	"strings"

	watypes "go.mau.fi/whatsmeow/types"

	"whatsapp-go-api/internal/whatsapp/address"
)

var phonePattern = regexp.MustCompile(`^\d{8,20}$`)

func ResolveRecipient(input RecipientInput) (watypes.JID, error) {
	raw, err := RecipientAddress(input)
	if err != nil {
		return watypes.JID{}, err
	}
	normalized, err := address.NormalizeAddress(raw)
	if err != nil {
		return watypes.JID{}, fmt.Errorf("%w: %w", ErrRecipientInvalid, err)
	}
	if strings.Contains(normalized, "@") {
		jid, err := watypes.ParseJID(normalized)
		if err != nil {
			return watypes.JID{}, fmt.Errorf("%w: %w", ErrRecipientInvalid, err)
		}
		return jid.ToNonAD(), nil
	}
	if !phonePattern.MatchString(normalized) {
		return watypes.JID{}, fmt.Errorf("%w: invalid phone", ErrRecipientInvalid)
	}
	return watypes.NewJID(normalized, watypes.DefaultUserServer), nil
}

func RecipientAddress(input RecipientInput) (string, error) {
	values := make([]string, 0, 3)
	for _, value := range []*string{input.Number, input.Chat, input.Recipient} {
		if value != nil {
			values = append(values, strings.TrimSpace(*value))
		}
	}
	if len(values) != 1 {
		return "", fmt.Errorf("%w: exactly one recipient alias is required", ErrRecipientInvalid)
	}
	raw := values[0]
	if raw == "" {
		return "", fmt.Errorf("%w: recipient cannot be empty", ErrRecipientInvalid)
	}
	return raw, nil
}

func normalizePhone(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("+", "", " ", "", "-", "", "(", "", ")", "", ".", "")
	return replacer.Replace(value)
}

func validRecipientJID(jid watypes.JID) bool {
	if jid.User == "" || jid.Device != 0 || jid.RawAgent != 0 {
		return false
	}
	switch jid.Server {
	case watypes.DefaultUserServer:
		return phonePattern.MatchString(jid.User)
	case watypes.GroupServer:
		return phonePattern.MatchString(jid.User)
	default:
		return false
	}
}
