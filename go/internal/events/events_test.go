package events

import (
	"testing"
	"time"

	"turnstile/internal/metrics"
)

func TestRecordPublishesToSubscribers(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Record(metrics.Sample{SessionID: "s1", Adapter: "openrouter", OverheadMs: 0.5})
	select {
	case ev := <-ch:
		if ev.Type != "request" || ev.SessionID != "s1" || ev.TS == 0 {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestBlockedSampleBecomesBlockedEvent(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Record(metrics.Sample{SessionID: "s2", Blocked: true, BlockReason: "loop_detected"})
	ev := <-ch
	if ev.Type != "blocked" || ev.Reason != "loop_detected" {
		t.Fatalf("expected blocked event, got %+v", ev)
	}
}

func TestPublishDropsWhenSubscriberFull(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)
	// Overfill well past the 64 buffer; Publish must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.Publish(Event{Type: "request", SessionID: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a full subscriber")
	}
}
