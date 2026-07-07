package request

import (
	"encoding/json"
	"errors"
)

type CreateInstanceRequest struct {
	InstanceName       *string         `json:"instanceName" validate:"omitempty,max=255"`
	Description        *string         `json:"description" validate:"omitempty,max=255"`
	ExternalAttributes json.RawMessage `json:"externalAttributes"`
}

type RefreshInstanceTokenRequest struct {
	OldToken string `json:"oldToken" validate:"required"`
}

type SetWebhookRequest struct {
	Enabled   *bool           `json:"enabled,omitempty"`
	URL       string          `json:"url" validate:"required,max=500,url"`
	Events    map[string]bool `json:"events,omitempty"`
	EventsSet bool            `json:"-"`
}

func (r *SetWebhookRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for field := range fields {
		switch field {
		case "enabled", "url", "events":
		default:
			return errors.New("unknown field: " + field)
		}
	}

	var raw struct {
		Enabled *bool           `json:"enabled,omitempty"`
		URL     string          `json:"url"`
		Events  json.RawMessage `json:"events,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Enabled = raw.Enabled
	r.URL = raw.URL
	r.Events = nil
	r.EventsSet = raw.Events != nil

	if raw.Events == nil {
		return nil
	}
	if string(raw.Events) == "null" {
		return errors.New("events must be an object")
	}
	var events map[string]bool
	if err := json.Unmarshal(raw.Events, &events); err != nil {
		return err
	}
	if events == nil {
		events = map[string]bool{}
	}
	r.Events = events
	return nil
}
