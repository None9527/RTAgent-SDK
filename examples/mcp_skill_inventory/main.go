package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	rtagent "github.com/None9527/RTAgent-SDK/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "rtagent-mcp-skill-inventory-*")
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
			MCP:   exampleMCPInventory(),
			Skill: exampleSkillInventory(),
		},
	})
	if err != nil {
		return err
	}
	defer rt.Close()

	if _, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     "example-inventory-run",
		SessionID: "example-inventory-session",
		Input:     "project inventory",
	}, rtagent.Identity{ActorID: "example"}); err != nil {
		return err
	}
	world, err := rt.WorldState(ctx, rtagent.WorldStateQuery{
		RunID:     "example-inventory-run",
		Partition: rtagent.WorldStatePartitionCapability,
	})
	if err != nil {
		return err
	}
	for _, handle := range world.Handles {
		fmt.Println(handle.Handle)
	}
	return nil
}

func exampleMCPInventory() rtagent.MCPProviderFunc {
	return func(context.Context, rtagent.ExecutionScope) ([]rtagent.CapabilityInventoryItem, error) {
		return []rtagent.CapabilityInventoryItem{{
			ID:         "filesystem",
			Kind:       "mcp_server",
			Summary:    "filesystem MCP inventory",
			Visible:    true,
			Available:  true,
			ReadOnly:   true,
			Permission: rtagent.PermissionCapabilityMCPCall,
		}}, nil
	}
}

func exampleSkillInventory() rtagent.SkillProviderFunc {
	return func(context.Context, rtagent.ExecutionScope) ([]rtagent.CapabilityInventoryItem, error) {
		return []rtagent.CapabilityInventoryItem{{
			ID:         "planner",
			Kind:       "skill",
			Summary:    "planner skill inventory",
			Visible:    true,
			Available:  true,
			Permission: rtagent.PermissionCapabilityToolCall,
		}}, nil
	}
}
