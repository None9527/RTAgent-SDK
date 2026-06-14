package rtagent

import (
	"context"
	"strings"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func (r *Runtime) addMemoryWorldState(ctx context.Context, runID string, events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, maxSeq int64) {
	draft := draftFor(WorldStatePartitionMemory, "memory_provider", "memory_store", maxSeq)
	if r.kernel != nil && r.kernel.store != nil {
		memories, err := r.kernel.store.ListMemoriesByRunID(ctx, runID)
		if err != nil {
			draft.addWarning("memory store unavailable: " + err.Error())
		} else {
			for _, memory := range memories {
				if !memoryVisibleInWorldState(memory) {
					continue
				}
				if memory.Stage == persistence.MemoryStageProposed {
					continue
				}
				sourceSeq := sourceSeqForMemory(events, memory)
				if sourceSeq == 0 {
					sourceSeq = maxSeq
				}
				observedAt := memory.CreatedAt
				if observedAt == "" {
					observedAt = eventObservedAt(events, sourceSeq)
				}
				subject := firstNonEmpty(memory.Topic, memory.Scope, memory.RecordID)
				summary := firstNonEmpty(memory.Topic, string(memory.Kind), memory.RecordID)
				if memory.Content != "" {
					summary = summary + ": " + previewString(memory.Content, 160)
				}
				draft.addEntry(WorldStateEntry{
					ID:               "memory:" + memory.RecordID,
					Partition:        WorldStatePartitionMemory,
					Kind:             firstNonEmpty(string(memory.Kind), "memory"),
					Subject:          subject,
					StateOrPredicate: firstNonEmpty(string(memory.Stage), "observed"),
					Summary:          summary,
					Source:           "memory_store",
					SourceID:         memory.RecordID,
					SourceSeq:        sourceSeq,
					Authority:        firstNonEmpty(string(memory.Origin), "observed"),
					Confidence:       memoryConfidence(memory),
					ObservedAt:       observedAt,
					EvidenceRefs:     append([]string(nil), memory.CitationIDs...),
					Metadata: map[string]any{
						"scope":         memory.Scope,
						"origin":        string(memory.Origin),
						"source_kind":   string(memory.Source.Kind),
						"source_run_id": memory.Source.RunID,
						"supersedes_id": memory.SupersedesID,
						"freshness_ttl": memory.FreshnessTTL,
					},
				})
			}
		}
	}
	if r.memoryProvider == nil {
		return
	}
	scope := worldStateScopeFromEvents(runID, events)
	facts, err := r.memoryProvider.MemoryFacts(ctx, scope)
	if err != nil {
		draft.addWarning("host memory provider unavailable: " + err.Error())
		return
	}
	for _, fact := range facts {
		id := firstNonEmpty(fact.ID, shortHash(fact.Subject+"|"+fact.Summary+"|"+fact.SourceID))
		draft.addEntry(WorldStateEntry{
			ID:               "memory:host:" + id,
			Partition:        WorldStatePartitionMemory,
			Kind:             firstNonEmpty(fact.Kind, "memory"),
			Subject:          firstNonEmpty(fact.Subject, id),
			StateOrPredicate: firstNonEmpty(fact.State, "observed"),
			Summary:          firstNonEmpty(fact.Summary, fact.Content, fact.Subject),
			Source:           firstNonEmpty(fact.Source, "host_memory_provider"),
			SourceID:         fact.SourceID,
			SourceSeq:        maxSeq,
			Authority:        "host_projection",
			Confidence:       fact.Confidence,
			ObservedAt:       fact.ObservedAt,
			EvidenceRefs:     append([]string(nil), fact.EvidenceRefs...),
			Metadata:         clonePayload(fact.Metadata),
		})
	}
}

func (r *Runtime) addHypothesisWorldState(ctx context.Context, runID string, events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, maxSeq int64) {
	draft := draftFor(WorldStatePartitionHypothesis, "hypothesis_provider", "memory_store", maxSeq)
	if r.kernel != nil && r.kernel.store != nil {
		memories, err := r.kernel.store.ListMemoriesByRunID(ctx, runID)
		if err != nil {
			draft.addWarning("memory store unavailable for hypothesis projection: " + err.Error())
		} else {
			for _, memory := range memories {
				if memory.Invalidated || memory.Stage != persistence.MemoryStageProposed {
					continue
				}
				sourceSeq := sourceSeqForMemory(events, memory)
				if sourceSeq == 0 {
					sourceSeq = maxSeq
				}
				draft.addEntry(WorldStateEntry{
					ID:               "hypothesis:memory:" + memory.RecordID,
					Partition:        WorldStatePartitionHypothesis,
					Kind:             firstNonEmpty(string(memory.Kind), "hypothesis"),
					Subject:          firstNonEmpty(memory.Topic, memory.Scope, memory.RecordID),
					StateOrPredicate: string(memory.Stage),
					Summary:          firstNonEmpty(memory.Content, memory.Topic, memory.RecordID),
					Source:           "memory_store",
					SourceID:         memory.RecordID,
					SourceSeq:        sourceSeq,
					Authority:        firstNonEmpty(string(memory.Origin), "inferred"),
					Confidence:       memoryConfidence(memory),
					ObservedAt:       firstNonEmpty(memory.CreatedAt, eventObservedAt(events, sourceSeq)),
					EvidenceRefs:     append([]string(nil), memory.CitationIDs...),
					Metadata: map[string]any{
						"scope":       memory.Scope,
						"origin":      string(memory.Origin),
						"source_kind": string(memory.Source.Kind),
					},
				})
			}
		}
	}
	if r.hypothesisProvider == nil {
		return
	}
	scope := worldStateScopeFromEvents(runID, events)
	facts, err := r.hypothesisProvider.Hypotheses(ctx, scope)
	if err != nil {
		draft.addWarning("host hypothesis provider unavailable: " + err.Error())
		return
	}
	for _, fact := range facts {
		id := firstNonEmpty(fact.ID, shortHash(fact.Subject+"|"+fact.Predicate+"|"+fact.Summary))
		draft.addEntry(WorldStateEntry{
			ID:               "hypothesis:host:" + id,
			Partition:        WorldStatePartitionHypothesis,
			Kind:             firstNonEmpty(fact.Kind, "hypothesis"),
			Subject:          firstNonEmpty(fact.Subject, id),
			StateOrPredicate: firstNonEmpty(fact.Predicate, "inferred"),
			Summary:          firstNonEmpty(fact.Summary, fact.Predicate, fact.Subject),
			Source:           firstNonEmpty(fact.Source, "host_hypothesis_provider"),
			SourceID:         fact.SourceID,
			SourceSeq:        maxSeq,
			Authority:        "host_projection",
			Confidence:       fact.Confidence,
			ObservedAt:       fact.ObservedAt,
			EvidenceRefs:     append([]string(nil), fact.EvidenceRefs...),
			Metadata:         clonePayload(fact.Metadata),
		})
	}
}

func memoryVisibleInWorldState(memory persistence.MemoryRecord) bool {
	if memory.Invalidated {
		return false
	}
	switch memory.Stage {
	case persistence.MemoryStageRejected, persistence.MemoryStageSuperseded:
		return false
	default:
		return true
	}
}

func memoryConfidence(memory persistence.MemoryRecord) float64 {
	if memory.Confidence > 0 {
		return memory.Confidence
	}
	switch memory.Stage {
	case persistence.MemoryStageCommitted:
		return 1
	case persistence.MemoryStageProposed:
		return 0.5
	default:
		return 0.7
	}
}

func sourceSeqForMemory(events []RuntimeEventEnvelope, memory persistence.MemoryRecord) int64 {
	sourceID := strings.TrimSpace(memory.Source.RecordID)
	if sourceID == "" {
		return 0
	}
	for _, event := range events {
		if event.EventID == sourceID {
			return event.Sequence
		}
	}
	return 0
}
