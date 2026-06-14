package rtagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
)

func (r *Runtime) SubmitRun(ctx context.Context, req SubmitRunRequest, ident Identity) (RuntimeStateProjection, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeStateProjection{}, err
	}
	req = r.normalizeSubmitRunRequest(req)
	ident = normalizeIdentity(ident)
	permission := runtimePermissionMode(req.Mode)
	planning := runtimePlanningState(req.PlanningState)
	prePlanPermission := strings.TrimSpace(req.PrePlanPermissionMode)
	if planning == PlanningPlan && prePlanPermission == "" {
		prePlanPermission = permission
	}
	payload := map[string]any{
		"kind":                     req.Kind,
		"run_id":                   req.RunID,
		"session_id":               req.SessionID,
		"session_alias":            req.SessionAlias,
		"working_dir":              req.WorkingDir,
		"target":                   req.Target,
		"input":                    req.Input,
		"mode":                     permission,
		"requested_mode":           req.Mode,
		"planning_state":           planning,
		"pre_plan_permission_mode": prePlanPermission,
		"profile":                  req.Profile,
		"root_run_id":              req.RootRunID,
		"parent_run_id":            req.ParentRunID,
		"task_id":                  req.TaskID,
		"role":                     req.Role,
		"scope":                    clonePayload(req.Scope),
		"args":                     clonePayload(req.Args),
		"actor_id":                 ident.ActorID,
		"owner_id":                 ident.OwnerID,
	}
	cmd := RuntimeCommand{
		ID:   "cmd-" + req.RunID,
		Kind: req.Kind,
		Scope: ExecutionScope{
			WorkspaceID:    firstNonEmpty(req.WorkingDir, r.workDir),
			SessionID:      req.SessionID,
			RunID:          req.RunID,
			RootRunID:      firstNonEmpty(req.RootRunID, req.RunID),
			ParentRunID:    req.ParentRunID,
			TaskID:         req.TaskID,
			ActorID:        ident.ActorID,
			OwnerID:        ident.OwnerID,
			ActorKind:      "user",
			PermissionMode: permission,
			PlanningState:  planning,
			TraceID:        "trace-" + req.RunID,
		},
		Payload:     payload,
		RequestedBy: ident.ActorID,
		CreatedAt:   time.Now().UTC(),
	}
	return r.Run(ctx, cmd)
}

func (r *Runtime) initializeRun(ctx context.Context, cmd RuntimeCommand) (RuntimeStateProjection, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeStateProjection{}, err
	}
	cmd = r.normalizeCommand(cmd)
	if err := r.ensureSessionCanAcceptRun(ctx, cmd.Scope.SessionID); err != nil {
		return RuntimeStateProjection{}, err
	}
	objective := firstPayloadString(cmd.Payload, "objective", "input", "message")
	rec := persistence.RunRecord{
		RunID:         cmd.Scope.RunID,
		ResumeID:      cmd.Scope.SessionID,
		RootRunID:     cmd.Scope.RootRunID,
		ParentRunID:   cmd.Scope.ParentRunID,
		TaskID:        cmd.Scope.TaskID,
		UserObjective: objective,
		IngressKind:   firstNonEmpty(cmd.Kind, "sdk"),
		Status:        RuntimeStatusRunning,
		CreatedAt:     nowUTC(cmd.CreatedAt).Format(time.RFC3339),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := r.kernel.store.PutRun(ctx, rec); err != nil {
		return RuntimeStateProjection{}, fmt.Errorf("put run: %w", err)
	}
	sessionStarted, err := r.recordSessionRun(ctx, cmd, objective)
	if err != nil {
		return RuntimeStateProjection{}, err
	}
	if sessionStarted {
		if err := r.emitSessionStarted(ctx, cmd); err != nil {
			return RuntimeStateProjection{}, err
		}
	}

	payload := clonePayload(cmd.Payload)
	payload["session_id"] = cmd.Scope.SessionID
	payload["root_run_id"] = cmd.Scope.RootRunID
	payload["parent_run_id"] = cmd.Scope.ParentRunID
	payload["task_id"] = cmd.Scope.TaskID
	payload["objective"] = objective
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:      cmd.Scope.RunID,
		Kind:       EventKindRunCreated,
		OccurredAt: nowUTC(cmd.CreatedAt),
		Message:    "Run initialized via RTAgent SDK",
		Payload:    payload,
	}); err != nil {
		return RuntimeStateProjection{}, err
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:      cmd.Scope.RunID,
		Kind:       EventKindTurnStarted,
		OccurredAt: nowUTC(cmd.CreatedAt),
		Message:    "Turn started via RTAgent SDK",
		Payload: map[string]any{
			"session_id":      cmd.Scope.SessionID,
			"run_id":          cmd.Scope.RunID,
			"root_run_id":     cmd.Scope.RootRunID,
			"parent_run_id":   cmd.Scope.ParentRunID,
			"task_id":         cmd.Scope.TaskID,
			"permission_mode": cmd.Scope.PermissionMode,
			"planning_state":  cmd.Scope.PlanningState,
			"trace_id":        cmd.Scope.TraceID,
		},
	}); err != nil {
		return RuntimeStateProjection{}, err
	}

	return RuntimeStateProjection{
		RunID:      cmd.Scope.RunID,
		SessionID:  cmd.Scope.SessionID,
		Status:     RuntimeStatusRunning,
		Resolution: RuntimeStatusRunning,
	}, nil
}

func (r *Runtime) normalizeCommand(cmd RuntimeCommand) RuntimeCommand {
	if strings.TrimSpace(cmd.Kind) == "" {
		cmd.Kind = "message"
	}
	if strings.TrimSpace(cmd.Scope.RunID) == "" {
		cmd.Scope.RunID = firstNonEmpty(cmd.ID, fmt.Sprintf("run_%d", time.Now().UnixNano()))
	}
	if strings.TrimSpace(cmd.Scope.SessionID) == "" {
		cmd.Scope.SessionID = cmd.Scope.RunID
	}
	if strings.TrimSpace(cmd.Scope.RootRunID) == "" {
		cmd.Scope.RootRunID = cmd.Scope.RunID
	}
	if strings.TrimSpace(cmd.Scope.WorkspaceID) == "" {
		cmd.Scope.WorkspaceID = r.workDir
	}
	if strings.TrimSpace(cmd.Scope.ActorKind) == "" {
		cmd.Scope.ActorKind = "user"
	}
	if strings.TrimSpace(cmd.Scope.PermissionMode) == "" {
		cmd.Scope.PermissionMode = PermissionDefault
	}
	if strings.TrimSpace(cmd.Scope.PlanningState) == "" {
		cmd.Scope.PlanningState = PlanningOff
	}
	if strings.TrimSpace(cmd.Scope.TraceID) == "" {
		cmd.Scope.TraceID = "trace-" + cmd.Scope.RunID
	}
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = time.Now().UTC()
	}
	return cmd
}

func (r *Runtime) normalizeSubmitRunRequest(req SubmitRunRequest) SubmitRunRequest {
	if strings.TrimSpace(req.Kind) == "" {
		req.Kind = "message"
	}
	if strings.TrimSpace(req.RunID) == "" {
		req.RunID = fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = firstNonEmpty(req.SessionAlias, req.RunID)
	}
	if strings.TrimSpace(req.WorkingDir) == "" {
		req.WorkingDir = r.workDir
	}
	if strings.TrimSpace(req.RootRunID) == "" {
		req.RootRunID = req.RunID
	}
	if req.Scope == nil {
		req.Scope = map[string]any{}
	}
	if req.Args == nil {
		req.Args = map[string]any{}
	}
	return req
}

func normalizeIdentity(ident Identity) Identity {
	if strings.TrimSpace(ident.ActorID) == "" {
		ident.ActorID = "local"
	}
	if strings.TrimSpace(ident.OwnerID) == "" {
		ident.OwnerID = ident.ActorID
	}
	return ident
}

func runtimePermissionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case PermissionAcceptEdits:
		return PermissionAcceptEdits
	case PermissionYolo:
		return PermissionYolo
	default:
		return PermissionDefault
	}
}

func runtimePlanningState(state string) string {
	if strings.TrimSpace(state) == PlanningPlan {
		return PlanningPlan
	}
	return PlanningOff
}
