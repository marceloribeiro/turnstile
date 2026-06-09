package openrouter

import (
	"encoding/json"
	"net/http"
	"testing"

	"turnstile/internal/adapter"
)

func TestForwardURLPassesPathAndQuery(t *testing.T) {
	a := New("https://openrouter.ai")
	r := &adapter.Request{Path: "/api/v1/chat/completions", RawQuery: "x=1", Header: http.Header{}}
	if got, want := a.ForwardURL(r), "https://openrouter.ai/api/v1/chat/completions?x=1"; got != want {
		t.Errorf("ForwardURL = %q, want %q", got, want)
	}
}

func TestNewDefaultsBaseAndTrimsSlash(t *testing.T) {
	if New("").base != DefaultBase {
		t.Errorf("empty base should default to %q", DefaultBase)
	}
	if New("http://localhost:9/").base != "http://localhost:9" {
		t.Errorf("trailing slash not trimmed")
	}
}

func TestExtractRequestMeta(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"openai/gpt-4o-mini","user":"u-7","messages":[` +
		`{"role":"system","content":"be brief"},` +
		`{"role":"user","content":"hello"},` +
		`{"role":"assistant","content":"hi"},` +
		`{"role":"user","content":"again"}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Body: body, Header: http.Header{}})
	if m.Model != "openai/gpt-4o-mini" {
		t.Errorf("model: got %q", m.Model)
	}
	if m.User != "u-7" {
		t.Errorf("user: got %q", m.User)
	}
	if want := "be brief\x00hello"; m.Anchor != want {
		t.Errorf("anchor: got %q, want %q (first system + first user only)", m.Anchor, want)
	}
}

func TestExtractRequestMetaEmptyOnGarbage(t *testing.T) {
	a := New("")
	m := a.ExtractRequestMeta(&adapter.Request{Body: []byte(`not json`), Header: http.Header{}})
	if m.Anchor != "" || m.Model != "" {
		t.Errorf("garbage body should yield empty meta, got %+v", m)
	}
}

func TestParseUsage(t *testing.T) {
	a := New("")
	tests := []struct {
		name         string
		data         string
		wantOK       bool
		prompt, comp int
		cost         float64
		reported     bool
	}{
		{"done sentinel", "[DONE]", false, 0, 0, 0, false},
		{"content delta (no usage)", `{"choices":[{"delta":{"content":"hi"}}]}`, false, 0, 0, 0, false},
		{"garbage", `not json`, false, 0, 0, 0, false},
		{
			name: "usage with reported cost", data: `{"usage":{"prompt_tokens":14,"completion_tokens":6,"cost":0.0003}}`,
			wantOK: true, prompt: 14, comp: 6, cost: 0.0003, reported: true,
		},
		{
			name: "usage without cost", data: `{"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
			wantOK: true, prompt: 5, comp: 2, cost: 0, reported: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, ok := a.ParseUsage([]byte(tt.data))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if u.PromptTokens != tt.prompt || u.CompletionTokens != tt.comp {
				t.Errorf("tokens = %d+%d, want %d+%d", u.PromptTokens, u.CompletionTokens, tt.prompt, tt.comp)
			}
			if u.ReportedCost != tt.cost || u.HasReportedCost != tt.reported {
				t.Errorf("cost = %v/%v, want %v/%v", u.ReportedCost, u.HasReportedCost, tt.cost, tt.reported)
			}
		})
	}
}

func TestNativeError(t *testing.T) {
	a := New("")
	status, ct, body := a.NativeError("loop_detected")
	if status != 429 {
		t.Errorf("status = %d, want 429", status)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var parsed struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body not valid JSON: %v (%s)", err, body)
	}
	if parsed.Error.Type != "turnstile_blocked" || parsed.Error.Code != "loop_detected" {
		t.Errorf("unexpected error body: %s", body)
	}
}

func TestMatch(t *testing.T) {
	a := New("")
	cases := map[string]bool{
		"/api/v1/chat/completions": true,
		"/api/v1/models":           true,
		"/v1/chat/completions":     true,
		"/healthz":                 false,
	}
	for path, want := range cases {
		if got := a.Match(&adapter.Request{Path: path, Header: http.Header{}}); got != want {
			t.Errorf("Match(%q) = %v, want %v", path, got, want)
		}
	}
}
