// Package events is a tiny in-process pub/sub that turns each completed or
// blocked request (and manual kills) into a live event stream the control plane
// serves over SSE. It implements metrics.Sink, so it observes every request
// without the proxy knowing about it.
package events

import (
	"sync"
	"time"

	"turnstile/internal/metrics"
)

// Event is one thing that happened, shaped for the dashboard.
type Event struct {
	Type       string  `json:"type"` // "request" | "blocked" | "killed"
	SessionID  string  `json:"session_id"`
	Source     string  `json:"source,omitempty"`
	Adapter    string  `json:"adapter,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	OverheadMs float64 `json:"overhead_ms,omitempty"`
	TS         int64   `json:"ts"` // unix millis
}

// Broker fans events out to subscribers (SSE handlers).
type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewBroker builds an empty broker.
func NewBroker() *Broker { return &Broker{subs: map[chan Event]struct{}{}} }

// Subscribe returns a buffered channel of events. Unsubscribe when done.
func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Publish broadcasts an event, dropping it for any subscriber whose buffer is
// full (a slow dashboard must never back up the proxy).
func (b *Broker) Publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Record implements metrics.Sink: every request/block becomes an event.
func (b *Broker) Record(s metrics.Sample) {
	ev := Event{
		SessionID:  s.SessionID,
		Source:     s.SessionSource,
		Adapter:    s.Adapter,
		OverheadMs: s.OverheadMs,
		TS:         time.Now().UnixMilli(),
	}
	if s.Blocked {
		ev.Type = "blocked"
		ev.Reason = s.BlockReason
	} else {
		ev.Type = "request"
	}
	b.Publish(ev)
}
