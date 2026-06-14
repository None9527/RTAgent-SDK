package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Runtime) Inspect(ctx context.Context, query InspectQuery) (RuntimeInspectSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeInspectSnapshot{}, err
	}
	runID := strings.TrimSpace(query.RunID)
	sessionID := strings.TrimSpace(query.SessionID)
	if runID == "" {
		if sessionID == "" {
			return RuntimeInspectSnapshot{}, errors.New("run_id or session_id is required")
		}
		thread, runs, err := r.loadSessionAndRuns(ctx, sessionID)
		if err != nil {
			return RuntimeInspectSnapshot{}, err
		}
		runID = strings.TrimSpace(thread.LatestRunID)
		if runID == "" && len(runs) > 0 {
			runID = runs[len(runs)-1].RunID
		}
		if runID == "" {
			return RuntimeInspectSnapshot{}, fmt.Errorf("session %s has no runs", sessionID)
		}
	}
	run, err := r.kernel.store.GetRun(ctx, runID)
	if err != nil {
		return RuntimeInspectSnapshot{}, fmt.Errorf("get run: %w", err)
	}
	if sessionID != "" && run.ResumeID != sessionID {
		return RuntimeInspectSnapshot{}, fmt.Errorf("run_id %s does not belong to session_id %s", runID, sessionID)
	}
	events, err := r.ListEvents(ctx, EventQuery{RunID: runID, SessionID: sessionID, AfterSeq: query.AfterSeq})
	if err != nil {
		return RuntimeInspectSnapshot{}, err
	}
	lastSeq := query.AfterSeq
	for _, event := range events {
		if event.Sequence > lastSeq {
			lastSeq = event.Sequence
		}
	}
	world, err := r.WorldState(ctx, WorldStateQuery{RunID: runID})
	if err != nil {
		return RuntimeInspectSnapshot{}, err
	}
	permission, err := r.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: runID})
	if err != nil {
		return RuntimeInspectSnapshot{}, err
	}
	status := firstNonEmpty(run.Status, RuntimeStatusOK)
	active := status == RuntimeStatusRunning
	blocked := status == RuntimeStatusBlocked || status == RuntimeStatusSuspended
	return RuntimeInspectSnapshot{
		SchemaVersion:   SchemaRuntimeInspectV1,
		SessionID:       run.ResumeID,
		RunID:           run.RunID,
		Status:          status,
		Resolution:      run.Resolution,
		Active:          active,
		Blocked:         blocked,
		LastSeq:         lastSeq,
		EventsAfterSeq:  events,
		Events:          events,
		PermissionMode:  firstEventPayloadString(events, "permission_mode", "mode"),
		PlanningState:   firstEventPayloadString(events, "planning_state"),
		Profile:         firstEventPayloadString(events, "profile"),
		Workspace:       r.workDir,
		ActorID:         firstEventPayloadString(events, "actor_id"),
		OwnerID:         firstEventPayloadString(events, "owner_id"),
		WorldState:      &world,
		Permission:      &permission,
		RuntimeContract: "v1",
	}, nil
}

func (r *Runtime) InterruptRun(ctx context.Context, runID string) (InterruptRunResult, error) {
	if err := r.ensureReady(); err != nil {
		return InterruptRunResult{}, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return InterruptRunResult{}, errors.New("run_id is required")
	}
	rec, err := r.kernel.store.GetRun(ctx, runID)
	if err != nil {
		return InterruptRunResult{}, fmt.Errorf("get run: %w", err)
	}
	if isTerminalStatus(rec.Status) {
		if err := r.completeDrainedSessionIfIdle(ctx, rec.ResumeID); err != nil {
			return InterruptRunResult{}, err
		}
		return InterruptRunResult{
			SchemaVersion:   SchemaRuntimeInspectV1,
			Status:          firstNonEmpty(rec.Status, RuntimeStatusOK),
			RunID:           runID,
			SessionID:       rec.ResumeID,
			CancellationBy:  "already_terminal",
			RuntimeContract: "v1",
		}, nil
	}
	rec.Status = RuntimeStatusCanceled
	rec.Resolution = "interrupted"
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	rec.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if err := r.kernel.store.PutRun(ctx, rec); err != nil {
		return InterruptRunResult{}, fmt.Errorf("put interrupted run: %w", err)
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   runID,
		Kind:    EventKindRunInterrupted,
		Message: "run interrupt requested",
		Payload: map[string]any{"session_id": rec.ResumeID, "run_id": runID},
	}); err != nil {
		return InterruptRunResult{}, err
	}
	if err := r.completeDrainedSessionIfIdle(ctx, rec.ResumeID); err != nil {
		return InterruptRunResult{}, err
	}
	return InterruptRunResult{
		SchemaVersion:   SchemaRuntimeInspectV1,
		Status:          "interrupted",
		RunID:           runID,
		SessionID:       rec.ResumeID,
		CancellationBy:  "explicit_run_interrupt",
		RuntimeContract: "v1",
	}, nil
}

func (r *Runtime) WorldState(ctx context.Context, query WorldStateQuery) (WorldStateSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return WorldStateSnapshot{}, err
	}
	runID := strings.TrimSpace(query.RunID)
	if runID == "" {
		return WorldStateSnapshot{}, errors.New("run_id is required")
	}
	if err := r.ensureRunExists(ctx, runID); err != nil {
		return WorldStateSnapshot{}, err
	}
	selectedPartition := strings.TrimSpace(query.Partition)

	// Fast path: for unfiltered queries, serve from the in-memory WorldState
	// cache when it is fresh up to the latest event sequence. This avoids the
	// full recompute (RebuildAll + ListEvents + 8-partition rebuild) on every
	// query when no new events have been appended. See world_state_cache.go and
	// docs/api/world-state.md Projection Determinism.
	if selectedPartition == "" {
		maxSeq, err := r.maxEventSequence(ctx, runID)
		if err == nil {
			if cached, ok := r.wsCache.get(runID, maxSeq); ok {
				return cached, nil
			}
		}
	}

	if err := r.kernel.wsBuilder.RebuildAll(ctx, runID); err != nil {
		return WorldStateSnapshot{}, err
	}
	entries, err := r.kernel.wsBuilder.GetLatestSnapshot(ctx, runID)
	if err != nil {
		return WorldStateSnapshot{}, err
	}
	events, err := r.ListEvents(ctx, EventQuery{RunID: runID})
	if err != nil {
		return WorldStateSnapshot{}, err
	}
	snapshot := r.buildWorldStateSnapshot(ctx, runID, selectedPartition, entries, events)

	// Cache only full (unfiltered) snapshots; filtered results depend on
	// partition-narrowed external providers and are not safely derivable from a
	// cached full snapshot.
	if selectedPartition == "" {
		if maxSeq, err := r.maxEventSequence(ctx, runID); err == nil {
			r.wsCache.put(runID, snapshot, maxSeq)
		}
	}
	return snapshot, nil
}
