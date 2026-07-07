package address

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func TestLegacyBrazilianNumberWithoutNinthDigit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{name: "remove nono digito para DDD 31 e prefixo antigo 7", input: "5531971715555", expected: "553171715555", changed: true},
		{name: "mantem numero quando DDD menor que 31", input: "5511971715555", expected: "5511971715555", changed: false},
		{name: "mantem quando primeiro digito antigo menor que 7", input: "5531961715555", expected: "5531961715555", changed: false},
		{name: "mantem numero nao brasileiro", input: "14155552671", expected: "14155552671", changed: false},
		{name: "mantem numero com tamanho inesperado", input: "553171715555", expected: "553171715555", changed: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := LegacyBrazilianNumberWithoutNinthDigit(tt.input)
			if got != tt.expected || changed != tt.changed {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, changed, tt.expected, tt.changed)
			}
		})
	}
}

func TestBuildCandidates(t *testing.T) {
	got := BuildCandidates("5531971715555")
	want := []string{"5531971715555", "553171715555"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildCandidates() = %#v, want %#v", got, want)
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "formatted phone", input: "+55 (31) 97171-5555", want: "5531971715555"},
		{name: "plain phone", input: "5531971715555", want: "5531971715555"},
		{name: "default user jid", input: "5531971715555@s.whatsapp.net", want: "5531971715555"},
		{name: "legacy user jid", input: "5531971715555@c.us", want: "5531971715555"},
		{name: "group jid", input: "120363000000000000@g.us", want: "120363000000000000@g.us"},
		{name: "lid jid", input: "123456789012345@lid", want: "123456789012345@lid"},
		{name: "newsletter jid", input: "120363000000000000@newsletter", want: "120363000000000000@newsletter"},
		{name: "broadcast jid", input: "status@broadcast", want: "status@broadcast"},
		{name: "letters rejected", input: "55abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAddress(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAddress() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolver(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		input     string
		responses []types.IsOnWhatsAppResponse
		err       error
		cache     *AddressMapping
		wantJID   string
		wantErr   error
		wantCalls int
	}{
		{
			name:  "nono digito nao encontrado e versao sem nono encontrada",
			input: "5531971715555",
			responses: []types.IsOnWhatsAppResponse{
				{Query: "5531971715555", IsIn: false},
				{Query: "553171715555", IsIn: true, JID: types.NewJID("553171715555", types.DefaultUserServer)},
			},
			wantJID:   "553171715555@s.whatsapp.net",
			wantCalls: 1,
		},
		{
			name:  "versao com nono encontrada",
			input: "5531971715555",
			responses: []types.IsOnWhatsAppResponse{
				{Query: "5531971715555", IsIn: true, JID: types.NewJID("5531971715555", types.DefaultUserServer)},
			},
			wantJID:   "5531971715555@s.whatsapp.net",
			wantCalls: 1,
		},
		{
			name:  "ambas retornam mesmo JID",
			input: "5531971715555",
			responses: []types.IsOnWhatsAppResponse{
				{Query: "5531971715555", IsIn: true, JID: types.NewJID("553171715555", types.DefaultUserServer)},
				{Query: "553171715555", IsIn: true, JID: types.NewJID("553171715555", types.DefaultUserServer)},
			},
			wantJID:   "553171715555@s.whatsapp.net",
			wantCalls: 1,
		},
		{
			name:  "ambas retornam JIDs diferentes",
			input: "5531971715555",
			responses: []types.IsOnWhatsAppResponse{
				{Query: "5531971715555", IsIn: true, JID: types.NewJID("5531971715555", types.DefaultUserServer)},
				{Query: "553171715555", IsIn: true, JID: types.NewJID("553171715555", types.DefaultUserServer)},
			},
			wantErr:   ErrAmbiguousRecipient,
			wantCalls: 1,
		},
		{
			name:      "nenhuma encontrada",
			input:     "5531971715555",
			responses: []types.IsOnWhatsAppResponse{{Query: "5531971715555", IsIn: false}},
			wantErr:   ErrRecipientNotOnWhatsApp,
			wantCalls: 1,
		},
		{
			name:      "erro de rede",
			input:     "5531971715555",
			err:       errors.New("network"),
			wantErr:   errors.New("network"),
			wantCalls: 1,
		},
		{
			name:      "resultado com JID vazio",
			input:     "5531971715555",
			responses: []types.IsOnWhatsAppResponse{{Query: "5531971715555", IsIn: true}},
			wantErr:   ErrRecipientNotOnWhatsApp,
			wantCalls: 1,
		},
		{
			name:  "cache valido",
			input: "5531971715555",
			cache: &AddressMapping{
				InstanceID:      42,
				NormalizedPhone: "5531971715555",
				CanonicalJID:    "553171715555@s.whatsapp.net",
				Aliases:         []string{"5531971715555"},
				ResolvedAt:      now.Add(-time.Hour),
				ExpiresAt:       now.Add(time.Hour),
			},
			wantJID:   "553171715555@s.whatsapp.net",
			wantCalls: 0,
		},
		{
			name:  "cache expirado",
			input: "5531971715555",
			cache: &AddressMapping{
				InstanceID:      42,
				NormalizedPhone: "5531971715555",
				CanonicalJID:    "553171715555@s.whatsapp.net",
				Aliases:         []string{"5531971715555"},
				ResolvedAt:      now.Add(-2 * time.Hour),
				ExpiresAt:       now.Add(-time.Hour),
			},
			responses: []types.IsOnWhatsAppResponse{
				{Query: "553171715555", IsIn: true, JID: types.NewJID("553171715555", types.DefaultUserServer)},
			},
			wantJID:   "553171715555@s.whatsapp.net",
			wantCalls: 1,
		},
		{
			name:      "grupo ignora consulta",
			input:     "120363000000000000@g.us",
			wantJID:   "120363000000000000@g.us",
			wantCalls: 0,
		},
		{
			name:      "lid ignora regra brasileira",
			input:     "123456789012345@lid",
			wantJID:   "123456789012345@lid",
			wantCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &memoryRepo{mapping: tt.cache}
			lookup := &mockLookup{responses: tt.responses, err: tt.err}
			resolver := NewResolver(repo, time.Hour, zerologNop())
			resolver.now = func() time.Time { return now }

			got, err := resolver.Resolve(context.Background(), lookup, ResolveInput{InstanceID: 42, Address: tt.input})
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			} else if got.CanonicalJID.String() != tt.wantJID {
				t.Fatalf("CanonicalJID = %q, want %q", got.CanonicalJID.String(), tt.wantJID)
			}
			if lookup.calls != tt.wantCalls {
				t.Fatalf("IsOnWhatsApp calls = %d, want %d", lookup.calls, tt.wantCalls)
			}
		})
	}
}

type mockLookup struct {
	responses []types.IsOnWhatsAppResponse
	err       error
	calls     int
}

func (m *mockLookup) IsOnWhatsApp(context.Context, []string) ([]types.IsOnWhatsAppResponse, error) {
	m.calls++
	return m.responses, m.err
}

type memoryRepo struct {
	mapping *AddressMapping
}

func (r *memoryRepo) FindByAlias(_ context.Context, _ int32, alias string) (*AddressMapping, error) {
	if r.mapping == nil {
		return nil, ErrAddressMappingNotFound
	}
	for _, item := range r.mapping.Aliases {
		if item == alias {
			return r.mapping, nil
		}
	}
	return nil, ErrAddressMappingNotFound
}

func (r *memoryRepo) Upsert(_ context.Context, mapping AddressMapping) error {
	r.mapping = &mapping
	return nil
}

func (r *memoryRepo) DeleteByCanonicalJID(context.Context, int32, string) error {
	r.mapping = nil
	return nil
}
