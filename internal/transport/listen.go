// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/log"

	"go.uber.org/zap"
)

// Bounds on what an unauthenticated peer can cost this process. A listener the
// network can reach answers 403 and 401 to strangers, and those refusals must
// themselves be cheap.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 64 << 10

	// maxLoggedHeaderLen caps how much of a rejected header reaches the log.
	// The value is attacker-controlled on exactly the requests this server
	// refuses, so logging it whole lets a stranger write to the operator's disk
	// for free.
	maxLoggedHeaderLen = 128

	// maxLoggedHeaderLen bounds the SIZE of one refusal line; these bound the
	// RATE. Without them an unauthenticated peer still writes to the operator's
	// disk without limit, just in more lines rather than longer ones. The burst
	// is generous enough that a human debugging a genuinely misconfigured proxy
	// sees the refusals immediately, and the suppressed count means a flood is
	// still visible as a flood.
	refusalLogBurst  = 10
	refusalLogWindow = time.Second
)

// addrIsLoopbackOnly reports whether every address this listener can be reached
// on is a loopback address.
//
// It reads the address actually bound, never the configured string: a
// configured value can resolve differently than expected. The unspecified
// address ("::" or "0.0.0.0", which is what a bare ":port" produces) is
// reachable from the network and is therefore not loopback-only.
//
// This decides only whether an allow-list is REQUIRED and whether loopback
// names are implicitly acceptable. It deliberately decides nothing about
// credentials — see newFactory for why.
func addrIsLoopbackOnly(a net.Addr) bool {
	tcpAddr, ok := a.(*net.TCPAddr)
	if !ok {
		// Unknown address shape: assume it is reachable. Failing towards the
		// stricter policy is the safe direction.
		return false
	}
	addr, ok := netip.AddrFromSlice(tcpAddr.IP)
	if !ok {
		return false
	}
	return addr.IsLoopback()
}

// hostPolicy decides which Host values this listener answers to.
type hostPolicy struct {
	// allowed holds the operator's declared names, lower-cased. An entry may
	// carry a port ("mcp.example.org:8443"), in which case it must match
	// exactly, or omit one, in which case any port matches — the port already
	// had to be ours for the request to arrive.
	allowed []string
	// loopbackOnly means the listener cannot be reached from the network, so
	// loopback names are acceptable whatever else was declared. Declaring a
	// name for a proxy must not stop the operator's own machine from
	// connecting directly.
	loopbackOnly bool
}

func (p hostPolicy) permits(value string) bool {
	return config.HostAllowed(p.allowed, p.loopbackOnly, value)
}

// originPolicy decides which Origin values this listener accepts.
//
// Host and Origin are deliberately NOT checked against the same list. They mean
// different things: a Host names this server, an Origin names the web page
// making the request. Reusing one list for both silently accepts every other
// service sharing a declared hostname on any port, and every plaintext-HTTP
// origin on it — which is the cross-site case the Origin check exists to stop.
type originPolicy struct {
	// allowed holds full origins, normalised by config.NormalizeOrigin. Empty means no
	// browser origin is accepted, which is correct for a server whose clients
	// are not browsers.
	allowed []string
}

func (p originPolicy) permits(value string) bool {
	normalised, ok := config.NormalizeOrigin(value)
	if !ok {
		return false
	}
	for _, entry := range p.allowed {
		if entry == normalised {
			return true
		}
	}
	return false
}

// truncateForLog bounds an attacker-controlled value before it reaches the log.
//
// The cut is made on a rune boundary. Slicing bytes can land inside a multi-byte
// sequence and emit a lone replacement character, which is cosmetic in a log
// line but makes the logged value differ from what the peer actually sent --
// and the reason to log it at all is to show what was sent.
func truncateForLog(v string) string {
	if len(v) <= maxLoggedHeaderLen {
		return v
	}
	cut := maxLoggedHeaderLen
	for cut > 0 && !utf8.RuneStart(v[cut]) {
		cut--
	}
	return v[:cut] + "…(truncated)"
}

// refusalLimiter bounds how many refusal lines reach the log per window and
// counts what it dropped, so a flood is reported as a flood rather than
// written out one line at a time.
type refusalLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	logged      int
	suppressed  int
}

// admit reports whether this refusal may be logged, and how many were
// suppressed in the window that just closed (0 when none were, or when this is
// not the first admission of a new window).
func (l *refusalLimiter) admit(now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	dropped := 0
	if now.Sub(l.windowStart) >= refusalLogWindow {
		dropped, l.suppressed = l.suppressed, 0
		l.windowStart, l.logged = now, 0
	}
	if l.logged < refusalLogBurst {
		l.logged++
		return true, dropped
	}
	l.suppressed++
	return false, dropped
}

// write emits one refusal line through emit unless the rate bound says otherwise,
// and reports the lines it suppressed in the window that just closed.
//
// The limiter is owned by the listener that serves it: one per listener, created
// with its configuration, and a resource-server handler served by that listener
// writes through the same instance. The bound applies to what that listener
// writes to the operator's disk. Two listeners in one process each get their own
// budget.
func (l *refusalLimiter) write(emit func(string, ...zap.Field), msg string, fields ...zap.Field) {
	ok, dropped := l.admit(time.Now())
	if dropped > 0 {
		log.Warn("Suppressed refusal log lines over the preceding window",
			log.IntField("suppressed", dropped),
		)
	}
	if ok {
		emit(msg, fields...)
	}
}

// guardRequests is the middleware required by the Model Context Protocol's
// Streamable HTTP transport: "Servers MUST validate the Origin header on all
// incoming connections to prevent DNS rebinding attacks", alongside "Servers
// SHOULD implement proper authentication for all connections".
//
// It applies three checks, in order of how cheaply they refuse:
//
//   - Host, against DNS rebinding. An attacker's page rebinds its own name to
//     127.0.0.1; the browser then treats the local server as same-origin and no
//     cross-origin check runs. The Host header still carries the attacker's
//     name, which is what catches it.
//   - Origin, against ordinary cross-site request forgery. A page at any origin
//     can address a loopback listener directly with a correct Host header. Only
//     the Origin header distinguishes that from a real client, and the library
//     underneath does not look at it. A request with no Origin header at all is
//     allowed: a non-browser client — which is what an MCP client normally is —
//     sends none, and the Host check still applies to it.
//   - Authorization, when this server does not fall back to its own credential.
//     Refusing at the door rather than at the forge client means a caller that
//     presents no credential cannot open a session, enumerate the tool
//     catalogue, or hold an event stream open either. The check is for presence
//     and shape only: any string under a recognised scheme passes, and it is the
//     forge that rejects an invalid one, on the first call that reaches it. A
//     call that never reaches the forge is not protected by this check.
//
// The Authorization check is the authentication port's refusal, passed in as
// authorize; nil means the door does not look at Authorization.
//
// The body ceiling, maxBody, is checked before all three. A request whose
// declared length is over it is refused without reading anything; a chunked
// one is wrapped so that the first read past the ceiling is refused with 413.
// Nothing downstream, authentication included, sees a body over the ceiling.
func guardRequests(next http.Handler, hosts hostPolicy, origins originPolicy, authorize func(*http.Request) error, refusals *refusalLimiter, maxBody int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBody {
			refusals.write(log.Warn, "Rejected request body over the size limit",
				log.StringField("declared_length", truncateForLog(strconv.FormatInt(r.ContentLength, 10))),
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			refuseTooLarge(w, maxBody)
			return
		}
		if r.Body != nil && r.Body != http.NoBody {
			body := &bodyLimit{ReadCloser: http.MaxBytesReader(w, r.Body, maxBody)}
			r.Body = body
			w = &bodyLimitWriter{ResponseWriter: w, body: body, maxBody: maxBody, refusals: refusals, path: r.URL.Path}
		}
		if !hosts.permits(r.Host) {
			refusals.write(log.Warn, "Rejected request with unacceptable Host header",
				log.StringField("host", truncateForLog(r.Host)),
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			http.Error(w, "Forbidden: unacceptable Host header", http.StatusForbidden)
			return
		}
		// Read the header map directly: a present-but-empty Origin must not be
		// mistaken for an absent one, and Header.Get cannot tell them apart.
		if origin, present := r.Header["Origin"]; present {
			value := ""
			if len(origin) > 0 {
				value = origin[0]
			}
			if !origins.permits(value) {
				refusals.write(log.Warn, "Rejected request with unacceptable Origin header",
					log.StringField("origin", truncateForLog(value)),
					log.StringField("path", truncateForLog(r.URL.Path)),
				)
				http.Error(w, "Forbidden: unacceptable Origin header", http.StatusForbidden)
				return
			}
		}
		if authorize != nil && authorize(r) != nil {
			refusals.write(log.Warn, "Rejected request with no usable Authorization header",
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			w.Header().Set("WWW-Authenticate", `Bearer realm="forgejo-mcp"`)
			http.Error(w, "Unauthorized: this server requires a per-request token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refuseTooLarge answers 413 with the ceiling in the body, so a client can
// reduce its payload. It closes the connection, since the unread body would
// otherwise be read off the wire to keep it alive.
func refuseTooLarge(w http.ResponseWriter, maxBody int64) {
	w.Header().Set("Connection", "close")
	http.Error(w, fmt.Sprintf("Request Entity Too Large: the request body exceeds the %d-byte limit", maxBody), http.StatusRequestEntityTooLarge)
}

// bodyLimit records whether the reader gave up because the body went over the
// ceiling. A chunked body has no length to check up front, so only the read can
// tell.
type bodyLimit struct {
	io.ReadCloser
	exceeded bool
}

func (b *bodyLimit) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		b.exceeded = true
	}
	return n, err
}

// bodyLimitWriter turns the handler's response into a 413 when the body it
// read went over the ceiling. The handler sees the read fail and answers with
// its own status, which would otherwise be what the client gets.
type bodyLimitWriter struct {
	http.ResponseWriter
	body     *bodyLimit
	maxBody  int64
	refusals *refusalLimiter
	path     string
	wrote    bool
}

// header commits the status for the first write, substituting the 413 when the
// body was over the ceiling.
func (w *bodyLimitWriter) header(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	if !w.body.exceeded {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.refusals.write(log.Warn, "Rejected request body over the size limit",
		log.StringField("path", truncateForLog(w.path)),
	)
	refuseTooLarge(w.ResponseWriter, w.maxBody)
}

func (w *bodyLimitWriter) WriteHeader(code int) { w.header(code) }

func (w *bodyLimitWriter) Write(p []byte) (int, error) {
	w.header(http.StatusOK)
	if w.body.exceeded {
		// The refusal is already written; the handler's output is dropped.
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

// Flush passes through so that streamed responses still flush. A flush before
// any header commits the status the same way a write does.
func (w *bodyLimitWriter) Flush() {
	w.header(http.StatusOK)
	if f, ok := w.ResponseWriter.(http.Flusher); ok && !w.body.exceeded {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *bodyLimitWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// transportConfig is the decision this change exists to make explicit, resolved
// once from configuration before anything is bound.
type transportConfig struct {
	// listenHosts are the addresses to bind.
	//
	// A loopback NAME resolves to both loopback families, because binding only
	// 127.0.0.1 leaves a client that resolves "localhost" to ::1 unable to
	// connect — and connecting to "localhost" is what this project's own
	// documentation tells clients to do.
	//
	// A loopback ADDRESS binds exactly itself. An operator who writes an address
	// asked for that address, and that is the way out when this machine's other
	// loopback family fails for a reason isFamilyUnavailable cannot recognise.
	listenHosts []string
	// loopbackOnly is the configured intent; the bound listeners are checked
	// against it afterwards rather than trusted from here.
	loopbackOnly bool
	hosts        hostPolicy
	origins      originPolicy
	// requireAuth is true unless the operator explicitly opted back in to the
	// old behaviour. It is never inferred from the bind address.
	requireAuth bool
	// resourceServer is true in resource-server mode, where the handler
	// authenticates its protected route itself.
	resourceServer bool
	// app is the Config the authentication port reads at request time.
	app config.Config
	// refusals is this listener's refusal-log budget. It is created with the
	// configuration, so each listener starts with its own.
	refusals *refusalLimiter
	// maxBody is the request body ceiling. Validate refuses a non-positive
	// configured value at startup; a zero value here means a Config that never
	// went through Validate, and takes the default.
	maxBody int64
}

// resolveTransportConfig derives the listener decisions from a Config that
// config.Validate has accepted: Validate refuses an empty bind address, a
// network-reachable bind with no allowed hosts, and an unusable origin, so none
// of those can reach here.
func resolveTransportConfig(cfg config.Config) transportConfig {
	l := cfg.Listen
	tc := transportConfig{
		requireAuth:    !l.AllowOperatorTokenFallback,
		resourceServer: cfg.ResourceServerMode(),
		app:            cfg,
		refusals:       &refusalLimiter{},
		maxBody:        l.MaxRequestBodyBytes,
	}
	if tc.maxBody == 0 {
		tc.maxBody = config.DefaultMaxRequestBodyBytes
	}

	host := strings.TrimSpace(l.Host)
	tc.loopbackOnly = config.IsLoopbackHostname(host)
	switch {
	case !tc.loopbackOnly, config.IsIPLiteral(host):
		tc.listenHosts = []string{strings.Trim(host, "[]")}
	default:
		tc.listenHosts = []string{"127.0.0.1", "::1"}
	}

	tc.hosts = hostPolicy{allowed: config.NormalizeList(l.AllowedHosts), loopbackOnly: tc.loopbackOnly}
	for _, raw := range config.NormalizeList(l.AllowedOrigins) {
		if normalised, ok := config.NormalizeOrigin(raw); ok {
			tc.origins.allowed = append(tc.origins.allowed, normalised)
		}
	}
	return tc
}

// newMCPHTTPServer wraps handler in the policy stack for an already-resolved
// configuration.
func newMCPHTTPServer(handler http.Handler, cfg transportConfig) *http.Server {
	// In resource-server mode the handler authenticates its protected route
	// itself, and its public documents must be reachable without a token, so the
	// door does not look at Authorization. The credential fallback stays refused
	// either way: cfg.requireAuth still drives the door; the fallback itself is the port's (see withRequestToken).
	var authorize func(*http.Request) error
	if cfg.requireAuth && !cfg.resourceServer {
		authorize = func(r *http.Request) error {
			_, err := authenticatePassthrough(r, cfg.app)
			return err
		}
	}
	return &http.Server{
		Handler:           guardRequests(handler, cfg.hosts, cfg.origins, authorize, cfg.refusals, cfg.maxBody),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// WriteTimeout is deliberately ABSENT, and must stay absent. Go extends
		// that deadline only when a new request header is read; it is never
		// extended during a streaming response. Both transports served here are
		// stream-based -- SSE holds /sse open indefinitely, and Streamable HTTP
		// holds its GET channel open for server-to-client notifications -- so
		// any WriteTimeout is an absolute cap on the life of every stream, not a
		// bound on a slow write. ReadHeaderTimeout and IdleTimeout carry the
		// cheap-refusal property instead: a stranger's handshake is bounded, and
		// a connection that stops making progress is reaped.
		// TestEventStreamOutlivesAWriteDeadline fails if this is reintroduced.
		// Go replaces srv.Handler entirely for "OPTIONS *" unless this is set,
		// which would route that one request shape around the whole policy.
		DisableGeneralOptionsHandler: true,
	}
}

// listenFunc has net.Listen's signature. bindListeners takes one so the tests
// can make a chosen loopback family fail in a chosen way.
type listenFunc func(network, address string) (net.Listener, error)

// isFamilyUnavailable reports whether a bind error means this machine cannot use
// the address family at all — IPv6 disabled, or absent from the kernel — as
// opposed to the address being taken or forbidden.
func isFamilyUnavailable(err error) bool {
	return errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.EAFNOSUPPORT)
}

// bindListeners binds every address in cfg.listenHosts on port.
//
// On a loopback configuration, a family this machine cannot use is skipped,
// whichever family it is, so a host with IPv6 disabled still starts. Every other
// bind failure refuses to start. That includes an address already in use, on
// either family: starting on the other family alone would leave "localhost"
// resolving, for some clients, to whatever process holds the port — and those
// clients would send that process their Authorization header.
func bindListeners(transportName string, cfg transportConfig, port int, listen listenFunc) ([]net.Listener, error) {
	var listeners []net.Listener
	var skipped []error
	closeAll := func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}
	for _, host := range cfg.listenHosts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := listen("tcp", addr)
		if err != nil {
			if cfg.loopbackOnly && isFamilyUnavailable(err) {
				log.Info("Loopback address family unavailable on this machine, continuing without it",
					log.StringField("address", addr),
					log.ErrorField(err),
				)
				skipped = append(skipped, err)
				continue
			}
			closeAll()
			log.Error("Failed to bind listener",
				log.StringField("transport", transportName),
				log.StringField("address", addr),
				log.ErrorField(err),
			)
			return nil, fmt.Errorf("failed to bind %s listener on %s: %w%s",
				transportName, addr, err, oneFamilyHint(cfg))
		}
		if cfg.loopbackOnly && !addrIsLoopbackOnly(ln.Addr()) {
			// The configuration said loopback and the kernel disagreed. Trust
			// the listener, not the string.
			closeAll()
			_ = ln.Close()
			return nil, fmt.Errorf(
				"refusing to start: %s transport asked for loopback but bound %s",
				transportName, ln.Addr())
		}
		listeners = append(listeners, ln)
	}
	if len(listeners) == 0 {
		if why := errors.Join(skipped...); why != nil {
			// Without the errnos, a machine with no usable loopback at all is
			// indistinguishable from one whose families were never tried.
			return nil, fmt.Errorf("failed to bind any %s listener: %w", transportName, why)
		}
		return nil, fmt.Errorf("failed to bind any %s listener", transportName)
	}
	return listeners, nil
}

// oneFamilyHint names the way out of a refusal the operator did not configure
// directly. A loopback NAME expands to both families, so the failing address can
// be one the operator never wrote down; saying which flag binds a single family
// keeps that from being a source-reading exercise.
func oneFamilyHint(cfg transportConfig) string {
	if len(cfg.listenHosts) < 2 {
		return ""
	}
	return "; a loopback host name binds both 127.0.0.1 and ::1, " +
		"so pass -host 127.0.0.1 or -host ::1 (or FORGEJO_MCP_HOST) to bind one family only"
}

// serveMCPOverHTTP binds the listeners for a network transport and serves
// handler on all of them. cfg is the listener's resolved configuration. The
// caller resolves it once, so a handler built before this call can share the
// listener's refusal budget.
//
// Both network transports go through here rather than calling the library's own
// Start method, so that the listen address, the header checks and the
// credential policy are decided in exactly one place. A transport that bound
// its own listener would silently miss all three.
func serveMCPOverHTTP(ctx context.Context, transportName string, handler http.Handler, port int, cfg transportConfig, grace time.Duration) error {
	listeners, err := bindListeners(transportName, cfg, port, net.Listen)
	if err != nil {
		return err
	}
	defer func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	bound := make([]string, 0, len(listeners))
	for _, ln := range listeners {
		bound = append(bound, ln.Addr().String())
	}
	logListening(transportName, bound, cfg)

	srv := newMCPHTTPServer(handler, cfg)
	return serveUntilDone(ctx, srv, listeners, grace)
}

// serveUntilDone serves srv on listeners until either serving fails or ctx is
// cancelled. On cancellation it stops accepting connections and gives in-flight
// requests up to grace to finish. Anything still open at the deadline is closed
// forcibly and logged as a warning. A cancelled context is a requested shutdown,
// so it returns nil.
func serveUntilDone(ctx context.Context, srv *http.Server, listeners []net.Listener, grace time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- serveAll(srv, listeners) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("Shutdown requested; draining in-flight requests",
		log.StringField("grace_period", grace.String()),
	)
	drainCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		log.Warn("Grace period elapsed; closing remaining connections, including any open event streams",
			log.StringField("grace_period", grace.String()),
			log.ErrorField(err),
		)
		_ = srv.Close()
	}
	// serveAll returns once every Serve call has observed the close.
	if err := <-errc; err != nil {
		return err
	}
	log.Info("Shutdown complete")
	return nil
}

// serveAll runs srv on every listener and returns the first error.
func serveAll(srv *http.Server, listeners []net.Listener) error {
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				once.Do(func() { first = err; _ = srv.Close() })
			}
		}(ln)
	}
	wg.Wait()
	return first
}

func logListening(transportName string, bound []string, cfg transportConfig) {
	fields := []zap.Field{
		log.StringField("transport", transportName),
		log.StringField("address", strings.Join(bound, ", ")),
	}
	if cfg.requireAuth {
		fields = append(fields, log.StringField("authentication", "every request must carry its own Authorization header"))
	} else {
		fields = append(fields, log.StringField("authentication",
			"DISABLED: requests with no Authorization header are served using this server's own credential"))
	}
	if len(cfg.origins.allowed) > 0 {
		fields = append(fields, log.StringField("allowed_origins", strings.Join(cfg.origins.allowed, ",")))
	}
	if cfg.loopbackOnly {
		fields = append(fields,
			log.StringField("reachable_from", "this machine only, unless a proxy forwards to it"),
			log.StringField("to_expose", "set -host to an address the network can reach, and declare -allowed-hosts"),
		)
		if cfg.requireAuth {
			log.Info("MCP server listening on loopback only", fields...)
		} else {
			log.Warn("MCP server listening on loopback only, with credential fallback enabled", fields...)
		}
		return
	}
	fields = append(fields, log.StringField("allowed_hosts", strings.Join(cfg.hosts.allowed, ",")))
	log.Warn("MCP server listening on a network-reachable address", fields...)
}
