package message

import "testing"

func TestResolveRecipient(t *testing.T) {
	tests := []struct {
		name    string
		input   RecipientInput
		want    string
		wantErr bool
	}{
		{name: "number valid", input: RecipientInput{Number: ptr("5531999999999")}, want: "5531999999999@s.whatsapp.net"},
		{name: "chat valid", input: RecipientInput{Chat: ptr("5531888888888")}, want: "5531888888888@s.whatsapp.net"},
		{name: "recipient valid", input: RecipientInput{Recipient: ptr("+55 (31) 77777-7777")}, want: "5531777777777@s.whatsapp.net"},
		{name: "no recipient", input: RecipientInput{}, wantErr: true},
		{name: "two aliases", input: RecipientInput{Number: ptr("5531999999999"), Chat: ptr("5531888888888")}, wantErr: true},
		{name: "invalid phone", input: RecipientInput{Number: ptr("abc")}, wantErr: true},
		{name: "private jid", input: RecipientInput{Number: ptr("5531999999999@s.whatsapp.net")}, want: "5531999999999@s.whatsapp.net"},
		{name: "group jid", input: RecipientInput{Number: ptr("120363000000000000@g.us")}, want: "120363000000000000@g.us"},
		{name: "invalid jid server", input: RecipientInput{Number: ptr("5531999999999@evil.local")}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveRecipient(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveRecipient() error = %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("jid mismatch: want %q got %q", tt.want, got.String())
			}
		})
	}
}

func ptr(value string) *string {
	return &value
}
