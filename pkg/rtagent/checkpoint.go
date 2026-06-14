package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
)

const (
	checkpointNodeContextPacket   = "context_packet"
	checkpointNodeModelRequest    = "model_request"
	checkpointNodeModelResponse   = "model_response"
	checkpointNodeToolCall        = "tool_call"
	checkpointNodeToolObservation = "tool_observation"
	checkpointNodeApprovalPending = "approval_pending"
	checkpointNodeTerminal        = "terminal"

	checkpointStatusReady     = "ready"
	checkpointStatusCompleted = "completed"
	checkpointStatusBlocked   = "blocked"
	checkpointStatusTerminal  = "terminal"
)

type loopContinuation struct {
	Scope                ExecutionScope    `json:"scope"`
	Packet               ContextPacket     `json:"packet"`
	Messages             []ModelMessage    `json:"messages,omitempty"`
	Observations         []ToolObservation `json:"observations,omitempty"`
	PendingToolCalls     []ToolCall        `json:"pending_tool_calls,omitempty"`
	PlanArtifact         *PlanArtifact     `json:"plan_artifact,omitempty"`
	Input                string            `json:"input,omitempty"`
	Payload              map[string]any    `json:"payload,omitempty"`
	Iteration            int               `json:"iteration,omitempty"`
	ToolRounds           int               `json:"tool_rounds,omitempty"`
	ToolSchemaSnapshotID string            `json:"tool_schema_snapshot_id,omitempty"`
	ToolSchemaHash       string            `json:"tool_schema_hash,omitempty"`
	ApprovalID           string            `json:"approval_id,omitempty"`
}

func (r *Runtime) appendLoopCheckpoint(ctx context.Context, scope ExecutionScope, node, route, nextNode, status string, state loopContinuation, metadata map[string]any) (string, error) {
	if r == nil || r.kernel == nil || r.kernel.store == nil {
		return "", errors.New("rtagent runtime is not initialized")
	}
	if strings.TrimSpace(scope.RunID) == "" {
		return "", errors.New("checkpoint run_id is required")
	}
	payload := map[string]any{
		"continuation": state,
		"metadata":     clonePayload(metadata),
	}
	stateBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal checkpoint state: %w", err)
	}
	seq, _ := r.maxEventSequence(ctx, scope.RunID)
	checkpointID := "checkpoint:" + scope.RunID + ":" + fmt.Sprintf("%06d", seq+1) + ":" + shortHash(node+"|"+route+"|"+time.Now().UTC().Format(time.RFC3339Nano))
	rec := persistence.CheckpointRecord{
		RunID:        scope.RunID,
		CheckpointID: checkpointID,
		GraphID:      "graph:" + scope.RunID,
		Node:         node,
		Route:        route,
		NextNode:     nextNode,
		Status:       firstNonEmpty(status, checkpointStatusReady),
		StatePayload: stateBytes,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Source:       "rtagent.core_loop",
	}
	if err := r.kernel.store.AppendCheckpoint(ctx, rec); err != nil {
		return "", fmt.Errorf("append checkpoint: %w", err)
	}
	if err := r.updateLatestCheckpoint(ctx, scope, checkpointID); err != nil {
		return "", err
	}
	return checkpointID, nil
}

func (r *Runtime) updateLatestCheckpoint(ctx context.Context, scope ExecutionScope, checkpointID string) error {
	checkpointID = strings.TrimSpace(checkpointID)
	if checkpointID == "" {
		return nil
	}
	if run, err := r.kernel.store.GetRun(ctx, scope.RunID); err == nil {
		run.LastCheckpoint = checkpointID
		run.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := r.kernel.store.PutRun(ctx, run); err != nil {
			return fmt.Errorf("put run latest checkpoint: %w", err)
		}
	}
	if scope.SessionID != "" {
		if thread, err := r.kernel.store.GetThread(ctx, scope.SessionID); err == nil {
			thread.LatestCheckpointID = checkpointID
			thread.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := r.kernel.store.PutThread(ctx, thread); err != nil {
				return fmt.Errorf("put session latest checkpoint: %w", err)
			}
		}
	}
	return nil
}

func decodeCheckpointContinuation(rec persistence.CheckpointRecord) (loopContinuation, map[string]any, error) {
	var payload struct {
		Continuation loopContinuation `json:"continuation"`
		Metadata     map[string]any   `json:"metadata"`
	}
	if err := json.Unmarshal(rec.StatePayload, &payload); err != nil {
		return loopContinuation{}, nil, fmt.Errorf("decode checkpoint state: %w", err)
	}
	return payload.Continuation, clonePayload(payload.Metadata), nil
}

func (r *Runtime) CheckpointGraph(ctx context.Context, query CheckpointGraphQuery) (CheckpointGraphSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return CheckpointGraphSnapshot{}, err
	}
	runID := strings.TrimSpace(query.RunID)
	if runID == "" {
		return CheckpointGraphSnapshot{}, errors.New("run_id is required")
	}
	run, err := r.kernel.store.GetRun(ctx, runID)
	if err != nil {
		return CheckpointGraphSnapshot{}, fmt.Errorf("get run: %w", err)
	}
	resumeDisabledWarnings := checkpointGraphWarnings(checkpointResumeDisabledByRunStatus(run))
	resumeDisabledByRun := len(resumeDisabledWarnings) > 0
	resumeDisabledWarning, resumeDisabledBySession := r.checkpointResumeDisabledBySession(ctx, run.ResumeID)
	resumeDisabledWarnings = append(resumeDisabledWarnings, checkpointGraphWarnings(resumeDisabledWarning)...)
	records, err := r.kernel.store.ListCheckpointsByRunID(ctx, runID)
	if err != nil {
		return CheckpointGraphSnapshot{}, fmt.Errorf("list checkpoints: %w", err)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].CreatedAt < records[j].CreatedAt
	})
	nodes := make([]CheckpointNode, 0, len(records))
	edges := make([]CheckpointEdge, 0, checkpointEdgeCapacity(len(records)))
	for i, rec := range records {
		continuation, metadata, _ := decodeCheckpointContinuation(rec)
		resumeReady := checkpointResumeReady(rec, continuation)
		if resumeReady && (resumeDisabledByRun || resumeDisabledBySession) {
			resumeReady = false
		}
		nodes = append(nodes, CheckpointNode{
			CheckpointID: rec.CheckpointID,
			GraphID:      rec.GraphID,
			Node:         rec.Node,
			Route:        rec.Route,
			NextNode:     rec.NextNode,
			Status:       rec.Status,
			Source:       rec.Source,
			CreatedAt:    rec.CreatedAt,
			ResumeReady:  resumeReady,
			Summary:      checkpointSummary(rec),
			Metadata:     metadata,
		})
		if i > 0 {
			edges = append(edges, CheckpointEdge{
				FromCheckpointID: records[i-1].CheckpointID,
				ToCheckpointID:   rec.CheckpointID,
				Kind:             "sequence",
			})
		}
	}
	return CheckpointGraphSnapshot{
		SchemaVersion:      SchemaCheckpointGraphV1,
		RunID:              runID,
		SessionID:          run.ResumeID,
		LatestCheckpointID: firstNonEmpty(run.LastCheckpoint, latestCheckpointID(records)),
		Nodes:              nodes,
		Edges:              edges,
		RuntimeContract:    "v1",
		Warnings:           resumeDisabledWarnings,
	}, nil
}

func checkpointEdgeCapacity(nodeCount int) int {
	if nodeCount <= 1 {
		return 0
	}
	return nodeCount - 1
}

func checkpointResumeDisabledByRunStatus(run persistence.RunRecord) string {
	if runStatusCanResume(run.Status) {
		return ""
	}
	return fmt.Sprintf("checkpoint resume disabled because run %s is %s", run.RunID, run.Status)
}

func (r *Runtime) checkpointResumeDisabledBySession(ctx context.Context, sessionID string) (string, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", false
	}
	thread, err := r.kernel.store.GetThread(ctx, sessionID)
	if err != nil {
		return "", false
	}
	switch thread.Status {
	case SessionStatusStopping, SessionStatusStopped:
		return fmt.Sprintf("checkpoint resume disabled because session %s is %s", sessionID, thread.Status), true
	default:
		return "", false
	}
}

func checkpointGraphWarnings(warnings ...string) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if strings.TrimSpace(warning) != "" {
			out = append(out, warning)
		}
	}
	return out
}

func checkpointResumeReady(rec persistence.CheckpointRecord, continuation loopContinuation) bool {
	if rec.Node == checkpointNodeApprovalPending || rec.Status == checkpointStatusReady || rec.Status == checkpointStatusBlocked {
		return true
	}
	return len(continuation.PendingToolCalls) > 0 && strings.TrimSpace(continuation.Scope.RunID) != ""
}

func checkpointSummary(rec persistence.CheckpointRecord) string {
	switch rec.Node {
	case checkpointNodeContextPacket:
		return "context packet assembled"
	case checkpointNodeModelRequest:
		return "model request ready"
	case checkpointNodeModelResponse:
		return "model response recorded"
	case checkpointNodeToolCall:
		return "tool call pending or executing"
	case checkpointNodeToolObservation:
		return "tool observation recorded"
	case checkpointNodeApprovalPending:
		return "approval pending"
	case checkpointNodeTerminal:
		return "terminal run state recorded"
	default:
		return rec.Node
	}
}

func latestCheckpointID(records []persistence.CheckpointRecord) string {
	if len(records) == 0 {
		return ""
	}
	return records[len(records)-1].CheckpointID
}
