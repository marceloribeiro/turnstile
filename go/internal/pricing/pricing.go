// Package pricing computes the dollar cost of a call, applying the Q8 precedence:
//
//	provider-reported cost  →  customer override  →  pulled/loaded table  →  unknown
//
// Prices are data, not code: a per-model table loadable from OpenRouter's models
// endpoint or a JSON file, with a customer override overlay for negotiated rates.
// (The signed-CDN manifest refresh from §5 is operational tooling layered on top
// of SetTable; the precedence engine here is the core.)
package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"turnstile/internal/adapter"
)

// loadClient bounds the OpenRouter table fetch independently of the caller's
// context, so a missing/forgotten deadline can't hang startup indefinitely.
var loadClient = &http.Client{Timeout: 10 * time.Second}

// Price is the per-token cost of a model, in USD.
type Price struct {
	Prompt     float64
	Completion float64
}

// Engine resolves prices and computes cost.
type Engine struct {
	mu       sync.RWMutex
	table    map[string]Price // base manifest (e.g. from OpenRouter /models)
	override map[string]Price // customer negotiated rates — win over the table
}

// New returns an empty engine. With no table, provider-reported costs still work
// (and OpenRouter reports cost in-stream), so an empty engine is still useful.
func New() *Engine {
	return &Engine{table: map[string]Price{}, override: map[string]Price{}}
}

// SetTable replaces the base price table.
func (e *Engine) SetTable(t map[string]Price) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.table = t
}

// SetOverride replaces the customer override overlay.
func (e *Engine) SetOverride(o map[string]Price) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.override = o
}

// CostSource records which precedence rung produced a cost.
type CostSource string

const (
	SourceReported CostSource = "reported"
	SourceOverride CostSource = "override"
	SourceTable    CostSource = "table"
	SourceUnknown  CostSource = "unknown"
)

// Cost returns the dollar cost of a usage and the precedence rung used.
func (e *Engine) Cost(model string, u adapter.Usage) (float64, CostSource) {
	if u.HasReportedCost {
		return u.ReportedCost, SourceReported
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if p, ok := e.override[model]; ok {
		return cost(p, u), SourceOverride
	}
	if p, ok := e.table[model]; ok {
		return cost(p, u), SourceTable
	}
	return 0, SourceUnknown
}

func cost(p Price, u adapter.Usage) float64 {
	return float64(u.PromptTokens)*p.Prompt + float64(u.CompletionTokens)*p.Completion
}

// ParseTableJSON parses a {"model":{"prompt":<usd/token>,"completion":<usd/token>}}
// document — the format for override files and bundled defaults.
func ParseTableJSON(b []byte) (map[string]Price, error) {
	var raw map[string]Price
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// LoadOpenRouterTable fetches OpenRouter's model list and builds a price table.
// OpenRouter reports per-token USD prices as strings under data[].pricing.
func LoadOpenRouterTable(ctx context.Context) (map[string]Price, error) {
	return loadOpenRouterTable(ctx, "https://openrouter.ai/api/v1/models")
}

func loadOpenRouterTable(ctx context.Context, url string) (map[string]Price, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := loadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	m := make(map[string]Price, len(body.Data))
	for _, d := range body.Data {
		p, _ := strconv.ParseFloat(d.Pricing.Prompt, 64)
		c, _ := strconv.ParseFloat(d.Pricing.Completion, 64)
		m[d.ID] = Price{Prompt: p, Completion: c}
	}
	return m, nil
}
