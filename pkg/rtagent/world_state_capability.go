package rtagent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/None9527/RTAgent/internal/domain/persistence"
)

func (r *Runtime) addCapabilityWorldState(ctx context.Context, runID string, events []RuntimeEventEnvelope, draftFor func(string, string, string, int64) *worldStatePartitionDraft, maxSeq int64, permissionSnapshot *PermissionSnapshot) {
	contextEvent, snapshotID := latestToolSchemaSnapshotEvent(events)
	draft := draftFor(WorldStatePartitionCapability, "capability_provider", "inventory", maxSeq)
	permissionMode := firstEventPayloadString(events, "permission_mode", "mode")
	if snapshotID != "" {
		draft.partition.Provider = "tool_provider"
		draft.partition.Source = "tool_schema_snapshot"
		draft.partition.SourceSeq = contextEvent.Sequence
		snapshot, err := r.kernel.store.GetToolSchemaSnapshot(ctx, snapshotID)
		if err != nil {
			draft.addWarning("tool schema snapshot " + snapshotID + " unavailable: " + err.Error())
		} else {
			var payload toolSchemaSnapshotPayload
			if err := json.Unmarshal([]byte(snapshot.SnapshotJSON), &payload); err != nil {
				draft.addWarning("tool schema snapshot " + snapshotID + " is invalid: " + err.Error())
			} else {
				for _, spec := range payload.Specs {
					spec = cloneToolSpec(spec)
					spec.Name = strings.TrimSpace(spec.Name)
					if spec.Name == "" {
						continue
					}
					handle := "tool:" + spec.Name
					scope := worldStateScopeFromEvents(runID, events)
					state := r.capabilityStateFromToolSpec(ctx, scope, spec, snapshot, contextEvent.Sequence, permissionMode, handle)
					if permissionSnapshot != nil && state.PolicyHash == "" {
						state.PolicyHash = permissionSnapshot.PolicyHash
					}
					summary := firstNonEmpty(spec.Description, spec.Name)
					draft.addEntry(WorldStateEntry{
						ID:               "capability:" + spec.Name,
						Partition:        WorldStatePartitionCapability,
						Kind:             "capability",
						Subject:          spec.Name,
						StateOrPredicate: state.Status,
						Summary:          summary,
						Source:           "tool_schema_snapshot",
						SourceID:         snapshot.SnapshotID,
						SourceSeq:        contextEvent.Sequence,
						Authority:        "observed",
						Confidence:       1,
						ObservedAt:       contextEvent.OccurredAt,
						Capability:       &state,
						Metadata: map[string]any{
							"context_packet_id": snapshot.ContextPacketID,
							"tool_schema_hash":  snapshot.SchemaHash,
						},
					})
					draft.addHandle(WorldStateHandle{
						Handle:              handle,
						Kind:                "tool",
						Partition:           WorldStatePartitionCapability,
						Summary:             summary,
						Source:              "tool_schema_snapshot",
						SourceID:            snapshot.SnapshotID,
						SourceSeq:           contextEvent.Sequence,
						ContentHash:         spec.SchemaHash,
						ValidWithinSnapshot: true,
						ValidUntilSourceSeq: maxSeq,
						Reloadable:          true,
						RequiresPermission:  !state.Authorized,
						Permission:          state.Permission,
						RedactionLevel:      "metadata",
					})
				}
			}
		}
	}
	r.addInventoryCapabilityWorldState(ctx, runID, events, draft, maxSeq, permissionSnapshot)
}

func (r *Runtime) addInventoryCapabilityWorldState(ctx context.Context, runID string, events []RuntimeEventEnvelope, draft *worldStatePartitionDraft, maxSeq int64, permissionSnapshot *PermissionSnapshot) {
	scope := worldStateScopeFromEvents(runID, events)
	inventories := []struct {
		kind     string
		provider string
		items    []CapabilityInventoryItem
		err      error
	}{
		{kind: "mcp", provider: "mcp_provider"},
		{kind: "skill", provider: "skill_provider"},
	}
	if r.mcpProvider != nil {
		inventories[0].items, inventories[0].err = r.mcpProvider.MCPInventory(ctx, scope)
	}
	if r.skillProvider != nil {
		inventories[1].items, inventories[1].err = r.skillProvider.SkillInventory(ctx, scope)
	}
	for _, inventory := range inventories {
		if inventory.err != nil {
			draft.addWarning(inventory.provider + " unavailable: " + inventory.err.Error())
			continue
		}
		for _, item := range inventory.items {
			item.ID = strings.TrimSpace(item.ID)
			if item.ID == "" {
				continue
			}
			state := r.capabilityStateFromInventoryItem(ctx, scope, item, inventory.kind, inventory.provider, maxSeq, permissionSnapshot)
			handle := inventory.kind + ":" + item.ID
			state.Handle = handle
			summary := firstNonEmpty(item.Summary, item.ID)
			subject := inventory.kind + ":" + item.ID
			draft.addEntry(WorldStateEntry{
				ID:               "capability:" + subject,
				Partition:        WorldStatePartitionCapability,
				Kind:             "capability",
				Subject:          subject,
				StateOrPredicate: state.Status,
				Summary:          summary,
				Source:           inventory.provider,
				SourceID:         item.ID,
				SourceSeq:        maxSeq,
				Authority:        "host_projection",
				Confidence:       1,
				ObservedAt:       latestObservedAt(events),
				Capability:       &state,
				Metadata:         clonePayload(item.Metadata),
			})
			draft.addHandle(WorldStateHandle{
				Handle:              handle,
				Kind:                inventory.kind,
				Partition:           WorldStatePartitionCapability,
				Summary:             summary,
				Source:              inventory.provider,
				SourceID:            item.ID,
				SourceSeq:           maxSeq,
				ContentHash:         item.SchemaHash,
				ValidWithinSnapshot: true,
				ValidUntilSourceSeq: maxSeq,
				Reloadable:          true,
				RequiresPermission:  !state.Authorized,
				Permission:          state.Permission,
				RedactionLevel:      "metadata",
			})
		}
	}
}

func (r *Runtime) capabilityStateFromInventoryItem(ctx context.Context, scope ExecutionScope, item CapabilityInventoryItem, kind, provider string, sourceSeq int64, permissionSnapshot *PermissionSnapshot) CapabilityState {
	permission := firstNonEmpty(item.Permission, permissionForInventoryKind(kind))
	spec := ToolSpec{
		Name:           item.ID,
		Description:    item.Summary,
		Namespace:      item.Namespace,
		ProviderName:   firstNonEmpty(item.ProviderName, provider),
		ReadOnly:       item.ReadOnly,
		RiskLevel:      item.Risk,
		SideEffectKind: item.Boundary,
		SchemaHash:     item.SchemaHash,
		Version:        item.Version,
		RequiredGrants: item.RequiredGrants,
	}
	if len(spec.RequiredGrants) == 0 && permission != "" {
		spec.RequiredGrants = []ScopedPermissionGrant{{Capability: permission, Scope: PermissionGrantScopeRun}}
	}
	auth := r.capabilityAuthorizationForToolSpec(ctx, scope, spec, scope.PermissionMode)
	if permission != "" && permission != toolSpecPermission(spec) {
		auth.Required = normalizeRequestedGrants(requiredGrantsForToolSpec(scope, spec, permission), scope)
	}
	status := CapabilityStateAvailable
	if !item.Available {
		status = CapabilityStateVisible
	}
	if auth.Authorized {
		status = CapabilityStateAuthorized
	}
	if permissionSnapshot != nil && auth.PolicyHash == "" {
		auth.PolicyHash = permissionSnapshot.PolicyHash
	}
	matchedGrantID := ""
	matchedGrantScope := ""
	if auth.Grant != nil {
		matchedGrantID = auth.Grant.ID
		matchedGrantScope = auth.Grant.Scope
	}
	return CapabilityState{
		ID:                  item.ID,
		Kind:                kind,
		Namespace:           item.Namespace,
		Version:             item.Version,
		Status:              status,
		Visible:             item.Visible,
		Available:           item.Available,
		Authorized:          auth.Authorized,
		Source:              provider,
		SourceID:            item.ID,
		SourceSeq:           sourceSeq,
		Authority:           "host_projection",
		Summary:             item.Summary,
		Risk:                item.Risk,
		Boundary:            item.Boundary,
		Permission:          permission,
		SchemaHash:          item.SchemaHash,
		ProviderName:        firstNonEmpty(item.ProviderName, provider),
		ReadOnly:            item.ReadOnly,
		AuthorizationReason: auth.Reason,
		MatchedGrantID:      matchedGrantID,
		MatchedGrantScope:   matchedGrantScope,
		PolicyHash:          auth.PolicyHash,
		RequiredGrants:      append([]ScopedPermissionGrant(nil), auth.Required...),
	}
}

func permissionForInventoryKind(kind string) string {
	switch kind {
	case "mcp":
		return PermissionCapabilityMCPCall
	case "skill":
		return PermissionCapabilityToolCall
	default:
		return PermissionCapabilityToolCall
	}
}

func (r *Runtime) capabilityStateFromToolSpec(ctx context.Context, scope ExecutionScope, spec ToolSpec, snapshot persistence.ToolSchemaSnapshotRecord, sourceSeq int64, permissionMode, handle string) CapabilityState {
	permission := toolSpecPermission(spec)
	auth := r.capabilityAuthorizationForToolSpec(ctx, scope, spec, permissionMode)
	authorized := auth.Authorized
	status := CapabilityStateAvailable
	if authorized {
		status = CapabilityStateAuthorized
	}
	matchedGrantID := ""
	matchedGrantScope := ""
	if auth.Grant != nil {
		matchedGrantID = auth.Grant.ID
		matchedGrantScope = auth.Grant.Scope
	}
	return CapabilityState{
		ID:                  spec.Name,
		Kind:                "tool",
		Namespace:           spec.Namespace,
		Version:             firstNonEmpty(spec.Version, spec.Epoch),
		Status:              status,
		Visible:             true,
		Available:           true,
		Authorized:          authorized,
		Source:              "tool_schema_snapshot",
		SourceID:            snapshot.SnapshotID,
		SourceSeq:           sourceSeq,
		Authority:           "observed",
		Summary:             spec.Description,
		Risk:                spec.RiskLevel,
		Boundary:            spec.SideEffectKind,
		Permission:          permission,
		SchemaHash:          firstNonEmpty(spec.SchemaHash, snapshot.SchemaHash),
		ProviderName:        spec.ProviderName,
		ReadOnly:            spec.ReadOnly,
		ConcurrencySafe:     spec.ConcurrencySafe,
		ResourceLocks:       append([]ResourceLock(nil), spec.ResourceLocks...),
		Handle:              handle,
		AuthorizationReason: auth.Reason,
		MatchedGrantID:      matchedGrantID,
		MatchedGrantScope:   matchedGrantScope,
		PolicyHash:          auth.PolicyHash,
		RequiredGrants:      append([]ScopedPermissionGrant(nil), auth.Required...),
	}
}

func (r *Runtime) toolSpecAuthorizedByGrant(ctx context.Context, scope ExecutionScope, spec ToolSpec, permission string) bool {
	if r == nil || r.kernel == nil || r.kernel.store == nil {
		return false
	}
	check := permissionCheck{
		scope:      scope,
		action:     ProposedAction{Kind: PermissionCapabilityToolCall, Target: spec.Name},
		capability: permission,
		toolTarget: spec.Name,
		resource:   spec.Name,
		readOnly:   spec.ReadOnly,
	}
	if strings.TrimSpace(check.action.ActionID) == "" {
		check.action.ActionID = "worldstate:" + spec.Name
	}
	_, ok := r.findPermissionGrant(ctx, check)
	return ok
}

func toolSpecPermission(spec ToolSpec) string {
	switch strings.TrimSpace(spec.SideEffectKind) {
	case "workspace.write", "fs.write", "file.write":
		return PermissionCapabilityWorkspaceWrite
	}
	if len(spec.RequiredGrants) > 0 && strings.TrimSpace(spec.RequiredGrants[0].Capability) != "" {
		return spec.RequiredGrants[0].Capability
	}
	return PermissionCapabilityToolCall
}
