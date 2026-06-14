package rtagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func permissionGrantForDecision(check permissionCheck, decision string) ScopedPermissionGrant {
	grant := ScopedPermissionGrant{
		Capability: check.capability,
		ToolTarget: check.toolTarget,
		Resource:   check.resource,
		Scope:      PermissionGrantScopeAction,
		ActionID:   check.action.ActionID,
		RunID:      check.scope.RunID,
		SessionID:  check.scope.SessionID,
		RootRunID:  check.scope.RootRunID,
		TaskID:     check.scope.TaskID,
		ActorID:    check.scope.ActorID,
		OwnerID:    check.scope.OwnerID,
		Decision:   decision,
	}
	switch decision {
	case PermissionDecisionAllowForRun:
		grant.Scope = PermissionGrantScopeRun
		grant.ActionID = ""
	case PermissionDecisionAllowForSession:
		grant.Scope = PermissionGrantScopeSession
		grant.ActionID = ""
		grant.RunID = ""
		grant.RootRunID = ""
		grant.TaskID = ""
	case PermissionDecisionAllowAllForRun:
		grant.Scope = PermissionGrantScopeRun
		grant.ActionID = ""
		grant.Capability = PermissionCapabilityAny
		grant.ToolTarget = PermissionCapabilityAny
		grant.Resource = PermissionCapabilityAny
	case PermissionDecisionAllowAllForSession:
		grant.Scope = PermissionGrantScopeSession
		grant.ActionID = ""
		grant.RunID = ""
		grant.RootRunID = ""
		grant.TaskID = ""
		grant.Capability = PermissionCapabilityAny
		grant.ToolTarget = PermissionCapabilityAny
		grant.Resource = PermissionCapabilityAny
	case PermissionDecisionAllowOnce:
		grant.Scope = PermissionGrantScopeAction
	default:
		grant.Decision = PermissionDecisionAllowOnce
	}
	return normalizePermissionGrant(grant)
}

func normalizePermissionGrant(grant ScopedPermissionGrant) ScopedPermissionGrant {
	if strings.TrimSpace(grant.Capability) == "" {
		grant.Capability = PermissionCapabilityAny
	}
	if strings.TrimSpace(grant.Scope) == "" {
		grant.Scope = PermissionGrantScopeAction
	}
	if strings.TrimSpace(grant.Decision) == "" {
		switch grant.Scope {
		case PermissionGrantScopeRun:
			if grant.Capability == PermissionCapabilityAny && grant.Resource == PermissionCapabilityAny {
				grant.Decision = PermissionDecisionAllowAllForRun
			} else {
				grant.Decision = PermissionDecisionAllowForRun
			}
		case PermissionGrantScopeSession:
			if grant.Capability == PermissionCapabilityAny && grant.Resource == PermissionCapabilityAny {
				grant.Decision = PermissionDecisionAllowAllForSession
			} else {
				grant.Decision = PermissionDecisionAllowForSession
			}
		default:
			grant.Decision = PermissionDecisionAllowOnce
		}
	}
	if strings.TrimSpace(grant.ID) == "" {
		if grant.Scope == PermissionGrantScopeSession {
			grant.ActionID = ""
			grant.RunID = ""
			grant.RootRunID = ""
			grant.TaskID = ""
		}
		grant.ID = permissionGrantID(grant)
	}
	return grant
}

func normalizeRequestedGrants(grants []ScopedPermissionGrant, scope ExecutionScope) []ScopedPermissionGrant {
	out := make([]ScopedPermissionGrant, 0, len(grants))
	for _, grant := range grants {
		if grant.ActorID == "" {
			grant.ActorID = scope.ActorID
		}
		if grant.OwnerID == "" {
			grant.OwnerID = scope.OwnerID
		}
		if grant.RunID == "" {
			grant.RunID = scope.RunID
		}
		if grant.SessionID == "" {
			grant.SessionID = scope.SessionID
		}
		if grant.RootRunID == "" {
			grant.RootRunID = scope.RootRunID
		}
		out = append(out, normalizePermissionGrant(grant))
	}
	return out
}

func permissionDecisionOptions(scope ExecutionScope) []ApprovalDecisionOption {
	options := []ApprovalDecisionOption{
		{Decision: PermissionDecisionDeny, Label: "Deny", Scope: PermissionGrantScopeAction},
		{Decision: PermissionDecisionAllowOnce, Label: "Allow once", Scope: PermissionGrantScopeAction},
		{Decision: PermissionDecisionAllowForRun, Label: "Allow for current run", Scope: PermissionGrantScopeRun},
	}
	if strings.TrimSpace(scope.SessionID) != "" {
		options = append(options, ApprovalDecisionOption{
			Decision: PermissionDecisionAllowForSession,
			Label:    "Allow for current session",
			Scope:    PermissionGrantScopeSession,
		})
	}
	options = append(options, ApprovalDecisionOption{
		Decision: PermissionDecisionAllowAllForRun,
		Label:    "Allow all for current run",
		Scope:    PermissionGrantScopeRun,
	})
	if strings.TrimSpace(scope.SessionID) != "" {
		options = append(options, ApprovalDecisionOption{
			Decision: PermissionDecisionAllowAllForSession,
			Label:    "Allow all for current session",
			Scope:    PermissionGrantScopeSession,
		})
	}
	return options
}

func normalizePermissionDecision(decision string) string {
	switch strings.TrimSpace(decision) {
	case PermissionDecisionAllowOnce:
		return PermissionDecisionAllowOnce
	case PermissionDecisionAllowForRun:
		return PermissionDecisionAllowForRun
	case PermissionDecisionAllowForSession:
		return PermissionDecisionAllowForSession
	case PermissionDecisionAllowAllForRun:
		return PermissionDecisionAllowAllForRun
	case PermissionDecisionAllowAllForSession:
		return PermissionDecisionAllowAllForSession
	default:
		return PermissionDecisionDeny
	}
}

func permissionIDForCheck(check permissionCheck) string {
	return "approval:" + shortHash(strings.Join([]string{
		check.scope.RunID,
		check.scope.SessionID,
		check.action.ActionID,
		check.capability,
		check.toolTarget,
		check.resource,
		check.scope.ActorID,
	}, "|"))
}

func permissionGrantID(grant ScopedPermissionGrant) string {
	return "grant:" + shortHash(strings.Join([]string{
		grant.Scope,
		grant.Decision,
		grant.Capability,
		grant.ToolTarget,
		grant.Resource,
		grant.ActionID,
		grant.RunID,
		grant.SessionID,
		grant.RootRunID,
		grant.TaskID,
		grant.ActorID,
		grant.OwnerID,
	}, "|"))
}

func permissionCapabilityID(grant ScopedPermissionGrant) string {
	return "capability:" + shortHash(strings.Join([]string{
		grant.Capability,
		grant.ToolTarget,
		grant.Resource,
		grant.Scope,
		grant.RunID,
		grant.SessionID,
		grant.ActorID,
	}, "|"))
}

func permissionGrantScopeString(grant ScopedPermissionGrant) string {
	payload, err := json.Marshal(grant)
	if err != nil {
		return grant.Scope
	}
	return string(payload)
}

func permissionDescription(check permissionCheck) string {
	switch check.capability {
	case PermissionCapabilityToolCall:
		return "Tool call requires approval: " + check.toolTarget
	case PermissionCapabilityWorkspaceWrite:
		return "Workspace write requires approval: " + check.resource
	case PermissionCapabilityShellExec:
		return "Shell command requires approval: " + check.resource
	case PermissionCapabilityMCPCall:
		return "MCP call requires approval: " + check.resource
	default:
		return "Action requires approval: " + firstNonEmpty(check.resource, check.capability)
	}
}

func permissionCheckFromRecordScope(stored permissionRecordScope) permissionCheck {
	req := PermissionCheckRequest{
		Scope:      stored.Scope,
		Action:     stored.Action,
		ActivityID: stored.ActivityID,
		Reason:     stored.Reason,
	}
	check := normalizePermissionCheck(req)
	if stored.Grant.Capability != "" {
		check.capability = stored.Grant.Capability
		check.toolTarget = stored.Grant.ToolTarget
		check.resource = stored.Grant.Resource
	}
	return check
}

func mergePermissionScope(base, override ExecutionScope) ExecutionScope {
	if override.WorkspaceID != "" {
		base.WorkspaceID = override.WorkspaceID
	}
	if override.SessionID != "" {
		base.SessionID = override.SessionID
	}
	if override.TurnID != "" {
		base.TurnID = override.TurnID
	}
	if override.RunID != "" {
		base.RunID = override.RunID
	}
	if override.RootRunID != "" {
		base.RootRunID = override.RootRunID
	}
	if override.ParentRunID != "" {
		base.ParentRunID = override.ParentRunID
	}
	if override.TaskID != "" {
		base.TaskID = override.TaskID
	}
	if override.ActorID != "" {
		base.ActorID = override.ActorID
	}
	if override.OwnerID != "" {
		base.OwnerID = override.OwnerID
	}
	if override.ActorKind != "" {
		base.ActorKind = override.ActorKind
	}
	if override.PermissionMode != "" {
		base.PermissionMode = override.PermissionMode
	}
	if override.PlanningState != "" {
		base.PlanningState = override.PlanningState
	}
	if override.TraceID != "" {
		base.TraceID = override.TraceID
	}
	return base
}

func payloadPreview(payload map[string]any, max int) string {
	if len(payload) == 0 {
		return ""
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprint(payload)
	}
	text := string(bytes)
	if max > 0 && len(text) > max {
		return text[:max]
	}
	return text
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:20]
}
