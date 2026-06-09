// Package proxy is Turnstile's data-plane: a transparent streaming reverse proxy
// that routes each request to a provider adapter, runs the synchronous
// enforcement gate, forwards the request, and streams the response back
// byte-faithfully — metering and self-instrumenting off the hot path.
package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"turnstile/internal/adapter"
	"turnstile/internal/enforce"
	"turnstile/internal/metrics"
	"turnstile/internal/session"
)

// Metering prices and records a completed call's usage against its session. The
// proxy depends on this interface; meter.Meter implements it.
type Metering interface {
	Observe(ident session.Identity, model string, u adapter.Usage)
}

// Enforcer is the circuit breaker. The proxy depends on this interface;
// enforce.Enforcer implements it.
type Enforcer interface {
	Pre(ident session.Identity, body []byte) enforce.Decision
	Killed(id string) bool
}

// Proxy routes requests through an adapter registry and forwards them.
type Proxy struct {
	reg        *adapter.Registry
	resolver   *session.Resolver
	metering   Metering
	enforcer   Enforcer
	failClosed bool
	client     *http.Client
	sink       metrics.Sink

	// MaxMeterBytes caps the metering buffer for non-streaming responses; the
	// client always gets the full body regardless. <=0 means the 8MiB default.
	// Set by main from config.
	MaxMeterBytes int64

	// Debug logs one line per proxied request. Set by main from config.
	Debug bool
}

const defaultMaxMeterBytes = 8 << 20

// New builds a Proxy over an adapter registry. A nil sink disables instrumentation;
// a nil resolver disables session resolution; a nil metering disables metering;
// a nil enforcer disables enforcement.
func New(reg *adapter.Registry, resolver *session.Resolver, metering Metering, enforcer Enforcer, failClosed bool, sink metrics.Sink) *Proxy {
	return &Proxy{
		reg:        reg,
		resolver:   resolver,
		metering:   metering,
		enforcer:   enforcer,
		failClosed: failClosed,
		// Default transport keeps connections alive (HTTP keep-alive) — the
		// connection-pooling rung of the latency ladder, for free.
		client: &http.Client{},
		sink:   sink,
	}
}

var (
	sseDataPrefix = []byte("data:")
	usageMarker   = []byte(`"usage"`)
)

// hop-by-hop headers are connection-specific and must not be forwarded.
var hopByHop = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true,
	"Proxy-Authorization": true, "Te": true, "Trailer": true,
	"Transfer-Encoding": true, "Upgrade": true,
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	// Read the inbound body so we can forward it (and so adapters may inspect it).
	// A genuinely unreadable client request is the client's failure, not a
	// Turnstile-added one — fail-open is about our logic, not inventing a body.
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "read request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ar := &adapter.Request{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Host:     r.Host,
		Header:   r.Header,
		Body:     body,
	}
	a := p.reg.Pick(ar)
	if a == nil {
		http.Error(w, "no upstream adapter matched request", http.StatusBadGateway)
		return
	}

	// Resolve the session synchronously — it feeds the enforcement gate, so by the
	// latency principle it's part of our hot-path overhead.
	var ident session.Identity
	if p.resolver != nil {
		ident = p.resolver.Resolve(r.Header, a.ExtractRequestMeta(ar))
	}

	// Pre-execution enforcement gate (synchronous). On a block we return a
	// provider-native error and never touch the upstream.
	if p.enforcer != nil {
		if d := p.preGate(ident, body); !d.Allow {
			status, ct, eb := a.NativeError(string(d.Reason))
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			w.Write(eb)
			p.recordBlocked(a.Name(), ident, string(d.Reason), t0)
			return
		}
	}

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, a.ForwardURL(ar), bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	copyHeaders(upReq.Header, r.Header) // faithful passthrough, incl. Authorization (never logged, Q9)

	t1 := time.Now() // pre-forward work done

	resp, err := p.client.Do(upReq)
	if err != nil {
		// Upstream unreachable fails the call exactly as a direct call would.
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)

	// Forward the response. Streaming (SSE) responses are teed line-by-line so a
	// token is never delayed; non-streaming JSON responses (e.g. a plain
	// chat/completions call without stream=true) are read whole, forwarded, then
	// parsed for usage from the complete body. Both paths meter.
	var t2, t3 time.Time // first upstream byte; first byte forwarded to client
	var usage adapter.Usage
	usageFound := false
	meterOn := p.metering != nil
	killable := p.enforcer != nil

	if isEventStream(resp.Header.Get("Content-Type")) {
		reader := bufio.NewReader(resp.Body)
		for {
			// Live gate: if the session is killed mid-stream, stop forwarding.
			if killable && p.enforcer.Killed(ident.ID) {
				log.Printf("enforce: aborting in-flight stream for killed session %s", ident.ID)
				break
			}
			line, rerr := reader.ReadBytes('\n')
			if len(line) > 0 {
				if t2.IsZero() {
					t2 = time.Now()
				}
				// Forward first (R1), so usage parsing never delays a token.
				w.Write(line)
				if flusher != nil {
					flusher.Flush()
				}
				if t3.IsZero() {
					t3 = time.Now()
				}
				// Then, only on candidate frames, extract usage from the data: frame.
				if meterOn && bytes.Contains(line, usageMarker) {
					if t := bytes.TrimSpace(line); bytes.HasPrefix(t, sseDataPrefix) {
						payload := bytes.TrimSpace(t[len(sseDataPrefix):])
						if u, ok := a.ParseUsage(payload); ok {
							usage, usageFound = u, true
						}
					}
				}
			}
			if rerr != nil {
				break // io.EOF is the normal end of stream
			}
		}
	} else {
		// Non-streaming: stream the body straight to the client while teeing a
		// SIZE-LIMITED copy for metering. The client always gets the full,
		// byte-faithful body (transparency, Q5); only the metering buffer is
		// bounded, so a giant response can't blow up Turnstile's memory (DoS).
		cap := p.MaxMeterBytes
		if cap <= 0 {
			cap = defaultMaxMeterBytes
		}
		var meterBuf bytes.Buffer
		src := io.Reader(resp.Body)
		if meterOn {
			src = io.TeeReader(resp.Body, &limitedWriter{w: &meterBuf, n: cap})
		}
		t2 = time.Now()
		io.Copy(w, src) // forward the ORIGINAL bytes untouched (incl. any gzip)
		if flusher != nil {
			flusher.Flush()
		}
		t3 = time.Now()
		if meterOn {
			// Providers gzip the JSON when the client sends Accept-Encoding: gzip
			// (OpenRouter does). Decompress a COPY for parsing; the client already
			// got the faithful original above. If the body exceeded the meter cap
			// the buffer is truncated JSON and ParseUsage simply fails — metering is
			// best-effort and never affects the response.
			parseBody := maybeDecompress(meterBuf.Bytes(), resp.Header.Get("Content-Encoding"))
			if u, ok := a.ParseUsage(bytes.TrimSpace(parseBody)); ok {
				usage, usageFound = u, true
			}
		}
	}
	end := time.Now()

	// Link this response to its session so a follow-up OpenAI Responses-API turn
	// that references it (previous_response_id) resolves to the same trajectory.
	// After the response is fully forwarded, so it never touches the latency gate.
	if usageFound && p.resolver != nil && usage.ResponseID != "" {
		p.resolver.LinkResponse(usage.ResponseID, ident.ID)
	}

	if p.Debug {
		log.Printf("[debug] %s %s -> %d | adapter=%s session=%s(%s) model=%q ce=%q usage=%v tokens=%d+%d",
			r.Method, r.URL.Path, resp.StatusCode, a.Name(), ident.ID, ident.Source,
			ident.Model, resp.Header.Get("Content-Encoding"), usageFound,
			usage.PromptTokens, usage.CompletionTokens)
	}

	p.record(a.Name(), ident, t0, t1, t2, t3, end, resp.StatusCode)

	// Meter off the hot path — the response is already complete.
	if usageFound {
		metering := p.metering
		go func() {
			defer func() { _ = recover() }()
			metering.Observe(ident, ident.Model, usage)
		}()
	}
}

// preGate runs the enforcer with a fail-open guard (Q3): if enforcement panics,
// the default is to allow the request through (fail-closed only if configured).
func (p *Proxy) preGate(ident session.Identity, body []byte) (d enforce.Decision) {
	d = enforce.Decision{Allow: true}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("enforce: pre-gate panic (%v) — failing %s", r, openClosed(p.failClosed))
			d = enforce.Decision{Allow: !p.failClosed}
		}
	}()
	return p.enforcer.Pre(ident, body)
}

func openClosed(failClosed bool) string {
	if failClosed {
		return "closed (blocking)"
	}
	return "open (allowing)"
}

// recordBlocked emits a Sample for a blocked request. For a block, the whole
// handler time is our overhead (we did the work; no upstream call happened).
func (p *Proxy) recordBlocked(adapterName string, ident session.Identity, reason string, t0 time.Time) {
	if p.sink == nil {
		return
	}
	ov := ms(time.Since(t0))
	s := metrics.Sample{
		Adapter:       adapterName,
		SessionID:     ident.ID,
		SessionSource: string(ident.Source),
		Blocked:       true,
		BlockReason:   reason,
		PreMs:         ov,
		OverheadMs:    ov,
		TotalMs:       ov,
		Ok:            false,
	}
	go func() {
		defer func() { _ = recover() }()
		p.sink.Record(s)
	}()
}

// record builds a Sample and dispatches it to the Sink asynchronously. It is
// recover-guarded and runs after the response is written, so neither a slow nor
// a panicking Sink can ever add latency to or break the request (fail-open).
func (p *Proxy) record(adapterName string, ident session.Identity, t0, t1, t2, t3, end time.Time, status int) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("metrics: record panic ignored: %v", rec)
		}
	}()
	if p.sink == nil {
		return
	}

	s := metrics.Sample{
		Adapter:       adapterName,
		SessionID:     ident.ID,
		SessionSource: string(ident.Source),
		PreMs:         ms(t1.Sub(t0)),
		TotalMs:       ms(end.Sub(t0)),
		Ok:            status >= 200 && status < 300,
	}
	if !t2.IsZero() {
		s.UpstreamWaitMs = ms(t2.Sub(t1))
	}
	if !t2.IsZero() && !t3.IsZero() {
		s.FirstByteForwardMs = ms(t3.Sub(t2))
	}
	s.OverheadMs = s.PreMs + s.FirstByteForwardMs

	sink := p.sink
	go func() {
		defer func() { _ = recover() }()
		sink.Record(s)
	}()
}

// limitedWriter buffers at most n bytes and silently drops the rest. It never
// returns an error or short write, so a TeeReader using it keeps streaming the
// full body to the client even once the metering cap is hit.
type limitedWriter struct {
	w io.Writer
	n int64 // remaining capacity
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		take := int64(len(p))
		if take > l.n {
			take = l.n
		}
		l.w.Write(p[:take])
		l.n -= take
	}
	return len(p), nil
}

func isEventStream(contentType string) bool {
	return strings.Contains(contentType, "text/event-stream")
}

// maybeDecompress returns a gzip-decoded copy of body when the provider
// gzip-encoded the response (for usage parsing only). On any error it returns
// the original body — metering is best-effort and never affects the response.
func maybeDecompress(body []byte, contentEncoding string) []byte {
	if !strings.Contains(contentEncoding, "gzip") {
		return body
	}
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return body
	}
	defer gz.Close()
	if dec, err := io.ReadAll(gz); err == nil {
		return dec
	}
	return body
}

func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		if hopByHop[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }
