package rtagent

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestRuntimeConcurrentEmitAndSubmitRunAreRaceFree(t *testing.T) {
	// Regression coverage for the v1 concurrency contract documented in
	// docs/api/public-compatibility.md: Emit sequence allocation is serialized
	// within a single Runtime, and concurrent facade use must not race. Run
	// under `go test -race` (via scripts/validate_sdk.sh) to catch data races.
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{Output: "ok", StopReason: "stop"}, nil
		})
	})

	const goroutines = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*2)

	// Half the goroutines hammer Emit on a shared run (exercises eventMu).
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope:   ExecutionScope{SessionID: "session-race", RunID: "run-race"},
		Payload: map[string]any{"objective": "race coverage"},
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 8; j++ {
				if _, err := rt.Emit(ctx, RuntimeEventDraft{
					RunID:   "run-race",
					Kind:    EventKindRunHeartbeat,
					Message: "concurrent heartbeat",
				}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}

	// The other half submit distinct runs concurrently (exercises the full
	// loop path alongside Emit on the same Runtime instance).
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		n := i
		go func() {
			defer wg.Done()
			<-start
			_, err := rt.SubmitRun(ctx, SubmitRunRequest{
				Kind:      "message",
				RunID:     fmt.Sprintf("run-race-submit-%d", n),
				SessionID: "session-race-submit",
				Mode:      PermissionAcceptEdits,
				Input:     "concurrent submit",
			}, Identity{ActorID: "tester", OwnerID: "owner"})
			if err != nil {
				errs <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent facade use failed: %v", err)
		}
	}

	// Emit sequences on the shared run must be contiguous starting at 1.
	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-race"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	wantCount := (goroutines/2)*8 + 3 // heartbeats plus the 3 initializeRun events
	if len(events) != wantCount {
		t.Fatalf("len(events) = %d, want %d", len(events), wantCount)
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("events[%d].Sequence = %d, want %d (non-contiguous allocation)", i, event.Sequence, i+1)
		}
	}
}
