package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type ContactRepository interface {
	Create(ctx context.Context, input types.CreateContactInput) (types.Contact, error)
	Upsert(ctx context.Context, input types.CreateContactInput) (types.Contact, error)
	List(ctx context.Context, instanceID int32, filters types.ContactFilters) ([]types.Contact, error)
}

type contactRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewContactRepository(pool *pgxpool.Pool, logger zerolog.Logger) ContactRepository {
	return &contactRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "contact_repository").Logger(),
	}
}

func (r *contactRepository) Create(ctx context.Context, input types.CreateContactInput) (types.Contact, error) {
	exists, err := r.q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Contact{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Contact{}, ErrInstanceNotFound
	}
	contact, err := r.q.CreateContact(ctx, db.CreateContactParams{
		Remotejid:     input.RemoteJid,
		PushName:      textFromPtr(input.PushName),
		ProfilePicUrl: textFromPtr(input.ProfilePicUrl),
		Instanceid:    input.InstanceID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return types.Contact{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "contact.create").Int32("instanceId", input.InstanceID).Msg("failed to create contact")
		return types.Contact{}, fmt.Errorf("create contact: %w", err)
	}
	return mapContact(contact), nil
}

func (r *contactRepository) Upsert(ctx context.Context, input types.CreateContactInput) (types.Contact, error) {
	exists, err := r.q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Contact{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Contact{}, ErrInstanceNotFound
	}
	contact, err := r.q.UpsertContact(ctx, db.UpsertContactParams{
		Remotejid:     input.RemoteJid,
		PushName:      textFromPtr(input.PushName),
		ProfilePicUrl: textFromPtr(input.ProfilePicUrl),
		Instanceid:    input.InstanceID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return types.Contact{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "contact.upsert").Int32("instanceId", input.InstanceID).Str("remoteJid", input.RemoteJid).Msg("failed to upsert contact")
		return types.Contact{}, fmt.Errorf("upsert contact: %w", err)
	}
	return mapContact(contact), nil
}

func (r *contactRepository) List(ctx context.Context, instanceID int32, filters types.ContactFilters) ([]types.Contact, error) {
	params := db.ListContactsParams{Instanceid: instanceID}
	if filters.ID != nil {
		params.Filterid = true
		params.ID = *filters.ID
	} else {
		if filters.RemoteJid != nil {
			params.Filterremotejid = true
			params.Remotejid = *filters.RemoteJid
		}
		if filters.PushName != nil {
			params.Filterpushname = true
			params.Pushname = textFromPtr(filters.PushName)
		}
	}
	rows, err := r.q.ListContacts(ctx, params)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "contact.list").Int32("instanceId", instanceID).Msg("failed to list contacts")
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	result := make([]types.Contact, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapContact(row))
	}
	return result, nil
}
