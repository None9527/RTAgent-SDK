package adapters

import (
	"context"
	"encoding/json"
	"time"

	"rtagent/internal/domain/persistence"
)

func (b *SQLiteBundle) PutActivity(ctx context.Context, rec persistence.ActivityRecord) error {
	input, _ := json.Marshal(rec.InputRefs)
	output, _ := json.Marshal(rec.OutputRefs)
	evidence, _ := json.Marshal(rec.EvidenceRefs)
	m := ActivityModel{
		ActivityID:       rec.ActivityID,
		Kind:             rec.Kind,
		Status:           rec.Status,
		Owner:            rec.Owner,
		ParentActivityID: rec.ParentActivityID,
		RunID:            rec.RunID,
		StartedAt:        timeOrNow(rec.StartedAt),
		UpdatedAt:        timeOrNow(rec.UpdatedAt),
		CompletedAt:      optionalTime(rec.CompletedAt),
		InputRefsJSON:    string(input),
		OutputRefsJSON:   string(output),
		EvidenceRefsJSON: string(evidence),
		ErrorText:        rec.Error,
		Authority:        rec.Authority,
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetActivity(ctx context.Context, activityID string) (persistence.ActivityRecord, error) {
	var m ActivityModel
	err := b.db.WithContext(ctx).First(&m, "activity_id = ?", activityID).Error
	if err != nil {
		return persistence.ActivityRecord{}, err
	}
	return activityRecordFromModel(m), nil
}

func activityRecordFromModel(m ActivityModel) persistence.ActivityRecord {
	var input, output, evidence []string
	_ = json.Unmarshal([]byte(m.InputRefsJSON), &input)
	_ = json.Unmarshal([]byte(m.OutputRefsJSON), &output)
	_ = json.Unmarshal([]byte(m.EvidenceRefsJSON), &evidence)

	return persistence.ActivityRecord{
		ActivityID:       m.ActivityID,
		Kind:             m.Kind,
		Status:           m.Status,
		Owner:            m.Owner,
		ParentActivityID: m.ParentActivityID,
		RunID:            m.RunID,
		StartedAt:        m.StartedAt.Format(time.RFC3339),
		UpdatedAt:        m.UpdatedAt.Format(time.RFC3339),
		CompletedAt:      optionalTimeString(m.CompletedAt),
		InputRefs:        input,
		OutputRefs:       output,
		EvidenceRefs:     evidence,
		Error:            m.ErrorText,
		Authority:        m.Authority,
	}
}
