package gemini

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
		{"generateContent", "/v1beta/models/gemini-1.5-pro:generateContent", nil, true},
		{"streamGenerateContent", "/v1beta/models/gemini-1.5-flash:streamGenerateContent", nil, true},
		{"stable v1 surface", "/v1/models/gemini-pro:generateContent", nil, true},
		{"x-goog-api-key header", "/anything", http.Header{"X-Goog-Api-Key": {"k"}}, true},
		{"openai chat", "/v1/chat/completions", nil, false},
		{"anthropic messages", "/v1/messages", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := a.Match(&adapter.Request{Path: c.path, Header: c.header}); got != c.want {
				t.Errorf("Match = %v, want %v", got, c.want)
			}
		})
	}
}

func TestForwardURLPreservesKeyQuery(t *testing.T) {
	a := New("https://generativelanguage.googleapis.com/")
	got := a.ForwardURL(&adapter.Request{
		Path:     "/v1beta/models/gemini-1.5-pro:generateContent",
		RawQuery: "key=secret-123",
	})
	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent?key=secret-123"
	if got != want {
		t.Errorf("ForwardURL = %q, want %q", got, want)
	}
}

func TestModelFromPath(t *testing.T) {
	cases := map[string]string{
		"/v1beta/models/gemini-1.5-pro:generateContent":         "gemini-1.5-pro",
		"/v1beta/models/gemini-1.5-flash:streamGenerateContent": "gemini-1.5-flash",
		"/v1/models/gemini-pro:generateContent":                 "gemini-pro",
		"/v1beta/models/gemini-2.0-flash":                       "gemini-2.0-flash",
		"/no/model/here":                                        "",
	}
	for path, want := range cases {
		if got := modelFromPath(path); got != want {
			t.Errorf("modelFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestExtractRequestMeta(t *testing.T) {
	a := New("")
	body := []byte(`{"systemInstruction":{"parts":[{"text":"be brief"}]},
		"contents":[{"role":"user","parts":[{"text":"hi"},{"text":" there"}]},
		            {"role":"model","parts":[{"text":"hello"}]}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{
		Path: "/v1beta/models/gemini-1.5-pro:generateContent",
		Body: body,
	})
	if m.Model != "gemini-1.5-pro" {
		t.Errorf("model = %q", m.Model)
	}
	if m.Anchor != "be brief\x00hi there" {
		t.Errorf("anchor = %q", m.Anchor)
	}
	if m.User != "" || m.PreviousResponseID != "" {
		t.Errorf("gemini has no native user / response chain: user=%q prev=%q", m.User, m.PreviousResponseID)
	}
}

func TestExtractRequestMetaRolelessContent(t *testing.T) {
	a := New("")
	body := []byte(`{"contents":[{"parts":[{"text":"just a prompt"}]}]}`)
	m := a.ExtractRequestMeta(&adapter.Request{Path: "/v1beta/models/m:generateContent", Body: body})
	if m.Anchor != "\x00just a prompt" {
		t.Errorf("roleless content anchor = %q", m.Anchor)
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
		{"non-streaming", `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":34,"totalTokenCount":46}}`, true, 12, 34, ""},
		{"with responseId", `{"responseId":"abc","usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":7}}`, true, 5, 7, "abc"},
		{"content chunk no usage", `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`, false, 0, 0, ""},
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
				t.Error("Gemini never reports cost")
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
	if !bytes.Contains(body, []byte("turnstile_blocked")) ||
		!bytes.Contains(body, []byte("budget_exceeded")) ||
		!bytes.Contains(body, []byte(`"code":429`)) {
		t.Errorf("body missing markers: %s", body)
	}
}
