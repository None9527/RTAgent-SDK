package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	rtagent "github.com/None9527/RTAgent/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "rtagent-minimal-runtime-*")
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
		return err
	}
	defer rt.Close()

	projection, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     "example-minimal-run",
		SessionID: "example-minimal-session",
		Input:     "hello runtime sdk",
	}, rtagent.Identity{ActorID: "example"})
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", projection.Status, projection.Output)
	return nil
}
