// Package telemetry ships the Go data plane's session aggregates to the FastAPI
// platform's ingest endpoint (M7). It periodically snapshots the meter store and
// POSTs it, authed by a per-deployment ingest key. The Go container holds no DB
// credentials — its only contract with the platform is this HTTP endpoint.
//
// Fail-open: telemetry runs in its own goroutine, off the hot path, and never
// affects request handling. Network errors are logged and dropped.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"turnstile/internal/meter"
)

// Snapshotter yields the current per-session aggregates (meter.MemStore satisfies it).
type Snapshotter interface {
	Sessions() []meter.Record
}

// Client periodically pushes session snapshots to the ingest endpoint.
type Client struct {
	url      string
	key      string
	interval time.Duration
	source   Snapshotter
	http     *http.Client
}

// New builds a telemetry client.
func New(url, key string, interval time.Duration, source Snapshotter) *Client {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Client{
		url:      url,
		key:      key,
		interval: interval,
		source:   source,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Run pushes on each tick until ctx is cancelled, then does a final flush.
func (c *Client) Run(ctx context.Context) {
	log.Printf("telemetry: pushing to %s every %s", c.url, c.interval)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Final flush on shutdown, bounded by the client Timeout rather than the
			// now-cancelled loop context.
			flush, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
			c.push(flush)
			cancel()
			return
		case <-ticker.C:
			c.push(ctx)
		}
	}
}

type ingestBody struct {
	Sessions []meter.Record `json:"sessions"`
}

// push snapshots the store and POSTs it. Always fail-open — any error is logged
// and dropped; telemetry never breaks the data plane.
func (c *Client) push(ctx context.Context) {
	sessions := c.source.Sessions()
	if len(sessions) == 0 {
		return
	}
	body, err := json.Marshal(ingestBody{Sessions: sessions})
	if err != nil {
		log.Printf("telemetry: marshal failed (dropped): %v", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		log.Printf("telemetry: request build failed (dropped): %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Turnstile-Ingest-Key", c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		log.Printf("telemetry: push failed (dropped): %v", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("telemetry: ingest returned %d (dropped %d sessions)", resp.StatusCode, len(sessions))
	}
}
