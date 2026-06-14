package rtagent

import (
	"context"
	"strings"
	"testing"
)

func TestConvergenceRepeatDetectionTriggersReplan(t *testing.T) {
	c := newConvergenceController()
	call := ToolCall{Name: "echo", Arguments: map[string]any{"value": "x"}}
	obs := ToolObservation{ToolCallID: "tc-1", Status: "ok", ModelVisibleSummary: "echo: x"}

	// First two repeats: no replan yet (threshold = 3).
	d1 := c.observe(1, 100, []ToolCall{call}, []ToolObservation{obs})
	if d1.isActive() {
		t.Fatalf("iteration 1: decision = %+v, want inactive (below threshold)", d1)
	}
	d2 := c.observe(2, 100, []ToolCall{call}, []ToolObservation{obs})
	if d2.isActive() {
		t.Fatalf("iteration 2: decision = %+v, want inactive (below threshold)", d2)
	}
	// Third repeat: replan triggers.
	d3 := c.observe(3, 100, []ToolCall{call}, []ToolObservation{obs})
	if !d3.ShouldReplan {
		t.Fatalf("iteration 3: decision = %+v, want ShouldReplan", d3)
	}
	if d3.Reason != "repeated_tool_interaction" {
		t.Fatalf("Reason = %q, want repeated_tool_interaction", d3.Reason)
	}
}

func TestConvergenceRepeatWithDifferentResultsIsNovel(t *testing.T) {
	c := newConvergenceController()
	call := ToolCall{Name: "echo", Arguments: map[string]any{"value": "x"}}

	// Same call but different observation summaries — should be novel, no repeat.
	for i := 1; i <= 5; i++ {
		obs := ToolObservation{ToolCallID: "tc", Status: "ok", ModelVisibleSummary: "result-" + intToStr(i)}
		d := c.observe(i, 100, []ToolCall{call}, []ToolObservation{obs})
		if d.ShouldReplan && d.Reason == "repeated_tool_interaction" {
			t.Fatalf("iteration %d: wrongly flagged as repeated (observations differ)", i)
		}
	}
}

func TestConvergenceReplanDedup(t *testing.T) {
	c := newConvergenceController()
	call := ToolCall{Name: "echo", Arguments: map[string]any{"value": "x"}}
	obs := ToolObservation{ToolCallID: "tc", Status: "ok", ModelVisibleSummary: "same"}

	// Accumulate repeats across iterations 1-3.
	for i := 1; i <= 2; i++ {
		d := c.observe(i, 100, []ToolCall{call}, []ToolObservation{obs})
		if d.isActive() {
			t.Fatalf("iteration %d: want inactive (below threshold), got %+v", i, d)
		}
	}
	// Third repeat (iteration 3): replan triggers.
	d3 := c.observe(3, 100, []ToolCall{call}, []ToolObservation{obs})
	if !d3.ShouldReplan {
		t.Fatalf("iteration 3: want replan, got %+v", d3)
	}
	// Same reason on iteration 4 should be deduped (empty decision).
	d4 := c.observe(4, 100, []ToolCall{call}, []ToolObservation{obs})
	if d4.isActive() {
		t.Fatalf("iteration 4: replan should be deduped, got %+v", d4)
	}
}

func TestConvergenceHardBudgetPreflush(t *testing.T) {
	c := newConvergenceController()
	novelCall := ToolCall{Name: "probe", Arguments: map[string]any{"i": 1}}

	// Simulate approaching the hard limit with novel calls (no repeat/no-progress).
	limit := 5
	for i := 1; i < limit-1; i++ {
		call := ToolCall{Name: "probe", Arguments: map[string]any{"i": i}}
		obs := ToolObservation{ToolCallID: "tc", Status: "ok", ModelVisibleSummary: "novel-" + intToStr(i)}
		d := c.observe(i, limit, []ToolCall{call}, []ToolObservation{obs})
		if d.isActive() {
			t.Fatalf("iteration %d: want inactive (novel calls), got %+v", i, d)
		}
	}
	// At iteration limit-1 the pre-flush finalize must fire.
	d := c.observe(limit-1, limit, []ToolCall{novelCall}, []ToolObservation{{ToolCallID: "tc", Status: "ok", ModelVisibleSummary: "novel"}})
	if !d.ShouldFinalize {
		t.Fatalf("iteration %d: want hard_budget_preflush finalize, got %+v", limit-1, d)
	}
	if d.Reason != "hard_budget_preflush" {
		t.Fatalf("Reason = %q, want hard_budget_preflush", d.Reason)
	}
}

func TestConvergenceNoProgressAfterFloor(t *testing.T) {
	c := newConvergenceController()
	// Produce novel observations up to the floor.
	for i := 1; i <= softNoProgressIterationFloor; i++ {
		call := ToolCall{Name: "probe", Arguments: map[string]any{"i": i}}
		obs := ToolObservation{ToolCallID: "tc", Status: "ok", ModelVisibleSummary: "novel-" + intToStr(i)}
		d := c.observe(i, 100, []ToolCall{call}, []ToolObservation{obs})
		if d.isActive() {
			t.Fatalf("iteration %d (novel): want inactive, got %+v", i, d)
		}
	}
	// Now stall: turns with no calls at all (model gave empty/no-tool responses
	// but the loop kept going). These accumulate the no-progress streak without
	// triggering repeat detection (repeat requires calls).
	fired := false
	for i := softNoProgressIterationFloor + 1; i <= softNoProgressIterationFloor+noProgressFinalizationThreshold; i++ {
		d := c.observe(i, 100, nil, nil)
		if d.ShouldReplan && d.Reason == "no_new_observation" {
			fired = true
		}
	}
	if !fired {
		t.Fatalf("no_new_observation replan never fired after %d no-call turns past floor %d", noProgressFinalizationThreshold, softNoProgressIterationFloor)
	}
}

func TestConvergenceFinalizationMessageStripsTools(t *testing.T) {
	// convergenceToolSpecs(nil) in finalization mode.
	specs := []ToolSpec{{Name: "echo"}, {Name: "read"}}
	got := convergenceToolSpecs(true, specs)
	if got != nil {
		t.Fatalf("finalization mode: tool specs = %v, want nil", got)
	}
	got = convergenceToolSpecs(false, specs)
	if len(got) != 2 {
		t.Fatalf("normal mode: len(specs) = %d, want 2", len(got))
	}
}

func TestRuntimeLoopConvergenceFinalizesOnRepeatedToolCalls(t *testing.T) {
	// End-to-end: a model that repeats the same tool call should be finalized
	// by the convergence controller rather than failing with a hard limit error.
	// The repeat replan is advisory (the model may ignore it), but the
	// hard-budget pre-flush finalize is the guaranteed backstop. Either way the
	// run must COMPLETE (not fail), and the final model request must have tools
	// stripped.
	ctx := context.Background()
	var modelRequests []ModelRequest
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo", ReadOnly: true}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Runtime.MaxToolIterations = 10
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelRequests = append(modelRequests, req)
			// In finalization mode tools are stripped — produce a final answer.
			if len(req.ToolSpecs) == 0 {
				return ModelResponse{Output: "final answer after convergence", StopReason: RuntimeStatusCompleted}, nil
			}
			// Otherwise keep repeating the same tool call (ignoring any replan
			// steering message, to exercise the hard-budget backstop).
			return ModelResponse{
				ToolCalls:  []ToolCall{{Name: "echo", Arguments: map[string]any{"value": "repeat"}, ReadOnly: true}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-convergence-repeat",
		SessionID: "session-convergence-repeat",
		Input:     "test convergence repeat detection",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	// The run must COMPLETE via convergence finalization, NOT fail with a hard
	// iteration limit error. This is the core convergence contract: the loop
	// always finds a graceful exit, never a hard failure.
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q (convergence must finalize, not fail at hard limit)", projection.Status, RuntimeStatusCompleted)
	}
	if !strings.Contains(projection.Output, "final answer") {
		t.Fatalf("Output = %q, want final answer from convergence finalization", projection.Output)
	}
	// The final model request must have had tools stripped (finalization mode).
	if len(modelRequests) == 0 {
		t.Fatalf("model was never called")
	}
	lastReq := modelRequests[len(modelRequests)-1]
	if len(lastReq.ToolSpecs) != 0 {
		t.Fatalf("final model request had %d tool specs, want 0 (finalization strips tools)", len(lastReq.ToolSpecs))
	}
	// Convergence must have fired before or at the hard limit, not exceeded it.
	// MaxToolIterations=10; the pre-flush fires at iteration 9, finalization
	// turn is iteration 10, so at most ~10 model calls.
	if len(modelRequests) > 11 {
		t.Fatalf("model called %d times — convergence did not finalize before/at hard limit", len(modelRequests))
	}
}

func TestRuntimeLoopConvergenceEmitsHeartbeatEvent(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Runtime.MaxToolIterations = 50
		cfg.Host.Tools = []ToolProvider{&recordingToolProvider{
			specs: []ToolSpec{{Name: "echo", Description: "echo", ReadOnly: true}},
		}}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.ToolSpecs) == 0 {
				return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
			}
			return ModelResponse{
				ToolCalls:  []ToolCall{{Name: "echo", Arguments: map[string]any{"value": "repeat"}, ReadOnly: true}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	_, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-convergence-event",
		SessionID: "session-convergence-event",
		Input:     "test convergence event emission",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-convergence-event"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	foundConvergence := false
	for _, ev := range events {
		if ev.Kind == EventKindRunHeartbeat {
			if kind := firstPayloadString(ev.Payload, "kind"); strings.HasPrefix(kind, "runtime.convergence_") {
				foundConvergence = true
				break
			}
		}
	}
	if !foundConvergence {
		t.Fatalf("no convergence heartbeat event emitted in %v", eventKinds(events))
	}
}

// intToStr is a tiny test helper avoiding strconv import.
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
