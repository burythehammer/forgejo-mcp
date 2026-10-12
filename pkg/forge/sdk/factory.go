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
// from the typed Config; everything it knows about the instance and the
// operator's credential lives on the value, not in the package. It holds no
// credential policy. It builds a client only from the credential its request
// is given, and the authentication port decides what that is.
type Factory struct {
	cfg config.Forgejo
	ua  string
	hc  *http.Client

	// serverVersion is the Forgejo version read at startup, or nothing. Every
	// SDK client built afterwards is told it, and so skips the version request
	// the SDK otherwise sends each time a client is constructed.
	serverVersion atomic.Value

	mu     sync.Mutex
	shared *forgejo_sdk.Client // the operator's client, built on first use

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
	return &Factory{cfg: cfg, ua: ua, hc: sharedHTTPClient, settings: &settings{}}
}

// BaseURL is the Forgejo instance the factory serves.
func (f *Factory) BaseURL() string { return f.cfg.URL }

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
// build one never falls through to the shared client. A request the
// authentication port marked for the operator's credential gets the shared
// client. Any other request is refused, so no client is built for it.
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

	if !forge.UsesOperatorCredential(ctx) {
		log.ErrorCtx(ctx, "Refusing to serve request with the server's own credential",
			log.ErrorField(forge.ErrNoRequestToken),
		)
		return nil, forge.ErrNoRequestToken
	}
	c, err := f.operatorClient()
	if err != nil {
		return nil, err
	}
	return f.adapter(c, f.cfg.Token.Reveal()), nil
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
// the operator's own client, whatever credential a request is given.
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
