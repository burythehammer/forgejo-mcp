// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Factory is the real forge.Factory. It is built once, by the composition root,
// from the typed Config; everything it knows about the instance and the
// operator's credential lives on the value, not in the package. It holds no
// credential policy. It builds a client only from the credential its request
// is given, and the authentication port decides what that is.
type Factory struct {
	cfg config.Forgejo
	ua  string
	// hc is the request client: the SDK clients and the raw client share it,
	// so every Forgejo call gives up after the same RequestTimeout.
	hc *http.Client
	// up is the upload client. It carries the same transport as hc, with the
	// longer uploadRequestTimeout bound, so a large upload is not cut off by the
	// request timeout.
	up *http.Client
	tr *http.Transport // TLS transport shared with the SDK client; nil for the default

	// serverVersion is the Forgejo version read at startup, or nothing. Every
	// SDK client built afterwards is told it, and so skips the version request
	// the SDK otherwise sends each time a client is constructed.
	serverVersion atomic.Value

	settings *settings // the instance's pagination ceiling, shared by adapters
}

var _ forge.Factory = (*Factory)(nil)

// NewFactory builds the factory for the Forgejo instance cfg describes. version
// is the build version, used for the default user agent.
func NewFactory(cfg config.Forgejo, version string) *Factory {
	ua := cfg.UserAgent
	if ua == "" {
		ua = "forgejo-mcp/" + version
	}
	if cfg.URL != "" {
		if _, err := url.Parse(cfg.URL); err != nil {
			log.Errorf("Forgejo URL is not a valid URL: %v", err)
		}
	}
	// One TLS setting serves the SDK client, the raw client and the upload
	// client. Built here, once per factory, so keep-alives are shared across
	// tool calls.
	tr := tlsTransport(cfg.TLS)
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = config.DefaultRequestTimeout
	}
	hc := newHTTPClient(timeout, tr)
	up := newHTTPClient(uploadRequestTimeout, tr)
	if cfg.TLS.SkipVerify {
		log.Warn("TLS certificate verification is OFF for the Forgejo instance (--tls-skip-verify); "+
			"the server does not check who it is talking to",
			log.SanitizedURLField("url", cfg.URL),
		)
	}
	return &Factory{cfg: cfg, ua: ua, hc: hc, up: up, tr: tr, settings: &settings{}}
}

// BaseURL is the Forgejo instance the factory serves.
func (f *Factory) BaseURL() string { return f.cfg.URL }

func (f *Factory) knownServerVersion() string {
	v, _ := f.serverVersion.Load().(string)
	return v
}

// newClient builds an SDK client for token, bound to ctx: once ctx is
// cancelled, the calls made with this client stop. It tells the client the
// recorded server version, when there is one.
func (f *Factory) newClient(ctx context.Context, token string) (*forgejo_sdk.Client, error) {
	opts := []forgejo_sdk.ClientOption{
		forgejo_sdk.SetToken(token),
		forgejo_sdk.SetUserAgent(f.ua),
		// The shared request client: same transport and RequestTimeout as the
		// raw client, so a stalled Forgejo cannot hang an SDK-backed call.
		forgejo_sdk.SetHTTPClient(f.hc),
		// The SDK's context is per client, so each request gets its own client
		// rather than sharing one whose context another request would change.
		forgejo_sdk.SetContext(ctx),
	}
	if v := f.knownServerVersion(); v != "" {
		opts = append(opts, forgejo_sdk.SetForgejoVersion(v))
	}
	return forgejo_sdk.NewClient(f.cfg.URL, opts...)
}

// operatorClient builds an SDK client bound to ctx with the operator's
// credential. It is built per call, so no request's context can reach another.
func (f *Factory) operatorClient(ctx context.Context) (*forgejo_sdk.Client, error) {
	c, err := f.newClient(ctx, f.cfg.Token.Reveal())
	if err != nil {
		log.Error("Failed to create Forgejo client",
			log.SanitizedURLField("url", f.cfg.URL),
			log.ErrorField(err),
		)
		// Never fatal: the caller decides. At startup the connection check
		// turns this into a clean "connection test failed" exit; in tests it
		// stays an ordinary error instead of killing the test binary.
		return nil, fmt.Errorf("create forgejo client: %w", err)
	}
	return c, nil
}

// For implements forge.Factory.
//
// A request carrying its own token gets a fresh client bound to it; failure to
// build one never falls through to the operator's client. A request the
// authentication port marked for the operator's credential gets a fresh client
// with the operator's token. Both are bound to ctx, so cancelling the request
// stops its SDK calls. Any other request is refused, so no client is built for it.
func (f *Factory) For(ctx context.Context) (forge.Forgejo, error) {
	if token := forge.TokenFromContext(ctx); token != "" {
		c, err := f.newClient(ctx, token)
		if err != nil {
			log.ErrorCtx(ctx, "Failed to create ephemeral Forgejo client",
				log.SanitizedURLField("url", f.cfg.URL),
				log.ErrorField(err),
			)
			return nil, fmt.Errorf("create ephemeral client: %w", err)
		}
		return f.adapter(c, token), nil
	}

	if !forge.UsesOperatorCredential(ctx) {
		log.ErrorCtx(ctx, "Refusing to serve request with the server's own credential",
			log.ErrorField(forge.ErrNoRequestToken),
		)
		return nil, forge.ErrNoRequestToken
	}
	c, err := f.operatorClient(ctx)
	if err != nil {
		return nil, err
	}
	return f.adapter(c, f.cfg.Token.Reveal()), nil
}

func (f *Factory) adapter(c *forgejo_sdk.Client, token string) *Adapter {
	return &Adapter{
		client:   c,
		raw:      &rawClient{base: f.cfg.URL, token: token, ua: f.ua, hc: f.hc, up: f.up},
		settings: f.settings,
	}
}

// VerifyConnection checks that the configured instance answers. It uses the
// /version endpoint (no auth required) so that tokens scoped only to
// repo/issue -- e.g. organisation tokens -- are not rejected, and it runs with
// the operator's own client, whatever credential a request is given.
func (f *Factory) VerifyConnection(ctx context.Context) error {
	start := time.Now()
	log.Debug("Starting connection verification",
		log.SanitizedURLField("url", f.cfg.URL),
	)

	client, err := f.operatorClient(ctx)
	if err != nil {
		return err
	}
	version, resp, err := client.ServerVersion()
	duration := time.Since(start)
	if err != nil {
		log.Error("Connection verification failed",
			log.SanitizedURLField("url", f.cfg.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("failed to connect to Forgejo instance at %s: %w", f.cfg.URL, err)
	}

	log.Info("Connection verification successful",
		log.SanitizedURLField("url", f.cfg.URL),
		log.DurationField("duration", duration),
		log.StringField("server_version", version),
		log.IntField("response_status", resp.StatusCode),
	)
	return nil
}

// HealthCheck performs a lightweight health check.
func (f *Factory) HealthCheck(ctx context.Context) error {
	start := time.Now()
	log.Debug("Starting health check")

	client, err := f.operatorClient(ctx)
	if err != nil {
		return err
	}
	version, resp, err := client.ServerVersion()
	duration := time.Since(start)
	if err != nil {
		log.Error("Health check failed",
			log.SanitizedURLField("url", f.cfg.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("health check failed: %w", err)
	}

	log.Debug("Health check successful",
		log.SanitizedURLField("url", f.cfg.URL),
		log.DurationField("duration", duration),
		log.StringField("server_version", version),
		log.IntField("response_status", resp.StatusCode),
	)
	return nil
}
