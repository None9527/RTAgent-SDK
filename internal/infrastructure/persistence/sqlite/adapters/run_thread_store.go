package adapters

import (
	"context"
	"time"

	"rtagent/internal/domain/persistence"
)

func (b *SQLiteBundle) PutRun(ctx context.Context, rec persistence.RunRecord) error {
	m := RunModel{
		RunID:          rec.RunID,
		ResumeID:       rec.ResumeID,
		RootRunID:      rec.RootRunID,
		ParentRunID:    rec.ParentRunID,
		TaskID:         rec.TaskID,
		UserObjective:  rec.UserObjective,
		IngressKind:    rec.IngressKind,
		Title:          rec.Title,
		Status:         rec.Status,
		Resolution:     rec.Resolution,
		CreatedAt:      timeOrNow(rec.CreatedAt),
		UpdatedAt:      timeOrNow(rec.UpdatedAt),
		CompletedAt:    optionalTime(rec.CompletedAt),
		LastCheckpoint: rec.LastCheckpoint,
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetRun(ctx context.Context, runID string) (persistence.RunRecord, error) {
	var m RunModel
	err := b.db.WithContext(ctx).First(&m, "run_id = ?", runID).Error
	if err != nil {
		return persistence.RunRecord{}, err
	}
	return runRecordFromModel(m), nil
}

func (b *SQLiteBundle) ListRunsBySession(ctx context.Context, sessionID string) ([]persistence.RunRecord, error) {
	var models []RunModel
	err := b.db.WithContext(ctx).Order("created_at ASC").Find(&models, "resume_id = ?", sessionID).Error
	if err != nil {
		return nil, err
	}
	results := make([]persistence.RunRecord, 0, len(models))
	for _, m := range models {
		results = append(results, runRecordFromModel(m))
	}
	return results, nil
}

func runRecordFromModel(m RunModel) persistence.RunRecord {
	return persistence.RunRecord{
		RunID:          m.RunID,
		ResumeID:       m.ResumeID,
		RootRunID:      m.RootRunID,
		ParentRunID:    m.ParentRunID,
		TaskID:         m.TaskID,
		UserObjective:  m.UserObjective,
		IngressKind:    m.IngressKind,
		Title:          m.Title,
		Status:         m.Status,
		Resolution:     m.Resolution,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.Format(time.RFC3339),
		CompletedAt:    optionalTimeString(m.CompletedAt),
		LastCheckpoint: m.LastCheckpoint,
	}
}

func (b *SQLiteBundle) PutThread(ctx context.Context, rec persistence.ThreadRecord) error {
	m := ThreadModel{
		ResumeID:           rec.ResumeID,
		Title:              rec.Title,
		Status:             rec.Status,
		LatestRunID:        rec.LatestRunID,
		LatestCheckpointID: rec.LatestCheckpointID,
		LatestMessageAt:    timeOrZero(rec.LatestMessageAt),
		CreatedAt:          timeOrNow(rec.CreatedAt),
		UpdatedAt:          timeOrNow(rec.UpdatedAt),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetThread(ctx context.Context, resumeID string) (persistence.ThreadRecord, error) {
	var m ThreadModel
	err := b.db.WithContext(ctx).First(&m, "resume_id = ?", resumeID).Error
	if err != nil {
		return persistence.ThreadRecord{}, err
	}
	return persistence.ThreadRecord{
		ResumeID:           m.ResumeID,
		Title:              m.Title,
		Status:             m.Status,
		LatestRunID:        m.LatestRunID,
		LatestCheckpointID: m.LatestCheckpointID,
		LatestMessageAt:    timeStringOrEmpty(m.LatestMessageAt),
		CreatedAt:          m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          m.UpdatedAt.Format(time.RFC3339),
	}, nil
}
