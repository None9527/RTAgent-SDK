package rtagent

import "fmt"

func addGovernanceWorldState(events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, permission *PermissionSnapshot, maxSeq int64) {
	if permission == nil {
		return
	}
	draft := draftFor(WorldStatePartitionGovernance, "permission_center", "permission_projection", maxSeq)
	for _, approval := range permission.PendingDecisions {
		draft.addEntry(WorldStateEntry{
			ID:               "governance:pending:" + approval.ID,
			Partition:        WorldStatePartitionGovernance,
			Kind:             "permission_decision",
			Subject:          approval.ID,
			StateOrPredicate: PermissionStatusRequiresApproval,
			Summary:          firstNonEmpty(approval.Description, approval.Permission, approval.Kind, approval.ID),
			Source:           "permission_snapshot",
			SourceID:         approval.ID,
			SourceSeq:        maxSeq,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       latestObservedAt(events),
			Metadata: map[string]any{
				"tool_target":       approval.ToolTarget,
				"permission":        approval.Permission,
				"risk":              approval.Risk,
				"available_choices": approval.AvailableDecisions,
			},
		})
	}
	for _, decision := range permission.DeniedDecisions {
		draft.addEntry(WorldStateEntry{
			ID:               "governance:denied:" + decision.ID,
			Partition:        WorldStatePartitionGovernance,
			Kind:             "permission_decision",
			Subject:          firstNonEmpty(decision.ID, decision.ToolTarget, decision.Resource),
			StateOrPredicate: PermissionStatusDenied,
			Summary:          firstNonEmpty(decision.Reason, "permission denied"),
			Source:           "permission_snapshot",
			SourceID:         decision.SourceEventID,
			SourceSeq:        decision.SourceSeq,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       decision.CreatedAt,
			Metadata: map[string]any{
				"capability":  decision.Capability,
				"tool_target": decision.ToolTarget,
				"resource":    decision.Resource,
				"risk":        decision.Risk,
			},
		})
	}
	for _, grant := range permission.ActiveGrants {
		draft.addEntry(WorldStateEntry{
			ID:               "governance:grant:" + grant.ID,
			Partition:        WorldStatePartitionGovernance,
			Kind:             "permission_grant",
			Subject:          firstNonEmpty(grant.ToolTarget, grant.Resource, grant.Capability),
			StateOrPredicate: PermissionStatusAllowed,
			Summary:          "active permission grant for " + firstNonEmpty(grant.ToolTarget, grant.Resource, grant.Capability),
			Source:           "permission_snapshot",
			SourceID:         grant.ID,
			SourceSeq:        maxSeq,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       latestObservedAt(events),
			Metadata: map[string]any{
				"scope":       grant.Scope,
				"decision":    grant.Decision,
				"capability":  grant.Capability,
				"resource":    grant.Resource,
				"approval_id": grant.ApprovalID,
				"expires_at":  grant.ExpiresAt,
			},
		})
	}
	for _, rule := range permission.ResourceRules {
		draft.addEntry(WorldStateEntry{
			ID:               "governance:rule:" + shortHash(rule.Kind+"|"+rule.Resource+"|"+rule.Decision),
			Partition:        WorldStatePartitionGovernance,
			Kind:             "resource_rule",
			Subject:          firstNonEmpty(rule.Resource, rule.Kind),
			StateOrPredicate: rule.Decision,
			Summary:          rule.Reason,
			Source:           rule.Source,
			SourceSeq:        maxSeq,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       latestObservedAt(events),
			Metadata: map[string]any{
				"rule_kind": rule.Kind,
			},
		})
	}
	for _, warning := range permission.Warnings {
		draft.addWarning(warning)
	}
}

func addActivityWorldState(events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft) {
	for _, event := range events {
		if event.Kind != EventKindToolInvoked && event.Kind != EventKindToolSucceeded && event.Kind != EventKindToolFailed {
			continue
		}
		toolCallID := firstPayloadString(event.Payload, "tool_call_id")
		if toolCallID == "" {
			continue
		}
		toolName := firstPayloadString(event.Payload, "tool_name", "name")
		state := toolActivityState(event)
		draftFor(WorldStatePartitionActivity, "activity_provider", "runtime_event", event.Sequence).addEntry(WorldStateEntry{
			ID:               "activity:tool:" + toolCallID,
			Partition:        WorldStatePartitionActivity,
			Kind:             "tool_activity",
			Subject:          toolCallID,
			StateOrPredicate: state,
			Summary:          fmt.Sprintf("tool %s %s", firstNonEmpty(toolName, toolCallID), state),
			Source:           "runtime_event",
			SourceID:         worldStateEventRef(event),
			SourceSeq:        event.Sequence,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       event.OccurredAt,
			Metadata: map[string]any{
				"tool_name":    toolName,
				"tool_call_id": toolCallID,
				"event_kind":   string(event.Kind),
			},
		})
	}
}

func addTaskWorldState(events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, maxSeq int64) {
	draft := draftFor(WorldStatePartitionTask, "task_provider", "runtime_event", maxSeq)
	start := len(events) - 12
	if start < 0 {
		start = 0
	}
	for _, event := range events[start:] {
		draft.addEntry(WorldStateEntry{
			ID:               "event:" + worldStateEventRef(event),
			Partition:        WorldStatePartitionTask,
			Kind:             "runtime_event",
			Subject:          string(event.Kind),
			StateOrPredicate: eventState(event),
			Summary:          firstNonEmpty(event.Message, string(event.Kind)),
			Source:           "runtime_event",
			SourceID:         worldStateEventRef(event),
			SourceSeq:        event.Sequence,
			Authority:        "runtime_control",
			Confidence:       1,
			ObservedAt:       event.OccurredAt,
			Metadata: map[string]any{
				"event_kind":   string(event.Kind),
				"payload_keys": payloadKeys(event.Payload),
			},
		})
	}
}
