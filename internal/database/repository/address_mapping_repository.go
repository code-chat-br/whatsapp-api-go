package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/postgres"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/whatsapp/address"
)

type addressMappingRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	logger zerolog.Logger
}

func NewAddressMappingRepository(pool *pgxpool.Pool, logger zerolog.Logger) address.AddressMappingRepository {
	return &addressMappingRepository{
		pool:   pool,
		q:      db.New(pool),
		logger: logger.With().Str("component", "address_mapping_repository").Logger(),
	}
}

func (r *addressMappingRepository) FindByAlias(ctx context.Context, instanceID int32, alias string) (*address.AddressMapping, error) {
	row, err := r.q.FindAddressMappingByAlias(ctx, db.FindAddressMappingByAliasParams{
		Instanceid: instanceID,
		Alias:      alias,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, address.ErrAddressMappingNotFound
		}
		r.logger.Debug().Err(err).Int32("instance_id", instanceID).Msg("failed to find address mapping")
		return nil, fmt.Errorf("find address mapping: %w", err)
	}
	aliases, err := r.q.ListAddressMappingAliases(ctx, db.ListAddressMappingAliasesParams{
		Instanceid:   instanceID,
		Canonicaljid: row.CanonicalJid,
	})
	if err != nil {
		return nil, fmt.Errorf("list address mapping aliases: %w", err)
	}
	return mapAddressMapping(row, aliases), nil
}

func (r *addressMappingRepository) Upsert(ctx context.Context, mapping address.AddressMapping) error {
	return postgres.WithTransaction(ctx, r.pool, r.logger, func(q *db.Queries) error {
		if _, err := q.DeleteAddressMappingByCanonicalJID(ctx, db.DeleteAddressMappingByCanonicalJIDParams{
			Instanceid:   mapping.InstanceID,
			Canonicaljid: mapping.CanonicalJID,
		}); err != nil {
			return fmt.Errorf("delete old address mapping aliases: %w", err)
		}
		for _, alias := range mapping.Aliases {
			if alias == "" {
				continue
			}
			if _, err := q.UpsertAddressMappingAlias(ctx, db.UpsertAddressMappingAliasParams{
				Instanceid:      mapping.InstanceID,
				Alias:           alias,
				Normalizedphone: mapping.NormalizedPhone,
				Canonicaljid:    mapping.CanonicalJID,
				LidJid:          textFromPtr(mapping.LIDJID),
				Resolvedat:      pgTimestamp(mapping.ResolvedAt),
				Expiresat:       pgTimestamp(mapping.ExpiresAt),
			}); err != nil {
				return fmt.Errorf("upsert address mapping alias: %w", err)
			}
		}
		return nil
	})
}

func (r *addressMappingRepository) DeleteByCanonicalJID(ctx context.Context, instanceID int32, canonicalJID string) error {
	if _, err := r.q.DeleteAddressMappingByCanonicalJID(ctx, db.DeleteAddressMappingByCanonicalJIDParams{
		Instanceid:   instanceID,
		Canonicaljid: canonicalJID,
	}); err != nil {
		return fmt.Errorf("delete address mapping: %w", err)
	}
	return nil
}

func mapAddressMapping(row db.WhatsAppAddressMapping, aliases []string) *address.AddressMapping {
	return &address.AddressMapping{
		InstanceID:      row.InstanceId,
		NormalizedPhone: row.NormalizedPhone,
		CanonicalJID:    row.CanonicalJid,
		LIDJID:          textPtr(row.LidJid),
		Aliases:         aliases,
		ResolvedAt:      timestamp(row.ResolvedAt),
		ExpiresAt:       timestamp(row.ExpiresAt),
	}
}
