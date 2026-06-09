// Package meter accumulates per-session spend (tokens + cost), pricing each call
// via the pricing engine. It implements the proxy's Metering interface.
//
// Spend lives behind a Store interface; the default is in-memory. Durable
// persistence can slot in as another Store implementation when spend needs to
// survive restarts — durable history is otherwise delegated to the platform via
// telemetry.
package meter

import (
	"sort"
	"sync"

	"turnstile/internal/adapter"
	"turnstile/internal/pricing"
	"turnstile/internal/session"
)

// Spend is a running total for one session.
type Spend struct {
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	Blocks           int64   `json:"blocks"`    // enforcement blocks against this session
	Prevented        float64 `json:"prevented"` // estimated dollars prevented by those blocks
}

// Record is a session's identity plus its accumulated spend.
type Record struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	User     string `json:"user"`
	KeyFP    string `json:"key_fp"`
	Model    string `json:"model"`
	LastCost string `json:"last_cost"` // last pricing precedence rung used
	Spend
}

// Store persists session spend. The in-memory implementation is the M4 default.
// SessionSpend + RecordPrevented also satisfy enforce.Ledger (M5).
type Store interface {
	Add(ident session.Identity, model string, delta Spend, costSource string)
	Sessions() []Record
	Get(id string) (Record, bool)
	SessionSpend(id string) (cost float64, requests int64)
	RecordPrevented(id string, amount float64)
}

// Meter prices a usage and records it against a session.
type Meter struct {
	prices *pricing.Engine
	store  Store
}

// New builds a Meter over a pricing engine and a store.
func New(prices *pricing.Engine, store Store) *Meter {
	return &Meter{prices: prices, store: store}
}

// Store exposes the underlying store (for the future control-plane/dashboard).
func (m *Meter) Store() Store { return m.store }

// Observe prices the usage and accumulates it against the session. Called off the
// hot path by the proxy after a response completes.
func (m *Meter) Observe(ident session.Identity, model string, u adapter.Usage) {
	c, src := m.prices.Cost(model, u)
	m.store.Add(ident, model, Spend{
		Requests:         1,
		PromptTokens:     int64(u.PromptTokens),
		CompletionTokens: int64(u.CompletionTokens),
		Cost:             c,
	}, string(src))
}

// MemStore is the in-memory Store.
type MemStore struct {
	mu       sync.RWMutex
	sessions map[string]*Record
}

// NewMemStore builds an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{sessions: map[string]*Record{}} }

func (s *MemStore) Add(ident session.Identity, model string, d Spend, costSource string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.sessions[ident.ID]
	if rec == nil {
		rec = &Record{ID: ident.ID, Source: string(ident.Source), User: ident.User, KeyFP: ident.KeyFP}
		s.sessions[ident.ID] = rec
	}
	rec.Model = model
	rec.LastCost = costSource
	rec.Requests += d.Requests
	rec.PromptTokens += d.PromptTokens
	rec.CompletionTokens += d.CompletionTokens
	rec.Cost += d.Cost
}

// SessionSpend returns accumulated cost and request count (enforce.Ledger).
func (s *MemStore) SessionSpend(id string) (float64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.sessions[id]; ok {
		return r.Cost, r.Requests
	}
	return 0, 0
}

// RecordPrevented books a blocked call's estimated savings (enforce.Ledger).
func (s *MemStore) RecordPrevented(id string, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.sessions[id]
	if r == nil {
		r = &Record{ID: id}
		s.sessions[id] = r
	}
	r.Blocks++
	r.Prevented += amount
}

func (s *MemStore) Get(id string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.sessions[id]; ok {
		return *r, true
	}
	return Record{}, false
}

func (s *MemStore) Sessions() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.sessions))
	for _, r := range s.sessions {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost > out[j].Cost })
	return out
}
