package types

import (
	"encoding/json"
	"time"
)

type ChatType string

const (
	ChatTypeChats ChatType = "chats"
	ChatTypeGroup ChatType = "group"
)

func (t ChatType) IsValid() bool {
	return t == ChatTypeChats || t == ChatTypeGroup
}

type Chat struct {
	ID         int32           `json:"id"`
	RemoteJid  string          `json:"remoteJid"`
	Content    json.RawMessage `json:"content,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	InstanceID int32           `json:"instanceId"`
}

type CreateChatInput struct {
	RemoteJid  string
	Content    json.RawMessage
	InstanceID int32
}
