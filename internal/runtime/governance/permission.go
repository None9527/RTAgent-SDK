package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainGov "rtagent/internal/domain/governance"
	"rtagent/internal/domain/persistence"
)

type LocalPermissionCenter struct {
	leaseMgr domainGov.LeaseManager
	store    persistence.Bundle
}

func NewLocalPermissionCenter(lm domainGov.LeaseManager, store persistence.Bundle) *LocalPermissionCenter {
	return &LocalPermissionCenter{
		leaseMgr: lm,
		store:    store,
	}
}

func (pc *LocalPermissionCenter) EvaluateProposal(ctx context.Context, agentID string, act domainGov.ProposedAction, activeActivityID string) error {
	// 1. Sandbox validation for shell commands
	if act.Kind == "shell.exec" {
		cmd, _ := act.Args["command"].(string)
		if strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "chmod") {
			return errors.New("policy violation: destructive command is blocked by sandbox policy")
		}
	}

	// 2. Capability and workspace path validation for file writes
	if act.Kind == "fs.write" {
		filePath := act.Target
		if !strings.HasPrefix(filePath, "/Users/mac/Desktop/rtagent/") && !strings.HasPrefix(filePath, "/Users/mac/.gemini/antigravity/scratch/ngoagent-design/") {
			return errors.New("capability violation: write path is out of granted workspace scope")
		}

		// 3. Acquire temporary Lease lock
		_, err := pc.leaseMgr.Acquire(ctx, "file://"+filePath, activeActivityID, 60*time.Second)
		if err != nil {
			return fmt.Errorf("lease acquisition failed: %w", err)
		}
	}

	return nil
}
