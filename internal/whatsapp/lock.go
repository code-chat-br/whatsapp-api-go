package whatsapp

import (
	"context"

	"whatsapp-go-api/internal/database/repository"
)

type InstanceConnectionLock interface {
	TryAcquire(ctx context.Context, instanceID string) (bool, error)
	Release(ctx context.Context, instanceID string) error
}

type PostgresInstanceConnectionLock struct {
	instances repository.InstanceRepository
}

func NewPostgresInstanceConnectionLock(instances repository.InstanceRepository) PostgresInstanceConnectionLock {
	return PostgresInstanceConnectionLock{instances: instances}
}

func (l PostgresInstanceConnectionLock) TryAcquire(ctx context.Context, instanceID string) (bool, error) {
	return l.instances.TryAcquireConnectionLock(ctx, instanceID)
}

func (l PostgresInstanceConnectionLock) Release(ctx context.Context, instanceID string) error {
	return l.instances.ReleaseConnectionLock(ctx, instanceID)
}
