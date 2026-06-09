// Package metrics holds Turnstile's self-instrumentation: the measurement of our
// OWN in-process overhead, isolated from upstream/provider latency.
//
// This is the Phase 0 R4 fix. End-to-end timing is useless for measuring our
// overhead because provider jitter (often seconds) swamps our sub-millisecond
// processing. So we time only our slice — work done before forwarding, plus the
// cost of forwarding the first byte — and explicitly exclude time spent blocked
// waiting on the upstream.
package metrics

import (
	"sort"
	"sync"
)

// Sample is one request's timing breakdown, all in milliseconds.
type Sample struct {
	Adapter            string  // which provider adapter served the request
	SessionID          string  // resolved session/trajectory id
	SessionSource      string  // header | inferred | unattributed
	Blocked            bool    // enforcement blocked this request
	BlockReason        string  // why, when Blocked
	PreMs              float64 // request received -> upstream request issued (ours)
	UpstreamWaitMs     float64 // upstream request issued -> first upstream byte (NOT ours)
	FirstByteForwardMs float64 // first upstream byte -> forwarded to client (ours)
	OverheadMs         float64 // PreMs + FirstByteForwardMs — our added time-to-first-token
	TotalMs            float64 // request received -> stream complete
	Ok                 bool
}

// Sink receives samples. The proxy records asynchronously through a Sink so that
// instrumentation can never add latency to, or break, the request path.
type Sink interface {
	Record(Sample)
}

// MultiSink fans a Sample out to several sinks, guarding each so one panicking
// sink can't stop the others. Lets the recorder and the live-event broker both
// observe every request.
type MultiSink []Sink

func (m MultiSink) Record(s Sample) {
	for _, snk := range m {
		func() {
			defer func() { _ = recover() }()
			snk.Record(s)
		}()
	}
}

// Recorder is the default in-memory Sink. It keeps a bounded ring of recent
// overhead values for percentile snapshots — fixed memory, no external deps.
type Recorder struct {
	mu          sync.Mutex
	n           int64
	capN        int
	overhead    []float64
	idx         int
	maxOverhead float64
}

// NewRecorder returns a Recorder retaining the last `capacity` overhead samples.
func NewRecorder(capacity int) *Recorder {
	if capacity < 1 {
		capacity = 1
	}
	return &Recorder{capN: capacity, overhead: make([]float64, 0, capacity)}
}

// Record stores one sample. Safe for concurrent use.
func (r *Recorder) Record(s Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n++
	if s.OverheadMs > r.maxOverhead {
		r.maxOverhead = s.OverheadMs
	}
	if len(r.overhead) < r.capN {
		r.overhead = append(r.overhead, s.OverheadMs)
	} else {
		r.overhead[r.idx] = s.OverheadMs
		r.idx = (r.idx + 1) % r.capN
	}
}

// Snapshot summarises our overhead distribution.
type Snapshot struct {
	Count       int64
	OverheadP50 float64
	OverheadP99 float64
	OverheadMax float64
}

// Snapshot computes percentiles over the retained window.
func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	cp := append([]float64(nil), r.overhead...)
	n := r.n
	mx := r.maxOverhead
	r.mu.Unlock()

	sort.Float64s(cp)
	return Snapshot{
		Count:       n,
		OverheadP50: pct(cp, 0.50),
		OverheadP99: pct(cp, 0.99),
		OverheadMax: mx,
	}
}

func pct(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)) * p)
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}
