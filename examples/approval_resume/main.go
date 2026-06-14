package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	rtagent "rtagent/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "rtagent-approval-resume-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	rt, err := rtagent.Open(ctx, rtagent.Config{
		Runtime: rtagent.RuntimeConfig{
			DSN:     filepath.Join(tmpDir, "runtime.db"),
			WorkDir: tmpDir,
		},
		Host: rtagent.HostPorts{
			Model: approvalModel(),
			Tools: []rtagent.ToolProvider{approvalTools()},
		},
	})
	if err != nil {
		return err
	}
	defer rt.Close()

	pending, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     "example-approval-run",
		SessionID: "example-approval-session",
		Input:     "edit a file",
	}, rtagent.Identity{ActorID: "example"})
	if err != nil {
		return err
	}
	if pending.ApprovalRequest == nil {
		return errors.New("expected approval request")
	}
	resumed, err := rt.ResolveApproval(ctx, pending.ApprovalRequest.ID, rtagent.PermissionDecisionAllowForRun)
	if err != nil {
		return err
	}
	fmt.Println(resumed.Output)
	return nil
}

func approvalModel() rtagent.ModelProviderFunc {
	return func(_ context.Context, req rtagent.ModelRequest, _ rtagent.ModelStreamHandler) (rtagent.ModelResponse, error) {
		if len(req.Observations) == 0 {
			return rtagent.ModelResponse{
				ToolCalls: []rtagent.ToolCall{{
					Name:      "edit",
					Arguments: map[string]any{"value": req.Input},
				}},
				StopReason: "tool_calls",
			}, nil
		}
		return rtagent.ModelResponse{
			Output:     "resumed after approval: " + req.Observations[0].ModelVisibleSummary,
			StopReason: rtagent.RuntimeStatusCompleted,
		}, nil
	}
}

func approvalTools() rtagent.ToolProviderAdapter {
	return rtagent.ToolProviderAdapter{
		Specs: func(context.Context, rtagent.ExecutionScope) ([]rtagent.ToolSpec, error) {
			return []rtagent.ToolSpec{{Name: "edit", SideEffectKind: "workspace.write"}}, nil
		},
		Execute: func(_ context.Context, _ rtagent.ExecutionScope, call rtagent.ToolCall) (rtagent.ToolObservation, error) {
			return rtagent.ToolObservation{
				ToolCallID:          call.ID,
				Name:                call.Name,
				Status:              rtagent.RuntimeStatusOK,
				ModelVisibleSummary: "edit accepted",
			}, nil
		},
	}
}
