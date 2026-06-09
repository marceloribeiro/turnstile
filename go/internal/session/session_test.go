package session

import (
	"net/http"
	"testing"

	"turnstile/internal/adapter"
)

func hdr(pairs ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(pairs); i += 2 {
		h.Set(pairs[i], pairs[i+1])
	}
	return h
}

func TestHeaderTakesPrecedence(t *testing.T) {
	r := NewResolver([]byte("salt"))
	id := r.Resolve(
		hdr("X-Turnstile-Session", "trace-42", "Authorization", "Bearer sk-abc"),
		adapter.RequestMeta{Anchor: "sys\x00hi", User: "u1", Model: "m"},
	)
	if id.Source != SourceHeader || id.ID != "sess:trace-42" {
		t.Fatalf("header should win: got %q / %q", id.Source, id.ID)
	}
	if id.User != "u1" || id.KeyFP == "" {
		t.Errorf("native fields must still be linked: user=%q keyfp=%q", id.User, id.KeyFP)
	}
}

func TestInferenceStableAcrossTurns(t *testing.T) {
	r := NewResolver([]byte("salt"))
	h := hdr("Authorization", "Bearer sk-abc")
	// Same root, growing history → same anchor → same session id.
	turn1 := r.Resolve(h, adapter.RequestMeta{Anchor: "sys\x00first question", User: "u1"})
	turn2 := r.Resolve(h, adapter.RequestMeta{Anchor: "sys\x00first question", User: "u1"})
	if turn1.Source != SourceInferred {
		t.Fatalf("expected inferred, got %q", turn1.Source)
	}
	if turn1.ID != turn2.ID {
		t.Errorf("same root must yield same session: %q vs %q", turn1.ID, turn2.ID)
	}
}

func TestInferenceDeCollidesByKeyAndUser(t *testing.T) {
	r := NewResolver([]byte("salt"))
	meta := adapter.RequestMeta{Anchor: "sys\x00identical opening", User: "u1"}
	a := r.Resolve(hdr("Authorization", "Bearer key-A"), meta)
	b := r.Resolve(hdr("Authorization", "Bearer key-B"), meta)
	if a.ID == b.ID {
		t.Errorf("identical anchor under different keys must not collide: %q", a.ID)
	}
	meta2 := adapter.RequestMeta{Anchor: "sys\x00identical opening", User: "u2"}
	c := r.Resolve(hdr("Authorization", "Bearer key-A"), meta2)
	if a.ID == c.ID {
		t.Errorf("different users must not collide: %q", a.ID)
	}
}

func TestUnattributedWhenNoAnchor(t *testing.T) {
	r := NewResolver([]byte("salt"))
	id := r.Resolve(hdr("Authorization", "Bearer sk-abc"), adapter.RequestMeta{})
	if id.Source != SourceUnattributed {
		t.Fatalf("expected unattributed, got %q", id.Source)
	}
	// Same key → same bucket.
	id2 := r.Resolve(hdr("Authorization", "Bearer sk-abc"), adapter.RequestMeta{})
	if id.ID != id2.ID {
		t.Errorf("unattributed bucket must be stable per key")
	}
}

func TestKeyNeverAppearsInFingerprint(t *testing.T) {
	r := NewResolver([]byte("salt"))
	id := r.Resolve(hdr("Authorization", "Bearer sk-super-secret-123"), adapter.RequestMeta{})
	if got := id.KeyFP; got == "" || len(got) != 32 {
		t.Fatalf("expected 128-bit hex fingerprint, got %q", got)
	}
	if containsRaw(id.ID, "sk-super-secret-123") || containsRaw(id.KeyFP, "secret") {
		t.Error("raw key leaked into identity")
	}
}

func containsRaw(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestSanitizeBoundsAndStrips(t *testing.T) {
	r := NewResolver([]byte("salt"))
	id := r.Resolve(hdr("X-Turnstile-Session", "ab\x00\x07cd"), adapter.RequestMeta{})
	if id.ID != "sess:abcd" {
		t.Errorf("control chars not stripped: %q", id.ID)
	}
}
