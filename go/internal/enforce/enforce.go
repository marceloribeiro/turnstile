// Package enforce is Turnstile's circuit breaker — the product's reason to exist.
//
// Three rules (Q6), evaluated at a synchronous pre-execution gate:
//
//  1. manual kill      — an operator killed this session
//  2. spend ceiling    — the session's accumulated cost crossed its budget
//  3. loop detection   — too many near-identical calls in a rolling window (Q7)
//
// A tripped rule blocks the call (the proxy returns a provider-native error) and
// records the dollars prevented. Only the offending session is affected. The
// proxy wraps every call here in a fail-open guard (Q3): if enforcement itself
// errors, the request forwards rather than breaking the customer's app.
package enforce

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"turnstile/internal/session"
)

// Config tunes the three rules. Per-tier defaults + customer overrides feed this.
type Config struct {
	SessionBudgetUSD float64       // per-session hard ceiling; 0 disables
	LoopEnabled      bool          // loop detection on/off
	LoopThreshold    int           // identical calls in the window that trip a loop
	LoopWindow       time.Duration // the rolling window
}

// Reason names a block cause; it travels into the provider-native error.
type Reason string

const (
	ReasonKilled Reason = "session_killed"
	ReasonBudget Reason = "budget_exceeded"
	ReasonLoop   Reason = "loop_detected"
)

// Decision is the gate's verdict.
type Decision struct {
	Allow  bool
	Reason Reason
}

// Ledger is the enforcer's read/write view of session spend (implemented by the
// meter store). SessionSpend reads accumulated cost; RecordPrevented books the
// estimated savings of a blocked call.
type Ledger interface {
	SessionSpend(id string) (cost float64, requests int64)
	RecordPrevented(id string, amount float64)
}

// Enforcer holds kill state and the loop-tracking windows.
type Enforcer struct {
	cfg    Config
	ledger Ledger

	mu       sync.Mutex
	killed   map[string]bool
	loops    map[string][]int64 // sessionID\x00bodyFP -> unix-nano timestamps in window
	lastSwep int64              // unix-nano of the last full eviction sweep
}

// New builds an Enforcer.
func New(cfg Config, ledger Ledger) *Enforcer {
	return &Enforcer{
		cfg:    cfg,
		ledger: ledger,
		killed: map[string]bool{},
		loops:  map[string][]int64{},
	}
}

// Kill marks a session for termination (called by the control plane, M6).
func (e *Enforcer) Kill(id string) {
	e.mu.Lock()
	e.killed[id] = true
	e.mu.Unlock()
}

// Unkill clears a manual kill.
func (e *Enforcer) Unkill(id string) {
	e.mu.Lock()
	delete(e.killed, id)
	e.mu.Unlock()
}

// Killed reports whether a session is killed (used by the live mid-stream gate).
func (e *Enforcer) Killed(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.killed[id]
}

// Pre is the synchronous pre-execution gate. It returns Allow=false to block.
func (e *Enforcer) Pre(ident session.Identity, body []byte) Decision {
	// 1. Manual kill.
	if e.Killed(ident.ID) {
		return e.block(ident, ReasonKilled)
	}
	// 2. Spend ceiling.
	if e.cfg.SessionBudgetUSD > 0 {
		if cost, _ := e.ledger.SessionSpend(ident.ID); cost >= e.cfg.SessionBudgetUSD {
			return e.block(ident, ReasonBudget)
		}
	}
	// 3. Loop detection.
	if e.cfg.LoopEnabled && e.loopTrips(ident.ID, body) {
		return e.block(ident, ReasonLoop)
	}
	return Decision{Allow: true}
}

// block records the estimated dollars prevented and returns a deny decision. The
// estimate is the session's mean cost-per-request so far — a defensible proxy for
// what the blocked call would have cost (0 when we have no prior data).
func (e *Enforcer) block(ident session.Identity, reason Reason) Decision {
	if e.ledger != nil {
		if cost, reqs := e.ledger.SessionSpend(ident.ID); reqs > 0 {
			e.ledger.RecordPrevented(ident.ID, cost/float64(reqs))
		}
	}
	return Decision{Allow: false, Reason: reason}
}

// loopTrips records this request's body fingerprint in the session's rolling
// window and reports whether the count has reached the threshold (Q7: exact-ish —
// identical bodies; fuzzy/semantic similarity is a later refinement).
func (e *Enforcer) loopTrips(id string, body []byte) bool {
	key := id + "\x00" + fingerprint(body)
	now := time.Now().UnixNano()
	cutoff := now - e.cfg.LoopWindow.Nanoseconds()

	e.mu.Lock()
	defer e.mu.Unlock()
	ts := e.loops[key]
	kept := ts[:0]
	for _, t := range ts {
		if t >= cutoff {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	e.loops[key] = kept

	e.sweepLocked(now, cutoff)
	return len(kept) >= e.cfg.LoopThreshold
}

// sweepLocked drops keys whose windows are fully expired so the map can't grow
// unbounded over a long-lived process. Runs at most once per window. Caller holds mu.
func (e *Enforcer) sweepLocked(now, cutoff int64) {
	if now-e.lastSwep < e.cfg.LoopWindow.Nanoseconds() {
		return
	}
	e.lastSwep = now
	for k, ts := range e.loops {
		live := false
		for _, t := range ts {
			if t >= cutoff {
				live = true
				break
			}
		}
		if !live {
			delete(e.loops, k)
		}
	}
}

func fingerprint(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:8])
}
