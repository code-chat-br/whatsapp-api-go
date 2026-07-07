package chat

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMediaDataRequestValidateModes(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		mode    MediaDataMode
		wantErr error
	}{
		{name: "id", body: `{"id":1855}`, mode: MediaDataModeID},
		{name: "key id", body: `{"keyId":" ABC "}`, mode: MediaDataModeKeyID},
		{name: "payload", body: `{"messageType":"imageMessage","content":{"directPath":"/media/path"}}`, mode: MediaDataModePayload},
		{name: "payload with key id", body: `{"keyId":"ABC","messageType":"imageMessage","content":{"directPath":"/media/path"}}`, mode: MediaDataModePayload},
		{name: "empty", body: `{}`, wantErr: ErrInvalidMediaRequest},
		{name: "zero id", body: `{"id":0}`, wantErr: ErrInvalidMediaRequest},
		{name: "empty key id", body: `{"keyId":" "}`, wantErr: ErrInvalidMediaRequest},
		{name: "id and key id", body: `{"id":1,"keyId":"ABC"}`, wantErr: ErrInvalidMediaRequest},
		{name: "id and payload", body: `{"id":1,"messageType":"imageMessage","content":{}}`, wantErr: ErrInvalidMediaRequest},
		{name: "unsupported payload", body: `{"messageType":"conversation","content":{"text":"teste"}}`, wantErr: ErrUnsupportedMediaType},
		{name: "missing content", body: `{"messageType":"imageMessage"}`, wantErr: ErrInvalidMediaRequest},
		{name: "array content", body: `{"messageType":"imageMessage","content":[]}`, wantErr: ErrInvalidMediaContent},
		{name: "empty object content", body: `{"messageType":"imageMessage","content":{}}`, wantErr: ErrInvalidMediaContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request MediaDataRequest
			if err := json.Unmarshal([]byte(tt.body), &request); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			mode, err := request.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				if mode != tt.mode {
					t.Fatalf("Validate() mode = %q, want %q", mode, tt.mode)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %v", tt.wantErr)
			}
			if errors.Is(tt.wantErr, ErrInvalidMediaRequest) {
				var validation ValidationError
				if !errors.As(err, &validation) {
					t.Fatalf("Validate() error = %v, want validation error", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMediaRequestFromStoredMessageRejectsNonMedia(t *testing.T) {
	_, _, _, err := MediaRequestFromStoredMessage(messageForMediaTest("conversation", `{"text":"hello"}`))
	if !errors.Is(err, ErrMessageIsNotMedia) {
		t.Fatalf("MediaRequestFromStoredMessage() error = %v, want ErrMessageIsNotMedia", err)
	}
}
