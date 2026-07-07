package address

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/types"
	"golang.org/x/sync/singleflight"
)

type ResolutionSource string

const (
	ResolutionSourceDirect   ResolutionSource = "direct"
	ResolutionSourceCache    ResolutionSource = "cache"
	ResolutionSourceWhatsApp ResolutionSource = "whatsapp"
	ResolutionSourceLegacy   ResolutionSource = "legacy"
)

var (
	ErrInvalidAddress         = errors.New("invalid whatsapp address")
	ErrRecipientNotOnWhatsApp = errors.New("recipient not on whatsapp")
	ErrAmbiguousRecipient     = errors.New("ambiguous whatsapp recipient")
	ErrAddressMappingNotFound = errors.New("address mapping not found")
)

type ResolveInput struct {
	InstanceID int32
	Address    string
}

type ResolveResult struct {
	Input        string
	Normalized   string
	Candidates   []string
	CanonicalJID types.JID
	Source       ResolutionSource
	RemovedNinth bool
}

type Resolver interface {
	Resolve(ctx context.Context, client WhatsAppLookup, input ResolveInput) (ResolveResult, error)
}

type WhatsAppLookup interface {
	IsOnWhatsApp(ctx context.Context, phones []string) ([]types.IsOnWhatsAppResponse, error)
}

type AddressMappingRepository interface {
	FindByAlias(ctx context.Context, instanceID int32, alias string) (*AddressMapping, error)
	Upsert(ctx context.Context, mapping AddressMapping) error
	DeleteByCanonicalJID(ctx context.Context, instanceID int32, canonicalJID string) error
}

type AddressMapping struct {
	InstanceID      int32
	NormalizedPhone string
	CanonicalJID    string
	LIDJID          *string
	Aliases         []string
	ResolvedAt      time.Time
	ExpiresAt       time.Time
}

type DefaultResolver struct {
	repository AddressMappingRepository
	ttl        time.Duration
	logger     zerolog.Logger
	group      singleflight.Group
	now        func() time.Time
}

func NewResolver(repository AddressMappingRepository, ttl time.Duration, logger zerolog.Logger) *DefaultResolver {
	if ttl <= 0 {
		ttl = 168 * time.Hour
	}
	return &DefaultResolver{
		repository: repository,
		ttl:        ttl,
		logger:     logger.With().Str("component", "address_resolver").Logger(),
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (r *DefaultResolver) Resolve(ctx context.Context, client WhatsAppLookup, input ResolveInput) (ResolveResult, error) {
	parsed, err := parseAddress(input.Address)
	if err != nil {
		return ResolveResult{}, err
	}
	if parsed.direct {
		return ResolveResult{
			Input:        input.Address,
			Normalized:   parsed.jid.String(),
			CanonicalJID: parsed.jid,
			Source:       ResolutionSourceDirect,
		}, nil
	}
	if client == nil {
		return ResolveResult{}, fmt.Errorf("%w: whatsapp client is required", ErrInvalidAddress)
	}

	key := strconv.FormatInt(int64(input.InstanceID), 10) + ":" + parsed.number
	value, err, _ := r.group.Do(key, func() (any, error) {
		return r.resolveNumber(ctx, client, input, parsed.number)
	})
	if err != nil {
		return ResolveResult{}, err
	}
	return value.(ResolveResult), nil
}

func (r *DefaultResolver) resolveNumber(ctx context.Context, client WhatsAppLookup, input ResolveInput, normalized string) (ResolveResult, error) {
	candidates := BuildCandidates(normalized)
	removedNinth := len(candidates) > 1 && len(candidates[1]) < len(candidates[0])

	if mapping, ok := r.findCached(ctx, input.InstanceID, aliasesForCandidates(candidates)); ok {
		jid, err := types.ParseJID(mapping.CanonicalJID)
		if err != nil || jid.IsEmpty() {
			_ = r.repository.DeleteByCanonicalJID(ctx, input.InstanceID, mapping.CanonicalJID)
		} else {
			return ResolveResult{
				Input:        input.Address,
				Normalized:   normalized,
				Candidates:   candidates,
				CanonicalJID: jid.ToNonAD(),
				Source:       ResolutionSourceCache,
				RemovedNinth: removedNinth,
			}, nil
		}
	}

	queries := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		queries = append(queries, "+"+candidate)
	}
	responses, err := client.IsOnWhatsApp(ctx, queries)
	if err != nil {
		return ResolveResult{}, err
	}

	byJID := make(map[string]types.JID)
	for _, response := range responses {
		if !response.IsIn || response.JID.IsEmpty() {
			continue
		}
		jid := response.JID.ToNonAD()
		byJID[jid.String()] = jid
	}
	if len(byJID) == 0 {
		return ResolveResult{}, ErrRecipientNotOnWhatsApp
	}
	if len(byJID) > 1 {
		r.logger.Debug().
			Int32("instance_id", input.InstanceID).
			Str("input", MaskAddress(input.Address)).
			Strs("candidates", maskAll(candidates)).
			Msg("ambiguous WhatsApp address resolution")
		return ResolveResult{}, ErrAmbiguousRecipient
	}

	var canonical types.JID
	for _, jid := range byJID {
		canonical = jid
	}

	source := ResolutionSourceWhatsApp
	if removedNinth && canonical.User == candidates[1] {
		source = ResolutionSourceLegacy
	}
	result := ResolveResult{
		Input:        input.Address,
		Normalized:   normalized,
		Candidates:   candidates,
		CanonicalJID: canonical,
		Source:       source,
		RemovedNinth: removedNinth,
	}

	if r.repository != nil {
		now := r.now()
		aliases := aliasesForCandidates(candidates)
		aliases = appendUnique(aliases, canonical.String())
		if err := r.repository.Upsert(ctx, AddressMapping{
			InstanceID:      input.InstanceID,
			NormalizedPhone: normalized,
			CanonicalJID:    canonical.String(),
			Aliases:         aliases,
			ResolvedAt:      now,
			ExpiresAt:       now.Add(r.ttl),
		}); err != nil {
			r.logger.Debug().Err(err).Int32("instance_id", input.InstanceID).Msg("failed to persist WhatsApp address mapping")
		}
	}

	r.logger.Debug().
		Int32("instance_id", input.InstanceID).
		Str("input", MaskAddress(input.Address)).
		Strs("candidates", maskAll(candidates)).
		Str("canonical_jid", MaskAddress(canonical.String())).
		Str("source", string(source)).
		Bool("removed_ninth_digit", removedNinth).
		Msg("resolved WhatsApp address")

	return result, nil
}

func (r *DefaultResolver) findCached(ctx context.Context, instanceID int32, aliases []string) (*AddressMapping, bool) {
	if r.repository == nil {
		return nil, false
	}
	now := r.now()
	for _, alias := range aliases {
		mapping, err := r.repository.FindByAlias(ctx, instanceID, alias)
		if err != nil {
			if !errors.Is(err, ErrAddressMappingNotFound) {
				r.logger.Debug().Err(err).Int32("instance_id", instanceID).Msg("failed to read WhatsApp address mapping")
			}
			continue
		}
		if mapping.ExpiresAt.After(now) {
			return mapping, true
		}
		_ = r.repository.DeleteByCanonicalJID(ctx, instanceID, mapping.CanonicalJID)
	}
	return nil, false
}

func aliasesForCandidates(candidates []string) []string {
	aliases := make([]string, 0, len(candidates)*2)
	for _, candidate := range candidates {
		aliases = appendUnique(aliases, candidate)
		aliases = appendUnique(aliases, candidate+"@"+types.DefaultUserServer)
	}
	return aliases
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func maskAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, MaskAddress(value))
	}
	return out
}
