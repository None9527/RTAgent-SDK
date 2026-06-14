package rtagent

import "time"

type ApprovalRequest struct {
	ID                   string                   `json:"id"`
	Kind                 string                   `json:"kind"`
	Reviewer             string                   `json:"reviewer,omitempty"`
	Scope                string                   `json:"scope,omitempty"`
	ToolTarget           string                   `json:"tool_target,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Permission           string                   `json:"permission,omitempty"`
	ArgumentsPreview     string                   `json:"arguments_preview,omitempty"`
	AvailableDecisions   []ApprovalDecisionOption `json:"available_decisions,omitempty"`
	RequestedGrants      []ScopedPermissionGrant  `json:"requested_grants,omitempty"`
	Risk                 string                   `json:"risk,omitempty"`
	BoundaryDecisionID   string                   `json:"boundary_decision_id,omitempty"`
	ToolSchemaSnapshotID string                   `json:"tool_schema_snapshot_id,omitempty"`
	ToolSchemaHash       string                   `json:"tool_schema_hash,omitempty"`
	ToolCallID           string                   `json:"tool_call_id,omitempty"`
	ToolEpoch            string                   `json:"tool_epoch,omitempty"`
	RunID                string                   `json:"run_id,omitempty"`
	SessionID            string                   `json:"session_id,omitempty"`
	RootRunID            string                   `json:"root_run_id,omitempty"`
	TaskID               string                   `json:"task_id,omitempty"`
	ActorID              string                   `json:"actor_id,omitempty"`
	OwnerID              string                   `json:"owner_id,omitempty"`
	ScopeHash            string                   `json:"scope_hash,omitempty"`
	IntegrityHash        string                   `json:"integrity_hash,omitempty"`
	EvidenceRefs         []EvidenceRef            `json:"evidence_refs,omitempty"`
}

type ApprovalDecisionOption struct {
	Decision string `json:"decision"`
	Label    string `json:"label"`
	Scope    string `json:"scope,omitempty"`
}

type ScopedPermissionGrant struct {
	ID         string `json:"id,omitempty"`
	Capability string `json:"capability"`
	ToolTarget string `json:"tool_target,omitempty"`
	Resource   string `json:"resource,omitempty"`
	Scope      string `json:"scope"`
	ActionID   string `json:"action_id,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	RootRunID  string `json:"root_run_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	OwnerID    string `json:"owner_id,omitempty"`
	Decision   string `json:"decision,omitempty"`
	ApprovalID string `json:"approval_id,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

type PermissionCheckRequest struct {
	Scope                ExecutionScope `json:"scope"`
	Action               ProposedAction `json:"action"`
	ToolCall             *ToolCall      `json:"tool_call,omitempty"`
	ToolSpec             *ToolSpec      `json:"tool_spec,omitempty"`
	ToolSchemaSnapshotID string         `json:"tool_schema_snapshot_id,omitempty"`
	ActivityID           string         `json:"activity_id,omitempty"`
	Reason               string         `json:"reason,omitempty"`
}

type PermissionCheckResult struct {
	Status          string                 `json:"status"`
	Decision        string                 `json:"decision,omitempty"`
	PermissionID    string                 `json:"permission_id,omitempty"`
	ApprovalRequest *ApprovalRequest       `json:"approval_request,omitempty"`
	Grant           *ScopedPermissionGrant `json:"grant,omitempty"`
	Reason          string                 `json:"reason,omitempty"`
}

type PermissionDecisionRequest struct {
	ApprovalID string         `json:"approval_id"`
	Decision   string         `json:"decision"`
	Scope      ExecutionScope `json:"scope,omitempty"`
	ActorID    string         `json:"actor_id,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type PermissionDecisionResult struct {
	Status       string                 `json:"status"`
	PermissionID string                 `json:"permission_id,omitempty"`
	Grant        *ScopedPermissionGrant `json:"grant,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
}

type PermissionRequiredError struct {
	ApprovalRequest *ApprovalRequest `json:"approval_request,omitempty"`
	Message         string           `json:"message,omitempty"`
}

func (e *PermissionRequiredError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return "permission approval required"
}

type EvidenceRef struct {
	ID      string `json:"id"`
	Kind    string `json:"kind,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type PlanArtifact struct {
	ID               string              `json:"id"`
	RunID            string              `json:"run_id"`
	SessionID        string              `json:"session_id,omitempty"`
	Revision         int                 `json:"revision"`
	State            string              `json:"state"`
	Goal             string              `json:"goal,omitempty"`
	Content          string              `json:"content,omitempty"`
	Constraints      []string            `json:"constraints,omitempty"`
	Invariants       []string            `json:"invariants,omitempty"`
	Decisions        []string            `json:"decisions,omitempty"`
	VerificationPlan []string            `json:"verification_plan,omitempty"`
	Dialogue         []PlanDialogueEntry `json:"dialogue,omitempty"`
	UserFeedback     string              `json:"user_feedback,omitempty"`
	Source           string              `json:"source,omitempty"`
	CreatedAt        time.Time           `json:"created_at,omitempty"`
	UpdatedAt        time.Time           `json:"updated_at,omitempty"`
}

type PlanDialogueEntry struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type PermissionSnapshotQuery struct {
	RunID string `json:"run_id"`
}

type PermissionSnapshot struct {
	SchemaVersion    string                     `json:"schema_version"`
	SessionID        string                     `json:"session_id,omitempty"`
	RunID            string                     `json:"run_id,omitempty"`
	Mode             string                     `json:"mode,omitempty"`
	PolicyHash       string                     `json:"policy_hash,omitempty"`
	SourceSeq        int64                      `json:"source_seq,omitempty"`
	ActiveGrants     []ScopedPermissionGrant    `json:"active_grants,omitempty"`
	PendingDecisions []ApprovalRequest          `json:"pending_decisions,omitempty"`
	DeniedDecisions  []PermissionDecisionRecord `json:"denied_decisions,omitempty"`
	ResourceRules    []ResourceRuleSnapshot     `json:"resource_rules,omitempty"`
	Warnings         []string                   `json:"warnings,omitempty"`
}

type PermissionDecisionRecord struct {
	ID            string `json:"id,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Capability    string `json:"capability,omitempty"`
	ToolTarget    string `json:"tool_target,omitempty"`
	Resource      string `json:"resource,omitempty"`
	Decision      string `json:"decision,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Risk          string `json:"risk,omitempty"`
	SourceEventID string `json:"source_event_id,omitempty"`
	SourceSeq     int64  `json:"source_seq,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type ResourceRuleSnapshot struct {
	Kind     string `json:"kind"`
	Resource string `json:"resource,omitempty"`
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Source   string `json:"source,omitempty"`
}

type ProposedAction struct {
	ActionID string         `json:"action_id"`
	Kind     string         `json:"kind"`
	Target   string         `json:"target"`
	Args     map[string]any `json:"args"`
}
