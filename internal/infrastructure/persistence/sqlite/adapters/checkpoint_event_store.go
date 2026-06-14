package adapters

import (
	"context"
	"time"

	"rtagent/internal/domain/persistence"
)

func (b *SQLiteBundle) AppendCheckpoint(ctx context.Context, rec persistence.CheckpointRecord) error {
	m := CheckpointModel{
		RunID:        rec.RunID,
		CheckpointID: rec.CheckpointID,
		GraphID:      rec.GraphID,
		Node:         rec.Node,
		Route:        rec.Route,
		NextNode:     rec.NextNode,
		Status:       rec.Status,
		RouteTrace:   rec.RouteTrace,
		StatePayload: rec.StatePayload,
		CreatedAt:    time.Now().UTC(),
		Source:       rec.Source,
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetCheckpoint(ctx context.Context, runID, checkpointID string) (persistence.CheckpointRecord, error) {
	var m CheckpointModel
	err := b.db.WithContext(ctx).First(&m, "run_id = ? AND checkpoint_id = ?", runID, checkpointID).Error
	if err != nil {
		return persistence.CheckpointRecord{}, err
	}
	return checkpointRecordFromModel(m), nil
}

func (b *SQLiteBundle) ListCheckpointsByRunID(ctx context.Context, runID string) ([]persistence.CheckpointRecord, error) {
	var models []CheckpointModel
	err := b.db.WithContext(ctx).Order("created_at ASC").Find(&models, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	out := make([]persistence.CheckpointRecord, 0, len(models))
	for _, m := range models {
		out = append(out, checkpointRecordFromModel(m))
	}
	return out, nil
}

func checkpointRecordFromModel(m CheckpointModel) persistence.CheckpointRecord {
	return persistence.CheckpointRecord{
		RunID:        m.RunID,
		CheckpointID: m.CheckpointID,
		GraphID:      m.GraphID,
		Node:         m.Node,
		Route:        m.Route,
		NextNode:     m.NextNode,
		Status:       m.Status,
		RouteTrace:   m.RouteTrace,
		StatePayload: m.StatePayload,
		CreatedAt:    m.CreatedAt.Format(time.RFC3339),
		Source:       m.Source,
	}
}

func (b *SQLiteBundle) AppendRuntimeEvent(ctx context.Context, rec persistence.RuntimeEventRecord) error {
	m := EventModel{
		EventID:     rec.EventID,
		RunID:       rec.RunID,
		Kind:        rec.Kind,
		Sequence:    rec.Sequence,
		OccurredAt:  timeOrNow(rec.OccurredAt),
		Message:     rec.Message,
		PayloadJSON: rec.PayloadJSON,
	}
	return b.db.WithContext(ctx).Create(&m).Error
}

func (b *SQLiteBundle) ListRuntimeEvents(ctx context.Context, runID string) ([]persistence.RuntimeEventRecord, error) {
	var models []EventModel
	err := b.db.WithContext(ctx).Order("sequence ASC").Find(&models, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	var results []persistence.RuntimeEventRecord
	for _, m := range models {
		results = append(results, persistence.RuntimeEventRecord{
			EventID:     m.EventID,
			RunID:       m.RunID,
			Kind:        m.Kind,
			Sequence:    m.Sequence,
			OccurredAt:  m.OccurredAt.Format(time.RFC3339),
			Message:     m.Message,
			PayloadJSON: m.PayloadJSON,
		})
	}
	return results, nil
}

// MaxEventSequence returns the highest event sequence for a run via a DB
// aggregate query (O(log N) with the index) instead of loading every event
// row into memory. Used for next-sequence allocation and cache-freshness
// checks.
func (b *SQLiteBundle) MaxEventSequence(ctx context.Context, runID string) (int64, error) {
	var maxSeq int64
	err := b.db.WithContext(ctx).
		Model(&EventModel{}).
		Where("run_id = ?", runID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		return 0, err
	}
	return maxSeq, nil
}
