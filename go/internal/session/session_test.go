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

func TestPreviousResponseIDLinksToSession(t *testing.T) {
	r := NewResolver([]byte("salt"))
	h := hdr("Authorization", "Bearer sk-abc")

	// Turn 1: no previous id → inferred from the anchor. The proxy records the
	// response id once the call completes.
	turn1 := r.Resolve(h, adapter.RequestMeta{Anchor: "sys\x00start", User: "u1"})
	if turn1.Source != SourceInferred {
		t.Fatalf("turn1 expected inferred, got %q", turn1.Source)
	}
	r.LinkResponse("resp_1", turn1.ID)

	// Turn 2 references resp_1 with a *different* anchor (just the new message) yet
	// still joins turn1's session via the link.
	turn2 := r.Resolve(h, adapter.RequestMeta{Anchor: "sys\x00follow up", User: "u1", PreviousResponseID: "resp_1"})
	if turn2.Source != SourceLinked {
		t.Fatalf("turn2 expected linked, got %q", turn2.Source)
	}
	if turn2.ID != turn1.ID {
		t.Errorf("linked turn must join turn1's session: %q vs %q", turn2.ID, turn1.ID)
	}

	// Turn 3 chains off turn2's response → still one session.
	r.LinkResponse("resp_2", turn2.ID)
	turn3 := r.Resolve(h, adapter.RequestMeta{PreviousResponseID: "resp_2"})
	if turn3.ID != turn1.ID {
		t.Errorf("chain must stay on one session: %q vs %q", turn3.ID, turn1.ID)
	}
}

func TestUnknownPreviousResponseIDFallsBack(t *testing.T) {
	r := NewResolver([]byte("salt"))
	h := hdr("Authorization", "Bearer sk-abc")

	// An unknown (e.g. evicted) previous id falls through to anchor inference.
	id := r.Resolve(h, adapter.RequestMeta{Anchor: "sys\x00q", PreviousResponseID: "resp_missing"})
	if id.Source != SourceInferred {
		t.Errorf("unknown previous id should fall back to inference, got %q", id.Source)
	}

	// An explicit session header still wins over a link.
	r.LinkResponse("resp_h", "infer:whatever")
	hid := r.Resolve(
		hdr("X-Turnstile-Session", "explicit", "Authorization", "Bearer sk-abc"),
		adapter.RequestMeta{PreviousResponseID: "resp_h"},
	)
	if hid.Source != SourceHeader || hid.ID != "sess:explicit" {
		t.Errorf("explicit header must win over link: %q/%q", hid.Source, hid.ID)
	}
}

func TestKeyFingerprintCoversNonBearerAuth(t *testing.T) {
	r := NewResolver([]byte("salt"))
	meta := adapter.RequestMeta{Anchor: "sys\x00identical opening"}
	// Each provider's auth style must produce a non-empty fingerprint and
	// de-collide an identical anchor: Bearer (OpenAI), x-api-key (Anthropic),
	// x-goog-api-key (Gemini).
	bearer := r.Resolve(hdr("Authorization", "Bearer key-1"), meta)
	xapi := r.Resolve(hdr("X-Api-Key", "key-2"), meta)
	xgoog := r.Resolve(hdr("X-Goog-Api-Key", "key-3"), meta)

	for _, id := range []Identity{bearer, xapi, xgoog} {
		if id.KeyFP == "" {
			t.Fatalf("key fingerprint must be non-empty across auth styles")
		}
	}
	ids := map[string]bool{bearer.ID: true, xapi.ID: true, xgoog.ID: true}
	if len(ids) != 3 {
		t.Errorf("distinct keys must de-collide identical anchors, got ids %v", ids)
	}
}

func TestSanitizeBoundsAndStrips(t *testing.T) {
	r := NewResolver([]byte("salt"))
	id := r.Resolve(hdr("X-Turnstile-Session", "ab\x00\x07cd"), adapter.RequestMeta{})
	if id.ID != "sess:abcd" {
		t.Errorf("control chars not stripped: %q", id.ID)
	}
}
