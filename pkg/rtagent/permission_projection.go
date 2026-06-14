package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

type capabilityAuthorization struct {
	Authorized bool
	Reason     string
	Grant      *ScopedPermissionGrant
	PolicyHash string
	Required   []ScopedPermissionGrant
}

func (r *Runtime) PermissionSnapshot(ctx context.Context, query PermissionSnapshotQuery) (PermissionSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return PermissionSnapshot{}, err
	}
	runID := strings.TrimSpace(query.RunID)
	if runID == "" {
		return PermissionSnapshot{}, errors.New("run_id is required")
	}
	if err := r.ensureRunExists(ctx, runID); err != nil {
		return PermissionSnapshot{}, err
	}
	events, err := r.ListEvents(ctx, EventQuery{RunID: runID})
	if err != nil {
		return PermissionSnapshot{}, err
	}
	return r.permissionSnapshotFromEvents(ctx, runID, events), nil
}

func (r *Runtime) permissionSnapshotFromEvents(ctx context.Context, runID string, events []RuntimeEventEnvelope) PermissionSnapshot {
	scope := worldStateScopeFromEvents(runID, events)
	sourceSeq := lastRuntimeEventSeq(events)
	policyHash := permissionPolicyHash(scope)
	pendingByID := map[string]ApprovalRequest{}
	denied := make([]PermissionDecisionRecord, 0)
	active := map[string]ScopedPermissionGrant{}
	warnings := []string{}

	for _, event := range events {
		permissionID := firstPayloadString(event.Payload, "permission_id", "approval_id")
		switch event.Kind {
		case EventKindPermissionRequested:
			approval, ok := approvalRequestFromEvent(event)
			if ok {
				if approval.ID == "" {
					approval.ID = permissionID
				}
				fillApprovalScope(&approval, scope)
				pendingByID[approval.ID] = approval
			} else if permissionID != "" {
				pendingByID[permissionID] = ApprovalRequest{
					ID:          permissionID,
					Kind:        firstPayloadString(event.Payload, "capability"),
					Description: firstNonEmpty(event.Message, "permission approval required"),
					Permission:  firstPayloadString(event.Payload, "capability"),
					ToolTarget:  firstPayloadString(event.Payload, "tool_target"),
					Risk:        firstPayloadString(event.Payload, "risk"),
					RunID:       scope.RunID,
					SessionID:   scope.SessionID,
					ActorID:     scope.ActorID,
					OwnerID:     scope.OwnerID,
				}
			}
		case EventKindPermissionGranted:
			if permissionID != "" {
				delete(pendingByID, permissionID)
			}
			if grant, ok := grantFromEvent(event); ok && !grantExpired(grant.ExpiresAt) {
				active[grant.ID] = grant
			}
		case EventKindPermissionDenied:
			if permissionID != "" {
				delete(pendingByID, permissionID)
			}
			denied = append(denied, PermissionDecisionRecord{
				ID:            permissionID,
				Kind:          firstPayloadString(event.Payload, "kind", "capability"),
				Capability:    firstPayloadString(event.Payload, "capability"),
				ToolTarget:    firstPayloadString(event.Payload, "tool_target"),
				Resource:      firstPayloadString(event.Payload, "resource"),
				Decision:      PermissionDecisionDeny,
				Reason:        firstPayloadString(event.Payload, "reason", "error"),
				Risk:          firstPayloadString(event.Payload, "risk"),
				SourceEventID: event.EventID,
				SourceSeq:     event.Sequence,
				CreatedAt:     event.OccurredAt,
			})
		}
	}

	if r == nil || r.kernel == nil || r.kernel.store == nil {
		warnings = append(warnings, "permission store unavailable")
	}
	sessionStatus, sessionBlocked := r.permissionSessionLifecycleStatus(ctx, runID, scope.SessionID)
	if sessionBlocked && len(active) > 0 {
		active = map[string]ScopedPermissionGrant{}
		warnings = append(warnings, "active grants inactive because session is "+sessionStatus)
	}
	if sessionBlocked && len(pendingByID) > 0 {
		for id, approval := range pendingByID {
			approval.AvailableDecisions = denyOnlyApprovalDecisionOptions(approval.AvailableDecisions)
			pendingByID[id] = approval
		}
		warnings = append(warnings, "pending approvals limited to deny because session is "+sessionStatus)
	}
	grants := make([]ScopedPermissionGrant, 0, len(active))
	for _, grant := range active {
		grants = append(grants, grant)
	}
	sort.SliceStable(grants, func(i, j int) bool {
		return grants[i].ID < grants[j].ID
	})

	pending := make([]ApprovalRequest, 0, len(pendingByID))
	for _, approval := range pendingByID {
		pending = append(pending, approval)
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return pending[i].ID < pending[j].ID
	})
	sort.SliceStable(denied, func(i, j int) bool {
		if denied[i].SourceSeq != denied[j].SourceSeq {
			return denied[i].SourceSeq < denied[j].SourceSeq
		}
		return denied[i].ID < denied[j].ID
	})

	return PermissionSnapshot{
		SchemaVersion:    SchemaPermissionSnapshotV1,
		SessionID:        scope.SessionID,
		RunID:            runID,
		Mode:             scope.PermissionMode,
		PolicyHash:       policyHash,
		SourceSeq:        sourceSeq,
		ActiveGrants:     grants,
		PendingDecisions: pending,
		DeniedDecisions:  denied,
		ResourceRules:    resourceRulesForScope(scope, sessionStatus),
		Warnings:         warnings,
	}
}

func (r *Runtime) capabilityAuthorizationForToolSpec(ctx context.Context, scope ExecutionScope, spec ToolSpec, permissionMode string) capabilityAuthorization {
	permission := toolSpecPermission(spec)
	check := permissionCheck{
		scope:      scope,
		action:     ProposedAction{Kind: PermissionCapabilityToolCall, Target: spec.Name},
		capability: permission,
		toolTarget: spec.Name,
		resource:   spec.Name,
		readOnly:   spec.ReadOnly,
	}
	check.action.ActionID = "worldstate:" + spec.Name
	check.requestedGrants = normalizeRequestedGrants(requiredGrantsForToolSpec(scope, spec, permission), scope)
	auth := capabilityAuthorization{
		Authorized: false,
		Reason:     "approval required by default permission policy",
		PolicyHash: permissionPolicyHash(scope),
		Required:   append([]ScopedPermissionGrant(nil), check.requestedGrants...),
	}
	if spec.ReadOnly {
		auth.Authorized = true
		auth.Reason = "read-only capability allowed"
		return auth
	}
	if sessionStatus, blocked := r.permissionSessionLifecycleStatus(ctx, scope.RunID, scope.SessionID); blocked {
		auth.Reason = "session " + sessionStatus + " blocks non-read-only capability"
		return auth
	}
	if permissionMode == PermissionYolo || scope.PermissionMode == PermissionYolo {
		auth.Authorized = true
		auth.Reason = "permission mode yolo allows this capability"
		return auth
	}
	if grant, ok := r.findPermissionGrant(ctx, check); ok {
		auth.Authorized = true
		auth.Reason = "matched existing permission grant"
		auth.Grant = &grant
		return auth
	}
	return auth
}

func (r *Runtime) permissionSessionLifecycleStatus(ctx context.Context, runID, fallbackSessionID string) (string, bool) {
	if r == nil || r.kernel == nil || r.kernel.store == nil {
		return "", false
	}
	sessionID := strings.TrimSpace(fallbackSessionID)
	if strings.TrimSpace(runID) != "" {
		if run, err := r.kernel.store.GetRun(ctx, strings.TrimSpace(runID)); err == nil && strings.TrimSpace(run.ResumeID) != "" {
			sessionID = strings.TrimSpace(run.ResumeID)
		}
	}
	if sessionID == "" {
		return "", false
	}
	thread, err := r.kernel.store.GetThread(ctx, sessionID)
	if err != nil {
		return "", false
	}
	status := strings.TrimSpace(thread.Status)
	switch status {
	case SessionStatusStopping, SessionStatusStopped:
		return status, true
	default:
		return status, false
	}
}

func requiredGrantsForToolSpec(scope ExecutionScope, spec ToolSpec, permission string) []ScopedPermissionGrant {
	if len(spec.RequiredGrants) > 0 {
		return append([]ScopedPermissionGrant(nil), spec.RequiredGrants...)
	}
	return []ScopedPermissionGrant{permissionGrantForDecision(permissionCheck{
		scope:      scope,
		action:     ProposedAction{Kind: PermissionCapabilityToolCall, Target: spec.Name},
		capability: permission,
		toolTarget: spec.Name,
		resource:   spec.Name,
	}, PermissionDecisionAllowForRun)}
}

func permissionPolicyHash(scope ExecutionScope) string {
	return "policy:" + shortHash(strings.Join([]string{
		"rtagent.permission.v1",
		scope.PermissionMode,
		scope.RunID,
		scope.SessionID,
		scope.ActorID,
		scope.OwnerID,
	}, "|"))
}

func resourceRulesForScope(scope ExecutionScope, sessionStatus string) []ResourceRuleSnapshot {
	rules := []ResourceRuleSnapshot{
		{Kind: "read_only", Resource: "*", Decision: PermissionStatusAllowed, Reason: "read-only actions are allowed", Source: "rtagent.permission_center"},
		{Kind: "dangerous_shell", Resource: "shell.exec", Decision: PermissionStatusDenied, Reason: "dangerous shell commands are denied by default", Source: "rtagent.permission_center"},
	}
	if sessionStatus == SessionStatusStopping || sessionStatus == SessionStatusStopped {
		rules = append(rules, ResourceRuleSnapshot{Kind: "session_lifecycle", Resource: "non_read_only", Decision: PermissionStatusDenied, Reason: "session " + sessionStatus + " blocks non-read-only permission checks", Source: "rtagent.session_lifecycle"})
	}
	switch scope.PermissionMode {
	case PermissionAcceptEdits:
		rules = append(rules, ResourceRuleSnapshot{Kind: "permission_mode", Resource: PermissionCapabilityWorkspaceWrite, Decision: PermissionStatusAllowed, Reason: "acceptEdits allows workspace write for current run", Source: "execution_scope"})
	case PermissionYolo:
		rules = append(rules, ResourceRuleSnapshot{Kind: "permission_mode", Resource: "*", Decision: PermissionStatusAllowed, Reason: "yolo allows all capabilities for current run", Source: "execution_scope"})
	default:
		rules = append(rules, ResourceRuleSnapshot{Kind: "default", Resource: "*", Decision: PermissionStatusRequiresApproval, Reason: "side-effecting actions require approval", Source: "rtagent.permission_center"})
	}
	return rules
}

func denyOnlyApprovalDecisionOptions(options []ApprovalDecisionOption) []ApprovalDecisionOption {
	filtered := make([]ApprovalDecisionOption, 0, len(options))
	for _, option := range options {
		if option.Decision == PermissionDecisionDeny {
			filtered = append(filtered, option)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return []ApprovalDecisionOption{{
		Decision: PermissionDecisionDeny,
		Label:    "Deny",
		Scope:    PermissionGrantScopeAction,
	}}
}

func approvalRequestFromEvent(event RuntimeEventEnvelope) (ApprovalRequest, bool) {
	raw, ok := event.Payload["approval_request"]
	if !ok {
		return ApprovalRequest{}, false
	}
	var approval ApprovalRequest
	if decodePayloadValue(raw, &approval) != nil {
		return ApprovalRequest{}, false
	}
	return approval, true
}

func grantFromEvent(event RuntimeEventEnvelope) (ScopedPermissionGrant, bool) {
	raw, ok := event.Payload["grant"]
	if !ok {
		return ScopedPermissionGrant{}, false
	}
	var grant ScopedPermissionGrant
	if decodePayloadValue(raw, &grant) != nil {
		return ScopedPermissionGrant{}, false
	}
	grant = normalizePermissionGrant(grant)
	return grant, grant.ID != ""
}

func decodePayloadValue(raw any, out any) error {
	bytes, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, out)
}
