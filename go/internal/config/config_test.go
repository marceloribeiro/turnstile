package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Unset every var Load reads so we observe pure defaults regardless of the
	// caller's environment. t.Setenv records the prior value for restore on cleanup;
	// Unsetenv then clears it for the duration of the test.
	for _, k := range []string{
		"TURNSTILE_LISTEN", "TURNSTILE_UPSTREAM", "TURNSTILE_FAIL_CLOSED",
		"TURNSTILE_SHUTDOWN_TIMEOUT", "TURNSTILE_SALT", "TURNSTILE_SESSION_BUDGET_USD",
		"TURNSTILE_LOOP_ENABLED", "TURNSTILE_LOOP_THRESHOLD", "TURNSTILE_LOOP_WINDOW",
		"TURNSTILE_CONTROL_LISTEN", "TURNSTILE_CONTROL_TOKEN", "TURNSTILE_DASHBOARD_ORIGIN",
		"TURNSTILE_INGEST_URL", "TURNSTILE_INGEST_KEY", "TURNSTILE_INGEST_INTERVAL",
		"TURNSTILE_MAX_METER_BYTES", "TURNSTILE_DEBUG",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	cfg := Load()
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.UpstreamBase != "https://openrouter.ai" {
		t.Errorf("UpstreamBase default = %q", cfg.UpstreamBase)
	}
	if cfg.FailClosed {
		t.Errorf("FailClosed default should be false")
	}
	if cfg.ControlListen != "127.0.0.1:8081" {
		t.Errorf("ControlListen default = %q", cfg.ControlListen)
	}
	if !cfg.LoopEnabled {
		t.Errorf("LoopEnabled default should be true")
	}
	if cfg.LoopThreshold != 20 {
		t.Errorf("LoopThreshold default = %d, want 20", cfg.LoopThreshold)
	}
	if cfg.LoopWindow != 60*time.Second {
		t.Errorf("LoopWindow default = %s, want 60s", cfg.LoopWindow)
	}
	if cfg.ShutdownTimeout != 25*time.Second {
		t.Errorf("ShutdownTimeout default = %s, want 25s", cfg.ShutdownTimeout)
	}
	if cfg.IngestInterval != 30*time.Second {
		t.Errorf("IngestInterval default = %s, want 30s", cfg.IngestInterval)
	}
	if cfg.DashboardOrigin != "http://localhost:3000" {
		t.Errorf("DashboardOrigin default = %q, want http://localhost:3000", cfg.DashboardOrigin)
	}
	if cfg.MaxMeterBytes != 8<<20 {
		t.Errorf("MaxMeterBytes default = %d, want %d", cfg.MaxMeterBytes, 8<<20)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("TURNSTILE_LISTEN", ":9090")
	t.Setenv("TURNSTILE_UPSTREAM", "https://example.test")
	t.Setenv("TURNSTILE_FAIL_CLOSED", "true")
	t.Setenv("TURNSTILE_SESSION_BUDGET_USD", "12.5")
	t.Setenv("TURNSTILE_LOOP_ENABLED", "false")
	t.Setenv("TURNSTILE_LOOP_THRESHOLD", "7")
	t.Setenv("TURNSTILE_LOOP_WINDOW", "90s")
	t.Setenv("TURNSTILE_INGEST_INTERVAL", "5s")
	t.Setenv("TURNSTILE_DEBUG", "true")

	cfg := Load()
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.UpstreamBase != "https://example.test" {
		t.Errorf("UpstreamBase = %q", cfg.UpstreamBase)
	}
	if !cfg.FailClosed {
		t.Errorf("FailClosed should be true")
	}
	if cfg.SessionBudgetUSD != 12.5 {
		t.Errorf("SessionBudgetUSD = %v", cfg.SessionBudgetUSD)
	}
	if cfg.LoopEnabled {
		t.Errorf("LoopEnabled should be false")
	}
	if cfg.LoopThreshold != 7 {
		t.Errorf("LoopThreshold = %d", cfg.LoopThreshold)
	}
	if cfg.LoopWindow != 90*time.Second {
		t.Errorf("LoopWindow = %s", cfg.LoopWindow)
	}
	if cfg.IngestInterval != 5*time.Second {
		t.Errorf("IngestInterval = %s", cfg.IngestInterval)
	}
	if !cfg.Debug {
		t.Errorf("Debug should be true")
	}
}

// Invalid typed values must fall back to the default, never crash.
func TestTypedParsersFallBackOnGarbage(t *testing.T) {
	t.Setenv("TURNSTILE_FAIL_CLOSED", "notabool")
	t.Setenv("TURNSTILE_SESSION_BUDGET_USD", "notafloat")
	t.Setenv("TURNSTILE_LOOP_THRESHOLD", "notanint")
	t.Setenv("TURNSTILE_LOOP_WINDOW", "notaduration")

	cfg := Load()
	if cfg.FailClosed {
		t.Errorf("garbage bool should keep default false")
	}
	if cfg.SessionBudgetUSD != 0 {
		t.Errorf("garbage float should keep default 0, got %v", cfg.SessionBudgetUSD)
	}
	if cfg.LoopThreshold != 20 {
		t.Errorf("garbage int should keep default 20, got %d", cfg.LoopThreshold)
	}
	if cfg.LoopWindow != 60*time.Second {
		t.Errorf("garbage duration should keep default 60s, got %s", cfg.LoopWindow)
	}
}
