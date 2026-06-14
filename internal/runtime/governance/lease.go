package governance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
)

type LocalLeaseManager struct {
	mu    sync.Mutex
	store persistence.LeaseStore
}

func NewLocalLeaseManager(store persistence.LeaseStore) *LocalLeaseManager {
	return &LocalLeaseManager{store: store}
}

func (m *LocalLeaseManager) Acquire(ctx context.Context, resource string, activityID string, ttl time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	activeLease, err := m.store.GetActiveLeaseByResource(ctx, resource)
	if err == nil && activeLease.LeaseID != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, activeLease.ExpiresAt)
		if parseErr == nil && expiresAt.After(now) {
			if activeLease.HolderActivityID != activityID {
				return "", fmt.Errorf("lease conflict: resource %s is locked by activity %s", resource, activeLease.HolderActivityID)
			}
			return activeLease.LeaseID, m.renewLease(ctx, activeLease.LeaseID, ttl)
		}
	}

	leaseID := fmt.Sprintf("lease:%s:%d", activityID, now.UnixNano())
	rec := persistence.LeaseRecord{
		LeaseID:          leaseID,
		Resource:         resource,
		HolderActivityID: activityID,
		AcquiredAt:       now.Format(time.RFC3339),
		ExpiresAt:        now.Add(ttl).Format(time.RFC3339),
	}

	if err := m.store.PutLease(ctx, rec); err != nil {
		return "", fmt.Errorf("persist lease record: %w", err)
	}

	return leaseID, nil
}

func (m *LocalLeaseManager) Release(ctx context.Context, leaseID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.store.GetLease(ctx, leaseID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	releasedAt := now.Format(time.RFC3339)
	rec.ReleasedAt = releasedAt
	rec.ExpiresAt = releasedAt

	return m.store.PutLease(ctx, rec)
}

func (m *LocalLeaseManager) renewLease(ctx context.Context, leaseID string, ttl time.Duration) error {
	rec, err := m.store.GetLease(ctx, leaseID)
	if err != nil {
		return err
	}
	rec.ExpiresAt = time.Now().UTC().Add(ttl).Format(time.RFC3339)
	return m.store.PutLease(ctx, rec)
}
