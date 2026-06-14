package rtagent

import (
	"context"
	"testing"

	"rtagent/internal/domain/persistence"
)

func TestRuntimeWorldStateProjectsTypedPartitions(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{&recordingToolProvider{
			specs: []ToolSpec{{
				Name:                 "echo",
				Description:          "echo input",
				Namespace:            "test",
				ProviderName:         "recording",
				ReadOnly:             true,
				RiskLevel:            "low",
				ConcurrencySafe:      true,
				SideEffectKind:       "none",
				ResourceLocks:        []ResourceLock{{Kind: "workspace", Key: "read", Mode: "shared"}},
				RequiredGrants:       []ScopedPermissionGrant{{Capability: PermissionCapabilityToolCall, Scope: PermissionGrantScopeRun}},
				OutputPolicy:         ToolOutputPolicy{MaxModelBytes: 512},
				OutputSchema:         map[string]any{"type": "object"},
				Parameters:           map[string]any{"type": "object"},
				FreeformGrammar:      "",
				ExecutionConstraints: ExecutionConstraints{Network: "none"},
			}},
		}}
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-worldstate-typed",
		SessionID: "session-worldstate-typed",
		Input:     "project typed world state",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if err := rt.RegisterContextHandle(ctx, ContextHandle{
		HandleID:              "notes",
		RunID:                 "run-worldstate-typed",
		Kind:                  "document",
		Title:                 "Runtime notes",
		Summary:               "notes handle",
		SourceRef:             "file:notes.md",
		TokenEstimate:         128,
		Freshness:             1,
		MaterializationPolicy: "on_demand",
		EvidenceRefs:          []string{"evidence:notes"},
	}); err != nil {
		t.Fatalf("RegisterContextHandle() error = %v", err)
	}
	if _, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:   "run-worldstate-typed",
		Kind:    EventKind("task.blocked"),
		Message: "task blocked for legacy worldstate coverage",
		Payload: map[string]any{
			"task_id":   "legacy-task",
			"objective": "cover legacy Entries filtering",
			"status":    "blocked",
		},
	}); err != nil {
		t.Fatalf("Emit(task.blocked) error = %v", err)
	}
	if _, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:   "run-worldstate-typed",
		Kind:    EventKind("file.modified"),
		Message: "file modified for legacy worldstate coverage",
		Payload: map[string]any{
			"filepath":    "notes.md",
			"artifact_id": "artifact-notes",
		},
	}); err != nil {
		t.Fatalf("Emit(file.modified) error = %v", err)
	}

	snapshot, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-worldstate-typed"})
	if err != nil {
		t.Fatalf("WorldState() error = %v", err)
	}
	if snapshot.SchemaVersion != SchemaWorldStateV1 {
		t.Fatalf("SchemaVersion = %q, want %q", snapshot.SchemaVersion, SchemaWorldStateV1)
	}
	if snapshot.RuntimeEpoch == "" {
		t.Fatalf("RuntimeEpoch is empty")
	}
	if snapshot.SourceWatermark == "" {
		t.Fatalf("SourceWatermark is empty")
	}
	if len(snapshot.Partitions) == 0 {
		t.Fatalf("Partitions is empty")
	}

	capabilityEntry := findWorldStateEntry(snapshot, WorldStatePartitionCapability, "echo")
	if capabilityEntry == nil {
		t.Fatalf("missing capability entry for echo")
	}
	if capabilityEntry.Capability == nil {
		t.Fatalf("capability entry has nil Capability")
	}
	if capabilityEntry.Capability.Status != CapabilityStateAuthorized {
		t.Fatalf("Capability.Status = %q, want %q", capabilityEntry.Capability.Status, CapabilityStateAuthorized)
	}
	if !capabilityEntry.Capability.ReadOnly {
		t.Fatalf("Capability.ReadOnly = false, want true")
	}
	if capabilityEntry.Capability.SchemaHash == "" {
		t.Fatalf("Capability.SchemaHash is empty")
	}
	if !hasWorldStateHandle(snapshot, WorldStatePartitionCapability, "tool:echo") {
		t.Fatalf("missing tool:echo handle")
	}
	if findWorldStateKind(snapshot, WorldStatePartitionContext, "context_packet") == nil {
		t.Fatalf("missing context packet entry")
	}
	if !hasWorldStateHandle(snapshot, WorldStatePartitionContext, "context_handle:notes") {
		t.Fatalf("missing registered context handle")
	}
	if findWorldStateKind(snapshot, WorldStatePartitionTask, "runtime_event") == nil {
		t.Fatalf("missing task runtime event trajectory")
	}
	if len(snapshot.Entries) == 0 {
		t.Fatalf("missing legacy compatibility entries")
	}

	capabilityOnly, err := rt.WorldState(ctx, WorldStateQuery{
		RunID:     "run-worldstate-typed",
		Partition: WorldStatePartitionCapability,
	})
	if err != nil {
		t.Fatalf("WorldState(capability) error = %v", err)
	}
	if len(capabilityOnly.Partitions) != 1 {
		t.Fatalf("capability partition count = %d, want 1", len(capabilityOnly.Partitions))
	}
	if capabilityOnly.Partitions[0].Partition != WorldStatePartitionCapability {
		t.Fatalf("partition = %q, want %q", capabilityOnly.Partitions[0].Partition, WorldStatePartitionCapability)
	}
	assertWorldStateSnapshotOnlyPartition(t, capabilityOnly, WorldStatePartitionCapability)
	if !hasWorldStateHandle(capabilityOnly, WorldStatePartitionCapability, "tool:echo") {
		t.Fatalf("filtered snapshot missing tool:echo handle")
	}
	if hasWorldStateHandle(capabilityOnly, WorldStatePartitionContext, "context_handle:notes") {
		t.Fatalf("filtered capability snapshot leaked context handle")
	}
	if len(capabilityOnly.Entries) >= len(snapshot.Entries) {
		t.Fatalf("filtered legacy entries len = %d, unfiltered len = %d; want filter to narrow compatibility entries", len(capabilityOnly.Entries), len(snapshot.Entries))
	}
}

func TestRuntimeWorldStateProjectsMemoryAndGrantAwareCapability(t *testing.T) {
	ctx := context.Background()
	editSpec := ToolSpec{
		Name:           "edit",
		Description:    "edit workspace",
		ProviderName:   "recording",
		ReadOnly:       false,
		RiskLevel:      "medium",
		SideEffectKind: "workspace.write",
		RequiredGrants: []ScopedPermissionGrant{{
			Capability: PermissionCapabilityToolCall,
			Scope:      PermissionGrantScopeRun,
		}},
		Parameters: map[string]any{"type": "object"},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{&recordingToolProvider{specs: []ToolSpec{editSpec}}}
	})
	scope := ExecutionScope{
		RunID:          "run-worldstate-memory-grant",
		SessionID:      "session-worldstate-memory-grant",
		RootRunID:      "run-worldstate-memory-grant",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}

	if _, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     scope.RunID,
		SessionID: scope.SessionID,
		Input:     "project memory and grants",
	}, Identity{ActorID: scope.ActorID, OwnerID: scope.OwnerID}); err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if err := rt.kernel.store.PutMemory(ctx, persistence.MemoryRecord{
		RecordID:   "mem-runtime-fact",
		Stage:      persistence.MemoryStageCommitted,
		Kind:       persistence.MemoryKindValidatedFact,
		Origin:     persistence.MemoryOriginValidatedRuntimeFact,
		Scope:      scope.RunID,
		Topic:      "runtime sdk fact",
		Content:    "WorldState can project durable memory.",
		Confidence: 0.95,
		Source: persistence.SourceRef{
			Kind:  persistence.SourceRun,
			RunID: scope.RunID,
		},
		CitationIDs: []string{"evidence:runtime-fact"},
		CreatedAt:   "2026-06-09T10:00:00Z",
	}); err != nil {
		t.Fatalf("PutMemory() error = %v", err)
	}
	check, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: scope,
		Action: ProposedAction{
			ActionID: "edit-action",
			Kind:     PermissionCapabilityToolCall,
			Target:   "edit",
		},
		ToolCall: &ToolCall{ID: "edit-action", Name: "edit"},
		ToolSpec: &editSpec,
		Reason:   "test grant-aware world state",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if check.Status != PermissionStatusRequiresApproval {
		t.Fatalf("permission Status = %q, want %q", check.Status, PermissionStatusRequiresApproval)
	}
	if check.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil")
	}
	if _, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: check.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		Scope:      scope,
		ActorID:    "reviewer",
	}); err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}

	snapshot, err := rt.WorldState(ctx, WorldStateQuery{RunID: scope.RunID})
	if err != nil {
		t.Fatalf("WorldState() error = %v", err)
	}
	memoryEntry := findWorldStateEntry(snapshot, WorldStatePartitionMemory, "runtime sdk fact")
	if memoryEntry == nil {
		t.Fatalf("missing memory entry")
	}
	if memoryEntry.StateOrPredicate != string(persistence.MemoryStageCommitted) {
		t.Fatalf("memory state = %q, want %q", memoryEntry.StateOrPredicate, persistence.MemoryStageCommitted)
	}
	if memoryEntry.Confidence != 0.95 {
		t.Fatalf("memory confidence = %v, want 0.95", memoryEntry.Confidence)
	}
	if len(memoryEntry.EvidenceRefs) != 1 || memoryEntry.EvidenceRefs[0] != "evidence:runtime-fact" {
		t.Fatalf("memory evidence refs = %v", memoryEntry.EvidenceRefs)
	}

	capabilityEntry := findWorldStateEntry(snapshot, WorldStatePartitionCapability, "edit")
	if capabilityEntry == nil || capabilityEntry.Capability == nil {
		t.Fatalf("missing edit capability")
	}
	if !capabilityEntry.Capability.Authorized {
		t.Fatalf("Capability.Authorized = false, want true after allow_for_run grant")
	}
	if capabilityEntry.Capability.Status != CapabilityStateAuthorized {
		t.Fatalf("Capability.Status = %q, want %q", capabilityEntry.Capability.Status, CapabilityStateAuthorized)
	}
	handle := findWorldStateHandle(snapshot, WorldStatePartitionCapability, "tool:edit")
	if handle == nil {
		t.Fatalf("missing tool:edit handle")
	}
	if handle.RequiresPermission {
		t.Fatalf("tool:edit RequiresPermission = true, want false after grant")
	}
}

func TestRuntimeWorldStateMarksGrantedCapabilityUnavailableAfterSessionStop(t *testing.T) {
	ctx := context.Background()
	editSpec := ToolSpec{
		Name:           "edit",
		Description:    "edit workspace",
		ProviderName:   "recording",
		ReadOnly:       false,
		RiskLevel:      "medium",
		SideEffectKind: "workspace.write",
		Parameters:     map[string]any{"type": "object"},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{&recordingToolProvider{specs: []ToolSpec{editSpec}}}
	})
	scope := ExecutionScope{
		RunID:          "run-worldstate-stopped-grant",
		SessionID:      "session-worldstate-stopped-grant",
		RootRunID:      "run-worldstate-stopped-grant",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}

	if _, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     scope.RunID,
		SessionID: scope.SessionID,
		Input:     "project stopped grant",
	}, Identity{ActorID: scope.ActorID, OwnerID: scope.OwnerID}); err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	check, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: scope,
		Action: ProposedAction{
			ActionID: "edit-action",
			Kind:     PermissionCapabilityWorkspaceWrite,
			Target:   "edit",
		},
		ToolCall: &ToolCall{ID: "edit-action", Name: "edit"},
		ToolSpec: &editSpec,
		Reason:   "test stopped grant-aware world state",
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if check.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil")
	}
	if _, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: check.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		Scope:      scope,
		ActorID:    "reviewer",
	}); err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if _, err := rt.StopSession(ctx, StopSessionRequest{SessionID: scope.SessionID}); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	snapshot, err := rt.WorldState(ctx, WorldStateQuery{RunID: scope.RunID})
	if err != nil {
		t.Fatalf("WorldState() error = %v", err)
	}
	capabilityEntry := findWorldStateEntry(snapshot, WorldStatePartitionCapability, "edit")
	if capabilityEntry == nil || capabilityEntry.Capability == nil {
		t.Fatalf("missing edit capability")
	}
	if capabilityEntry.Capability.Authorized {
		t.Fatalf("Capability.Authorized = true, want false after session stop")
	}
	if capabilityEntry.Capability.Status == CapabilityStateAuthorized {
		t.Fatalf("Capability.Status = %q, want non-authorized after session stop", capabilityEntry.Capability.Status)
	}
	if capabilityEntry.Capability.MatchedGrantID != "" {
		t.Fatalf("MatchedGrantID = %q, want empty after session stop", capabilityEntry.Capability.MatchedGrantID)
	}
	if capabilityEntry.Capability.AuthorizationReason != "session stopped blocks non-read-only capability" {
		t.Fatalf("AuthorizationReason = %q", capabilityEntry.Capability.AuthorizationReason)
	}
	handle := findWorldStateHandle(snapshot, WorldStatePartitionCapability, "tool:edit")
	if handle == nil {
		t.Fatalf("missing tool:edit handle")
	}
	if !handle.RequiresPermission {
		t.Fatalf("tool:edit RequiresPermission = false, want true after session stop")
	}
}

func TestRuntimeWorldStateProjectsHostProviders(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Memory = MemoryProviderFunc(func(ctx context.Context, scope ExecutionScope) ([]MemoryFact, error) {
			return []MemoryFact{{
				ID:         "host-memory",
				Kind:       "fact",
				Subject:    "host memory fact",
				State:      "committed",
				Summary:    "host projected committed memory",
				Confidence: 0.9,
			}}, nil
		})
		cfg.Host.Hypothesis = HypothesisProviderFunc(func(ctx context.Context, scope ExecutionScope) ([]HypothesisFact, error) {
			return []HypothesisFact{{
				ID:         "host-hypothesis",
				Kind:       "hypothesis",
				Subject:    "runtime design",
				Predicate:  "inferred",
				Summary:    "host projected hypothesis",
				Confidence: 0.4,
			}}, nil
		})
		cfg.Host.MCP = MCPProviderFunc(func(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error) {
			return []CapabilityInventoryItem{{
				ID:           "filesystem",
				Kind:         "mcp_server",
				Summary:      "filesystem MCP inventory",
				ProviderName: "host-mcp",
				Visible:      true,
				Available:    true,
				ReadOnly:     true,
				Permission:   PermissionCapabilityMCPCall,
			}}, nil
		})
		cfg.Host.Skill = SkillProviderFunc(func(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error) {
			return []CapabilityInventoryItem{{
				ID:           "planner",
				Kind:         "skill",
				Summary:      "planner skill inventory",
				ProviderName: "host-skill",
				Visible:      true,
				Available:    true,
				ReadOnly:     false,
				Permission:   PermissionCapabilityToolCall,
			}}, nil
		})
	})

	if _, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-worldstate-host-providers",
		SessionID: "session-worldstate-host-providers",
		Input:     "project host providers",
	}, Identity{ActorID: "tester"}); err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if err := rt.kernel.store.PutMemory(ctx, persistence.MemoryRecord{
		RecordID:   "proposed-fact",
		Stage:      persistence.MemoryStageProposed,
		Kind:       persistence.MemoryKindWorkflowLearning,
		Origin:     persistence.MemoryOriginModelInference,
		Scope:      "run-worldstate-host-providers",
		Topic:      "proposed runtime fact",
		Content:    "Proposed facts belong in hypothesis projection.",
		Confidence: 0.5,
		Source: persistence.SourceRef{
			Kind:  persistence.SourceRun,
			RunID: "run-worldstate-host-providers",
		},
	}); err != nil {
		t.Fatalf("PutMemory(proposed) error = %v", err)
	}

	snapshot, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-worldstate-host-providers"})
	if err != nil {
		t.Fatalf("WorldState() error = %v", err)
	}
	if findWorldStateEntry(snapshot, WorldStatePartitionMemory, "host memory fact") == nil {
		t.Fatalf("missing host memory projection")
	}
	if findWorldStateEntry(snapshot, WorldStatePartitionMemory, "proposed runtime fact") != nil {
		t.Fatalf("proposed memory leaked into committed memory partition")
	}
	if findWorldStateEntry(snapshot, WorldStatePartitionHypothesis, "proposed runtime fact") == nil {
		t.Fatalf("missing proposed memory in hypothesis partition")
	}
	if findWorldStateEntry(snapshot, WorldStatePartitionHypothesis, "runtime design") == nil {
		t.Fatalf("missing host hypothesis projection")
	}
	if !hasWorldStateHandle(snapshot, WorldStatePartitionCapability, "mcp:filesystem") {
		t.Fatalf("missing mcp inventory handle")
	}
	if !hasWorldStateHandle(snapshot, WorldStatePartitionCapability, "skill:planner") {
		t.Fatalf("missing skill inventory handle")
	}
}

func findWorldStateEntry(snapshot WorldStateSnapshot, partition, subject string) *WorldStateEntry {
	for _, part := range snapshot.Partitions {
		if part.Partition != partition {
			continue
		}
		for i := range part.Entries {
			if part.Entries[i].Subject == subject {
				return &part.Entries[i]
			}
		}
	}
	return nil
}

func findWorldStateKind(snapshot WorldStateSnapshot, partition, kind string) *WorldStateEntry {
	for _, part := range snapshot.Partitions {
		if part.Partition != partition {
			continue
		}
		for i := range part.Entries {
			if part.Entries[i].Kind == kind {
				return &part.Entries[i]
			}
		}
	}
	return nil
}

func hasWorldStateHandle(snapshot WorldStateSnapshot, partition, handle string) bool {
	return findWorldStateHandle(snapshot, partition, handle) != nil
}

func findWorldStateHandle(snapshot WorldStateSnapshot, partition, handle string) *WorldStateHandle {
	for i := range snapshot.Handles {
		if snapshot.Handles[i].Partition == partition && snapshot.Handles[i].Handle == handle {
			return &snapshot.Handles[i]
		}
	}
	return nil
}

func assertWorldStateSnapshotOnlyPartition(t *testing.T, snapshot WorldStateSnapshot, partition string) {
	t.Helper()
	for _, part := range snapshot.Partitions {
		if part.Partition != partition {
			t.Fatalf("snapshot leaked partition %q, want only %q", part.Partition, partition)
		}
	}
	for _, handle := range snapshot.Handles {
		if handle.Partition != partition {
			t.Fatalf("snapshot leaked handle partition %q for handle %q, want only %q", handle.Partition, handle.Handle, partition)
		}
	}
	for _, entry := range snapshot.Entries {
		if entry.Partition != partition {
			t.Fatalf("snapshot leaked legacy entry partition %q for entry %q, want only %q", entry.Partition, entry.ID, partition)
		}
	}
}
