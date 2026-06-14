package rtagent

import (
	"encoding/json"
	"strings"

	domainworld "rtagent/internal/domain/worldstate"
)

func worldStateEntryFromLegacy(entry domainworld.Entry, events []RuntimeEventEnvelope) WorldStateEntry {
	out := WorldStateEntry{
		ID:               entry.ID,
		Partition:        entry.Partition,
		Kind:             entry.Kind,
		Subject:          entry.Subject,
		StateOrPredicate: legacyStatePredicate(entry.StateJSON),
		StateJSON:        entry.StateJSON,
		Summary:          entry.Summary,
		Source:           legacyEntrySource(entry.SourceID),
		SourceID:         entry.SourceID,
		SourceSeq:        entry.SourceSeq,
		Authority:        "observed",
		Confidence:       entry.Confidence,
		ObservedAt:       eventObservedAt(events, entry.SourceSeq),
		EvidenceRefs:     append([]string(nil), entry.EvidenceRefs...),
		ExpiresAt:        entry.ExpiresAt,
		Version:          entry.Version,
	}
	if out.StateOrPredicate == "" {
		out.StateOrPredicate = "observed"
	}
	return out
}

func legacyEntrySource(sourceID string) string {
	switch {
	case strings.HasPrefix(sourceID, "artifact:"):
		return "artifact_store"
	case strings.HasPrefix(sourceID, "activity:"):
		return "activity_record"
	case strings.HasPrefix(sourceID, "task:"):
		return "task_record"
	case strings.HasPrefix(sourceID, "permission:"):
		return "permission_record"
	default:
		return "runtime_event"
	}
}

func legacyStatePredicate(stateJSON string) string {
	if strings.TrimSpace(stateJSON) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stateJSON), &payload); err != nil {
		return ""
	}
	for _, key := range []string{"status", "state"} {
		if value := firstPayloadString(payload, key); value != "" {
			return value
		}
	}
	return ""
}
