package rtagent

import (
	"context"
	"strings"
)

func (r *Runtime) WriteFile(ctx context.Context, req WriteFileRequest) (ArtifactRecord, error) {
	if err := r.ensureReady(); err != nil {
		return ArtifactRecord{}, err
	}
	scope := req.Scope
	if strings.TrimSpace(scope.RunID) == "" {
		scope.RunID = req.RunID
	}
	if strings.TrimSpace(scope.SessionID) == "" {
		scope.SessionID = scope.RunID
	}
	if strings.TrimSpace(scope.ActorID) == "" {
		scope.ActorID = "local"
	}
	if strings.TrimSpace(scope.OwnerID) == "" {
		scope.OwnerID = scope.ActorID
	}
	if strings.TrimSpace(scope.PermissionMode) == "" {
		scope.PermissionMode = PermissionDefault
	}
	permission, err := r.CheckPermission(ctx, PermissionCheckRequest{
		Scope: scope,
		Action: ProposedAction{
			ActionID: "write:" + shortHash(scope.RunID+"|"+req.RelativePath),
			Kind:     PermissionCapabilityWorkspaceWrite,
			Target:   req.RelativePath,
			Args: map[string]any{
				"relative_path": req.RelativePath,
				"byte_size":     len(req.Content),
			},
		},
		ActivityID: req.ActiveActivityID,
		Reason:     "workspace write requested",
	})
	if err != nil {
		return ArtifactRecord{}, err
	}
	switch permission.Status {
	case PermissionStatusDenied:
		return ArtifactRecord{}, &RuntimeError{Code: "permission_denied", Message: firstNonEmpty(permission.Reason, "workspace write denied")}
	case PermissionStatusRequiresApproval:
		return ArtifactRecord{}, &PermissionRequiredError{
			ApprovalRequest: permission.ApprovalRequest,
			Message:         firstNonEmpty(permission.Reason, "workspace write requires approval"),
		}
	}
	rec, err := r.kernel.workspace.WriteFile(ctx, req.RelativePath, req.Content, req.ActiveActivityID, scope.RunID)
	if err != nil {
		return ArtifactRecord{}, err
	}
	return ArtifactRecord{
		ArtifactID: rec.ArtifactID,
		Kind:       rec.Kind,
		Path:       rec.Path,
		SHA256:     rec.SHA256,
		ByteSize:   rec.ByteSize,
		Preview:    rec.Preview,
		CreatedAt:  rec.CreatedAt,
	}, nil
}

func (r *Runtime) EvaluateProposal(ctx context.Context, agentID string, action ProposedAction, activeActivityID string) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	scope := executionScopeFromAction(agentID, action)
	permission, err := r.CheckPermission(ctx, PermissionCheckRequest{
		Scope:      scope,
		Action:     action,
		ActivityID: activeActivityID,
		Reason:     "governance proposal evaluation",
	})
	if err != nil {
		return err
	}
	switch permission.Status {
	case PermissionStatusDenied:
		return &RuntimeError{Code: "permission_denied", Message: firstNonEmpty(permission.Reason, "proposal denied")}
	case PermissionStatusRequiresApproval:
		return &PermissionRequiredError{
			ApprovalRequest: permission.ApprovalRequest,
			Message:         firstNonEmpty(permission.Reason, "proposal requires approval"),
		}
	default:
		return nil
	}
}
