package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"rtagent/internal/domain/persistence"
)

func (r *Runtime) StopSession(ctx context.Context, req StopSessionRequest) (StopSessionResult, error) {
	if err := r.ensureReady(); err != nil {
		return StopSessionResult{}, err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return StopSessionResult{}, errors.New("session_id is required")
	}
	mode := normalizeStopSessionMode(req.Mode)
	thread, runs, err := r.loadSessionAndRuns(ctx, sessionID)
	if err != nil {
		return StopSessionResult{}, err
	}
	if thread.Status == SessionStatusStopped {
		return StopSessionResult{
			SchemaVersion:   SchemaSessionSnapshotV1,
			SessionID:       sessionID,
			Status:          SessionStatusStopped,
			Mode:            mode,
			AlreadyStopped:  true,
			RuntimeContract: "v1",
		}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	interrupted := []string{}
	activeRunIDs := []string{}
	for _, run := range runs {
		if isActiveRunStatus(run.Status) {
			activeRunIDs = append(activeRunIDs, run.RunID)
		}
	}
	nextStatus := SessionStatusStopped
	if mode == StopSessionModeDrain {
		if len(activeRunIDs) > 0 {
			nextStatus = SessionStatusStopping
		}
	} else {
		for _, runID := range activeRunIDs {
			if _, err := r.InterruptRun(ctx, runID); err != nil {
				return StopSessionResult{}, err
			}
			interrupted = append(interrupted, runID)
		}
	}

	thread.Status = nextStatus
	thread.UpdatedAt = now
	if thread.CreatedAt == "" {
		thread.CreatedAt = now
	}
	if thread.LatestMessageAt == "" {
		thread.LatestMessageAt = now
	}
	if err := r.kernel.store.PutThread(ctx, thread); err != nil {
		return StopSessionResult{}, fmt.Errorf("put session: %w", err)
	}
	if nextStatus == SessionStatusStopped {
		if err := r.emitSessionEnded(ctx, thread, req, interrupted); err != nil {
			return StopSessionResult{}, err
		}
	}
	return StopSessionResult{
		SchemaVersion:     SchemaSessionSnapshotV1,
		SessionID:         sessionID,
		Status:            nextStatus,
		Mode:              mode,
		InterruptedRunIDs: interrupted,
		RuntimeContract:   "v1",
	}, nil
}

func (r *Runtime) completeDrainedSessionIfIdle(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	thread, runs, err := r.loadSessionAndRuns(ctx, sessionID)
	if err != nil {
		return err
	}
	if thread.Status != SessionStatusStopping {
		return nil
	}
	for _, run := range runs {
		if isActiveRunStatus(run.Status) {
			return nil
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	thread.Status = SessionStatusStopped
	thread.UpdatedAt = now
	if thread.CreatedAt == "" {
		thread.CreatedAt = now
	}
	if thread.LatestMessageAt == "" {
		thread.LatestMessageAt = now
	}
	if err := r.kernel.store.PutThread(ctx, thread); err != nil {
		return fmt.Errorf("put drained session: %w", err)
	}
	return r.emitSessionEnded(ctx, thread, StopSessionRequest{
		SessionID:   sessionID,
		Mode:        StopSessionModeDrain,
		Reason:      "all active runs finished",
		RequestedBy: "rtagent.sdk",
	}, nil)
}

func (r *Runtime) emitSessionEnded(ctx context.Context, thread persistence.ThreadRecord, req StopSessionRequest, interrupted []string) error {
	runID := firstNonEmpty(thread.LatestRunID, "session:"+thread.ResumeID)
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   runID,
		Kind:    EventKindSessionEnded,
		Message: "Session stopped via RTAgent SDK",
		Payload: map[string]any{
			"session_id":          thread.ResumeID,
			"status":              SessionStatusStopped,
			"mode":                normalizeStopSessionMode(req.Mode),
			"reason":              req.Reason,
			"requested_by":        req.RequestedBy,
			"interrupted_run_ids": append([]string(nil), interrupted...),
		},
	})
	return err
}

func normalizeStopSessionMode(mode string) string {
	if strings.TrimSpace(mode) == StopSessionModeDrain {
		return StopSessionModeDrain
	}
	return StopSessionModeCancelActive
}

func isActiveRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case RuntimeStatusQueued, RuntimeStatusRunning, RuntimeStatusBlocked, RuntimeStatusSuspended:
		return true
	default:
		return false
	}
}
