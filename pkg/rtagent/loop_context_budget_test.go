package rtagent

import (
	"context"
	"testing"
)

func TestTrimMessagesToWindowPreservesFirstUserAndTail(t *testing.T) {
	// Build 10 messages: user(0), assistant(1), tool(2), assistant(3), ..., user(9).
	msgs := make([]ModelMessage, 10)
	msgs[0] = ModelMessage{Role: "user", Content: "task"}
	for i := 1; i < 10; i++ {
		msgs[i] = ModelMessage{Role: "assistant", Content: "msg-" + itoa(i)}
	}

	// max=5: keep first user + last 4.
	got := trimMessagesToWindow(msgs, 5)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0].Content != "task" {
		t.Fatalf("got[0] = %q, want first user message 'task'", got[0].Content)
	}
	// Tail should be the last 4: msg-6, msg-7, msg-8, msg-9.
	wantTail := []string{"msg-6", "msg-7", "msg-8", "msg-9"}
	for i, want := range wantTail {
		if got[1+i].Content != want {
			t.Fatalf("got[%d] = %q, want %q", 1+i, got[1+i].Content, want)
		}
	}
}

func TestTrimMessagesToWindowNoOpWhenDisabled(t *testing.T) {
	msgs := []ModelMessage{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "assistant", Content: "c"},
	}
	got := trimMessagesToWindow(msgs, 0)
	if len(got) != 3 {
		t.Fatalf("max=0 should be no-op; len = %d, want 3", len(got))
	}
	got = trimMessagesToWindow(msgs, -1)
	if len(got) != 3 {
		t.Fatalf("max<0 should be no-op; len = %d, want 3", len(got))
	}
}

func TestTrimMessagesToWindowNoOpWhenUnderLimit(t *testing.T) {
	msgs := []ModelMessage{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	// len=2, max=5: under limit, no trim.
	got := trimMessagesToWindow(msgs, 5)
	if len(got) != 2 {
		t.Fatalf("under-limit should be no-op; len = %d, want 2", len(got))
	}
}

func TestTrimMessagesToWindowReturnsFreshAllocation(t *testing.T) {
	msgs := []ModelMessage{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "assistant", Content: "c"},
		{Role: "assistant", Content: "d"},
		{Role: "assistant", Content: "e"},
	}
	got := trimMessagesToWindow(msgs, 3)
	// Mutate the result; original must be unaffected.
	got[0].Content = "mutated"
	if msgs[0].Content == "mutated" {
		t.Fatalf("trimming returned a slice aliasing the input; caller mutation leaked")
	}
}

func TestTrimMessagesToWindowNoUserMessage(t *testing.T) {
	// No user message at all — should fall back to tail-only.
	msgs := []ModelMessage{
		{Role: "assistant", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "assistant", Content: "c"},
		{Role: "assistant", Content: "d"},
		{Role: "assistant", Content: "e"},
	}
	got := trimMessagesToWindow(msgs, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Content != "d" || got[1].Content != "e" {
		t.Fatalf("tail = %q,%q, want d,e", got[0].Content, got[1].Content)
	}
}

func TestRuntimeLoopTrimsMessagesToConfiguredWindow(t *testing.T) {
	// End-to-end: configure a small MaxContextMessages and a model that calls a
	// tool for several iterations. Assert the model never sees more messages
	// than the window allows, and the first user message (task) is always
	// present in the request.
	ctx := context.Background()
	const windowSize = 4
	var observedMessageCounts []int
	var observedHasTask bool

	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Runtime.MaxToolIterations = 10
		cfg.Runtime.MaxContextMessages = windowSize
		cfg.Host.Tools = []ToolProvider{&recordingToolProvider{
			specs: []ToolSpec{{Name: "echo", Description: "echo", ReadOnly: true}},
		}}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			observedMessageCounts = append(observedMessageCounts, len(req.Messages))
			for _, m := range req.Messages {
				if m.Role == "user" && m.Content == "multi-turn task" {
					observedHasTask = true
				}
			}
			if req.Iteration < 5 {
				return ModelResponse{
					ToolCalls:  []ToolCall{{Name: "echo", Arguments: map[string]any{"value": "x"}, ReadOnly: true}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-context-budget",
		SessionID: "session-context-budget",
		Input:     "multi-turn task",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}

	if len(observedMessageCounts) == 0 {
		t.Fatalf("model was never called")
	}
	for i, count := range observedMessageCounts {
		if count > windowSize {
			t.Fatalf("iteration %d saw %d messages, want <= %d (window limit)", i, count, windowSize)
		}
	}
	if !observedHasTask {
		t.Fatalf("first user message 'multi-turn task' was never present in model requests; task context was lost")
	}

	// Confirm a context.compacted event was emitted once trimming kicked in.
	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-context-budget"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	foundCompacted := false
	for _, ev := range events {
		if ev.Kind == EventKindContextCompacted {
			foundCompacted = true
			break
		}
	}
	if !foundCompacted {
		t.Fatalf("no context.compacted event emitted despite window being exceeded")
	}
}

// itoa avoids importing strconv for a single tiny use in test helpers.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
