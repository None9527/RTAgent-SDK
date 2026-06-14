package rtagent

import "strings"

func normalizePermissionCheck(req PermissionCheckRequest) permissionCheck {
	scope := req.Scope
	if strings.TrimSpace(scope.PermissionMode) == "" {
		scope.PermissionMode = PermissionDefault
	}
	if strings.TrimSpace(scope.ActorID) == "" {
		scope.ActorID = "local"
	}
	if strings.TrimSpace(scope.OwnerID) == "" {
		scope.OwnerID = scope.ActorID
	}
	if strings.TrimSpace(scope.RootRunID) == "" {
		scope.RootRunID = scope.RunID
	}

	action := req.Action
	if req.ToolCall != nil {
		if strings.TrimSpace(action.ActionID) == "" {
			action.ActionID = req.ToolCall.ID
		}
		if strings.TrimSpace(action.Kind) == "" {
			action.Kind = PermissionCapabilityToolCall
		}
		if strings.TrimSpace(action.Target) == "" {
			action.Target = req.ToolCall.Name
		}
		if action.Args == nil {
			action.Args = clonePayload(req.ToolCall.Arguments)
		}
	}
	if strings.TrimSpace(action.ActionID) == "" {
		action.ActionID = "action:" + shortHash(action.Kind+"|"+action.Target+"|"+payloadPreview(action.Args, 2048))
	}

	capability := permissionCapabilityForAction(action)
	toolTarget := ""
	resource := strings.TrimSpace(action.Target)
	if capability == PermissionCapabilityToolCall {
		toolTarget = strings.TrimSpace(action.Target)
		resource = toolTarget
	}
	readOnly := isReadOnlyPermissionAction(action)
	workspaceWrite := capability == PermissionCapabilityWorkspaceWrite
	risk := "medium"
	toolSchemaHash := ""
	toolEpoch := ""
	var requestedGrants []ScopedPermissionGrant
	if req.ToolCall != nil {
		readOnly = readOnly || req.ToolCall.ReadOnly
		toolSchemaHash = strings.TrimSpace(req.ToolCall.SchemaHash)
		toolEpoch = strings.TrimSpace(req.ToolCall.Epoch)
	}
	if req.ToolSpec != nil {
		readOnly = readOnly || req.ToolSpec.ReadOnly
		toolSchemaHash = firstNonEmpty(toolSchemaHash, req.ToolSpec.SchemaHash)
		toolEpoch = firstNonEmpty(toolEpoch, req.ToolSpec.Epoch, req.ToolSpec.Version)
		if req.ToolSpec.RiskLevel != "" {
			risk = req.ToolSpec.RiskLevel
		}
		if req.ToolSpec.SideEffectKind != "" {
			sideEffect := strings.TrimSpace(req.ToolSpec.SideEffectKind)
			workspaceWrite = workspaceWrite || sideEffect == "workspace.write" || sideEffect == "fs.write" || sideEffect == "file.write"
			if workspaceWrite {
				capability = PermissionCapabilityWorkspaceWrite
			}
		}
		requestedGrants = append([]ScopedPermissionGrant(nil), req.ToolSpec.RequiredGrants...)
	}
	if capability == PermissionCapabilityShellExec {
		risk = "high"
	}
	if len(requestedGrants) == 0 {
		requestedGrants = []ScopedPermissionGrant{permissionGrantForDecision(permissionCheck{
			scope:      scope,
			action:     action,
			capability: capability,
			toolTarget: toolTarget,
			resource:   resource,
		}, PermissionDecisionAllowForRun)}
	}

	return permissionCheck{
		scope:                scope,
		action:               action,
		capability:           capability,
		toolTarget:           toolTarget,
		resource:             resource,
		activityID:           strings.TrimSpace(req.ActivityID),
		reason:               strings.TrimSpace(req.Reason),
		readOnly:             readOnly,
		workspaceWrite:       workspaceWrite,
		dangerousCommand:     capability == PermissionCapabilityShellExec && dangerousShellCommand(action),
		risk:                 risk,
		argumentsPreview:     payloadPreview(action.Args, 1000),
		toolSchemaSnapshotID: strings.TrimSpace(req.ToolSchemaSnapshotID),
		toolSchemaHash:       toolSchemaHash,
		toolEpoch:            toolEpoch,
		requestedGrants:      normalizeRequestedGrants(requestedGrants, scope),
	}
}

func permissionCapabilityForAction(action ProposedAction) string {
	kind := strings.TrimSpace(action.Kind)
	switch kind {
	case PermissionCapabilityToolCall, "tool", "tool_call":
		return PermissionCapabilityToolCall
	case PermissionCapabilityWorkspaceWrite, "fs.write", "file.write", "workspace.file.write":
		return PermissionCapabilityWorkspaceWrite
	case PermissionCapabilityShellExec, "command.exec", "exec", "shell":
		return PermissionCapabilityShellExec
	case PermissionCapabilityMCPCall, "mcp", "mcp.tool":
		return PermissionCapabilityMCPCall
	default:
		if strings.HasPrefix(kind, "tool.") {
			return PermissionCapabilityToolCall
		}
		if strings.Contains(kind, "write") {
			return PermissionCapabilityWorkspaceWrite
		}
		return kind
	}
}

func isReadOnlyPermissionAction(action ProposedAction) bool {
	kind := strings.TrimSpace(action.Kind)
	return kind == "fs.read" ||
		kind == "file.read" ||
		kind == "workspace.read" ||
		kind == "context.read" ||
		kind == "memory.read"
}

func dangerousShellCommand(action ProposedAction) bool {
	text := strings.ToLower(strings.TrimSpace(action.Target + " " + payloadPreview(action.Args, 2048)))
	dangerous := []string{
		"rm -rf /",
		"rm -rf ~",
		"rm -rf *",
		"chmod -r 777 /",
		"mkfs",
		":(){:|:&};:",
	}
	for _, pattern := range dangerous {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}
