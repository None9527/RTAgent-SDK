package rtagent

import (
	"encoding/json"
	"fmt"
	"sync"
)

// wsCacheEntry holds a full (unfiltered) WorldStateSnapshot and the event
// sequence up to which it was computed. It is the materialized read model for
// one run.
type wsCacheEntry struct {
	snapshot WorldStateSnapshot
	upToSeq  int64
}

// worldStateCache is a per-Runtime, in-memory cache of the latest full
// WorldStateSnapshot per run. It makes repeated WorldState queries O(1) when no
// new events have been appended since the last computation.
//
// Design notes (see docs/api/world-state.md Projection Determinism):
//   - Only full (unfiltered) snapshots are cached. Partition-filtered queries
//     bypass the cache because external WorldState providers may narrow their
//     output by partition, so a filtered result is not derivable from a cached
//     full snapshot without recomputation.
//   - Reads return a deep copy so callers cannot observe later incremental
//     mutations of the cached snapshot.
//   - Freshness is checked against the authoritative event sequence (DB
//     MAX(sequence)), not a local counter, so writes from any path invalidate
//     the cache correctly.
type worldStateCache struct {
	mu      sync.RWMutex
	entries map[string]*wsCacheEntry
}

func newWorldStateCache() *worldStateCache {
	return &worldStateCache{entries: make(map[string]*wsCacheEntry)}
}

// get returns a deep copy of the cached full snapshot for runID if it is fresh
// up to maxSeq, plus true. It returns false if there is no cache or the cache
// is stale (upToSeq < maxSeq).
func (c *worldStateCache) get(runID string, maxSeq int64) (WorldStateSnapshot, bool) {
	c.mu.RLock()
	entry, ok := c.entries[runID]
	c.mu.RUnlock()
	if !ok || entry == nil || entry.upToSeq < maxSeq {
		return WorldStateSnapshot{}, false
	}
	return deepCopyWorldStateSnapshot(entry.snapshot), true
}

// put stores a freshly computed full snapshot for runID, marking it fresh up to
// seq. It keeps a deep copy so the caller retains ownership of its value.
func (c *worldStateCache) put(runID string, snapshot WorldStateSnapshot, seq int64) {
	c.mu.Lock()
	c.entries[runID] = &wsCacheEntry{snapshot: deepCopyWorldStateSnapshot(snapshot), upToSeq: seq}
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
