package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	rtagent "rtagent/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	runID := fmt.Sprintf("cmd-run-%d", time.Now().UnixNano())
	sessionID := "cmd-session"
	tmpDir, err := os.MkdirTemp("", "rtagent-cmd-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	rt, err := rtagent.Open(ctx, rtagent.Config{
		Runtime: rtagent.RuntimeConfig{
			DSN:     filepath.Join(tmpDir, "runtime.db"),
			WorkDir: tmpDir,
		},
	})
	if err != nil {
		return fmt.Errorf("open runtime: %w", err)
	}
	defer rt.Close()

	projection, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     runID,
		SessionID: sessionID,
		Input:     "rtagent sdk smoke run",
	}, rtagent.Identity{ActorID: "cmd"})
	if err != nil {
		return fmt.Errorf("submit run: %w", err)
	}

	inspect, err := rt.Inspect(ctx, rtagent.InspectQuery{RunID: runID})
	if err != nil {
		return fmt.Errorf("inspect run: %w", err)
	}
	world, err := rt.WorldState(ctx, rtagent.WorldStateQuery{RunID: runID})
	if err != nil {
		return fmt.Errorf("world state: %w", err)
	}

	fmt.Printf("run=%s session=%s status=%s output=%q events=%d partitions=%d\n",
		projection.RunID,
		projection.SessionID,
		inspect.Status,
		projection.Output,
		len(inspect.Events),
		len(world.Partitions),
	)
	return nil
}
