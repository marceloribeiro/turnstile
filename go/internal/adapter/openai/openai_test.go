package openai

import (
	"bytes"
	"testing"

	"turnstile/internal/adapter"
)

func TestMatch(t *testing.T) {
	a := New("")
	cases := []struct {
		path string
		want bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/responses", true},
		{"/v1/embeddings", true},
		{"/api/v1/chat/completions", false}, // OpenRouter's surface
		{"/health", false},
		{"/", false},
	}
	for _, c := range cases {
		if got := a.Match(&adapter.Request{Path: c.path}); got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestForwardURL(t *testing.T) {
	a := New("https://api.openai.com/")
	if got := a.ForwardURL(&adapter.Request{Path: "/v1/responses"}); got != "https://api.openai.com/v1/responses" {
		t.Errorf("ForwardURL = %q", got)
	}
	got := a.ForwardURL(&adapter.Request{Path: "/v1/chat/completions", RawQuery: "x=1"})
	if got != "https://api.openai.com/v1/chat/completions?x=1" {
		t.Errorf("ForwardURL with query = %q", got)
	}
}

func TestExtractRequestMetaChat(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"gpt-4o","user":"u1","messages":[
		{"role":"system","content":"be terse"},
		{"role":"user","content":"hello"},
		{"role":"user","content":"second"}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/chat/completions", Body: body})
	if m.Model != "gpt-4o" || m.User != "u1" {
		t.Fatalf("model/user = %q/%q", m.Model, m.User)
	}
	if m.Anchor != "be terse\x00hello" {
		t.Errorf("anchor = %q", m.Anchor)
	}
	if m.PreviousResponseID != "" {
		t.Errorf("chat should not set PreviousResponseID, got %q", m.PreviousResponseID)
	}
}

func TestExtractRequestMetaChatMultimodal(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"gpt-4o","messages":[
		{"role":"user","content":[{"type":"text","text":"part-a"},{"type":"text","text":"part-b"}]}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/chat/completions", Body: body})
	if m.Anchor != "\x00part-apart-b" {
		t.Errorf("multimodal anchor = %q", m.Anchor)
	}
}

func TestExtractRequestMetaResponsesString(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"gpt-4o","user":"u2","instructions":"be brief","input":"hello","previous_response_id":"resp_1"}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/responses", Body: body})
	if m.Model != "gpt-4o" || m.User != "u2" {
		t.Fatalf("model/user = %q/%q", m.Model, m.User)
	}
	if m.Anchor != "be brief\x00hello" {
		t.Errorf("anchor = %q", m.Anchor)
	}
	if m.PreviousResponseID != "resp_1" {
		t.Errorf("previous_response_id = %q", m.PreviousResponseID)
	}
}

func TestExtractRequestMetaResponsesArray(t *testing.T) {
	a := New("")
	body := []byte(`{"model":"gpt-4o","metadata":{"user_id":"meta-u"},"input":[
		{"role":"user","content":[{"type":"input_text","text":"hey there"}]}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1/responses", Body: body})
	if m.User != "meta-u" {
		t.Errorf("metadata user fallback = %q", m.User)
	}
	if m.Anchor != "\x00hey there" {
		t.Errorf("array input anchor = %q", m.Anchor)
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
		{"done sentinel", "[DONE]", false, 0, 0, ""},
		{"no usage", `{"id":"x","choices":[]}`, false, 0, 0, ""},
		{"null usage chunk", `{"id":"chatcmpl-1","choices":[],"usage":null}`, false, 0, 0, ""},
		{"chat usage", `{"id":"chatcmpl-9","usage":{"prompt_tokens":30,"completion_tokens":24}}`, true, 30, 24, "chatcmpl-9"},
		{"responses non-streaming", `{"id":"resp_123","object":"response","usage":{"input_tokens":100,"output_tokens":40}}`, true, 100, 40, "resp_123"},
		{"responses completed event", `{"type":"response.completed","response":{"id":"resp_xyz","usage":{"input_tokens":12,"output_tokens":8}}}`, true, 12, 8, "resp_xyz"},
		{"garbage", `not json`, false, 0, 0, ""},
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
				t.Error("OpenAI never reports cost")
			}
		})
	}
}

func TestNativeError(t *testing.T) {
	a := New("")
	status, ct, body := a.NativeError("budget_exceeded")
	if status != 429 {
		t.Errorf("status = %d, want 429", status)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	if !bytes.Contains(body, []byte("turnstile_blocked")) || !bytes.Contains(body, []byte("budget_exceeded")) {
		t.Errorf("body missing markers: %s", body)
	}
}
