package rtagent

import (
	"encoding/json"
	"fmt"
	"sync"
)

// projectionRelevantEvents lists the event kinds that actually change the
// WorldState projection. These are the kinds consumed by the partition
// functions in world_state_*.go (context, capability, activity, task,
// governance). Events NOT in this set (run.heartbeat, model.requested,
// model.responded, model.delta, checkpoint.created, agent.started, run.created,
// session.started/ended, turn.started, agent.plan.proposed) do not alter any
// partition and therefore must not invalidate the cache.
//
// This is the foundation of the adaptive cache: high-frequency loop-internal
// events are filtered out so that a host inspecting a run mid-loop does not
// trigger a full WorldState recompute on every model turn or heartbeat.
var projectionRelevantEvents = map[EventKind]bool{
	EventKindContextPacketCreated: true,
	EventKindToolInvoked:          true,
	EventKindToolSucceeded:        true,
	EventKindToolFailed:           true,
	EventKindActivityStarted:      true,
	EventKindActivityCompleted:    true,
	EventKindPermissionRequested:  true,
	EventKindPermissionGranted:    true,
	EventKindPermissionDenied:     true,
	EventKindTurnCompleted:        true,
	EventKindTurnFailed:           true,
	EventKindTurnCancelled:        true,
	EventKindRunInterrupted:       true,
}

// eventIsProjectionRelevant reports whether a given event kind can change any
// WorldState partition. Non-relevant events do not invalidate the cache.
func eventIsProjectionRelevant(kind EventKind) bool {
	return projectionRelevantEvents[kind]
}

// wsCacheEntry holds a full (unfiltered) WorldStateSnapshot and the
// projection-relevant event sequence up to which it was computed. upToProjSeq
// is the highest sequence among projection-relevant events covered by the
// snapshot — NOT the absolute max sequence, so high-frequency non-relevant
// events (heartbeats, model deltas, checkpoints) do not make it stale.
type wsCacheEntry struct {
	snapshot    WorldStateSnapshot
	upToProjSeq int64
}

// worldStateCache is a per-Runtime, in-memory cache of the latest full
// WorldStateSnapshot per run, with projection-aware freshness.
//
// Adaptive freshness model:
//   - projSeq tracks the highest projection-relevant event sequence per run,
//     updated in-memory by the Emit path. Only events in
//     projectionRelevantEvents advance it.
//   - A cache hit requires upToProjSeq >= the run's current projSeq. This means
//     a heartbeat or model delta emitted after the snapshot does NOT invalidate
//     it, while a tool/permission/activity event does.
//   - This bounds the number of recomputes to the number of state-changing
//     events, not the total event volume — which is what makes the cache
//     effective during an active loop (host Inspect calls no longer recompute
//     on every model turn).
//
// Other design notes (see docs/api/world-state.md Projection Determinism):
//   - Only full (unfiltered) snapshots are cached. Partition-filtered queries
//     bypass the cache because external WorldState providers may narrow their
//     output by partition.
//   - Reads return a deep copy so callers cannot observe later mutations.
type worldStateCache struct {
	mu      sync.RWMutex
	entries map[string]*wsCacheEntry
	// projSeq tracks the highest projection-relevant event sequence per run.
	// Updated by observeEvent (called from the Emit path). Used as the
	// freshness watermark instead of MAX(sequence) so non-relevant events do
	// not invalidate the cache.
	projSeq map[string]int64
}

func newWorldStateCache() *worldStateCache {
	return &worldStateCache{
		entries: make(map[string]*wsCacheEntry),
		projSeq: make(map[string]int64),
	}
}

// observeEvent advances the projection-relevant sequence for a run when the
// event kind can change the WorldState projection. Called from the Emit path
// for every appended event; non-relevant kinds are a cheap no-op.
func (c *worldStateCache) observeEvent(runID string, kind EventKind, seq int64) {
	if !eventIsProjectionRelevant(kind) {
		return
	}
	c.mu.Lock()
	if seq > c.projSeq[runID] {
		c.projSeq[runID] = seq
	}
	c.mu.Unlock()
}

// currentProjSeq returns the highest projection-relevant event sequence for a
// run, or 0 if no projection-relevant event has been observed yet.
func (c *worldStateCache) currentProjSeq(runID string) int64 {
	c.mu.RLock()
	seq := c.projSeq[runID]
	c.mu.RUnlock()
	return seq
}

// get returns a deep copy of the cached full snapshot for runID if it is fresh
// up to the run's current projection-relevant sequence, plus true. Returns
// false if there is no cache or a projection-relevant event has arrived since
// the snapshot was built (upToProjSeq < current projSeq).
func (c *worldStateCache) get(runID string) (WorldStateSnapshot, bool) {
	projSeq := c.currentProjSeq(runID)
	c.mu.RLock()
	entry, ok := c.entries[runID]
	c.mu.RUnlock()
	if !ok || entry == nil || entry.upToProjSeq < projSeq {
		return WorldStateSnapshot{}, false
	}
	return deepCopyWorldStateSnapshot(entry.snapshot), true
}

// put stores a freshly computed full snapshot for runID, marking it fresh up to
// the run's current projection-relevant sequence. It keeps a deep copy so the
// caller retains ownership of its value.
func (c *worldStateCache) put(runID string, snapshot WorldStateSnapshot) {
	projSeq := c.currentProjSeq(runID)
	c.mu.Lock()
	c.entries[runID] = &wsCacheEntry{snapshot: deepCopyWorldStateSnapshot(snapshot), upToProjSeq: projSeq}
	c.mu.Unlock()
}

// invalidate removes the cached snapshot for runID, forcing the next query to
// recompute. Used on run/session lifecycle events that change projected state
// outside the event journal (e.g. session stop affecting capability
// authorization).
func (c *worldStateCache) invalidate(runID string) {
	c.mu.Lock()
	delete(c.entries, runID)
	c.mu.Unlock()
}

// deepCopyWorldStateSnapshot returns a fully independent copy of s so that
// neither the cache nor its callers alias any nested slice/map/pointer. It uses
// JSON round-trip because WorldStateSnapshot and its nested types (Entry,
// Handle, CapabilityState) carry stable snake_case JSON tags and contain only
// JSON-serializable fields, including nested slices (EvidenceRefs,
// ResourceLocks) and maps (Metadata). A panic here would indicate a
// non-serializable field was added to the WorldState types, which the cache
// cannot safely copy — so it surfaces loudly instead of silently aliasing.
func deepCopyWorldStateSnapshot(s WorldStateSnapshot) WorldStateSnapshot {
	bytes, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("worldstate cache: marshal snapshot for deep copy: %v", err))
	}
	var clone WorldStateSnapshot
	if err := json.Unmarshal(bytes, &clone); err != nil {
		panic(fmt.Sprintf("worldstate cache: unmarshal snapshot for deep copy: %v", err))
	}
	return clone
}
