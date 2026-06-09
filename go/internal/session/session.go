// Package session resolves which conversational trajectory a request belongs to
// (Q1). Precedence:
//
//  1. X-Turnstile-Session header  → authoritative, exact (the precision upgrade)
//  2. anchor-hash inference        → hash(conversation root + key-fp + user)
//  3. per-key "unattributed" bucket → when nothing is anchorable (still safe-able)
//
// Native provider fields (user / metadata.user_id) are always linked for the
// session → user → app/key → org rollup ladder.
//
// Hashing note: we use HMAC-SHA256 (stdlib, salted, non-reversible) for both the
// content-opaque session key and the API-key fingerprint (Q9 — raw keys are never
// stored). The build spec floated xxhash for hot-path speed; SHA-256 over a
// few-KB anchor is single-digit microseconds, well within budget, and the salt
// gives us non-reversibility for free. Swap the non-secret anchor hash to xxhash
// later only if profiling demands it.
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"turnstile/internal/adapter"
)

// Source records how a session id was determined.
type Source string

const (
	SourceHeader       Source = "header"
	SourceInferred     Source = "inferred"
	SourceUnattributed Source = "unattributed"
	// SourceLinked marks a turn resolved by following an OpenAI Responses-API
	// previous_response_id back to the session that produced it.
	SourceLinked Source = "linked"
)

// maxLinks bounds the response_id → session map so a long-lived process can't grow
// it without limit; an evicted link just degrades that chain to anchor inference.
const maxLinks = 100_000

// Identity is the resolved session plus its linked rollup attributes.
type Identity struct {
	ID     string // the enforcement key (prefixed by source)
	Source Source
	User   string // linked native user (OpenAI `user` / Anthropic metadata.user_id)
	KeyFP  string // salted fingerprint of the API key — never the raw key (Q9)
	Model  string
}

// Resolver turns a request's headers + extracted meta into an Identity.
type Resolver struct {
	salt []byte

	linkMu sync.RWMutex
	links  map[string]string // response id → session id (Responses-API chain)
}

// NewResolver builds a resolver. The salt feeds the HMAC for key fingerprints and
// session hashes; a stable salt across restarts keeps fingerprints comparable.
func NewResolver(salt []byte) *Resolver {
	return &Resolver{salt: salt, links: make(map[string]string)}
}

// LinkResponse records that a provider response id belongs to a session, so a
// later turn referencing it via previous_response_id resolves to the same
// trajectory. Safe for concurrent use; bounded in size.
func (r *Resolver) LinkResponse(responseID, sessionID string) {
	if responseID == "" || sessionID == "" {
		return
	}
	r.linkMu.Lock()
	defer r.linkMu.Unlock()
	if len(r.links) >= maxLinks {
		// Best-effort eviction: drop a batch (map order is randomized) to bound
		// memory. Losing a link only degrades that chain to anchor inference.
		dropped := 0
		for k := range r.links {
			delete(r.links, k)
			if dropped++; dropped >= maxLinks/2 {
				break
			}
		}
	}
	r.links[responseID] = sessionID
}

func (r *Resolver) linkedSession(responseID string) (string, bool) {
	r.linkMu.RLock()
	defer r.linkMu.RUnlock()
	sid, ok := r.links[responseID]
	return sid, ok
}

// Resolve applies the Q1 precedence.
func (r *Resolver) Resolve(h http.Header, meta adapter.RequestMeta) Identity {
	keyFP := r.fingerprint(bearer(h))
	id := Identity{User: meta.User, KeyFP: keyFP, Model: meta.Model}

	// 1. Explicit header — authoritative, no guessing.
	if s := sanitize(h.Get("X-Turnstile-Session")); s != "" {
		id.ID = "sess:" + s
		id.Source = SourceHeader
		return id
	}

	// 2. OpenAI Responses-API chain: a turn that references a prior response id
	//    joins that response's session (recorded via LinkResponse on completion).
	if meta.PreviousResponseID != "" {
		if sid, ok := r.linkedSession(meta.PreviousResponseID); ok {
			id.ID = sid
			id.Source = SourceLinked
			return id
		}
	}

	// 3. Anchor-hash inference. The real key combines the conversation root with
	//    the key fingerprint and the user to de-collide identical openings across
	//    separate runs.
	if meta.Anchor != "" {
		id.ID = "infer:" + r.fingerprint(meta.Anchor+"\x00"+keyFP+"\x00"+meta.User)
		id.Source = SourceInferred
		return id
	}

	// 4. Nothing anchorable → per-key unattributed bucket (still circuit-breakable
	//    at coarse grain).
	id.ID = "unattributed:" + keyFP
	id.Source = SourceUnattributed
	return id
}

// fingerprint is a salted, non-reversible, content-opaque 128-bit hex digest.
func (r *Resolver) fingerprint(s string) string {
	mac := hmac.New(sha256.New, r.salt)
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// bearer extracts the API token from the Authorization header (never stored raw).
func bearer(h http.Header) string {
	return strings.TrimSpace(strings.TrimPrefix(h.Get("Authorization"), "Bearer "))
}

// sanitize enforces the Q1 ID contract: opaque, bounded, no control characters.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.Map(func(rn rune) rune {
		if rn < 0x20 {
			return -1
		}
		return rn
	}, s)
}
