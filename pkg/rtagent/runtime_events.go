package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
	"github.com/None9527/RTAgent/internal/runtime/events"
)

func (r *Runtime) Emit(ctx context.Context, draft RuntimeEventDraft) (RuntimeEventEnvelope, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeEventEnvelope{}, err
	}
	draft.RunID = strings.TrimSpace(draft.RunID)
	if draft.RunID == "" {
		return RuntimeEventEnvelope{}, errors.New("run_id is required")
	}
	if strings.TrimSpace(string(draft.Kind)) == "" {
		return RuntimeEventEnvelope{}, errors.New("event kind is required")
	}
	if err := r.ensureRunExists(ctx, draft.RunID); err != nil {
		return RuntimeEventEnvelope{}, err
	}
	occurredAt := nowUTC(draft.OccurredAt)
	payload := clonePayload(draft.Payload)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return RuntimeEventEnvelope{}, fmt.Errorf("marshal event payload: %w", err)
	}

	r.eventMu.Lock()
	seq := draft.Sequence
	if seq <= 0 {
		next, err := r.nextEventSequence(ctx, draft.RunID)
		if err != nil {
			r.eventMu.Unlock()
			return RuntimeEventEnvelope{}, err
		}
		seq = next
	}
	eventID := strings.TrimSpace(draft.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("%s:%06d", draft.RunID, seq)
	}
	record := persistence.RuntimeEventRecord{
		EventID:     eventID,
		RunID:       draft.RunID,
		Kind:        string(draft.Kind),
		Sequence:    seq,
		OccurredAt:  occurredAt.Format(time.RFC3339),
		Message:     draft.Message,
		PayloadJSON: payloadBytes,
	}
	if err := r.kernel.store.AppendRuntimeEvent(ctx, record); err != nil {
		r.eventMu.Unlock()
		return RuntimeEventEnvelope{}, fmt.Errorf("append runtime event: %w", err)
	}
	r.eventMu.Unlock()

	if r.kernel.eventBus != nil {
		_ = r.kernel.eventBus.Publish(ctx, events.Event{
			ID:         eventID,
			RunID:      draft.RunID,
			Kind:       events.Kind(draft.Kind),
			Sequence:   seq,
			OccurredAt: occurredAt,
			Message:    draft.Message,
			Payload:    payload,
		})
	}

	// Advance the WorldState cache's projection-relevant sequence watermark.
	// Only events that can change a WorldState partition advance it; high-
	// frequency non-relevant events (heartbeats, model deltas, checkpoints)
	// are a cheap no-op, so a host inspecting mid-loop does not trigger a full
	// recompute on every model turn. See world_state_cache.go.
	if r.wsCache != nil {
		r.wsCache.observeEvent(draft.RunID, draft.Kind, seq)
	}

	return RuntimeEventEnvelope{
		SchemaVersion: SchemaRuntimeEventEnvelopeV1,
		EventID:       eventID,
		RunID:         draft.RunID,
		Kind:          draft.Kind,
		Sequence:      seq,
		OccurredAt:    occurredAt.Format(time.RFC3339),
		Message:       draft.Message,
		Payload:       payload,
	}, nil
}

func (r *Runtime) ListEvents(ctx context.Context, query EventQuery) ([]RuntimeEventEnvelope, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(query.RunID)
	sessionID := strings.TrimSpace(query.SessionID)
	if runID == "" && sessionID == "" {
		return nil, errors.New("run_id or session_id is required")
	}
	if runID != "" {
		if err := r.ensureRunExists(ctx, runID); err != nil {
			return nil, err
		}
		if err := r.ensureRunBelongsToSession(ctx, runID, sessionID); err != nil {
			return nil, err
		}
		return r.listEventsForRun(ctx, runID, query.AfterSeq)
	}
	_, runs, err := r.loadSessionAndRuns(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := []RuntimeEventEnvelope{}
	for _, run := range runs {
		events, err := r.listEventsForRun(ctx, run.RunID, query.AfterSeq)
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	return out, nil
}

func (r *Runtime) listEventsForRun(ctx context.Context, runID string, afterSeq int64) ([]RuntimeEventEnvelope, error) {
	records, err := r.kernel.store.ListRuntimeEvents(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list runtime events: %w", err)
	}
	out := make([]RuntimeEventEnvelope, 0, len(records))
	for _, rec := range records {
		if rec.Sequence <= afterSeq {
			continue
		}
		out = append(out, eventEnvelopeFromRecord(rec))
	}
	return out, nil
}

func (r *Runtime) ensureRunBelongsToSession(ctx context.Context, runID, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	run, err := r.kernel.store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run: %w", err)
	}
	if run.ResumeID != sessionID {
		return fmt.Errorf("run_id %s does not belong to session_id %s", runID, sessionID)
	}
	return nil
}

func (r *Runtime) nextEventSequence(ctx context.Context, runID string) (int64, error) {
	maxSeq, err := r.maxEventSequence(ctx, runID)
	if err != nil {
		return 0, err
	}
	return maxSeq + 1, nil
}

func (r *Runtime) maxEventSequence(ctx context.Context, runID string) (int64, error) {
	return r.kernel.store.MaxEventSequence(ctx, runID)
}

func eventEnvelopeFromRecord(rec persistence.RuntimeEventRecord) RuntimeEventEnvelope {
	payload := map[string]any{}
	_ = json.Unmarshal(rec.PayloadJSON, &payload)
	return RuntimeEventEnvelope{
		SchemaVersion: SchemaRuntimeEventEnvelopeV1,
		EventID:       rec.EventID,
		RunID:         rec.RunID,
		Kind:          EventKind(rec.Kind),
		Sequence:      rec.Sequence,
		OccurredAt:    rec.OccurredAt,
		Message:       rec.Message,
		Payload:       payload,
	}
}
