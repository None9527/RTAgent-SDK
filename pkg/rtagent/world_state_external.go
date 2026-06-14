package rtagent

import (
	"context"
	"strings"
)

func (r *Runtime) addExternalWorldStateProviders(ctx context.Context, runID, selectedPartition string, events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, snapshotID, watermark, builtAt string, maxSeq int64, permission *PermissionSnapshot) {
	if len(r.worldStateProviders) == 0 {
		return
	}
	scope := worldStateScopeFromEvents(runID, events)
	input := WorldStateProviderInput{
		Query:           WorldStateQuery{RunID: runID, Partition: selectedPartition},
		Scope:           scope,
		Events:          append([]RuntimeEventEnvelope(nil), events...),
		SnapshotID:      snapshotID,
		SourceWatermark: watermark,
		SourceSeq:       maxSeq,
		BuiltAt:         builtAt,
		Permission:      permission,
	}
	for _, provider := range r.worldStateProviders {
		if provider == nil {
			continue
		}
		partitionName := strings.TrimSpace(provider.Partition())
		if selectedPartition != "" && partitionName != "" && partitionName != selectedPartition {
			continue
		}
		partition, err := provider.BuildWorldState(ctx, input)
		targetPartition := firstNonEmpty(partition.Partition, partitionName)
		if targetPartition == "" {
			targetPartition = "external"
		}
		draft := draftFor(targetPartition, firstNonEmpty(partition.Provider, "host_worldstate_provider"), firstNonEmpty(partition.Source, "host_worldstate_provider"), firstNonZeroInt64(partition.SourceSeq, maxSeq))
		if err != nil {
			draft.addWarning("host world state provider unavailable: " + err.Error())
			continue
		}
		for _, entry := range partition.Entries {
			if entry.SourceSeq == 0 {
				entry.SourceSeq = maxSeq
			}
			if entry.Source == "" {
				entry.Source = firstNonEmpty(partition.Source, "host_worldstate_provider")
			}
			draft.addEntry(entry)
		}
		for _, handle := range partition.Handles {
			if handle.SourceSeq == 0 {
				handle.SourceSeq = maxSeq
			}
			if handle.SnapshotID == "" {
				handle.SnapshotID = snapshotID
			}
			if handle.Source == "" {
				handle.Source = firstNonEmpty(partition.Source, "host_worldstate_provider")
			}
			draft.addHandle(handle)
		}
		for _, warning := range partition.Warnings {
			draft.addWarning(warning)
		}
	}
}
