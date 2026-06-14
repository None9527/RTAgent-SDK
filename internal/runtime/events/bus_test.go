package events

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInProcessEventBusCloseIsIdempotent(t *testing.T) {
	bus := NewInProcessEventBus(1)

	bus.Close()
	bus.Close()

	err := bus.Publish(context.Background(), Event{Kind: KindRunCreated})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Publish() error = %v, want closed error", err)
	}
}

func TestInProcessEventBusSubscribeUnsubscribe(t *testing.T) {
	bus := NewInProcessEventBus(4)
	defer bus.Close()

	first := make(chan Event, 2)
	second := make(chan Event, 2)

	firstID := bus.Subscribe(KindRunCreated, func(_ context.Context, ev Event) {
		first <- ev
	})
	secondID := bus.Subscribe(KindRunCreated, func(_ context.Context, ev Event) {
		second <- ev
	})
	if firstID == 0 || secondID == 0 || firstID == secondID {
		t.Fatalf("subscription ids = %d, %d; want distinct non-zero ids", firstID, secondID)
	}

	if err := bus.Publish(context.Background(), Event{Kind: KindRunCreated, RunID: "run-one"}); err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	assertReceivedEvent(t, first, "run-one")
	assertReceivedEvent(t, second, "run-one")

	bus.Unsubscribe(KindRunCreated, firstID)
	if err := bus.Publish(context.Background(), Event{Kind: KindRunCreated, RunID: "run-two"}); err != nil {
		t.Fatalf("Publish(second) error = %v", err)
	}
	assertNoEvent(t, first)
	assertReceivedEvent(t, second, "run-two")
}

func assertReceivedEvent(t *testing.T, events <-chan Event, runID string) {
	t.Helper()
	select {
	case event := <-events:
		if event.RunID != runID {
			t.Fatalf("RunID = %q, want %q", event.RunID, runID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event %q", runID)
	}
}

func assertNoEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected event after unsubscribe: %+v", event)
	case <-time.After(50 * time.Millisecond):
	}
}
