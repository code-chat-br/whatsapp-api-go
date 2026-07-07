package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/postgres"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type InstanceRepository interface {
	Create(ctx context.Context, input types.CreateInstanceInput) (types.InstanceWithAuth, error)
	CreateTx(ctx context.Context, q *db.Queries, input types.CreateInstanceInput) (types.InstanceWithAuth, error)
	FindByName(ctx context.Context, name string) (types.InstanceWithAuth, error)
	FindByNameTx(ctx context.Context, q *db.Queries, name string) (types.InstanceWithAuth, error)
	ListDetails(ctx context.Context, name *string) ([]types.InstanceDetails, error)
	FetchDetailsByName(ctx context.Context, name string) (types.InstanceDetails, error)
	FindAutoConnectInstances(ctx context.Context) ([]types.Instance, error)
	List(ctx context.Context) ([]types.InstanceWithAuth, error)
	Update(ctx context.Context, instanceID int32, input types.UpdateInstanceInput) (types.InstanceWithAuth, error)
	UpdateStatus(ctx context.Context, instanceID int32, status types.InstanceStatus) error
	UpdateConnectionState(ctx context.Context, input types.UpdateConnectionStateInput) error
	SaveWhatsAppDevice(ctx context.Context, input types.SaveWhatsAppDeviceInput) error
	ClearWhatsAppDevice(ctx context.Context, instanceID int32) error
	UpdateProfilePicture(ctx context.Context, instanceID int32, profilePicURL *string, profilePicID *string) error
	TryAcquireConnectionLock(ctx context.Context, instanceID string) (bool, error)
	ReleaseConnectionLock(ctx context.Context, instanceID string) error
	EnsureDeletable(ctx context.Context, instanceID int32) error
	Delete(ctx context.Context, instanceID int32, force bool) error
}

type instanceRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	logger zerolog.Logger
}

func NewInstanceRepository(pool *pgxpool.Pool, logger zerolog.Logger) InstanceRepository {
	return &instanceRepository{
		pool:   pool,
		q:      db.New(pool),
		logger: logger.With().Str("component", "instance_repository").Logger(),
	}
}

func (r *instanceRepository) Create(ctx context.Context, input types.CreateInstanceInput) (types.InstanceWithAuth, error) {
	return r.create(ctx, r.q, input)
}

func (r *instanceRepository) CreateTx(ctx context.Context, q *db.Queries, input types.CreateInstanceInput) (types.InstanceWithAuth, error) {
	return r.create(ctx, q, input)
}

func (r *instanceRepository) create(ctx context.Context, q *db.Queries, input types.CreateInstanceInput) (types.InstanceWithAuth, error) {
	if input.Name == "" {
		return types.InstanceWithAuth{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	status := types.InstanceStatusOnline
	if input.Status != nil {
		status = *input.Status
	}
	if !status.IsValid() {
		return types.InstanceWithAuth{}, fmt.Errorf("%w: instance status", ErrInvalidEnum)
	}
	if err := validateOptionalJSON(input.ExternalAttributes); err != nil {
		return types.InstanceWithAuth{}, err
	}

	instance, err := q.CreateInstance(ctx, db.CreateInstanceParams{
		Name:               input.Name,
		Description:        textFromPtr(input.Description),
		Connectionstatus:   db.InstanceStatus(status),
		OwnerJid:           textFromPtr(input.OwnerJid),
		ProfilePicUrl:      textFromPtr(input.ProfilePicUrl),
		ExternalAttributes: nullableJSON(input.ExternalAttributes),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return types.InstanceWithAuth{}, fmt.Errorf("%w: %w", ErrInstanceNameAlreadyExists, err)
		}
		r.logger.Error().Err(err).Str("operation", "instance.create").Str("table", "Instance").Msg("failed to create instance")
		return types.InstanceWithAuth{}, fmt.Errorf("create instance: %w", err)
	}
	return types.InstanceWithAuth{Instance: mapInstance(instance)}, nil
}

func (r *instanceRepository) FindByName(ctx context.Context, name string) (types.InstanceWithAuth, error) {
	return r.findByName(ctx, r.q, name)
}

func (r *instanceRepository) FindByNameTx(ctx context.Context, q *db.Queries, name string) (types.InstanceWithAuth, error) {
	return r.findByName(ctx, q, name)
}

func (r *instanceRepository) findByName(ctx context.Context, q *db.Queries, name string) (types.InstanceWithAuth, error) {
	row, err := q.FindInstanceWithAuthByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.InstanceWithAuth{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "instance.find_by_name").Str("table", "Instance").Msg("failed to find instance")
		return types.InstanceWithAuth{}, fmt.Errorf("find instance by name: %w", err)
	}
	return mapInstanceWithAuthRow(row), nil
}

func (r *instanceRepository) ListDetails(ctx context.Context, name *string) ([]types.InstanceDetails, error) {
	params := db.ListInstanceDetailsParams{}
	if name != nil {
		params.FilterByName = true
		params.Name = pgtype.Text{String: *name, Valid: true}
	}
	rows, err := r.q.ListInstanceDetails(ctx, params)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "instance.list_details").Msg("failed to list instance details")
		return nil, fmt.Errorf("list instance details: %w", err)
	}
	result := make([]types.InstanceDetails, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapListInstanceDetailsRow(row))
	}
	return result, nil
}

func (r *instanceRepository) FetchDetailsByName(ctx context.Context, name string) (types.InstanceDetails, error) {
	row, err := r.q.FindInstanceDetailsByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.InstanceDetails{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "instance.fetch_details").Str("instanceName", name).Msg("failed to fetch instance details")
		return types.InstanceDetails{}, fmt.Errorf("fetch instance details: %w", err)
	}
	return mapFindInstanceDetailsRow(row), nil
}

func (r *instanceRepository) FindAutoConnectInstances(ctx context.Context) ([]types.Instance, error) {
	rows, err := r.q.FindAutoConnectInstances(ctx)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "instance.find_auto_connect").Msg("failed to find auto-connect instances")
		return nil, fmt.Errorf("find auto-connect instances: %w", err)
	}
	result := make([]types.Instance, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapAutoConnectInstanceRow(row))
	}
	return result, nil
}

func (r *instanceRepository) List(ctx context.Context) ([]types.InstanceWithAuth, error) {
	rows, err := r.q.ListInstancesWithAuth(ctx)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "instance.list").Str("table", "Instance").Msg("failed to list instances")
		return nil, fmt.Errorf("list instances: %w", err)
	}
	result := make([]types.InstanceWithAuth, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapListInstanceWithAuthRow(row))
	}
	return result, nil
}

func (r *instanceRepository) Update(ctx context.Context, instanceID int32, input types.UpdateInstanceInput) (types.InstanceWithAuth, error) {
	params := db.UpdateInstanceParams{
		ID:                    instanceID,
		Setname:               input.Name != nil,
		Setdescription:        input.Description.Set,
		Setprofilepicurl:      input.ProfilePicUrl.Set,
		Setexternalattributes: input.ExternalAttributes.Set,
	}
	if input.Name != nil {
		params.Name = *input.Name
	}
	if input.Description.Set {
		params.Description = textFromPtr(input.Description.Value)
	}
	if input.ProfilePicUrl.Set {
		params.ProfilePicUrl = textFromPtr(input.ProfilePicUrl.Value)
	}
	if input.ExternalAttributes.Set {
		if input.ExternalAttributes.Value != nil {
			if err := validateOptionalJSON(*input.ExternalAttributes.Value); err != nil {
				return types.InstanceWithAuth{}, err
			}
			params.ExternalAttributes = nullableJSON(*input.ExternalAttributes.Value)
		}
	}

	instance, err := r.q.UpdateInstance(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.InstanceWithAuth{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		if isUniqueViolation(err) {
			return types.InstanceWithAuth{}, fmt.Errorf("%w: %w", ErrInstanceNameAlreadyExists, err)
		}
		r.logger.Error().Err(err).Str("operation", "instance.update").Int32("instanceId", instanceID).Msg("failed to update instance")
		return types.InstanceWithAuth{}, fmt.Errorf("update instance: %w", err)
	}

	return r.FindByName(ctx, instance.Name)
}

func (r *instanceRepository) UpdateStatus(ctx context.Context, instanceID int32, status types.InstanceStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("%w: instance status", ErrInvalidEnum)
	}
	rows, err := r.q.UpdateInstanceStatus(ctx, db.UpdateInstanceStatusParams{
		Status: db.InstanceStatus(status),
		ID:     instanceID,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "instance.update_status").Int32("instanceId", instanceID).Msg("failed to update instance status")
		return fmt.Errorf("update instance status: %w", err)
	}
	if rows == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (r *instanceRepository) UpdateConnectionState(ctx context.Context, input types.UpdateConnectionStateInput) error {
	params := db.UpdateInstanceConnectionStateParams{
		ID:                input.InstanceID,
		Resetattempts:     input.ResetAttempts,
		Incrementattempts: input.IncrementAttempts,
	}
	if input.ConnectionStatus != nil {
		if !input.ConnectionStatus.IsValid() {
			return fmt.Errorf("%w: connection status", ErrInvalidEnum)
		}
		params.Setconnectionstatus = true
		params.Connectionstatus = string(*input.ConnectionStatus)
	}
	if input.LastConnectedAt != nil {
		params.Setlastconnectedat = true
		params.LastConnectedAt = pgTimestamp(*input.LastConnectedAt)
	}
	if input.LastDisconnectedAt != nil {
		params.Setlastdisconnectedat = true
		params.LastDisconnectedAt = pgTimestamp(*input.LastDisconnectedAt)
	}
	if input.LastConnectionAttemptAt != nil {
		params.Setlastconnectionattemptat = true
		params.LastConnectionAttemptAt = pgTimestamp(*input.LastConnectionAttemptAt)
	}
	if input.LastConnectionError.Set {
		params.Setlastconnectionerror = true
		params.LastConnectionError = textFromPtr(input.LastConnectionError.Value)
	}
	if input.LastConnectionEvent.Set {
		params.Setlastconnectionevent = true
		params.LastConnectionEvent = textFromPtr(input.LastConnectionEvent.Value)
	}

	rows, err := r.q.UpdateInstanceConnectionState(ctx, params)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "instance.update_connection_state").Int32("instanceId", input.InstanceID).Msg("failed to update connection state")
		return fmt.Errorf("update connection state: %w", err)
	}
	if rows == 0 && input.ConnectionStatus == nil &&
		input.LastConnectedAt == nil && input.LastDisconnectedAt == nil && input.LastConnectionAttemptAt == nil &&
		!input.LastConnectionError.Set && !input.LastConnectionEvent.Set && !input.ResetAttempts && !input.IncrementAttempts {
		return ErrInvalidInput
	}
	return nil
}

func (r *instanceRepository) SaveWhatsAppDevice(ctx context.Context, input types.SaveWhatsAppDeviceInput) error {
	rows, err := r.q.SaveWhatsAppDevice(ctx, db.SaveWhatsAppDeviceParams{
		ID:                  input.InstanceID,
		Whatsappdevicejid:   textFromPtr(&input.DeviceJID),
		Whatsappownerjid:    textFromPtr(&input.OwnerJID),
		Whatsappphonenumber: textFromPtr(&input.PhoneNumber),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %w", ErrWhatsAppDeviceAlreadyLinked, err)
		}
		return fmt.Errorf("save whatsapp device: %w", err)
	}
	if rows == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (r *instanceRepository) ClearWhatsAppDevice(ctx context.Context, instanceID int32) error {
	rows, err := r.q.ClearWhatsAppDevice(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("clear whatsapp device: %w", err)
	}
	if rows == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (r *instanceRepository) UpdateProfilePicture(ctx context.Context, instanceID int32, profilePicURL *string, profilePicID *string) error {
	rows, err := r.q.UpdateProfilePicture(ctx, db.UpdateProfilePictureParams{
		ID:            instanceID,
		ProfilePicUrl: textFromPtr(profilePicURL),
		ProfilePicId:  textFromPtr(profilePicID),
	})
	if err != nil {
		return fmt.Errorf("update profile picture: %w", err)
	}
	if rows == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (r *instanceRepository) TryAcquireConnectionLock(ctx context.Context, instanceID string) (bool, error) {
	locked, err := r.q.TryAcquireInstanceConnectionLock(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("try acquire instance connection lock: %w", err)
	}
	return locked, nil
}

func (r *instanceRepository) ReleaseConnectionLock(ctx context.Context, instanceID string) error {
	_, err := r.q.ReleaseInstanceConnectionLock(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("release instance connection lock: %w", err)
	}
	return nil
}

func (r *instanceRepository) EnsureDeletable(ctx context.Context, instanceID int32) error {
	counts, err := r.q.CountInstanceDependencies(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("count instance dependencies: %w", err)
	}
	if counts.Messages > 0 || counts.Chats > 0 || counts.Contacts > 0 || counts.Webhooks > 0 {
		return &InstanceDependenciesError{
			InstanceID: instanceID,
			Messages:   counts.Messages,
			Chats:      counts.Chats,
			Contacts:   counts.Contacts,
			Webhooks:   counts.Webhooks,
		}
	}
	return nil
}

func (r *instanceRepository) Delete(ctx context.Context, instanceID int32, force bool) error {
	return postgres.WithTransaction(ctx, r.pool, r.logger, func(q *db.Queries) error {
		if _, err := q.LockInstanceByID(ctx, instanceID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
			}
			return fmt.Errorf("lock instance: %w", err)
		}

		if !force {
			if err := ensureDeletableTx(ctx, q, instanceID); err != nil {
				return err
			}
		}

		rows, err := q.DeleteInstance(ctx, instanceID)
		if err != nil {
			r.logger.Error().Err(err).Str("operation", "instance.delete").Int32("instanceId", instanceID).Msg("failed to delete instance")
			return fmt.Errorf("delete instance: %w", err)
		}
		if rows == 0 {
			return ErrInstanceNotFound
		}
		return nil
	})
}

func ensureDeletableTx(ctx context.Context, q *db.Queries, instanceID int32) error {
	counts, err := q.CountInstanceDependencies(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("count instance dependencies: %w", err)
	}
	if counts.Messages > 0 || counts.Chats > 0 || counts.Contacts > 0 || counts.Webhooks > 0 {
		return &InstanceDependenciesError{
			InstanceID: instanceID,
			Messages:   counts.Messages,
			Chats:      counts.Chats,
			Contacts:   counts.Contacts,
			Webhooks:   counts.Webhooks,
		}
	}
	return nil
}

func validateOptionalJSON(value json.RawMessage) error {
	if len(value) == 0 {
		return nil
	}
	if !json.Valid(value) {
		return fmt.Errorf("%w", ErrInvalidJSON)
	}
	return nil
}
