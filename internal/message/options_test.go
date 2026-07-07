package message

import "testing"

func TestValidateOptions(t *testing.T) {
	delay := int64(1500)
	negative := int64(-1)
	tooHigh := MaxDelayMilliseconds + 1

	tests := []struct {
		name         string
		kind         MessageKind
		options      *MessageOptions
		wantPresence *string
		wantErr      bool
	}{
		{name: "nil options", kind: KindText},
		{name: "valid delay defaults composing", kind: KindText, options: &MessageOptions{Delay: &delay}, wantPresence: ptr("composing")},
		{name: "negative delay", kind: KindText, options: &MessageOptions{Delay: &negative}, wantErr: true},
		{name: "delay too high", kind: KindText, options: &MessageOptions{Delay: &tooHigh}, wantErr: true},
		{name: "invalid presence", kind: KindText, options: &MessageOptions{Presence: ptr("paused")}, wantErr: true},
		{name: "composing text", kind: KindText, options: &MessageOptions{Presence: ptr("composing")}, wantPresence: ptr("composing")},
		{name: "composing image", kind: KindImage, options: &MessageOptions{Presence: ptr("composing")}, wantPresence: ptr("composing")},
		{name: "composing contact", kind: KindContact, options: &MessageOptions{Presence: ptr("composing")}, wantPresence: ptr("composing")},
		{name: "composing location", kind: KindLocation, options: &MessageOptions{Presence: ptr("composing")}, wantPresence: ptr("composing")},
		{name: "recording audio", kind: KindAudio, options: &MessageOptions{Presence: ptr("recording")}, wantPresence: ptr("recording")},
		{name: "recording text", kind: KindText, options: &MessageOptions{Presence: ptr("recording")}, wantErr: true},
		{name: "composing ptv", kind: KindPTV, options: &MessageOptions{Presence: ptr("composing")}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			presence, _, err := validateOptions(tt.options, tt.kind)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateOptions() error = %v", err)
			}
			if tt.wantPresence == nil && presence != nil {
				t.Fatalf("presence mismatch: want nil got %q", *presence)
			}
			if tt.wantPresence != nil {
				if presence == nil || *presence != *tt.wantPresence {
					t.Fatalf("presence mismatch: want %q got %#v", *tt.wantPresence, presence)
				}
			}
		})
	}
}
