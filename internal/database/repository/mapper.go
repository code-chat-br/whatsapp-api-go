package repository

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

func textFromPtr(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func boolFromPtr(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}

func boolPtr(value pgtype.Bool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
}

func timestamp(value pgtype.Timestamp) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func timestampPtr(value pgtype.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func pgTimestamp(value time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{Time: value, Valid: true}
}

func nullableJSON(value json.RawMessage) []byte {
	if len(value) == 0 {
		return nil
	}
	return []byte(value)
}

func connectionAttempts(value pgtype.Int4) int32 {
	if !value.Valid {
		return 0
	}
	return value.Int32
}

func connectionStatus(value pgtype.Text) types.InstanceConnectionStatus {
	if !value.Valid {
		return types.InstanceConnectionStatusOffline
	}
	status := types.InstanceConnectionStatus(value.String)
	if !status.IsValid() {
		return types.InstanceConnectionStatusOffline
	}
	return status
}

func mapInstance(row db.Instance) types.Instance {
	return types.Instance{
		ID:                 row.ID,
		Name:               row.Name,
		Description:        textPtr(row.Description),
		Status:             types.InstanceStatus(row.ConnectionStatus),
		ConnectionStatus:   types.InstanceConnectionStatusOffline,
		OwnerJid:           textPtr(row.OwnerJid),
		ProfilePicUrl:      textPtr(row.ProfilePicUrl),
		CreatedAt:          timestamp(row.CreatedAt),
		UpdatedAt:          timestamp(row.UpdatedAt),
		ExternalAttributes: json.RawMessage(row.ExternalAttributes),
	}
}

func mapAuth(row db.Auth) types.Auth {
	return types.Auth{
		ID:         row.ID,
		Token:      row.Token,
		CreatedAt:  timestamp(row.CreatedAt),
		UpdatedAt:  timestamp(row.UpdatedAt),
		InstanceID: row.InstanceId,
	}
}

func mapInstanceWithAuthRow(row db.FindInstanceWithAuthByNameRow) types.InstanceWithAuth {
	result := types.InstanceWithAuth{
		Instance: types.Instance{
			ID:                 row.ID,
			Name:               row.Name,
			Description:        textPtr(row.Description),
			Status:             types.InstanceStatus(row.ConnectionStatus),
			ConnectionStatus:   connectionStatus(row.WhatsappConnectionStatus),
			OwnerJid:           textPtr(row.OwnerJid),
			ProfilePicUrl:      textPtr(row.ProfilePicUrl),
			WhatsAppDeviceJid:  textPtr(row.WhatsappDeviceJid),
			WhatsAppOwnerJid:   textPtr(row.WhatsappOwnerJid),
			WhatsAppPhone:      textPtr(row.WhatsappPhoneNumber),
			ProfilePicID:       textPtr(row.ProfilePicId),
			LastConnectedAt:    timestampPtr(row.LastConnectedAt),
			LastDisconnectedAt: timestampPtr(row.LastDisconnectedAt),
			LastAttemptAt:      timestampPtr(row.LastConnectionAttemptAt),
			LastError:          textPtr(row.LastConnectionError),
			LastEvent:          textPtr(row.LastConnectionEvent),
			ConnectionAttempts: connectionAttempts(row.ConnectionAttempts),
			CreatedAt:          timestamp(row.CreatedAt),
			UpdatedAt:          timestamp(row.UpdatedAt),
			ExternalAttributes: json.RawMessage(row.ExternalAttributes),
		},
	}
	if row.AuthId.Valid {
		result.Auth = &types.Auth{
			ID:         row.AuthId.Int32,
			Token:      row.AuthToken.String,
			CreatedAt:  timestamp(row.AuthCreatedAt),
			UpdatedAt:  timestamp(row.AuthUpdatedAt),
			InstanceID: row.AuthInstanceId.Int32,
		}
	}
	return result
}

func mapListInstanceWithAuthRow(row db.ListInstancesWithAuthRow) types.InstanceWithAuth {
	return mapInstanceWithAuthRow(db.FindInstanceWithAuthByNameRow(row))
}

func mapAutoConnectInstanceRow(row db.FindAutoConnectInstancesRow) types.Instance {
	return types.Instance{
		ID:                 row.ID,
		Name:               row.Name,
		Description:        textPtr(row.Description),
		Status:             types.InstanceStatus(row.ConnectionStatus),
		ConnectionStatus:   types.InstanceConnectionStatus(row.WhatsappConnectionStatus),
		OwnerJid:           textPtr(row.OwnerJid),
		ProfilePicUrl:      textPtr(row.ProfilePicUrl),
		WhatsAppDeviceJid:  textPtr(row.WhatsappDeviceJid),
		WhatsAppOwnerJid:   textPtr(row.WhatsappOwnerJid),
		WhatsAppPhone:      textPtr(row.WhatsappPhoneNumber),
		ProfilePicID:       textPtr(row.ProfilePicId),
		LastConnectedAt:    timestampPtr(row.LastConnectedAt),
		LastDisconnectedAt: timestampPtr(row.LastDisconnectedAt),
		LastAttemptAt:      timestampPtr(row.LastConnectionAttemptAt),
		LastError:          textPtr(row.LastConnectionError),
		LastEvent:          textPtr(row.LastConnectionEvent),
		ConnectionAttempts: row.ConnectionAttempts,
		CreatedAt:          timestamp(row.CreatedAt),
		UpdatedAt:          timestamp(row.UpdatedAt),
		ExternalAttributes: json.RawMessage(row.ExternalAttributes),
	}
}

func mapListInstanceDetailsRow(row db.ListInstanceDetailsRow) types.InstanceDetails {
	result := types.InstanceDetails{
		Instance: types.Instance{
			ID:                 row.ID,
			Name:               row.Name,
			Description:        textPtr(row.Description),
			Status:             types.InstanceStatus(row.ConnectionStatus),
			ConnectionStatus:   connectionStatus(row.WhatsappConnectionStatus),
			OwnerJid:           textPtr(row.OwnerJid),
			ProfilePicUrl:      textPtr(row.ProfilePicUrl),
			WhatsAppDeviceJid:  textPtr(row.WhatsappDeviceJid),
			WhatsAppOwnerJid:   textPtr(row.WhatsappOwnerJid),
			WhatsAppPhone:      textPtr(row.WhatsappPhoneNumber),
			ProfilePicID:       textPtr(row.ProfilePicId),
			LastConnectedAt:    timestampPtr(row.LastConnectedAt),
			LastDisconnectedAt: timestampPtr(row.LastDisconnectedAt),
			LastAttemptAt:      timestampPtr(row.LastConnectionAttemptAt),
			LastError:          textPtr(row.LastConnectionError),
			LastEvent:          textPtr(row.LastConnectionEvent),
			ConnectionAttempts: connectionAttempts(row.ConnectionAttempts),
			CreatedAt:          timestamp(row.CreatedAt),
			UpdatedAt:          timestamp(row.UpdatedAt),
			ExternalAttributes: json.RawMessage(row.ExternalAttributes),
		},
	}
	if row.AuthId.Valid {
		result.Auth = &types.Auth{
			ID:         row.AuthId.Int32,
			Token:      row.AuthToken.String,
			CreatedAt:  timestamp(row.AuthCreatedAt),
			UpdatedAt:  timestamp(row.AuthUpdatedAt),
			InstanceID: row.AuthInstanceId.Int32,
		}
	}
	if row.WebhookId.Valid {
		result.Webhook = &types.Webhook{
			ID:         row.WebhookId.Int32,
			URL:        row.WebhookUrl.String,
			Enabled:    row.WebhookEnabled.Bool,
			Events:     json.RawMessage(row.WebhookEvents),
			CreatedAt:  timestamp(row.WebhookCreatedAt),
			UpdatedAt:  timestamp(row.WebhookUpdatedAt),
			InstanceID: row.WebhookInstanceId.Int32,
		}
	}
	return result
}

func mapFindInstanceDetailsRow(row db.FindInstanceDetailsByNameRow) types.InstanceDetails {
	result := types.InstanceDetails{
		Instance: types.Instance{
			ID:                 row.ID,
			Name:               row.Name,
			Description:        textPtr(row.Description),
			Status:             types.InstanceStatus(row.ConnectionStatus),
			ConnectionStatus:   connectionStatus(row.WhatsappConnectionStatus),
			OwnerJid:           textPtr(row.OwnerJid),
			ProfilePicUrl:      textPtr(row.ProfilePicUrl),
			WhatsAppDeviceJid:  textPtr(row.WhatsappDeviceJid),
			WhatsAppOwnerJid:   textPtr(row.WhatsappOwnerJid),
			WhatsAppPhone:      textPtr(row.WhatsappPhoneNumber),
			ProfilePicID:       textPtr(row.ProfilePicId),
			LastConnectedAt:    timestampPtr(row.LastConnectedAt),
			LastDisconnectedAt: timestampPtr(row.LastDisconnectedAt),
			LastAttemptAt:      timestampPtr(row.LastConnectionAttemptAt),
			LastError:          textPtr(row.LastConnectionError),
			LastEvent:          textPtr(row.LastConnectionEvent),
			ConnectionAttempts: connectionAttempts(row.ConnectionAttempts),
			CreatedAt:          timestamp(row.CreatedAt),
			UpdatedAt:          timestamp(row.UpdatedAt),
			ExternalAttributes: json.RawMessage(row.ExternalAttributes),
		},
	}
	if row.WebhookId.Valid {
		result.Webhook = &types.Webhook{
			ID:         row.WebhookId.Int32,
			URL:        row.WebhookUrl.String,
			Enabled:    row.WebhookEnabled.Bool,
			Events:     json.RawMessage(row.WebhookEvents),
			CreatedAt:  timestamp(row.WebhookCreatedAt),
			UpdatedAt:  timestamp(row.WebhookUpdatedAt),
			InstanceID: row.WebhookInstanceId.Int32,
		}
	}
	return result
}

func mapMessage(row db.Message) types.Message {
	return types.Message{
		ID:                row.ID,
		KeyID:             row.KeyId,
		KeyRemoteJid:      textPtr(row.KeyRemoteJid),
		KeyLid:            textPtr(row.KeyLid),
		KeyFromMe:         row.KeyFromMe,
		KeyParticipant:    textPtr(row.KeyParticipant),
		KeyParticipantLid: textPtr(row.KeyParticipantLid),
		PushName:          textPtr(row.PushName),
		MessageType:       row.MessageType,
		Content:           row.Content,
		MessageTimestamp:  row.MessageTimestamp,
		Device:            types.DeviceMessage(row.Device),
		IsGroup:           boolPtr(row.IsGroup),
		InstanceID:        row.InstanceId,
		Metadata:          json.RawMessage(row.Metadata),
	}
}

func mapMessageUpdate(row db.MessageUpdate) types.MessageUpdate {
	return types.MessageUpdate{
		ID:        row.ID,
		DateTime:  timestamp(row.DateTime),
		Status:    row.Status,
		MessageID: row.MessageId,
	}
}

func mapChat(row db.Chat) types.Chat {
	return types.Chat{
		ID:         row.ID,
		RemoteJid:  row.RemoteJid,
		Content:    json.RawMessage(row.Content),
		CreatedAt:  timestamp(row.CreatedAt),
		UpdatedAt:  timestamp(row.UpdatedAt),
		InstanceID: row.InstanceId,
	}
}

func mapContact(row db.Contact) types.Contact {
	return types.Contact{
		ID:            row.ID,
		RemoteJid:     row.RemoteJid,
		PushName:      textPtr(row.PushName),
		ProfilePicUrl: textPtr(row.ProfilePicUrl),
		CreatedAt:     timestamp(row.CreatedAt),
		UpdatedAt:     timestamp(row.UpdatedAt),
		InstanceID:    row.InstanceId,
	}
}

func mapWebhook(row db.Webhook) types.Webhook {
	return types.Webhook{
		ID:         row.ID,
		URL:        row.Url,
		Enabled:    row.Enabled,
		Events:     json.RawMessage(row.Events),
		CreatedAt:  timestamp(row.CreatedAt),
		UpdatedAt:  timestamp(row.UpdatedAt),
		InstanceID: row.InstanceId,
	}
}
