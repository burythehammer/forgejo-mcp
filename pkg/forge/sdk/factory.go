// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Factory is the real forge.Factory. It is built once, by the composition root,
// from the typed Config; everything it knows about the instance, the operator's
// credential and the credential policy lives on the value, not in the package.
type Factory struct {
	cfg config.Forgejo
	ua  string
	hc  *http.Client

	// requireToken is the credential policy: when set, a request that carries
	// no credential of its own is refused (forge.ErrNoRequestToken) rather than
	// served with the operator's. See tokenFor.
	requireToken bool

	// serverVersion is the Forgejo version read at startup, or nothing. Every
	// SDK client built afterwards is told it, and so skips the version request
	// the SDK otherwise sends each time a client is constructed.
	serverVersion atomic.Value

	mu     sync.Mutex
	shared *forgejo_sdk.Client // the operator's client, built on first use

	settings *settings // the instance's pagination ceiling, shared by adapters
}

var _ forge.Factory = (*Factory)(nil)

// Option configures a Factory.
type Option func(*Factory)

// WithRequireRequestToken sets the credential policy: true refuses a request
// that carries no credential of its own, false lets the operator's configured
// credential stand in (the stdio behaviour). The default is false.
//
// It is deliberately a decision of the composition root, not derived from the
// address a listener bound: a loopback port is reachable by every local user,
// and a reverse proxy in front of one makes every observable look local on a
// request that came from the internet. A control whose failure mode selects for
// the vulnerable deployment is not a control.
func WithRequireRequestToken(require bool) Option {
	return func(f *Factory) { f.requireToken = require }
}

// NewFactory builds the factory for the Forgejo instance cfg describes. version
// is the build version, used for the default user agent.
func NewFactory(cfg config.Forgejo, version string, opts ...Option) *Factory {
	ua := cfg.UserAgent
	if ua == "" {
		ua = "forgejo-mcp/" + version
	}
	if cfg.URL != "" {
		if _, err := url.Parse(cfg.URL); err != nil {
			log.Errorf("Forgejo URL is not a valid URL: %v", err)
		}
	}
	f := &Factory{cfg: cfg, ua: ua, hc: sharedHTTPClient, settings: &settings{}}
	for _, o := range opts {
		o(f)
	}
	return f
}

// BaseURL is the Forgejo instance the factory serves.
func (f *Factory) BaseURL() string { return f.cfg.URL }

// RequireRequestToken reports the credential policy.
func (f *Factory) RequireRequestToken() bool { return f.requireToken }

// tokenFor resolves the credential to use for ctx. It is the single place the
// operator-credential fallback lives, so no call path can reach it without
// passing the policy.
func (f *Factory) tokenFor(ctx context.Context) (string, error) {
	if token := forge.TokenFromContext(ctx); token != "" {
		return token, nil
	}
	if f.requireToken {
		return "", forge.ErrNoRequestToken
	}
	return f.cfg.Token.Reveal(), nil
}

func (f *Factory) knownServerVersion() string {
	v, _ := f.serverVersion.Load().(string)
	return v
}

// newClient builds an SDK client for token, telling it the recorded server
// version, when there is one.
func (f *Factory) newClient(token string) (*forgejo_sdk.Client, error) {
	opts := []forgejo_sdk.ClientOption{
		forgejo_sdk.SetToken(token),
		forgejo_sdk.SetUserAgent(f.ua),
	}
	if v := f.knownServerVersion(); v != "" {
		opts = append(opts, forgejo_sdk.SetForgejoVersion(v))
	}
	return forgejo_sdk.NewClient(f.cfg.URL, opts...)
}

// operatorClient is the shared client bound to the operator's credential.
func (f *Factory) operatorClient() (*forgejo_sdk.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shared != nil {
		return f.shared, nil
	}
	c, err := f.newClient(f.cfg.Token.Reveal())
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
	f.shared = c
	log.Info("Successfully created Forgejo client",
		log.SanitizedURLField("url", f.cfg.URL),
		log.BoolField("token_configured", f.cfg.Token.Reveal() != ""),
		log.StringField("user_agent", f.ua),
	)
	return c, nil
}

// For implements forge.Factory.
//
// A request carrying its own token gets a fresh client bound to it; failure to
// build one never falls through to the shared client. Otherwise the policy
// decides whether the operator's credential may stand in.
func (f *Factory) For(ctx context.Context) (forge.Forgejo, error) {
	if token := forge.TokenFromContext(ctx); token != "" {
		c, err := f.newClient(token)
		if err != nil {
			log.ErrorCtx(ctx, "Failed to create ephemeral Forgejo client",
				log.SanitizedURLField("url", f.cfg.URL),
				log.ErrorField(err),
			)
			return nil, fmt.Errorf("create ephemeral client: %w", err)
		}
		return f.adapter(c, token), nil
	}

	token, err := f.tokenFor(ctx)
	if err != nil {
		log.ErrorCtx(ctx, "Refusing to serve request with the server's own credential",
			log.ErrorField(err),
		)
		return nil, err
	}
	c, err := f.operatorClient()
	if err != nil {
		return nil, err
	}
	return f.adapter(c, token), nil
}

func (f *Factory) adapter(c *forgejo_sdk.Client, token string) *Adapter {
	return &Adapter{
		client:   c,
		raw:      &rawClient{base: f.cfg.URL, token: token, ua: f.ua, hc: f.hc},
		settings: f.settings,
	}
}

// VerifyConnection checks that the configured instance answers. It uses the
// /version endpoint (no auth required) so that tokens scoped only to
// repo/issue -- e.g. organisation tokens -- are not rejected, and it runs with
// the operator's own client whatever the request policy is.
func (f *Factory) VerifyConnection(ctx context.Context) error {
	start := time.Now()
	log.Debug("Starting connection verification",
		log.SanitizedURLField("url", f.cfg.URL),
	)

	client, err := f.operatorClient()
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

	client, err := f.operatorClient()
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
