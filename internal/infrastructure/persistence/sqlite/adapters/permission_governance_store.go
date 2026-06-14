package adapters

import (
	"context"
	"time"

	"rtagent/internal/domain/persistence"
)

func (b *SQLiteBundle) PutCapability(ctx context.Context, rec persistence.CapabilityRecord) error {
	m := CapabilityModel{
		CapabilityID: rec.CapabilityID,
		Family:       rec.Subject,
		TargetScope:  rec.Scope,
		PolicyRule:   rec.Policy,
		Authority:    rec.Authority,
		CreatedAt:    time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) PutToolSchemaSnapshot(ctx context.Context, rec persistence.ToolSchemaSnapshotRecord) error {
	m := ToolSchemaSnapshotModel{
		SnapshotID:      rec.SnapshotID,
		RunID:           rec.RunID,
		ContextPacketID: rec.ContextPacketID,
		SchemaHash:      rec.SchemaHash,
		ToolCount:       rec.ToolCount,
		SnapshotJSON:    rec.SnapshotJSON,
		CreatedAt:       timeOrNow(rec.CreatedAt),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetToolSchemaSnapshot(ctx context.Context, snapshotID string) (persistence.ToolSchemaSnapshotRecord, error) {
	var m ToolSchemaSnapshotModel
	err := b.db.WithContext(ctx).First(&m, "snapshot_id = ?", snapshotID).Error
	if err != nil {
		return persistence.ToolSchemaSnapshotRecord{}, err
	}
	return persistence.ToolSchemaSnapshotRecord{
		SnapshotID:      m.SnapshotID,
		RunID:           m.RunID,
		ContextPacketID: m.ContextPacketID,
		SchemaHash:      m.SchemaHash,
		ToolCount:       m.ToolCount,
		SnapshotJSON:    m.SnapshotJSON,
		CreatedAt:       m.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (b *SQLiteBundle) PutPermission(ctx context.Context, rec persistence.PermissionRecord) error {
	m := PermissionModel{
		PermissionID:   rec.PermissionID,
		RunID:          rec.RunID,
		Subject:        rec.Subject,
		Scope:          rec.Scope,
		Granted:        rec.Granted,
		AuthorizedBy:   rec.AuthorizedBy,
		RequestedAt:    timeOrNow(rec.RequestedAt),
		ResolvedAt:     optionalTime(rec.ResolvedAt),
		PolicyWarnings: rec.PolicyWarnings,
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetPermission(ctx context.Context, permissionID string) (persistence.PermissionRecord, error) {
	var m PermissionModel
	err := b.db.WithContext(ctx).First(&m, "permission_id = ?", permissionID).Error
	if err != nil {
		return persistence.PermissionRecord{}, err
	}
	return persistence.PermissionRecord{
		PermissionID:   m.PermissionID,
		RunID:          m.RunID,
		Subject:        m.Subject,
		Scope:          m.Scope,
		Granted:        m.Granted,
		AuthorizedBy:   m.AuthorizedBy,
		RequestedAt:    m.RequestedAt.Format(time.RFC3339),
		ResolvedAt:     optionalTimeString(m.ResolvedAt),
		PolicyWarnings: m.PolicyWarnings,
	}, nil
}

func (b *SQLiteBundle) PutGrant(ctx context.Context, rec persistence.GrantRecord) error {
	m := GrantModel{
		GrantID:      rec.GrantID,
		CapabilityID: rec.CapabilityID,
		Grantee:      rec.Grantee,
		GrantedBy:    rec.GrantedBy,
		GrantedAt:    timeOrNow(rec.GrantedAt),
		ExpiresAt:    timeOrZero(rec.ExpiresAt),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetGrant(ctx context.Context, grantID string) (persistence.GrantRecord, error) {
	var m GrantModel
	err := b.db.WithContext(ctx).First(&m, "grant_id = ?", grantID).Error
	if err != nil {
		return persistence.GrantRecord{}, err
	}
	return persistence.GrantRecord{
		GrantID:      m.GrantID,
		CapabilityID: m.CapabilityID,
		Grantee:      m.Grantee,
		GrantedBy:    m.GrantedBy,
		GrantedAt:    m.GrantedAt.Format(time.RFC3339),
		ExpiresAt:    timeStringOrEmpty(m.ExpiresAt),
	}, nil
}

func (b *SQLiteBundle) PutLease(ctx context.Context, rec persistence.LeaseRecord) error {
	acq, _ := time.Parse(time.RFC3339, rec.AcquiredAt)
	exp, _ := time.Parse(time.RFC3339, rec.ExpiresAt)
	m := LeaseModel{
		LeaseID:          rec.LeaseID,
		Resource:         rec.Resource,
		HolderActivityID: rec.HolderActivityID,
		AcquiredAt:       acq,
		ExpiresAt:        exp,
	}
	if rec.ReleasedAt != "" {
		rel, _ := time.Parse(time.RFC3339, rec.ReleasedAt)
		m.ReleasedAt = &rel
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetLease(ctx context.Context, leaseID string) (persistence.LeaseRecord, error) {
	var m LeaseModel
	err := b.db.WithContext(ctx).First(&m, "lease_id = ?", leaseID).Error
	if err != nil {
		return persistence.LeaseRecord{}, err
	}
	return leaseRecordFromModel(m), nil
}

func (b *SQLiteBundle) GetActiveLeaseByResource(ctx context.Context, resource string) (persistence.LeaseRecord, error) {
	var m LeaseModel
	tx := b.db.WithContext(ctx).
		Order("acquired_at DESC").
		Limit(1).
		Find(&m, "resource = ? AND released_at IS NULL AND expires_at > ?", resource, time.Now().UTC())
	if tx.Error != nil {
		return persistence.LeaseRecord{}, tx.Error
	}
	if tx.RowsAffected == 0 {
		return persistence.LeaseRecord{}, nil
	}
	return leaseRecordFromModel(m), nil
}

func leaseRecordFromModel(m LeaseModel) persistence.LeaseRecord {
	rel := ""
	if m.ReleasedAt != nil {
		rel = m.ReleasedAt.Format(time.RFC3339)
	}
	return persistence.LeaseRecord{
		LeaseID:          m.LeaseID,
		Resource:         m.Resource,
		HolderActivityID: m.HolderActivityID,
		AcquiredAt:       m.AcquiredAt.Format(time.RFC3339),
		ExpiresAt:        m.ExpiresAt.Format(time.RFC3339),
		ReleasedAt:       rel,
	}
}
