package anthropic

import (
	"bytes"
	"net/http"
	"testing"

	"turnstile/internal/adapter"
)

func TestMatch(t *testing.T) {
	a := New("")
	cases := []struct {
		name   string
		path   string
		header http.Header
		want   bool
	}{
		{"messages path", "/v1/messages", nil, true},
		{"anthropic-version header", "/v1/chat/completions", http.Header{"Anthropic-Version": {"2023-06-01"}}, true},
		{"x-api-key header", "/v1/anything", http.Header{"X-Api-Key": {"sk-ant-…"}}, true},
		{"openai chat, no anthropic markers", "/v1/chat/completions", nil, false},
		{"openrouter surface", "/api/v1/chat/completions", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := a.Match(&adapter.Request{Path: c.path, Header: c.header}); got != c.want {
				t.Errorf("Match = %v, want %v", got, c.want)
			}
		})
	}
}

func TestForwardURL(t *testing.T) {
	a := New("https://api.anthropic.com/")
	if got := a.ForwardURL(&adapter.Request{Path: "/v1/messages"}); got != "https://api.anthropic.com/v1/messages" {
		t.Errorf("ForwardURL = %q", got)
	}
	if got := a.ForwardURL(&adapter.Request{Path: "/v1/messages", RawQuery: "beta=1"}); got != "https://api.anthropic.com/v1/messages?beta=1" {
		t.Errorf("ForwardURL with query = %q", got)
	}
}

func TestExtractRequestMetaStringSystem(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"claude-3-5-sonnet","system":"be brief",
		"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}],
		"metadata":{"user_id":"u9"}}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/messages", Body: body})
	if m.Model != "claude-3-5-sonnet" || m.User != "u9" {
		t.Fatalf("model/user = %q/%q", m.Model, m.User)
	}
	if m.Anchor != "be brief\x00hi" {
		t.Errorf("anchor = %q", m.Anchor)
	}
	if m.PreviousResponseID != "" {
		t.Errorf("anthropic has no response chain, got %q", m.PreviousResponseID)
	}
}

func TestExtractRequestMetaBlockSystem(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"m","system":[{"type":"text","text":"sysblock"}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/messages", Body: body})
	if m.Anchor != "sysblock\x00hello" {
		t.Errorf("block anchor = %q", m.Anchor)
	}
}

func TestParseUsage(t *testing.T) {
	a := New("")
	cases := []struct {
		name          string
		data          string
		wantOK        bool
		prompt, compl int
		responseID    string
	}{
		{"non-streaming", `{"id":"msg_1","type":"message","usage":{"input_tokens":25,"output_tokens":250}}`, true, 25, 250, "msg_1"},
		{"message_start", `{"type":"message_start","message":{"id":"msg_2","usage":{"input_tokens":25,"output_tokens":1}}}`, true, 25, 1, "msg_2"},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":250}}`, true, 0, 250, ""},
		{"content delta (no usage)", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}`, false, 0, 0, ""},
		{"garbage", `event: ping`, false, 0, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, ok := a.ParseUsage([]byte(c.data))
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if u.PromptTokens != c.prompt || u.CompletionTokens != c.compl {
				t.Errorf("tokens = %d+%d, want %d+%d", u.PromptTokens, u.CompletionTokens, c.prompt, c.compl)
			}
			if u.ResponseID != c.responseID {
				t.Errorf("ResponseID = %q, want %q", u.ResponseID, c.responseID)
			}
			if u.HasReportedCost {
				t.Error("Anthropic never reports cost")
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
	if !bytes.Contains(body, []byte(`"type":"error"`)) ||
		!bytes.Contains(body, []byte("turnstile_blocked")) ||
		!bytes.Contains(body, []byte("loop_detected")) {
		t.Errorf("body missing markers: %s", body)
	}
}
