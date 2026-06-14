package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func (r *Runtime) findPermissionGrant(ctx context.Context, check permissionCheck) (ScopedPermissionGrant, bool) {
	candidates := []ScopedPermissionGrant{
		permissionGrantForDecision(check, PermissionDecisionAllowOnce),
		permissionGrantForDecision(check, PermissionDecisionAllowForRun),
		permissionGrantForDecision(check, PermissionDecisionAllowForSession),
		permissionGrantForDecision(check, PermissionDecisionAllowAllForRun),
		permissionGrantForDecision(check, PermissionDecisionAllowAllForSession),
	}
	for _, candidate := range candidates {
		rec, err := r.kernel.store.GetGrant(ctx, candidate.ID)
		if err != nil {
			continue
		}
		if grantExpired(rec.ExpiresAt) {
			continue
		}
		candidate.ID = rec.GrantID
		return candidate, true
	}
	return ScopedPermissionGrant{}, false
}

func (r *Runtime) createPermissionRequest(ctx context.Context, check permissionCheck) (ApprovalRequest, error) {
	permissionID := permissionIDForCheck(check)
	grant := permissionGrantForDecision(check, PermissionDecisionAllowOnce)
	grant.ApprovalID = permissionID
	scope := permissionRecordScope{
		Scope:      check.scope,
		Action:     check.action,
		Grant:      grant,
		ActivityID: check.activityID,
		Reason:     check.reason,
	}
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("marshal permission scope: %w", err)
	}
	if err := r.kernel.store.PutPermission(ctx, persistence.PermissionRecord{
		PermissionID:   permissionID,
		RunID:          check.scope.RunID,
		Subject:        check.scope.ActorID,
		Scope:          string(scopeJSON),
		Granted:        false,
		RequestedAt:    time.Now().UTC().Format(time.RFC3339),
		PolicyWarnings: firstNonEmpty(check.reason, "approval required"),
	}); err != nil {
		return ApprovalRequest{}, fmt.Errorf("put permission: %w", err)
	}
	approval := ApprovalRequest{
		ID:                   permissionID,
		Kind:                 check.capability,
		Reviewer:             "host",
		Scope:                PermissionGrantScopeRun,
		ToolTarget:           check.toolTarget,
		Description:          permissionDescription(check),
		Permission:           check.capability,
		ArgumentsPreview:     check.argumentsPreview,
		AvailableDecisions:   permissionDecisionOptions(check.scope),
		RequestedGrants:      check.requestedGrants,
		Risk:                 check.risk,
		ToolCallID:           check.action.ActionID,
		ToolSchemaSnapshotID: check.toolSchemaSnapshotID,
		ToolSchemaHash:       check.toolSchemaHash,
		ToolEpoch:            check.toolEpoch,
		RunID:                check.scope.RunID,
		SessionID:            check.scope.SessionID,
		RootRunID:            check.scope.RootRunID,
		TaskID:               check.scope.TaskID,
		ActorID:              check.scope.ActorID,
		OwnerID:              check.scope.OwnerID,
	}
	return approval, nil
}

func (r *Runtime) storeModelApprovalContinuation(ctx context.Context, approval ApprovalRequest, scope ExecutionScope, activityID string, continuation approvalContinuation) (ApprovalRequest, error) {
	fillApprovalScope(&approval, scope)
	if strings.TrimSpace(approval.Kind) == "" {
		approval.Kind = PermissionCapabilityModelApproval
	}
	approval.Permission = PermissionCapabilityModelApproval
	if strings.TrimSpace(approval.Reviewer) == "" {
		approval.Reviewer = "host"
	}
	if strings.TrimSpace(approval.Scope) == "" {
		approval.Scope = PermissionGrantScopeRun
	}
	if strings.TrimSpace(approval.Description) == "" {
		approval.Description = "Model approval requested"
	}
	if strings.TrimSpace(approval.Risk) == "" {
		approval.Risk = "medium"
	}
	if len(approval.AvailableDecisions) == 0 {
		approval.AvailableDecisions = permissionDecisionOptions(scope)
	}

	action := ProposedAction{
		ActionID: firstNonEmpty(approval.BoundaryDecisionID, approval.ID),
		Kind:     PermissionCapabilityModelApproval,
		Target:   firstNonEmpty(approval.BoundaryDecisionID, approval.ID, approval.Description),
		Args: map[string]any{
			"approval_id": approval.ID,
			"kind":        approval.Kind,
			"description": approval.Description,
		},
	}
	check := normalizePermissionCheck(PermissionCheckRequest{
		Scope:      scope,
		Action:     action,
		ActivityID: activityID,
		Reason:     approval.Description,
	})
	grant := permissionGrantForDecision(check, PermissionDecisionAllowOnce)
	grant.ApprovalID = approval.ID
	if len(approval.RequestedGrants) == 0 {
		approval.RequestedGrants = []ScopedPermissionGrant{grant}
	}
	stored := permissionRecordScope{
		Scope:        check.scope,
		Action:       check.action,
		Grant:        grant,
		ActivityID:   activityID,
		Reason:       approval.Description,
		Continuation: cloneApprovalContinuation(continuation),
	}
	scopeJSON, err := json.Marshal(stored)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("marshal model approval scope: %w", err)
	}
	if err := r.kernel.store.PutPermission(ctx, persistence.PermissionRecord{
		PermissionID:   approval.ID,
		RunID:          scope.RunID,
		Subject:        scope.ActorID,
		Scope:          string(scopeJSON),
		Granted:        false,
		RequestedAt:    time.Now().UTC().Format(time.RFC3339),
		PolicyWarnings: approval.Description,
	}); err != nil {
		return ApprovalRequest{}, fmt.Errorf("put model approval: %w", err)
	}
	return approval, nil
}

func (r *Runtime) storePermissionApprovalContinuation(ctx context.Context, approvalID string, continuation approvalContinuation) error {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return errors.New("approval_id is required")
	}
	rec, err := r.kernel.store.GetPermission(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("get permission for continuation: %w", err)
	}
	stored, err := decodePermissionRecordScope(rec.Scope)
	if err != nil {
		return err
	}
	stored.Continuation = cloneApprovalContinuation(continuation)
	scopeJSON, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("marshal permission continuation: %w", err)
	}
	rec.Scope = string(scopeJSON)
	if err := r.kernel.store.PutPermission(ctx, rec); err != nil {
		return fmt.Errorf("put permission continuation: %w", err)
	}
	return nil
}

func cloneApprovalContinuation(continuation approvalContinuation) *approvalContinuation {
	if continuation.ToolCall != nil {
		call := *continuation.ToolCall
		call.Arguments = clonePayload(call.Arguments)
		continuation.ToolCall = &call
	}
	if len(continuation.PendingToolCalls) > 0 {
		pending := make([]ToolCall, 0, len(continuation.PendingToolCalls))
		for _, call := range continuation.PendingToolCalls {
			call.Arguments = clonePayload(call.Arguments)
			pending = append(pending, call)
		}
		continuation.PendingToolCalls = pending
	}
	if continuation.PlanArtifact != nil {
		plan := *continuation.PlanArtifact
		continuation.PlanArtifact = &plan
	}
	if continuation.Packet != nil {
		packet := *continuation.Packet
		packet.Payload = clonePayload(packet.Payload)
		packet.Events = append([]RuntimeEventEnvelope(nil), packet.Events...)
		packet.ToolSpecs = append([]ToolSpec(nil), packet.ToolSpecs...)
		continuation.Packet = &packet
	}
	continuation.Payload = clonePayload(continuation.Payload)
	continuation.Messages = append([]ModelMessage(nil), continuation.Messages...)
	continuation.Observations = append([]ToolObservation(nil), continuation.Observations...)
	return &continuation
}

func (r *Runtime) persistPermissionGrant(ctx context.Context, grant ScopedPermissionGrant, grantedBy string) error {
	grant = normalizePermissionGrant(grant)
	if err := r.kernel.store.PutCapability(ctx, persistence.CapabilityRecord{
		CapabilityID: permissionCapabilityID(grant),
		Subject:      grant.Capability,
		Scope:        permissionGrantScopeString(grant),
		ExpiresAt:    grant.ExpiresAt,
		Authority:    "rtagent.permission_center",
		Policy:       grant.Decision,
	}); err != nil {
		return fmt.Errorf("put capability: %w", err)
	}
	if err := r.kernel.store.PutGrant(ctx, persistence.GrantRecord{
		GrantID:      grant.ID,
		CapabilityID: permissionCapabilityID(grant),
		Grantee:      grant.ActorID,
		GrantedBy:    grantedBy,
		GrantedAt:    time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:    grant.ExpiresAt,
	}); err != nil {
		return fmt.Errorf("put grant: %w", err)
	}
	return nil
}

func decodePermissionRecordScope(value string) (permissionRecordScope, error) {
	var out permissionRecordScope
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return permissionRecordScope{}, fmt.Errorf("decode permission scope: %w", err)
	}
	return out, nil
}

func grantExpired(expiresAt string) bool {
	if strings.TrimSpace(expiresAt) == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return !parsed.After(time.Now().UTC())
}
