package message

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	watypes "go.mau.fi/whatsmeow/types"
)

const MaxDelayMilliseconds int64 = 120000

type MessageKind string

const (
	KindText     MessageKind = "text"
	KindLink     MessageKind = "link"
	KindImage    MessageKind = "image"
	KindDocument MessageKind = "document"
	KindVideo    MessageKind = "video"
	KindAudio    MessageKind = "audio"
	KindPTV      MessageKind = "ptv"
	KindContact  MessageKind = "contact"
	KindLocation MessageKind = "location"
)

func validateOptions(options *MessageOptions, kind MessageKind) (*string, time.Duration, error) {
	if options == nil {
		return nil, 0, nil
	}
	var delay time.Duration
	if options.Delay != nil {
		if *options.Delay < 0 {
			return nil, 0, fmt.Errorf("%w: negative delay", ErrDelayInvalid)
		}
		if *options.Delay > MaxDelayMilliseconds {
			return nil, 0, fmt.Errorf("%w: delay too high", ErrDelayInvalid)
		}
		delay = time.Duration(*options.Delay) * time.Millisecond
	}
	var presence *string
	if options.Presence != nil {
		normalized := strings.ToLower(strings.TrimSpace(*options.Presence))
		if normalized != "composing" && normalized != "recording" {
			return nil, 0, fmt.Errorf("%w: unsupported presence", ErrPresenceInvalid)
		}
		presence = &normalized
	}
	if presence == nil && delay > 0 {
		switch kind {
		case KindText, KindLink, KindImage, KindDocument, KindVideo, KindContact, KindLocation:
			value := "composing"
			presence = &value
		case KindAudio, KindPTV:
			value := "recording"
			presence = &value
		}
	}
	if presence != nil && !presenceAllowed(*presence, kind) {
		return nil, 0, fmt.Errorf("%w: incompatible presence", ErrPresenceInvalid)
	}
	return presence, delay, nil
}

func presenceAllowed(presence string, kind MessageKind) bool {
	switch presence {
	case "composing":
		switch kind {
		case KindText, KindLink, KindImage, KindDocument, KindVideo, KindContact, KindLocation:
			return true
		default:
			return false
		}
	case "recording":
		return kind == KindAudio || kind == KindPTV
	default:
		return false
	}
}

func applyPresenceAndDelay(ctx context.Context, client *whatsmeow.Client, to watypes.JID, presence *string, delay time.Duration) error {
	if presence == nil && delay == 0 {
		return nil
	}
	if presence != nil {
		media := watypes.ChatPresenceMediaText
		if *presence == "recording" {
			media = watypes.ChatPresenceMediaAudio
		}
		if err := client.SendChatPresence(ctx, to, watypes.ChatPresenceComposing, media); err != nil {
			return fmt.Errorf("%w: set presence: %w", ErrSendFailed, err)
		}
		defer func() {
			_ = client.SendChatPresence(context.Background(), to, watypes.ChatPresencePaused, watypes.ChatPresenceMediaText)
		}()
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
