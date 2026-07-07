package types

import "time"

type Contact struct {
	ID            int32     `json:"id"`
	RemoteJid     string    `json:"remoteJid"`
	PushName      *string   `json:"pushName"`
	ProfilePicUrl *string   `json:"profilePicUrl"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	InstanceID    int32     `json:"instanceId"`
}

type CreateContactInput struct {
	RemoteJid     string
	PushName      *string
	ProfilePicUrl *string
	InstanceID    int32
}

type ContactFilters struct {
	ID        *int32
	RemoteJid *string
	PushName  *string
}
