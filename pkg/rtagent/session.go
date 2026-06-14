package rtagent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func (r *Runtime) InspectSession(ctx context.Context, query SessionQuery) (SessionSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return SessionSnapshot{}, err
	}
	sessionID := strings.TrimSpace(query.SessionID)
	if sessionID == "" {
		return SessionSnapshot{}, errors.New("session_id is required")
	}
	thread, runs, err := r.loadSessionAndRuns(ctx, sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	return sessionSnapshotFromRecords(thread, runs), nil
}

func (r *Runtime) ensureSessionCanAcceptRun(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("session_id is required")
	}
	thread, err := r.kernel.store.GetThread(ctx, sessionID)
	if err != nil {
		return nil
	}
	switch thread.Status {
	case SessionStatusStopping, SessionStatusStopped:
		return &RuntimeError{
			Code:    "session_not_accepting_runs",
			Message: fmt.Sprintf("session %s is %s", sessionID, thread.Status),
		}
	default:
		return nil
	}
}

func (r *Runtime) recordSessionRun(ctx context.Context, cmd RuntimeCommand, objective string) (bool, error) {
	sessionID := strings.TrimSpace(cmd.Scope.SessionID)
	if sessionID == "" {
		return false, errors.New("session_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := nowUTC(cmd.CreatedAt).Format(time.RFC3339)
	thread, err := r.kernel.store.GetThread(ctx, sessionID)
	isNew := err != nil
	if isNew {
		thread = persistence.ThreadRecord{
			ResumeID:  sessionID,
			Status:    SessionStatusActive,
			Title:     objective,
			CreatedAt: createdAt,
		}
	}
	if thread.Status == "" {
		thread.Status = SessionStatusActive
	}
	if thread.Title == "" {
		thread.Title = objective
	}
	thread.LatestRunID = cmd.Scope.RunID
	thread.LatestMessageAt = createdAt
	thread.UpdatedAt = now
	if thread.CreatedAt == "" {
		thread.CreatedAt = createdAt
	}
	if err := r.kernel.store.PutThread(ctx, thread); err != nil {
		return false, fmt.Errorf("put session: %w", err)
	}
	return isNew, nil
}

func (r *Runtime) emitSessionStarted(ctx context.Context, cmd RuntimeCommand) error {
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:      cmd.Scope.RunID,
		Kind:       EventKindSessionStarted,
		OccurredAt: nowUTC(cmd.CreatedAt),
		Message:    "Session started via RTAgent SDK",
		Payload: map[string]any{
			"session_id":  cmd.Scope.SessionID,
			"run_id":      cmd.Scope.RunID,
			"root_run_id": cmd.Scope.RootRunID,
			"actor_id":    cmd.Scope.ActorID,
			"owner_id":    cmd.Scope.OwnerID,
		},
	})
	return err
}

func (r *Runtime) loadSessionAndRuns(ctx context.Context, sessionID string) (persistence.ThreadRecord, []persistence.RunRecord, error) {
	thread, threadErr := r.kernel.store.GetThread(ctx, sessionID)
	runs, err := r.kernel.store.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return persistence.ThreadRecord{}, nil, fmt.Errorf("list session runs: %w", err)
	}
	if threadErr != nil {
		if len(runs) == 0 {
			return persistence.ThreadRecord{}, nil, fmt.Errorf("session %s not found", sessionID)
		}
		thread = synthesizeThreadFromRuns(sessionID, runs)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt < runs[j].CreatedAt
	})
	return thread, runs, nil
}

func synthesizeThreadFromRuns(sessionID string, runs []persistence.RunRecord) persistence.ThreadRecord {
	thread := persistence.ThreadRecord{
		ResumeID: sessionID,
		Status:   SessionStatusActive,
	}
	if len(runs) == 0 {
		return thread
	}
	first := runs[0]
	last := runs[len(runs)-1]
	thread.Title = first.UserObjective
	thread.LatestRunID = last.RunID
	thread.LatestCheckpointID = last.LastCheckpoint
	thread.CreatedAt = first.CreatedAt
	thread.UpdatedAt = last.UpdatedAt
	thread.LatestMessageAt = last.CreatedAt
	return thread
}

func sessionSnapshotFromRecords(thread persistence.ThreadRecord, runs []persistence.RunRecord) SessionSnapshot {
	status := firstNonEmpty(thread.Status, SessionStatusActive)
	summaries := make([]SessionRunSummary, 0, len(runs))
	activeRunIDs := []string{}
	for _, run := range runs {
		summary := sessionRunSummaryFromRecord(run)
		summaries = append(summaries, summary)
		if isActiveRunStatus(summary.Status) {
			activeRunIDs = append(activeRunIDs, summary.RunID)
		}
	}
	canResume := status == SessionStatusActive && thread.LatestRunID != ""
	return SessionSnapshot{
		SchemaVersion:       SchemaSessionSnapshotV1,
		SessionID:           thread.ResumeID,
		Status:              status,
		Active:              status == SessionStatusActive,
		CanResume:           canResume,
		LatestRunID:         thread.LatestRunID,
		LatestCheckpointID:  thread.LatestCheckpointID,
		LatestMessageAt:     thread.LatestMessageAt,
		CreatedAt:           thread.CreatedAt,
		UpdatedAt:           thread.UpdatedAt,
		RunCount:            len(summaries),
		ActiveRunIDs:        activeRunIDs,
		Runs:                summaries,
		RuntimeContract:     "v1",
		ResumeCommandHint:   "--resume " + thread.ResumeID,
		ExternalResumeReady: canResume,
	}
}

func sessionRunSummaryFromRecord(run persistence.RunRecord) SessionRunSummary {
	return SessionRunSummary{
		RunID:          run.RunID,
		SessionID:      run.ResumeID,
		RootRunID:      firstNonEmpty(run.RootRunID, run.RunID),
		ParentRunID:    run.ParentRunID,
		TaskID:         run.TaskID,
		Status:         firstNonEmpty(run.Status, RuntimeStatusOK),
		Resolution:     run.Resolution,
		Title:          run.Title,
		Objective:      run.UserObjective,
		IngressKind:    run.IngressKind,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
		CompletedAt:    run.CompletedAt,
		LastCheckpoint: run.LastCheckpoint,
	}
}
