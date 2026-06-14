package rtagent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPublicQueryAndRequestJSONContractUsesSnakeCase(t *testing.T) {
	cases := []struct {
		name       string
		value      any
		wantKeys   []string
		forbidKeys []string
	}{
		{
			name: "submit run request",
			value: SubmitRunRequest{
				Kind:                  "chat",
				RunID:                 "run-json",
				SessionID:             "session-json",
				SessionAlias:          "alias-json",
				WorkingDir:            "/workspace",
				Target:                "target",
				Mode:                  PermissionDefault,
				PlanningState:         PlanningPlan,
				PrePlanPermissionMode: PermissionDefault,
				Profile:               "profile",
				RootRunID:             "root-json",
				ParentRunID:           "parent-json",
				TaskID:                "task-json",
				Role:                  "agent",
				Scope:                 map[string]any{"workspace_id": "workspace-json"},
				Input:                 "hello",
				Args:                  map[string]any{"temperature": 0.1},
			},
			wantKeys: []string{
				"kind", "run_id", "session_id", "session_alias", "working_dir", "target",
				"mode", "planning_state", "pre_plan_permission_mode", "profile", "root_run_id",
				"parent_run_id", "task_id", "role", "scope", "input", "args",
			},
			forbidKeys: []string{
				"Kind", "RunID", "SessionID", "SessionAlias", "WorkingDir", "Target",
				"Mode", "PlanningState", "PrePlanPermissionMode", "Profile", "RootRunID",
				"ParentRunID", "TaskID", "Role", "Scope", "Input", "Args",
			},
		},
		{
			name: "runtime command",
			value: RuntimeCommand{
				ID:             "command-json",
				Kind:           "run",
				Scope:          ExecutionScope{RunID: "run-json", SessionID: "session-json"},
				Payload:        map[string]any{"input": "hello"},
				IdempotencyKey: "idem-json",
				RequestedBy:    "tester",
				CreatedAt:      time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC),
			},
			wantKeys:   []string{"id", "kind", "scope", "payload", "idempotency_key", "requested_by", "created_at"},
			forbidKeys: []string{"ID", "Kind", "Scope", "Payload", "IdempotencyKey", "RequestedBy", "CreatedAt"},
		},
		{
			name: "runtime event draft",
			value: RuntimeEventDraft{
				EventID:    "event-json",
				RunID:      "run-json",
				Kind:       EventKindRunCreated,
				Sequence:   7,
				OccurredAt: time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC),
				Message:    "created",
				Payload:    map[string]any{"session_id": "session-json"},
			},
			wantKeys:   []string{"event_id", "run_id", "kind", "sequence", "occurred_at", "message", "payload"},
			forbidKeys: []string{"EventID", "RunID", "Kind", "Sequence", "OccurredAt", "Message", "Payload"},
		},
		{
			name: "event query",
			value: EventQuery{
				RunID:     "run-json",
				SessionID: "session-json",
				AfterSeq:  7,
			},
			wantKeys:   []string{"run_id", "session_id", "after_seq"},
			forbidKeys: []string{"RunID", "SessionID", "AfterSeq"},
		},
		{
			name: "inspect query",
			value: InspectQuery{
				RunID:     "run-json",
				SessionID: "session-json",
				AfterSeq:  7,
			},
			wantKeys:   []string{"run_id", "session_id", "after_seq"},
			forbidKeys: []string{"RunID", "SessionID", "AfterSeq"},
		},
		{
			name:       "session query",
			value:      SessionQuery{SessionID: "session-json"},
			wantKeys:   []string{"session_id"},
			forbidKeys: []string{"SessionID"},
		},
		{
			name: "session graph query",
			value: SessionGraphQuery{
				SessionID: "session-json",
				RootRunID: "root-json",
			},
			wantKeys:   []string{"session_id", "root_run_id"},
			forbidKeys: []string{"SessionID", "RootRunID"},
		},
		{
			name: "resume run request",
			value: ResumeRunRequest{
				RunID:        "run-json",
				CheckpointID: "checkpoint-json",
				ApprovalID:   "approval-json",
				Decision:     PermissionDecisionAllowOnce,
				Scope:        ExecutionScope{RunID: "run-json", SessionID: "session-json"},
			},
			wantKeys:   []string{"run_id", "checkpoint_id", "approval_id", "decision", "scope"},
			forbidKeys: []string{"RunID", "CheckpointID", "ApprovalID", "Decision", "Scope"},
		},
		{
			name:       "checkpoint graph query",
			value:      CheckpointGraphQuery{RunID: "run-json"},
			wantKeys:   []string{"run_id"},
			forbidKeys: []string{"RunID"},
		},
		{
			name: "stop session request",
			value: StopSessionRequest{
				SessionID:   "session-json",
				Mode:        StopSessionModeDrain,
				Reason:      "test",
				RequestedBy: "tester",
			},
			wantKeys:   []string{"session_id", "mode", "reason", "requested_by"},
			forbidKeys: []string{"SessionID", "Mode", "Reason", "RequestedBy"},
		},
		{
			name:       "permission snapshot query",
			value:      PermissionSnapshotQuery{RunID: "run-json"},
			wantKeys:   []string{"run_id"},
			forbidKeys: []string{"RunID"},
		},
		{
			name: "permission check request",
			value: PermissionCheckRequest{
				Scope:  ExecutionScope{RunID: "run-json", SessionID: "session-json"},
				Action: ProposedAction{ActionID: "action-json", Kind: PermissionCapabilityToolCall, Target: "tool-json"},
				ToolCall: &ToolCall{
					ID:   "tool-call-json",
					Name: "tool-json",
				},
				ToolSpec: &ToolSpec{
					Name: "tool-json",
				},
				ToolSchemaSnapshotID: "schema-json",
				ActivityID:           "activity-json",
				Reason:               "test",
			},
			wantKeys: []string{"scope", "action", "tool_call", "tool_spec", "tool_schema_snapshot_id", "activity_id", "reason"},
			forbidKeys: []string{
				"Scope", "Action", "ToolCall", "ToolSpec", "ToolSchemaSnapshotID", "ActivityID", "Reason",
			},
		},
		{
			name: "permission decision request",
			value: PermissionDecisionRequest{
				ApprovalID: "approval-json",
				Decision:   PermissionDecisionAllowForRun,
				Scope:      ExecutionScope{RunID: "run-json", SessionID: "session-json"},
				ActorID:    "tester",
				Reason:     "approved",
			},
			wantKeys:   []string{"approval_id", "decision", "scope", "actor_id", "reason"},
			forbidKeys: []string{"ApprovalID", "Decision", "Scope", "ActorID", "Reason"},
		},
		{
			name: "world state query",
			value: WorldStateQuery{
				RunID:     "run-json",
				Partition: WorldStatePartitionCapability,
			},
			wantKeys:   []string{"run_id", "partition"},
			forbidKeys: []string{"RunID", "Partition"},
		},
		{
			name: "write file request",
			value: WriteFileRequest{
				RelativePath:     "notes/result.txt",
				Content:          []byte("ok"),
				ActiveActivityID: "activity-json",
				RunID:            "run-json",
				Scope: ExecutionScope{
					RunID:     "run-json",
					SessionID: "session-json",
				},
			},
			wantKeys:   []string{"relative_path", "content", "active_activity_id", "run_id", "scope"},
			forbidKeys: []string{"RelativePath", "Content", "ActiveActivityID", "RunID", "Scope"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal public contract: %v", err)
			}
			var object map[string]any
			if err := json.Unmarshal(raw, &object); err != nil {
				t.Fatalf("unmarshal public contract: %v", err)
			}
			for _, key := range tc.wantKeys {
				if _, ok := object[key]; !ok {
					t.Fatalf("expected JSON key %q in %s", key, raw)
				}
			}
			for _, key := range tc.forbidKeys {
				if _, ok := object[key]; ok {
					t.Fatalf("unexpected Go-style JSON key %q in %s", key, raw)
				}
			}
		})
	}
}
