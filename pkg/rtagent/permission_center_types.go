package rtagent

type permissionCheck struct {
	scope                ExecutionScope
	action               ProposedAction
	capability           string
	toolTarget           string
	resource             string
	activityID           string
	reason               string
	readOnly             bool
	workspaceWrite       bool
	dangerousCommand     bool
	risk                 string
	argumentsPreview     string
	toolSchemaSnapshotID string
	toolSchemaHash       string
	toolEpoch            string
	requestedGrants      []ScopedPermissionGrant
}

type permissionRecordScope struct {
	Scope        ExecutionScope        `json:"scope"`
	Action       ProposedAction        `json:"action"`
	Grant        ScopedPermissionGrant `json:"grant"`
	ActivityID   string                `json:"activity_id,omitempty"`
	Reason       string                `json:"reason,omitempty"`
	Continuation *approvalContinuation `json:"continuation,omitempty"`
}

type approvalContinuation struct {
	ToolCall         *ToolCall         `json:"tool_call,omitempty"`
	PendingToolCalls []ToolCall        `json:"pending_tool_calls,omitempty"`
	PlanArtifact     *PlanArtifact     `json:"plan_artifact,omitempty"`
	Input            string            `json:"input,omitempty"`
	Payload          map[string]any    `json:"payload,omitempty"`
	Iteration        int               `json:"iteration,omitempty"`
	ToolRounds       int               `json:"tool_rounds,omitempty"`
	Messages         []ModelMessage    `json:"messages,omitempty"`
	Observations     []ToolObservation `json:"observations,omitempty"`
	Packet           *ContextPacket    `json:"packet,omitempty"`
}
