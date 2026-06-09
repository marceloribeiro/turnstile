// Command turnstile is the data-plane container entrypoint (M1).
//
// It starts the transparent streaming proxy, waits for SIGINT/SIGTERM, drains
// in-flight streams within the shutdown timeout, and prints a final overhead
// snapshot (our self-instrumented latency, not end-to-end).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"time"

	"turnstile/internal/adapter"
	"turnstile/internal/adapter/anthropic"
	"turnstile/internal/adapter/gemini"
	"turnstile/internal/adapter/openai"
	"turnstile/internal/adapter/openrouter"
	"turnstile/internal/config"
	"turnstile/internal/control"
	"turnstile/internal/enforce"
	"turnstile/internal/events"
	"turnstile/internal/meter"
	"turnstile/internal/metrics"
	"turnstile/internal/pricing"
	"turnstile/internal/proxy"
	"turnstile/internal/session"
	"turnstile/internal/telemetry"
)

func main() {
	cfg := config.Load()

	rec := metrics.NewRecorder(10000)
	// OpenRouter is the fallback (handles /api/v1/ and anything unmatched).
	// Match-first adapters are tried in order; OpenAI is LAST among them because it
	// broadly claims /v1/, which would otherwise swallow Anthropic's /v1/messages
	// and Gemini's /v1/models/...:generateContent. Anthropic and Gemini match on
	// their distinctive paths/headers, so their order relative to each other is
	// irrelevant — only that both precede OpenAI.
	reg := adapter.NewRegistry(
		openrouter.New(cfg.UpstreamBase),
		anthropic.New(cfg.AnthropicBase),
		gemini.New(cfg.GeminiBase),
		openai.New(cfg.OpenAIBase),
	)

	// M3: session resolver. A configured salt keeps key fingerprints / session
	// hashes stable across restarts; otherwise we generate an ephemeral one.
	salt := cfg.Salt
	if salt == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		salt = hex.EncodeToString(b)
		log.Printf("TURNSTILE_SALT unset — using an ephemeral salt; fingerprints won't persist across restarts")
	}
	resolver := session.NewResolver([]byte(salt))

	// M4: pricing engine + meter. The OpenRouter table is best-effort — even with
	// an empty table, OpenRouter's in-stream reported cost (preferred anyway) keeps
	// metering accurate.
	prices := pricing.New()
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if table, err := pricing.LoadOpenRouterTable(loadCtx); err != nil {
		log.Printf("pricing: could not load OpenRouter table (%v) — relying on provider-reported cost", err)
	} else {
		prices.SetTable(table)
		log.Printf("pricing: loaded %d model prices", len(table))
	}
	loadCancel()
	store := meter.NewMemStore()
	mtr := meter.New(prices, store)

	// M5: enforcement. The meter store doubles as the enforcer's ledger.
	enf := enforce.New(enforce.Config{
		SessionBudgetUSD: cfg.SessionBudgetUSD,
		LoopEnabled:      cfg.LoopEnabled,
		LoopThreshold:    cfg.LoopThreshold,
		LoopWindow:       cfg.LoopWindow,
	}, store)
	log.Printf("enforcement: budget=$%.4f/session loop=%v (threshold=%d window=%s) fail-open=%v",
		cfg.SessionBudgetUSD, cfg.LoopEnabled, cfg.LoopThreshold, cfg.LoopWindow, !cfg.FailClosed)

	// M6: the event broker is also a metrics sink, so every request/block becomes
	// a live event. The proxy records to both the recorder and the broker.
	broker := events.NewBroker()
	p := proxy.New(reg, resolver, mtr, enf, cfg.FailClosed, metrics.MultiSink{rec, broker})
	p.Debug = cfg.Debug
	p.MaxMeterBytes = cfg.MaxMeterBytes

	mux := http.NewServeMux()
	mux.Handle("/", p)
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	// Control plane — separate listener, auth-gated, serving the dashboard API.
	ctrl := control.New(store, enf, broker, cfg.ControlToken, cfg.DashboardOrigin)
	ctrlSrv := &http.Server{Addr: cfg.ControlListen, Handler: ctrl.Handler()}

	go func() {
		log.Printf("turnstile data-plane listening on %s -> %s (fail-open=%v)",
			cfg.ListenAddr, cfg.UpstreamBase, !cfg.FailClosed)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()
	go func() {
		log.Printf("turnstile control-plane listening on %s (auth=%v, dashboard-origin=%s)",
			cfg.ControlListen, cfg.ControlToken != "", cfg.DashboardOrigin)
		if err := ctrlSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("control serve: %v", err)
		}
	}()

	// M7: telemetry — ship session aggregates to the platform's ingest endpoint.
	// Off the hot path, fail-open. Disabled unless both URL and key are set.
	telCtx, telCancel := context.WithCancel(context.Background())
	telDone := make(chan struct{})
	telemetryOn := cfg.IngestURL != "" && cfg.IngestKey != ""
	if telemetryOn {
		client := telemetry.New(cfg.IngestURL, cfg.IngestKey, cfg.IngestInterval, store)
		go func() { client.Run(telCtx); close(telDone) }()
	} else {
		close(telDone)
		log.Printf("telemetry: disabled (set TURNSTILE_INGEST_URL + TURNSTILE_INGEST_KEY to enable)")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Printf("shutting down (draining in-flight streams, timeout %s)...", cfg.ShutdownTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("data-plane shutdown: %v", err)
	}
	if err := ctrlSrv.Shutdown(ctx); err != nil {
		log.Printf("control-plane shutdown: %v", err)
	}

	// Stop telemetry and let it do a final flush.
	telCancel()
	select {
	case <-telDone:
	case <-time.After(5 * time.Second):
		log.Printf("telemetry: final flush timed out")
	}

	s := rec.Snapshot()
	log.Printf("overhead snapshot (self-instrumented): n=%d p50=%.3fms p99=%.3fms max=%.3fms",
		s.Count, s.OverheadP50, s.OverheadP99, s.OverheadMax)

	// Spend snapshot — a stand-in for the dashboard (M6/M7) so metering is
	// observable now. Top sessions by cost.
	sessions := mtr.Store().Sessions()
	log.Printf("spend snapshot: %d session(s)", len(sessions))
	for i, sess := range sessions {
		if i >= 10 {
			break
		}
		log.Printf("  %s [%s] $%.6f  (%d req, %d+%d tok, cost via %s) | blocks=%d prevented=$%.6f",
			sess.ID, sess.Source, sess.Cost, sess.Requests,
			sess.PromptTokens, sess.CompletionTokens, sess.LastCost,
			sess.Blocks, sess.Prevented)
	}
}
