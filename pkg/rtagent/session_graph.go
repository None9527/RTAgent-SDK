package rtagent

import (
	"context"
	"errors"
	"strings"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func (r *Runtime) SessionGraph(ctx context.Context, query SessionGraphQuery) (SessionGraphSnapshot, error) {
	if err := r.ensureReady(); err != nil {
		return SessionGraphSnapshot{}, err
	}
	sessionID := strings.TrimSpace(query.SessionID)
	if sessionID == "" {
		return SessionGraphSnapshot{}, errors.New("session_id is required")
	}
	thread, runs, err := r.loadSessionAndRuns(ctx, sessionID)
	if err != nil {
		return SessionGraphSnapshot{}, err
	}
	rootFilter := strings.TrimSpace(query.RootRunID)
	nodes := make([]SessionRunNode, 0, len(runs))
	nodeByRunID := map[string]bool{}
	activeRunIDs := []string{}
	for _, run := range runs {
		rootRunID := firstNonEmpty(run.RootRunID, run.RunID)
		if rootFilter != "" && rootRunID != rootFilter && run.RunID != rootFilter {
			continue
		}
		node := sessionRunNodeFromRecord(run)
		nodes = append(nodes, node)
		nodeByRunID[node.RunID] = true
		if isActiveRunStatus(node.Status) {
			activeRunIDs = append(activeRunIDs, node.RunID)
		}
	}

	edges := []SessionRunEdge{}
	for _, node := range nodes {
		switch {
		case node.ParentRunID != "" && nodeByRunID[node.ParentRunID]:
			edges = append(edges, SessionRunEdge{
				FromRunID: node.ParentRunID,
				ToRunID:   node.RunID,
				Kind:      "parent",
			})
		case node.RootRunID != "" && node.RootRunID != node.RunID && nodeByRunID[node.RootRunID]:
			edges = append(edges, SessionRunEdge{
				FromRunID: node.RootRunID,
				ToRunID:   node.RunID,
				Kind:      "root",
			})
		}
	}

	return SessionGraphSnapshot{
		SchemaVersion:   SchemaSessionGraphV1,
		SessionID:       sessionID,
		Status:          firstNonEmpty(thread.Status, SessionStatusActive),
		LatestRunID:     thread.LatestRunID,
		RootRunID:       rootFilter,
		Nodes:           nodes,
		Edges:           edges,
		ActiveRunIDs:    activeRunIDs,
		RuntimeContract: "v1",
	}, nil
}

func sessionRunNodeFromRecord(run persistence.RunRecord) SessionRunNode {
	return SessionRunNode{
		RunID:          run.RunID,
		SessionID:      run.ResumeID,
		RootRunID:      firstNonEmpty(run.RootRunID, run.RunID),
		ParentRunID:    run.ParentRunID,
		TaskID:         run.TaskID,
		Status:         firstNonEmpty(run.Status, RuntimeStatusOK),
		Resolution:     run.Resolution,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
		CompletedAt:    run.CompletedAt,
		LastCheckpoint: run.LastCheckpoint,
	}
}
