package adapters

import (
	"context"
	"encoding/json"
	"time"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func (b *SQLiteBundle) PutMemory(ctx context.Context, rec persistence.MemoryRecord) error {
	citationIDs, _ := json.Marshal(rec.CitationIDs)
	m := MemoryModel{
		RecordID:         rec.RecordID,
		Stage:            string(rec.Stage),
		Kind:             string(rec.Kind),
		Origin:           string(rec.Origin),
		Scope:            rec.Scope,
		Topic:            rec.Topic,
		Content:          rec.Content,
		Confidence:       rec.Confidence,
		FreshnessTTL:     rec.FreshnessTTL,
		Invalidated:      rec.Invalidated,
		SupersedesID:     rec.SupersedesID,
		SourceKind:       string(rec.Source.Kind),
		SourceRunID:      rec.Source.RunID,
		SourceCheckpoint: rec.Source.CheckpointID,
		SourceRecordID:   rec.Source.RecordID,
		CitationIDsJSON:  string(citationIDs),
		CreatedAt:        timeOrNow(rec.CreatedAt),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) ListMemoriesByRunID(ctx context.Context, runID string) ([]persistence.MemoryRecord, error) {
	var models []MemoryModel
	err := b.db.WithContext(ctx).
		Order("created_at ASC").
		Find(&models, "source_run_id = ? OR scope = ?", runID, runID).Error
	if err != nil {
		return nil, err
	}
	results := make([]persistence.MemoryRecord, 0, len(models))
	for _, m := range models {
		results = append(results, memoryRecordFromModel(m))
	}
	return results, nil
}

func memoryRecordFromModel(m MemoryModel) persistence.MemoryRecord {
	var citationIDs []string
	_ = json.Unmarshal([]byte(m.CitationIDsJSON), &citationIDs)
	return persistence.MemoryRecord{
		RecordID:     m.RecordID,
		Stage:        persistence.MemoryStage(m.Stage),
		Kind:         persistence.MemoryKind(m.Kind),
		Origin:       persistence.MemoryOrigin(m.Origin),
		Scope:        m.Scope,
		Topic:        m.Topic,
		Content:      m.Content,
		Confidence:   m.Confidence,
		FreshnessTTL: m.FreshnessTTL,
		Invalidated:  m.Invalidated,
		SupersedesID: m.SupersedesID,
		Source: persistence.SourceRef{
			Kind:         persistence.SourceKind(m.SourceKind),
			RunID:        m.SourceRunID,
			CheckpointID: m.SourceCheckpoint,
			RecordID:     m.SourceRecordID,
		},
		CitationIDs: citationIDs,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
	}
}

func (b *SQLiteBundle) PutArtifact(ctx context.Context, rec persistence.ArtifactRecord) error {
	m := ArtifactModel{
		ArtifactID:  rec.ArtifactID,
		RunID:       rec.Source.RunID,
		ActivityID:  rec.Source.RecordID,
		Kind:        rec.Kind,
		FilePath:    rec.Path,
		SHA256:      rec.SHA256,
		ByteSize:    int64(rec.ByteSize),
		PreviewText: rec.Preview,
		SourceKind:  string(rec.Source.Kind),
		CreatedAt:   time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetArtifact(ctx context.Context, artifactID string) (persistence.ArtifactRecord, error) {
	var m ArtifactModel
	err := b.db.WithContext(ctx).First(&m, "artifact_id = ?", artifactID).Error
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	return artifactRecordFromModel(m), nil
}

func (b *SQLiteBundle) GetLatestArtifactByPath(ctx context.Context, path string) (persistence.ArtifactRecord, error) {
	var m ArtifactModel
	err := b.db.WithContext(ctx).Order("created_at DESC").First(&m, "file_path = ?", path).Error
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
	return artifactRecordFromModel(m), nil
}

func artifactRecordFromModel(m ArtifactModel) persistence.ArtifactRecord {
	return persistence.ArtifactRecord{
		ArtifactID: m.ArtifactID,
		Kind:       m.Kind,
		Path:       m.FilePath,
		SHA256:     m.SHA256,
		ByteSize:   int(m.ByteSize),
		Preview:    m.PreviewText,
		Source: persistence.SourceRef{
			Kind:     persistence.SourceKind(m.SourceKind),
			RunID:    m.RunID,
			RecordID: m.ActivityID,
		},
	}
}
