// Package control is Turnstile's control plane: a separate, auth-gated HTTP
// listener serving dashboard data and commands. It is distinct from the data
// plane (the proxy) — different port, different security posture (Q10) — and is
// the API the Next.js dashboard (M7), Slack alerts, and a future CLI consume.
//
// Endpoints (versioned):
//
//	GET  /v1/healthz                  liveness (unauthenticated)
//	GET  /v1/sessions                 list sessions (spend, blocks, prevented)
//	GET  /v1/sessions/{id}            one session
//	POST /v1/sessions/{id}/kill       manual kill (reaches the M5 enforcer)
//	POST /v1/sessions/{id}/unkill     clear a kill
//	GET  /v1/summary                  org rollup totals
//	GET  /v1/events                   live event stream (SSE)
package control

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"turnstile/internal/events"
	"turnstile/internal/meter"
)

// Killer is the enforcer surface the control plane needs (enforce.Enforcer).
type Killer interface {
	Kill(id string)
	Unkill(id string)
	Killed(id string) bool
}

// Server holds the control plane's dependencies.
type Server struct {
	store  meter.Store
	killer Killer
	broker *events.Broker
	token  string // bearer token; empty disables auth (rely on localhost binding)
	origin string // CORS allowed origin for the hosted dashboard
}

// New builds a control-plane server.
func New(store meter.Store, killer Killer, broker *events.Broker, token, origin string) *Server {
	if origin == "" {
		origin = "http://localhost:3000"
	}
	return &Server{store: store, killer: killer, broker: broker, token: token, origin: origin}
}

// Handler returns the routed, CORS- and auth-wrapped handler (Go 1.22 routing).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", s.health)
	mux.HandleFunc("GET /v1/sessions", s.listSessions)
	mux.HandleFunc("GET /v1/sessions/{id}", s.getSession)
	mux.HandleFunc("POST /v1/sessions/{id}/kill", s.kill)
	mux.HandleFunc("POST /v1/sessions/{id}/unkill", s.unkill)
	mux.HandleFunc("GET /v1/summary", s.summary)
	mux.HandleFunc("GET /v1/events", s.events)
	return s.cors(s.auth(mux))
}

// --- middleware ----------------------------------------------------------

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.URL.Path == "/v1/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		want := "Bearer " + s.token
		got := r.Header.Get("Authorization")
		if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- handlers ------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Sessions())
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) kill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.killer.Kill(id)
	s.broker.Publish(events.Event{Type: "killed", SessionID: id, TS: time.Now().UnixMilli()})
	writeJSON(w, http.StatusOK, map[string]any{"killed": id})
}

func (s *Server) unkill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.killer.Unkill(id)
	writeJSON(w, http.StatusOK, map[string]any{"unkilled": id})
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	var totalCost, prevented float64
	var blocks, reqs int64
	sessions := s.store.Sessions()
	for _, x := range sessions {
		totalCost += x.Cost
		prevented += x.Prevented
		blocks += x.Blocks
		reqs += x.Requests
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions":          len(sessions),
		"requests":          reqs,
		"total_cost":        totalCost,
		"blocks":            blocks,
		"dollars_prevented": prevented, // the Q13 hero metric
	})
}

// events streams live events over SSE until the client disconnects.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.broker.Subscribe()
	defer s.broker.Unsubscribe(ch)

	w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			b, _ := json.Marshal(ev)
			w.Write([]byte("data: "))
			w.Write(b)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
