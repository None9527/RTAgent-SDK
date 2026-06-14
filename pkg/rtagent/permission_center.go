package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Runtime) CheckPermission(ctx context.Context, req PermissionCheckRequest) (PermissionCheckResult, error) {
	if err := r.ensureReady(); err != nil {
		return PermissionCheckResult{}, err
	}
	check := normalizePermissionCheck(req)
	if strings.TrimSpace(check.scope.RunID) != "" || !check.readOnly {
		if err := r.ensureRunExists(ctx, check.scope.RunID); err != nil {
			return PermissionCheckResult{}, err
		}
	}
	if err := r.ensurePermissionSessionCanProceed(ctx, check); err != nil {
		return PermissionCheckResult{}, err
	}

	if grant, ok := r.findPermissionGrant(ctx, check); ok {
		if err := r.emitPermissionGranted(ctx, check, grant.Decision, grant); err != nil {
			return PermissionCheckResult{}, err
		}
		return PermissionCheckResult{
			Status:       PermissionStatusAllowed,
			Decision:     grant.Decision,
			PermissionID: permissionIDForCheck(check),
			Grant:        &grant,
			Reason:       "matched existing permission grant",
		}, nil
	}

	switch {
	case check.scope.PermissionMode == PermissionYolo:
		grant := permissionGrantForDecision(check, PermissionDecisionAllowAllForRun)
		if err := r.persistPermissionGrant(ctx, grant, firstNonEmpty(check.scope.ActorID, "local")); err != nil {
			return PermissionCheckResult{}, err
		}
		if err := r.emitPermissionGranted(ctx, check, PermissionDecisionAllowAllForRun, grant); err != nil {
			return PermissionCheckResult{}, err
		}
		return PermissionCheckResult{
			Status:       PermissionStatusAllowed,
			Decision:     PermissionDecisionAllowAllForRun,
			PermissionID: permissionIDForCheck(check),
			Grant:        &grant,
			Reason:       "permission mode yolo allows this action",
		}, nil
	case check.readOnly:
		grant := permissionGrantForDecision(check, PermissionDecisionAllowOnce)
		if err := r.emitPermissionGranted(ctx, check, PermissionDecisionAllowOnce, grant); err != nil {
			return PermissionCheckResult{}, err
		}
		return PermissionCheckResult{
			Status:       PermissionStatusAllowed,
			Decision:     PermissionDecisionAllowOnce,
			PermissionID: permissionIDForCheck(check),
			Grant:        &grant,
			Reason:       "read-only action allowed",
		}, nil
	case check.scope.PermissionMode == PermissionAcceptEdits && check.workspaceWrite:
		grant := permissionGrantForDecision(check, PermissionDecisionAllowForRun)
		if err := r.persistPermissionGrant(ctx, grant, firstNonEmpty(check.scope.ActorID, "local")); err != nil {
			return PermissionCheckResult{}, err
		}
		if err := r.emitPermissionGranted(ctx, check, PermissionDecisionAllowForRun, grant); err != nil {
			return PermissionCheckResult{}, err
		}
		return PermissionCheckResult{
			Status:       PermissionStatusAllowed,
			Decision:     PermissionDecisionAllowForRun,
			PermissionID: permissionIDForCheck(check),
			Grant:        &grant,
			Reason:       "acceptEdits allows workspace write actions for this run",
		}, nil
	case check.dangerousCommand:
		if err := r.emitPermissionDenied(ctx, check, "dangerous shell command denied by default policy"); err != nil {
			return PermissionCheckResult{}, err
		}
		return PermissionCheckResult{
			Status:       PermissionStatusDenied,
			Decision:     PermissionDecisionDeny,
			PermissionID: permissionIDForCheck(check),
			Reason:       "dangerous shell command denied by default policy",
		}, nil
	}

	approval, err := r.createPermissionRequest(ctx, check)
	if err != nil {
		return PermissionCheckResult{}, err
	}
	if err := r.emitPermissionRequested(ctx, check, approval); err != nil {
		return PermissionCheckResult{}, err
	}
	return PermissionCheckResult{
		Status:          PermissionStatusRequiresApproval,
		Decision:        "",
		PermissionID:    approval.ID,
		ApprovalRequest: &approval,
		Reason:          "permission approval required",
	}, nil
}

func (r *Runtime) ensurePermissionSessionCanProceed(ctx context.Context, check permissionCheck) error {
	if check.readOnly || strings.TrimSpace(check.scope.SessionID) == "" {
		return nil
	}
	return r.ensureSessionCanAcceptRun(ctx, check.scope.SessionID)
}

func (r *Runtime) ResolvePermission(ctx context.Context, req PermissionDecisionRequest) (PermissionDecisionResult, error) {
	if err := r.ensureReady(); err != nil {
		return PermissionDecisionResult{}, err
	}
	approvalID := strings.TrimSpace(req.ApprovalID)
	if approvalID == "" {
		return PermissionDecisionResult{}, errors.New("approval_id is required")
	}
	decision := normalizePermissionDecision(req.Decision)
	rec, err := r.kernel.store.GetPermission(ctx, approvalID)
	if err != nil {
		return PermissionDecisionResult{}, fmt.Errorf("get permission: %w", err)
	}
	stored, err := decodePermissionRecordScope(rec.Scope)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	if strings.TrimSpace(stored.Scope.RunID) == "" {
		stored.Scope.RunID = strings.TrimSpace(rec.RunID)
	}
	if req.Scope.RunID != "" {
		stored.Scope = mergePermissionScope(stored.Scope, req.Scope)
	}
	if err := r.ensureRunExists(ctx, stored.Scope.RunID); err != nil {
		return PermissionDecisionResult{}, err
	}
	check := permissionCheckFromRecordScope(stored)
	authorizedBy := firstNonEmpty(req.ActorID, req.Scope.ActorID, stored.Scope.ActorID, "local")
	if decision != PermissionDecisionDeny {
		if err := r.ensureSessionCanAcceptRun(ctx, stored.Scope.SessionID); err != nil {
			return PermissionDecisionResult{}, err
		}
	}

	rec.AuthorizedBy = authorizedBy
	rec.ResolvedAt = time.Now().UTC().Format(time.RFC3339)
	rec.Granted = decision != PermissionDecisionDeny
	if req.Reason != "" {
		if strings.TrimSpace(rec.PolicyWarnings) == "" {
			rec.PolicyWarnings = req.Reason
		} else {
			rec.PolicyWarnings = rec.PolicyWarnings + "; " + req.Reason
		}
	}
	if err := r.kernel.store.PutPermission(ctx, rec); err != nil {
		return PermissionDecisionResult{}, fmt.Errorf("put permission: %w", err)
	}
	if decision == PermissionDecisionDeny {
		if err := r.emitPermissionDenied(ctx, check, "permission denied by reviewer"); err != nil {
			return PermissionDecisionResult{}, err
		}
		return PermissionDecisionResult{
			Status:       PermissionStatusDenied,
			PermissionID: approvalID,
			Reason:       "permission denied by reviewer",
		}, nil
	}

	grant := permissionGrantForDecision(check, decision)
	grant.ApprovalID = approvalID
	if err := r.persistPermissionGrant(ctx, grant, authorizedBy); err != nil {
		return PermissionDecisionResult{}, err
	}
	if err := r.emitPermissionGranted(ctx, check, decision, grant); err != nil {
		return PermissionDecisionResult{}, err
	}
	return PermissionDecisionResult{
		Status:       PermissionStatusAllowed,
		PermissionID: approvalID,
		Grant:        &grant,
		Reason:       "permission grant recorded",
	}, nil
}
