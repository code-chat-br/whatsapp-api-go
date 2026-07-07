package message

import (
	"strings"

	"github.com/rs/zerolog"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func mentionAllEnabled(options *MessageOptions) bool {
	return options != nil && options.MentionAll != nil && *options.MentionAll
}

func cloneOptions(options *MessageOptions) *MessageOptions {
	if options == nil {
		return nil
	}
	cloned := *options
	cloned.QuotedMessage = cloneMap(options.QuotedMessage)
	cloned.ExternalAttributes = cloneMap(options.ExternalAttributes)
	return &cloned
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneAny(value)
	}
	return output
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = cloneAny(typed[i])
		}
		return out
	default:
		return typed
	}
}

func mentionedJIDsFromParticipants(participants []watypes.GroupParticipant, logger zerolog.Logger, processID string, instanceName string, remoteJID watypes.JID) []string {
	seen := make(map[string]struct{}, len(participants))
	mentioned := make([]string, 0, len(participants))
	for _, participant := range participants {
		jid := firstValidMentionJID(participant)
		if jid.IsEmpty() || jid.User == "" || jid.Server == "" {
			logger.Warn().
				Str("processId", processID).
				Str("instanceName", instanceName).
				Str("remoteJid", remoteJID.String()).
				Msg("ignoring invalid group participant jid")
			continue
		}
		normalized := jid.ToNonAD().String()
		if strings.TrimSpace(normalized) == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		mentioned = append(mentioned, normalized)
	}
	return mentioned
}

func firstValidMentionJID(participant watypes.GroupParticipant) watypes.JID {
	for _, jid := range []watypes.JID{participant.JID, participant.PhoneNumber, participant.LID} {
		if !jid.IsEmpty() && jid.User != "" && jid.Server != watypes.GroupServer {
			return jid.ToNonAD()
		}
	}
	return watypes.JID{}
}

func cloneMessage(message *wae2e.Message) *wae2e.Message {
	if message == nil {
		return nil
	}
	cloned, ok := proto.Clone(message).(*wae2e.Message)
	if !ok {
		return nil
	}
	return cloned
}

func applyMentionedJIDs(message *wae2e.Message, mentioned []string) {
	if message == nil || len(mentioned) == 0 {
		return
	}
	switch {
	case message.ExtendedTextMessage != nil:
		info := message.ExtendedTextMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.ExtendedTextMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.ImageMessage != nil:
		info := message.ImageMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.ImageMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.DocumentMessage != nil:
		info := message.DocumentMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.DocumentMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.VideoMessage != nil:
		info := message.VideoMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.VideoMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.PtvMessage != nil:
		info := message.PtvMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.PtvMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.AudioMessage != nil:
		info := message.AudioMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.AudioMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.ContactMessage != nil:
		info := message.ContactMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.ContactMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.ContactsArrayMessage != nil:
		info := message.ContactsArrayMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.ContactsArrayMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	case message.LocationMessage != nil:
		info := message.LocationMessage.ContextInfo
		if info == nil {
			info = &wae2e.ContextInfo{}
			message.LocationMessage.ContextInfo = info
		}
		info.MentionedJID = mergeMentionedJIDs(info.MentionedJID, mentioned)
	}
}

func mergeMentionedJIDs(existing []string, additional []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(additional))
	merged := make([]string, 0, len(existing)+len(additional))
	for _, jid := range append(existing, additional...) {
		normalized := strings.TrimSpace(jid)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		merged = append(merged, normalized)
	}
	return merged
}
