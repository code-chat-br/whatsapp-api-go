package message

import (
	"time"

	"whatsapp-go-api/internal/database/types"
)

type SendResult struct {
	Message  types.Message
	Accepted *ProcessingAcceptedResponse
}

type ProcessingAcceptedResponse struct {
	StatusCode   int    `json:"statusCode"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	ProcessID    string `json:"processId"`
	InstanceName string `json:"instanceName"`
}

type MentionAllWebhookData struct {
	ProcessID          string         `json:"processId"`
	Status             string         `json:"status"`
	MentionAll         bool           `json:"mentionAll"`
	Data               map[string]any `json:"data,omitempty"`
	Error              *WebhookError  `json:"error,omitempty"`
	ExternalAttributes map[string]any `json:"externalAttributes"`
}

type WebhookError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func acceptedProcessing(processID string, instanceName string) *ProcessingAcceptedResponse {
	return &ProcessingAcceptedResponse{
		StatusCode:   202,
		Status:       "processing",
		Message:      "A mensagem foi aceita e esta sendo processada.",
		ProcessID:    processID,
		InstanceName: instanceName,
	}
}

func successMentionAllWebhookData(processID string, messageID string, remoteJID string, participantCount int, timestamp time.Time, external map[string]any) MentionAllWebhookData {
	return MentionAllWebhookData{
		ProcessID:  processID,
		Status:     "sent",
		MentionAll: true,
		Data: map[string]any{
			"messageId":        messageID,
			"remoteJid":        remoteJID,
			"participantCount": participantCount,
			"timestamp":        timestamp.Format(time.RFC3339),
		},
		ExternalAttributes: cloneMap(external),
	}
}

func failedMentionAllWebhookData(processID string, code string, message string, external map[string]any) MentionAllWebhookData {
	return MentionAllWebhookData{
		ProcessID:  processID,
		Status:     "failed",
		MentionAll: true,
		Error: &WebhookError{
			Code:    code,
			Message: message,
		},
		ExternalAttributes: cloneMap(external),
	}
}
