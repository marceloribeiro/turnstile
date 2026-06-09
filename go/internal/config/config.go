// Package config loads Turnstile's runtime configuration from environment
// variables with sane defaults. Everything is overridable; nothing is required.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the data-plane configuration, loaded from the environment.
type Config struct {
	// ListenAddr is the data-plane (proxy) listen address.
	ListenAddr string
	// UpstreamBase is the default provider base URL the proxy forwards to
	// (OpenRouter), used by the fallback adapter when no other adapter matches.
	UpstreamBase string
	// OpenAIBase is the upstream for the direct OpenAI adapter (matches /v1/ paths,
	// e.g. /v1/chat/completions and /v1/responses).
	OpenAIBase string
	// AnthropicBase is the upstream for the direct Anthropic adapter (the Messages
	// API, /v1/messages).
	AnthropicBase string
	// GeminiBase is the upstream for the direct Gemini adapter (the Generative
	// Language API, /v1beta/models/{model}:generateContent).
	GeminiBase string
	// FailClosed inverts the default fail-open behaviour (Q3). When false (the
	// default), an internal Turnstile fault must never break the customer's call.
	FailClosed bool
	// ShutdownTimeout bounds how long we wait for in-flight streams to drain on
	// SIGTERM before forcing the server closed.
	ShutdownTimeout time.Duration
	// Salt seeds the HMAC for API-key fingerprints and session hashes (Q9). A
	// stable value keeps fingerprints comparable across restarts; if empty, a
	// random per-process salt is generated (fingerprints won't survive a restart).
	Salt string

	// Enforcement.
	SessionBudgetUSD float64       // per-session hard ceiling; 0 disables
	LoopEnabled      bool          // loop detection on/off (default on)
	LoopThreshold    int           // identical calls in the window that trip a loop
	LoopWindow       time.Duration // the rolling window

	// Control plane.
	ControlListen   string // listen address; localhost-bound by default
	ControlToken    string // bearer token; empty disables auth (rely on localhost)
	DashboardOrigin string // CORS allowed origin for the hosted dashboard

	// Telemetry ingest — ship aggregates to the platform. Empty URL or key
	// disables it. The container holds an ingest key, never DB creds.
	IngestURL      string
	IngestKey      string
	IngestInterval time.Duration

	// MaxMeterBytes caps how much of a non-streaming response body Turnstile
	// buffers for metering. The client always receives the full body unchanged;
	// only the metering copy is bounded, so a huge response can't blow up memory.
	MaxMeterBytes int64

	// Debug logs one line per proxied request (method, path, status, adapter,
	// session, content-encoding, whether usage was parsed). Off by default.
	Debug bool
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		ListenAddr:      env("TURNSTILE_LISTEN", ":8080"),
		UpstreamBase:    env("TURNSTILE_UPSTREAM", "https://openrouter.ai"),
		OpenAIBase:      env("TURNSTILE_OPENAI_BASE", "https://api.openai.com"),
		AnthropicBase:   env("TURNSTILE_ANTHROPIC_BASE", "https://api.anthropic.com"),
		GeminiBase:      env("TURNSTILE_GEMINI_BASE", "https://generativelanguage.googleapis.com"),
		FailClosed:      envBool("TURNSTILE_FAIL_CLOSED", false),
		ShutdownTimeout: envDur("TURNSTILE_SHUTDOWN_TIMEOUT", 25*time.Second),
		Salt:            env("TURNSTILE_SALT", ""),

		SessionBudgetUSD: envFloat("TURNSTILE_SESSION_BUDGET_USD", 0),
		LoopEnabled:      envBool("TURNSTILE_LOOP_ENABLED", true),
		LoopThreshold:    envInt("TURNSTILE_LOOP_THRESHOLD", 20),
		LoopWindow:       envDur("TURNSTILE_LOOP_WINDOW", 60*time.Second),

		ControlListen:   env("TURNSTILE_CONTROL_LISTEN", "127.0.0.1:8081"),
		ControlToken:    env("TURNSTILE_CONTROL_TOKEN", ""),
		DashboardOrigin: env("TURNSTILE_DASHBOARD_ORIGIN", "http://localhost:3000"),

		IngestURL:      env("TURNSTILE_INGEST_URL", ""),
		IngestKey:      env("TURNSTILE_INGEST_KEY", ""),
		IngestInterval: envDur("TURNSTILE_INGEST_INTERVAL", 30*time.Second),

		MaxMeterBytes: envInt64("TURNSTILE_MAX_METER_BYTES", 8<<20),

		Debug: envBool("TURNSTILE_DEBUG", false),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
