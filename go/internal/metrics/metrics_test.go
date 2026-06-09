package metrics

import (
	"sync"
	"testing"
)

func TestRecorderPercentilesAndMax(t *testing.T) {
	r := NewRecorder(1000)
	// 1..100 ms overheads.
	for i := 1; i <= 100; i++ {
		r.Record(Sample{OverheadMs: float64(i)})
	}
	snap := r.Snapshot()
	if snap.Count != 100 {
		t.Errorf("count = %d, want 100", snap.Count)
	}
	if snap.OverheadMax != 100 {
		t.Errorf("max = %v, want 100", snap.OverheadMax)
	}
	// pct uses index = floor(len*p): p50 -> index 50 -> value 51, p99 -> index 99 -> value 100.
	if snap.OverheadP50 != 51 {
		t.Errorf("p50 = %v, want 51", snap.OverheadP50)
	}
	if snap.OverheadP99 != 100 {
		t.Errorf("p99 = %v, want 100", snap.OverheadP99)
	}
}

func TestRecorderRingBufferBoundsMemory(t *testing.T) {
	r := NewRecorder(10)
	for i := 0; i < 100; i++ {
		r.Record(Sample{OverheadMs: float64(i)})
	}
	snap := r.Snapshot()
	if snap.Count != 100 {
		t.Errorf("count must reflect all observed samples: got %d, want 100", snap.Count)
	}
	// The window retains only the last 10 (90..99), so the max OVER THE WINDOW the
	// percentiles see is 99 — but OverheadMax is tracked all-time, so it's 99 too here.
	if got := len(r.overhead); got != 10 {
		t.Errorf("ring buffer should cap at 10, got %d", got)
	}
	if snap.OverheadMax != 99 {
		t.Errorf("all-time max = %v, want 99", snap.OverheadMax)
	}
}

func TestRecorderMaxSurvivesEviction(t *testing.T) {
	// The peak sample is evicted from the ring, but OverheadMax must still report it.
	r := NewRecorder(3)
	r.Record(Sample{OverheadMs: 99}) // the peak, will be evicted
	for i := 0; i < 5; i++ {
		r.Record(Sample{OverheadMs: 1})
	}
	if snap := r.Snapshot(); snap.OverheadMax != 99 {
		t.Errorf("max must survive ring eviction: got %v, want 99", snap.OverheadMax)
	}
}

func TestNewRecorderClampsCapacity(t *testing.T) {
	r := NewRecorder(0) // invalid; must clamp to >=1, not panic
	r.Record(Sample{OverheadMs: 7})
	if snap := r.Snapshot(); snap.Count != 1 || snap.OverheadP50 != 7 {
		t.Errorf("zero-capacity recorder mishandled: %+v", snap)
	}
}

func TestSnapshotEmpty(t *testing.T) {
	if snap := NewRecorder(10).Snapshot(); snap.Count != 0 || snap.OverheadP50 != 0 || snap.OverheadMax != 0 {
		t.Errorf("empty snapshot should be zero-valued: %+v", snap)
	}
}

func TestPct(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", nil, 0.5, 0},
		{"single", []float64{42}, 0.99, 42},
		{"p0", []float64{1, 2, 3, 4}, 0, 1},
		{"p100 clamps to last", []float64{1, 2, 3, 4}, 1.0, 4},
		{"p50 of four", []float64{1, 2, 3, 4}, 0.5, 3}, // index floor(4*0.5)=2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pct(tt.sorted, tt.p); got != tt.want {
				t.Errorf("pct(%v, %v) = %v, want %v", tt.sorted, tt.p, got, tt.want)
			}
		})
	}
}

// panicSink panics on Record, to prove MultiSink isolates a broken sink.
type panicSink struct{}

func (panicSink) Record(Sample) { panic("boom") }

// countSink counts the samples it receives.
type countSink struct{ n int }

func (c *countSink) Record(Sample) { c.n++ }

func TestMultiSinkIsolatesPanickingSink(t *testing.T) {
	good := &countSink{}
	ms := MultiSink{panicSink{}, good, panicSink{}}
	ms.Record(Sample{}) // must not panic
	if good.n != 1 {
		t.Errorf("healthy sink should still receive the sample despite panicking peers, got %d", good.n)
	}
}

func TestRecorderConcurrentSafe(t *testing.T) {
	// Exercised under -race: concurrent Record + Snapshot must not race.
	r := NewRecorder(100)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				r.Record(Sample{OverheadMs: float64(i)})
				if i%50 == 0 {
					_ = r.Snapshot()
				}
			}
		}()
	}
	wg.Wait()
	if snap := r.Snapshot(); snap.Count != 8*500 {
		t.Errorf("count = %d, want %d", snap.Count, 8*500)
	}
}
