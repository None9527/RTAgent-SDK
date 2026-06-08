package governance

import (
	"context"
	"time"
)

type ProposedAction struct {
	ActionID string                 `json:"action_id"`
	Kind     string                 `json:"kind"` // "fs.read", "fs.write", "shell.exec", "mcp.call"
	Target   string                 `json:"target"`
	Args     map[string]interface{} `json:"args"`
}

type CapabilityPolicy struct {
	Subject   string    `json:"subject"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
	Authority string    `json:"authority"`
	Policy    string    `json:"policy"`
}

type PermissionCenter interface {
	EvaluateProposal(ctx context.Context, agentID string, act ProposedAction, activeActivityID string) error
}

type LeaseManager interface {
	Acquire(ctx context.Context, resource string, activityID string, ttl time.Duration) (string, error)
	Release(ctx context.Context, leaseID string) error
}
