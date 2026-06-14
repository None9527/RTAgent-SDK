package worldstate

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
	domainworld "github.com/None9527/RTAgent/internal/domain/worldstate"
	"github.com/None9527/RTAgent/internal/runtime/events"
)

type WorldStateBuilder struct {
	mu            sync.RWMutex
	store         runtimeEventReader
	cachedState   map[string]*domainworld.Entry
	lastRebuiltAt map[string]time.Time
}

type runtimeEventReader interface {
	ListRuntimeEvents(ctx context.Context, runID string) ([]persistence.RuntimeEventRecord, error)
}

func NewWorldStateBuilder(store runtimeEventReader) *WorldStateBuilder {
	return &WorldStateBuilder{
		store:         store,
		cachedState:   make(map[string]*domainworld.Entry),
		lastRebuiltAt: make(map[string]time.Time),
	}
}

func (w *WorldStateBuilder) HandleEvent(ctx context.Context, ev events.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch ev.Kind {
	case events.KindFileModified:
		w.invalidatePartition(ev.RunID, domainworld.PartitionArtifact)
		w.updateArtifactState(ctx, ev)
	case events.KindActivityStarted, events.KindActivityCompleted:
		w.invalidatePartition(ev.RunID, domainworld.PartitionActivity)
		w.updateActivityState(ctx, ev)
	case events.KindTaskBlocked, events.KindTaskResumed:
		w.invalidatePartition(ev.RunID, domainworld.PartitionTask)
		w.updateTaskState(ctx, ev)
	case events.KindPermissionRequested, events.KindPermissionGranted, events.KindPermissionDenied:
		w.invalidatePartition(ev.RunID, domainworld.PartitionGovernance)
		w.updateGovernanceState(ctx, ev)
	}
}

func (w *WorldStateBuilder) RebuildAll(ctx context.Context, runID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	records, err := w.store.ListRuntimeEvents(ctx, runID)
	if err != nil {
		return fmt.Errorf("list runtime events: %w", err)
	}

	w.clearRun(runID)

	for _, rec := range records {
		var payload map[string]interface{}
		if err := json.Unmarshal(rec.PayloadJSON, &payload); err != nil {
			continue
		}

		ev := events.Event{
			RunID:    rec.RunID,
			Kind:     events.Kind(rec.Kind),
			Sequence: rec.Sequence,
			Payload:  payload,
		}
		w.replayEvent(ctx, ev)
	}

	w.lastRebuiltAt[runID] = time.Now().UTC()
	return nil
}

func (w *WorldStateBuilder) GetLatestSnapshot(ctx context.Context, runID string) ([]domainworld.Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var entries []domainworld.Entry
	for key, entry := range w.cachedState {
		if !runKeyMatches(key, runID) {
			continue
		}
		if entry.ID != "" {
			entries = append(entries, *entry)
		}
	}
	return entries, nil
}

func (w *WorldStateBuilder) invalidatePartition(runID string, partition domainworld.Partition) {
	for k, entry := range w.cachedState {
		if runKeyMatches(k, runID) && entry.Partition == string(partition) {
			delete(w.cachedState, k)
		}
	}
}

func (w *WorldStateBuilder) updateArtifactState(ctx context.Context, ev events.Event) {
	filePath, _ := ev.Payload["filepath"].(string)
	artID, _ := ev.Payload["artifact_id"].(string)

	key := stateKey(ev.RunID, string(domainworld.PartitionArtifact), filePath)
	w.cachedState[key] = &domainworld.Entry{
		ID:         "ws_" + artID,
		Partition:  string(domainworld.PartitionArtifact),
		Kind:       "file",
		Subject:    filePath,
		StateJSON:  fmt.Sprintf("{\"filepath\":\"%s\",\"artifact_id\":\"%s\"}", filePath, artID),
		Summary:    fmt.Sprintf("Modified file: %s", filePath),
		SourceID:   "artifact:" + artID,
		SourceSeq:  ev.Sequence,
		Confidence: 1.0,
		Version:    1,
	}
}

func (w *WorldStateBuilder) updateActivityState(ctx context.Context, ev events.Event) {
	actID, _ := ev.Payload["activity_id"].(string)
	kind, _ := ev.Payload["kind"].(string)

	key := stateKey(ev.RunID, string(domainworld.PartitionActivity), actID)
	w.cachedState[key] = &domainworld.Entry{
		ID:         "ws_" + actID,
		Partition:  string(domainworld.PartitionActivity),
		Kind:       kind,
		Subject:    actID,
		StateJSON:  fmt.Sprintf("{\"activity_id\":\"%s\",\"status\":\"%s\"}", actID, ev.Kind),
		Summary:    fmt.Sprintf("Activity %s changed status to %s", actID, ev.Kind),
		SourceID:   "activity:" + actID,
		SourceSeq:  ev.Sequence,
		Confidence: 1.0,
		Version:    1,
	}
}

func (w *WorldStateBuilder) updateTaskState(ctx context.Context, ev events.Event) {
	taskID, _ := ev.Payload["task_id"].(string)
	objective, _ := ev.Payload["objective"].(string)
	status, _ := ev.Payload["status"].(string)

	key := stateKey(ev.RunID, string(domainworld.PartitionTask), taskID)
	w.cachedState[key] = &domainworld.Entry{
		ID:         "ws_" + taskID,
		Partition:  string(domainworld.PartitionTask),
		Kind:       "task",
		Subject:    taskID,
		StateJSON:  fmt.Sprintf("{\"task_id\":\"%s\",\"objective\":\"%s\",\"status\":\"%s\"}", taskID, objective, status),
		Summary:    fmt.Sprintf("Task %s objective: %s, status: %s", taskID, objective, status),
		SourceID:   "task:" + taskID,
		SourceSeq:  ev.Sequence,
		Confidence: 1.0,
		Version:    1,
	}
}

func (w *WorldStateBuilder) updateGovernanceState(ctx context.Context, ev events.Event) {
	permissionID, _ := ev.Payload["permission_id"].(string)
	subject, _ := ev.Payload["subject"].(string)
	granted, _ := ev.Payload["granted"].(bool)

	key := stateKey(ev.RunID, string(domainworld.PartitionGovernance), permissionID)
	w.cachedState[key] = &domainworld.Entry{
		ID:         "ws_" + permissionID,
		Partition:  string(domainworld.PartitionGovernance),
		Kind:       "permission",
		Subject:    permissionID,
		StateJSON:  fmt.Sprintf("{\"permission_id\":\"%s\",\"subject\":\"%s\",\"granted\":%t}", permissionID, subject, granted),
		Summary:    fmt.Sprintf("Permission %s for %s granted status: %t", permissionID, subject, granted),
		SourceID:   "permission:" + permissionID,
		SourceSeq:  ev.Sequence,
		Confidence: 1.0,
		Version:    1,
	}
}

func (w *WorldStateBuilder) replayEvent(ctx context.Context, ev events.Event) {
	switch ev.Kind {
	case events.KindFileModified:
		w.updateArtifactState(ctx, ev)
	case events.KindActivityStarted, events.KindActivityCompleted:
		w.updateActivityState(ctx, ev)
	case events.KindTaskBlocked, events.KindTaskResumed:
		w.updateTaskState(ctx, ev)
	case events.KindPermissionRequested, events.KindPermissionGranted, events.KindPermissionDenied:
		w.updateGovernanceState(ctx, ev)
	}
}

func (w *WorldStateBuilder) clearRun(runID string) {
	for key := range w.cachedState {
		if runKeyMatches(key, runID) {
			delete(w.cachedState, key)
		}
	}
}

func stateKey(runID, partition, subject string) string {
	return fmt.Sprintf("%s:%s:%s", runID, partition, subject)
}

func runKeyMatches(key, runID string) bool {
	prefix := runID + ":"
	return len(key) >= len(prefix) && key[:len(prefix)] == prefix
}
