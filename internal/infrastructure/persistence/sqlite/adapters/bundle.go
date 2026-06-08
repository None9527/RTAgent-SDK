package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"rtagent/internal/domain/persistence"
)

type SQLiteBundle struct {
	db *gorm.DB
}

func NewSQLiteBundle(db *gorm.DB) *SQLiteBundle {
	return &SQLiteBundle{db: db}
}

// ----------------- RunStore -----------------

func (b *SQLiteBundle) PutRun(ctx context.Context, rec persistence.RunRecord) error {
	m := RunModel{
		RunID:         rec.RunID,
		ResumeID:      rec.ResumeID,
		UserObjective: rec.UserObjective,
		IngressKind:   rec.IngressKind,
		Title:         rec.Title,
		Status:        rec.Status,
		Resolution:    rec.Resolution,
		CreatedAt:     time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetRun(ctx context.Context, runID string) (persistence.RunRecord, error) {
	var m RunModel
	err := b.db.WithContext(ctx).First(&m, "run_id = ?", runID).Error
	if err != nil {
		return persistence.RunRecord{}, err
	}
	return persistence.RunRecord{
		RunID:         m.RunID,
		ResumeID:      m.ResumeID,
		UserObjective: m.UserObjective,
		IngressKind:   m.IngressKind,
		Title:         m.Title,
		Status:        m.Status,
		Resolution:    m.Resolution,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (b *SQLiteBundle) DeleteRun(ctx context.Context, runID string) error {
	return b.db.WithContext(ctx).Delete(&RunModel{}, "run_id = ?", runID).Error
}

// ----------------- ThreadStore -----------------

func (b *SQLiteBundle) PutThread(ctx context.Context, rec persistence.ThreadRecord) error {
	m := ThreadModel{
		ResumeID:    rec.ResumeID,
		Title:       rec.Title,
		Status:      rec.Status,
		LatestRunID: rec.LatestRunID,
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
		ResumeID:    m.ResumeID,
		Title:       m.Title,
		Status:      m.Status,
		LatestRunID: m.LatestRunID,
	}, nil
}

func (b *SQLiteBundle) DeleteThread(ctx context.Context, resumeID string) error {
	return b.db.WithContext(ctx).Delete(&ThreadModel{}, "resume_id = ?", resumeID).Error
}

// ----------------- MessageStore -----------------

func (b *SQLiteBundle) AppendMessage(ctx context.Context, rec persistence.MessageRecord) error {
	m := MessageModel{
		MessageID:   rec.MessageID,
		ResumeID:    rec.ResumeID,
		RunID:       rec.RunID,
		Role:        rec.Role,
		Kind:        rec.Kind,
		Sequence:    rec.Sequence,
		Content:     rec.Content,
		PayloadJSON: rec.PayloadJSON,
		CreatedAt:   time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Create(&m).Error
}

// ----------------- CheckpointStore -----------------

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
		CreatedAt:   time.Now().UTC(),
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
	}, nil
}

// ----------------- EvidenceStore -----------------

func (b *SQLiteBundle) AppendEvidence(ctx context.Context, rec persistence.EvidenceRecord) error {
	m := EvidenceModel{
		RecordID:     rec.RecordID,
		RunID:        rec.Source.RunID,
		ActivityID:   rec.Source.RecordID,
		Kind:         rec.Kind,
		Strength:     rec.Strength,
		ActionResult: rec.ActionResult,
		Observation:  rec.Observation,
		ArtifactID:   rec.ArtifactID,
		SourceKind:   string(rec.Source.Kind),
		SourceRunID:  rec.Source.RunID,
		CreatedAt:    time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Create(&m).Error
}

func (b *SQLiteBundle) ListEvidence(ctx context.Context, runID string) ([]persistence.EvidenceRecord, error) {
	var models []EvidenceModel
	err := b.db.WithContext(ctx).Find(&models, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	var results []persistence.EvidenceRecord
	for _, m := range models {
		results = append(results, persistence.EvidenceRecord{
			RecordID:     m.RecordID,
			Kind:         m.Kind,
			Strength:     m.Strength,
			ActionResult: m.ActionResult,
			Observation:  m.Observation,
			ArtifactID:   m.ArtifactID,
			Source: persistence.SourceRef{
				Kind:     persistence.SourceKind(m.SourceKind),
				RunID:    m.SourceRunID,
				RecordID: m.ActivityID,
			},
		})
	}
	return results, nil
}

// ----------------- RuntimeEventStore -----------------

func (b *SQLiteBundle) AppendRuntimeEvent(ctx context.Context, rec persistence.RuntimeEventRecord) error {
	m := EventModel{
		EventID:     rec.EventID,
		RunID:       rec.RunID,
		Kind:        rec.Kind,
		Sequence:    rec.Sequence,
		OccurredAt:  time.Now().UTC(),
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

// ----------------- MemoryStore -----------------

func (b *SQLiteBundle) PutMemory(ctx context.Context, rec persistence.MemoryRecord) error {
	m := MemoryModel{
		RecordID:     rec.RecordID,
		Stage:        string(rec.Stage),
		Kind:         string(rec.Kind),
		Origin:       string(rec.Origin),
		Scope:        rec.Scope,
		Topic:        rec.Topic,
		Content:      rec.Content,
		Confidence:   rec.Confidence,
		FreshnessTTL: rec.FreshnessTTL,
		Invalidated:  rec.Invalidated,
		SupersedesID: rec.SupersedesID,
		SourceKind:   string(rec.Source.Kind),
		SourceRunID:  rec.Source.RunID,
		CreatedAt:    time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetMemory(ctx context.Context, recordID string) (persistence.MemoryRecord, error) {
	var m MemoryModel
	err := b.db.WithContext(ctx).First(&m, "record_id = ?", recordID).Error
	if err != nil {
		return persistence.MemoryRecord{}, err
	}
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
			Kind:  persistence.SourceKind(m.SourceKind),
			RunID: m.SourceRunID,
		},
	}, nil
}

// ----------------- ArtifactStore -----------------

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
	}, nil
}

func (b *SQLiteBundle) GetLatestArtifactByPath(ctx context.Context, path string) (persistence.ArtifactRecord, error) {
	var m ArtifactModel
	err := b.db.WithContext(ctx).Order("created_at DESC").First(&m, "file_path = ?", path).Error
	if err != nil {
		return persistence.ArtifactRecord{}, err
	}
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
	}, nil
}

// ----------------- DatasetStore -----------------

func (b *SQLiteBundle) PutDatasetExport(ctx context.Context, rec persistence.DatasetExportRecord) error {
	return nil
}

func (b *SQLiteBundle) GetDatasetExport(ctx context.Context, exportID string) (persistence.DatasetExportRecord, error) {
	return persistence.DatasetExportRecord{}, nil
}

// ----------------- ActivityStore -----------------

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
		StartedAt:        time.Now().UTC(),
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
		InputRefs:        input,
		OutputRefs:       output,
		EvidenceRefs:     evidence,
		Error:            m.ErrorText,
		Authority:        m.Authority,
	}, nil
}

func (b *SQLiteBundle) ListActivitiesByRunID(ctx context.Context, runID string) ([]persistence.ActivityRecord, error) {
	var models []ActivityModel
	err := b.db.WithContext(ctx).Find(&models, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	var results []persistence.ActivityRecord
	for _, m := range models {
		var input, output, evidence []string
		_ = json.Unmarshal([]byte(m.InputRefsJSON), &input)
		_ = json.Unmarshal([]byte(m.OutputRefsJSON), &output)
		_ = json.Unmarshal([]byte(m.EvidenceRefsJSON), &evidence)
		results = append(results, persistence.ActivityRecord{
			ActivityID:       m.ActivityID,
			Kind:             m.Kind,
			Status:           m.Status,
			Owner:            m.Owner,
			ParentActivityID: m.ParentActivityID,
			RunID:            m.RunID,
			InputRefs:        input,
			OutputRefs:       output,
			EvidenceRefs:     evidence,
			Error:            m.ErrorText,
			Authority:        m.Authority,
		})
	}
	return results, nil
}

// ----------------- TaskStore -----------------

func (b *SQLiteBundle) PutTask(ctx context.Context, rec persistence.TaskRecord) error {
	dep, _ := json.Marshal(rec.Dependencies)
	m := TaskModel{
		TaskID:           rec.TaskID,
		Objective:        rec.Objective,
		Status:           rec.Status,
		DependenciesJSON: string(dep),
		ParentID:         rec.ParentID,
		RunID:            rec.RunID,
		CreatedAt:        time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetTask(ctx context.Context, taskID string) (persistence.TaskRecord, error) {
	var m TaskModel
	err := b.db.WithContext(ctx).First(&m, "task_id = ?", taskID).Error
	if err != nil {
		return persistence.TaskRecord{}, err
	}
	var dep []string
	_ = json.Unmarshal([]byte(m.DependenciesJSON), &dep)

	return persistence.TaskRecord{
		TaskID:       m.TaskID,
		Objective:    m.Objective,
		Status:       m.Status,
		Dependencies: dep,
		ParentID:     m.ParentID,
		RunID:        m.RunID,
	}, nil
}

func (b *SQLiteBundle) ListTasksByRunID(ctx context.Context, runID string) ([]persistence.TaskRecord, error) {
	var models []TaskModel
	err := b.db.WithContext(ctx).Find(&models, "run_id = ?", runID).Error
	if err != nil {
		return nil, err
	}
	var results []persistence.TaskRecord
	for _, m := range models {
		var dep []string
		_ = json.Unmarshal([]byte(m.DependenciesJSON), &dep)
		results = append(results, persistence.TaskRecord{
			TaskID:       m.TaskID,
			Objective:    m.Objective,
			Status:       m.Status,
			Dependencies: dep,
			ParentID:     m.ParentID,
			RunID:        m.RunID,
		})
	}
	return results, nil
}

// ----------------- CapabilityStore -----------------

func (b *SQLiteBundle) PutCapability(ctx context.Context, rec persistence.CapabilityRecord) error {
	m := CapabilityModel{
		CapabilityID: rec.CapabilityID,
		Family:       rec.Subject, // Subject is Family
		TargetScope:  rec.Scope,
		PolicyRule:   rec.Policy,
		Authority:    rec.Authority,
		CreatedAt:    time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Save(&m).Error
}

func (b *SQLiteBundle) GetCapability(ctx context.Context, capabilityID string) (persistence.CapabilityRecord, error) {
	var m CapabilityModel
	err := b.db.WithContext(ctx).First(&m, "capability_id = ?", capabilityID).Error
	if err != nil {
		return persistence.CapabilityRecord{}, err
	}
	return persistence.CapabilityRecord{
		CapabilityID: m.CapabilityID,
		Subject:      m.Family,
		Scope:        m.TargetScope,
		Policy:       m.PolicyRule,
		Authority:    m.Authority,
	}, nil
}

// ----------------- PermissionStore -----------------

func (b *SQLiteBundle) PutPermission(ctx context.Context, rec persistence.PermissionRecord) error {
	m := PermissionModel{
		PermissionID:   rec.PermissionID,
		RunID:          "",
		Subject:        rec.Subject,
		Scope:          rec.Scope,
		Granted:        rec.Granted,
		AuthorizedBy:   rec.AuthorizedBy,
		RequestedAt:    time.Now().UTC(),
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
		Subject:        m.Subject,
		Scope:          m.Scope,
		Granted:        m.Granted,
		AuthorizedBy:   m.AuthorizedBy,
		RequestedAt:    m.RequestedAt.Format(time.RFC3339),
		ResolvedAt:     "",
		PolicyWarnings: m.PolicyWarnings,
	}, nil
}

// ----------------- GrantStore -----------------

func (b *SQLiteBundle) PutGrant(ctx context.Context, rec persistence.GrantRecord) error {
	m := GrantModel{
		GrantID:      rec.GrantID,
		CapabilityID: rec.CapabilityID,
		Grantee:      rec.Grantee,
		GrantedBy:    rec.GrantedBy,
		GrantedAt:    time.Now().UTC(),
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
	}, nil
}

// ----------------- LeaseStore -----------------

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
	}, nil
}

func (b *SQLiteBundle) GetActiveLeaseByResource(ctx context.Context, resource string) (persistence.LeaseRecord, error) {
	var m LeaseModel
	err := b.db.WithContext(ctx).Order("acquired_at DESC").First(&m, "resource = ? AND released_at IS NULL AND expires_at > ?", resource, time.Now().UTC()).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return persistence.LeaseRecord{}, nil
		}
		return persistence.LeaseRecord{}, err
	}
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
	}, nil
}

// ----------------- AuditStore -----------------

func (b *SQLiteBundle) PutAuditLog(ctx context.Context, rec persistence.AuditRecord) error {
	m := AuditModel{
		AuditID:   rec.AuditID,
		Actor:     rec.Actor,
		Action:    rec.Action,
		Subject:   rec.Subject,
		Result:    rec.Result,
		Timestamp: time.Now().UTC(),
	}
	return b.db.WithContext(ctx).Create(&m).Error
}
