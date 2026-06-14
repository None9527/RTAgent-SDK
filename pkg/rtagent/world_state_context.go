package rtagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/None9527/RTAgent-SDK/internal/domain/contextual"
)

func (r *Runtime) addContextWorldState(ctx context.Context, runID string, events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, maxSeq int64) {
	draft := draftFor(WorldStatePartitionContext, "context_provider", "runtime_context", maxSeq)
	if strings.TrimSpace(r.workDir) != "" {
		draft.addEntry(WorldStateEntry{
			ID:               "workspace:" + r.workDir,
			Partition:        WorldStatePartitionContext,
			Kind:             "workspace",
			Subject:          r.workDir,
			StateOrPredicate: "current_workspace",
			Summary:          "current workspace " + r.workDir,
			Source:           "runtime_config",
			SourceSeq:        maxSeq,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       latestObservedAt(events),
		})
	}
	for _, event := range events {
		if event.Kind != EventKindContextPacketCreated {
			continue
		}
		packetID := firstPayloadString(event.Payload, "context_packet_id")
		if packetID == "" {
			continue
		}
		toolCount := worldStatePayloadInt(event.Payload, "tool_count")
		eventCount := worldStatePayloadInt(event.Payload, "event_count")
		toolSchemaSnapshotID := firstPayloadString(event.Payload, "tool_schema_snapshot_id")
		toolSchemaHash := firstPayloadString(event.Payload, "tool_schema_hash")
		handle := "context_packet:" + packetID
		summary := fmt.Sprintf("context packet assembled with %d events and %d tools", eventCount, toolCount)
		draft.addHandle(WorldStateHandle{
			Handle:              handle,
			Kind:                "context_packet",
			Partition:           WorldStatePartitionContext,
			Summary:             summary,
			Source:              "context_packet",
			SourceID:            packetID,
			SourceSeq:           event.Sequence,
			SnapshotID:          "world:" + runID + ":" + fmt.Sprintf("runtime_event:%d", maxSeq),
			ContentHash:         toolSchemaHash,
			ValidWithinSnapshot: true,
			ValidUntilSourceSeq: maxSeq,
			Reloadable:          true,
			RedactionLevel:      "summary",
		})
		draft.addEntry(WorldStateEntry{
			ID:               handle,
			Partition:        WorldStatePartitionContext,
			Kind:             "context_packet",
			Subject:          packetID,
			StateOrPredicate: "assembled",
			Summary:          summary,
			Source:           "context_packet",
			SourceID:         packetID,
			SourceSeq:        event.Sequence,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       event.OccurredAt,
			Metadata: map[string]any{
				"handle":                  handle,
				"session_id":              firstPayloadString(event.Payload, "session_id"),
				"event_count":             eventCount,
				"tool_count":              toolCount,
				"tool_schema_snapshot_id": toolSchemaSnapshotID,
				"tool_schema_hash":        toolSchemaHash,
			},
		})
	}
	if r.kernel != nil && r.kernel.contextRegistry != nil {
		handles, err := r.kernel.contextRegistry.ListByRunID(ctx, runID)
		if err != nil {
			draft.addWarning("context registry unavailable: " + err.Error())
			return
		}
		for _, handle := range handles {
			addRegisteredContextHandle(draft, handle, maxSeq, latestObservedAt(events))
		}
	}
}

func addRegisteredContextHandle(draft *worldStatePartitionDraft, handle contextual.ContextHandle, maxSeq int64, observedAt string) {
	handleID := strings.TrimSpace(handle.HandleID)
	if handleID == "" {
		return
	}
	worldHandle := "context_handle:" + handleID
	draft.addHandle(WorldStateHandle{
		Handle:              worldHandle,
		Kind:                string(handle.Kind),
		Partition:           WorldStatePartitionContext,
		Summary:             firstNonEmpty(handle.Summary, handle.Title, handleID),
		Source:              "context_registry",
		SourceID:            handleID,
		SourceSeq:           maxSeq,
		ValidWithinSnapshot: true,
		ValidUntilSourceSeq: maxSeq,
		Reloadable:          true,
		RedactionLevel:      "summary",
	})
	draft.addEntry(WorldStateEntry{
		ID:               worldHandle,
		Partition:        WorldStatePartitionContext,
		Kind:             "context_handle",
		Subject:          handleID,
		StateOrPredicate: "registered",
		Summary:          firstNonEmpty(handle.Summary, handle.Title, handleID),
		Source:           "context_registry",
		SourceID:         handleID,
		SourceSeq:        maxSeq,
		Authority:        "runtime_control",
		Confidence:       1,
		ObservedAt:       observedAt,
		EvidenceRefs:     append([]string(nil), handle.EvidenceRefs...),
		Metadata: map[string]any{
			"source_ref":             handle.SourceRef,
			"token_estimate":         handle.TokenEstimate,
			"freshness":              handle.Freshness,
			"materialization_policy": handle.MaterializationPolicy,
		},
	})
}
