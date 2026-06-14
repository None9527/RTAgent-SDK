package rtagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	domainworld "rtagent/internal/domain/worldstate"
)

type worldStatePartitionDraft struct {
	partition   WorldStatePartition
	entryIndex  map[string]int
	handleIndex map[string]int
}

func newWorldStatePartitionDraft(partition, provider, source, snapshotID, watermark, builtAt string, sourceSeq int64) *worldStatePartitionDraft {
	return &worldStatePartitionDraft{
		partition: WorldStatePartition{
			SchemaVersion:   SchemaWorldStateV1,
			SnapshotID:      snapshotID,
			Partition:       partition,
			Provider:        provider,
			Source:          source,
			SourceSeq:       sourceSeq,
			SourceWatermark: watermark,
			BuiltAt:         builtAt,
		},
		entryIndex:  map[string]int{},
		handleIndex: map[string]int{},
	}
}

func (d *worldStatePartitionDraft) addEntry(entry WorldStateEntry) {
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = strings.TrimSpace(entry.Partition) + ":" + strings.TrimSpace(entry.Kind) + ":" + strings.TrimSpace(entry.Subject)
	}
	if strings.TrimSpace(entry.Partition) == "" {
		entry.Partition = d.partition.Partition
	}
	if strings.TrimSpace(entry.Authority) == "" {
		entry.Authority = "observed"
	}
	if entry.Confidence == 0 {
		entry.Confidence = 1
	}
	if index, ok := d.entryIndex[entry.ID]; ok {
		if entry.SourceSeq >= d.partition.Entries[index].SourceSeq {
			d.partition.Entries[index] = entry
		}
		return
	}
	d.entryIndex[entry.ID] = len(d.partition.Entries)
	d.partition.Entries = append(d.partition.Entries, entry)
}

func (d *worldStatePartitionDraft) addHandle(handle WorldStateHandle) {
	if strings.TrimSpace(handle.Handle) == "" {
		return
	}
	if strings.TrimSpace(handle.Partition) == "" {
		handle.Partition = d.partition.Partition
	}
	if strings.TrimSpace(handle.SnapshotID) == "" {
		handle.SnapshotID = d.partition.SnapshotID
	}
	if index, ok := d.handleIndex[handle.Handle]; ok {
		if handle.SourceSeq >= d.partition.Handles[index].SourceSeq {
			d.partition.Handles[index] = handle
		}
		return
	}
	d.handleIndex[handle.Handle] = len(d.partition.Handles)
	d.partition.Handles = append(d.partition.Handles, handle)
}

func (d *worldStatePartitionDraft) addWarning(warning string) {
	warning = strings.TrimSpace(warning)
	if warning != "" {
		d.partition.Warnings = append(d.partition.Warnings, warning)
	}
}

func (d *worldStatePartitionDraft) build() WorldStatePartition {
	sort.SliceStable(d.partition.Entries, func(i, j int) bool {
		if d.partition.Entries[i].Subject != d.partition.Entries[j].Subject {
			return d.partition.Entries[i].Subject < d.partition.Entries[j].Subject
		}
		return d.partition.Entries[i].ID < d.partition.Entries[j].ID
	})
	sort.SliceStable(d.partition.Handles, func(i, j int) bool {
		return d.partition.Handles[i].Handle < d.partition.Handles[j].Handle
	})
	sort.Strings(d.partition.Warnings)
	return d.partition
}

func (r *Runtime) buildWorldStateSnapshot(ctx context.Context, runID, selectedPartition string, entries []domainworld.Entry, events []RuntimeEventEnvelope) WorldStateSnapshot {
	maxSeq := lastRuntimeEventSeq(events)
	watermark := fmt.Sprintf("runtime_event:%d", maxSeq)
	builtAt := time.Now().UTC().Format(time.RFC3339)
	snapshotID := "world:" + runID + ":" + watermark
	sessionID := firstEventPayloadString(events, "session_id")
	runtimeEpoch := "seq:" + fmt.Sprint(maxSeq)
	permission := r.permissionSnapshotFromEvents(ctx, runID, events)

	drafts := map[string]*worldStatePartitionDraft{}
	draftFor := func(partition, provider, source string, sourceSeq int64) *worldStatePartitionDraft {
		if existing, ok := drafts[partition]; ok {
			if sourceSeq > existing.partition.SourceSeq {
				existing.partition.SourceSeq = sourceSeq
			}
			return existing
		}
		draft := newWorldStatePartitionDraft(partition, provider, source, snapshotID, watermark, builtAt, sourceSeq)
		drafts[partition] = draft
		return draft
	}

	legacyEntries := make([]WorldStateEntry, 0, len(entries))
	for _, entry := range entries {
		publicEntry := worldStateEntryFromLegacy(entry, events)
		if publicEntry.Partition == "" {
			continue
		}
		draftFor(publicEntry.Partition, "runtime_worldstate_builder", "runtime_event", publicEntry.SourceSeq).addEntry(publicEntry)
		if selectedPartition == "" || publicEntry.Partition == selectedPartition {
			legacyEntries = append(legacyEntries, publicEntry)
		}
	}

	r.addContextWorldState(ctx, runID, events, draftFor, maxSeq)
	r.addMemoryWorldState(ctx, runID, events, draftFor, maxSeq)
	r.addHypothesisWorldState(ctx, runID, events, draftFor, maxSeq)
	r.addCapabilityWorldState(ctx, runID, events, draftFor, maxSeq, &permission)
	addGovernanceWorldState(events, draftFor, &permission, maxSeq)
	addActivityWorldState(events, draftFor)
	addTaskWorldState(events, draftFor, maxSeq)
	r.addExternalWorldStateProviders(ctx, runID, strings.TrimSpace(selectedPartition), events, draftFor, snapshotID, watermark, builtAt, maxSeq, &permission)

	partitions := make([]WorldStatePartition, 0, len(drafts))
	warnings := []string{}
	for _, partition := range worldStatePartitionOrder() {
		draft, ok := drafts[partition]
		if !ok {
			continue
		}
		built := draft.build()
		if selectedPartition != "" && built.Partition != selectedPartition {
			continue
		}
		if len(built.Entries) == 0 && len(built.Handles) == 0 && len(built.Warnings) == 0 {
			continue
		}
		partitions = append(partitions, built)
		for _, warning := range built.Warnings {
			warnings = append(warnings, built.Partition+": "+warning)
		}
	}

	handles := make([]WorldStateHandle, 0)
	for _, partition := range partitions {
		handles = append(handles, partition.Handles...)
	}
	sort.SliceStable(legacyEntries, func(i, j int) bool {
		if legacyEntries[i].Partition != legacyEntries[j].Partition {
			return legacyEntries[i].Partition < legacyEntries[j].Partition
		}
		if legacyEntries[i].Subject != legacyEntries[j].Subject {
			return legacyEntries[i].Subject < legacyEntries[j].Subject
		}
		return legacyEntries[i].ID < legacyEntries[j].ID
	})
	sort.SliceStable(handles, func(i, j int) bool {
		return handles[i].Handle < handles[j].Handle
	})

	return WorldStateSnapshot{
		SchemaVersion:   SchemaWorldStateV1,
		SnapshotID:      snapshotID,
		BuildID:         "build:" + shortHash(runID+"|"+watermark+"|"+builtAt),
		RuntimeEpoch:    runtimeEpoch,
		SessionID:       sessionID,
		RunID:           runID,
		Version:         1,
		GeneratedAt:     builtAt,
		BuiltAt:         builtAt,
		SourceWatermark: watermark,
		Partitions:      partitions,
		Handles:         handles,
		Warnings:        warnings,
		Entries:         legacyEntries,
	}
}
