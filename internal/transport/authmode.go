// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/jwtissuer"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/oauthrs"
)

const (
	// minForgejoMajorForResourceServer is the first Forgejo release with
	// Authorized Integrations.
	minForgejoMajorForResourceServer = 16
)

// resourceServer holds everything resource-server mode needs at request time.
type resourceServer struct {
	issuer        *jwtissuer.Issuer
	validator     *oauthrs.Validator
	resource      string
	authServer    string
	scopes        []string
	audienceClaim string
}

// providerClient is the one client for the identity provider's discovery and
// key documents, built once at startup. It trusts the operator's provider CA
// bundle when one is given; otherwise it uses the default transport. The
// Forgejo TLS settings never apply to it.
func providerClient(pool *x509.CertPool) *http.Client {
	if pool == nil {
		return oauthrs.NewHTTPClient(nil)
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	tr := base.Clone()
	// The pool already holds the system roots plus the bundle, so the bundle
	// is added to the system trust, not substituted for it.
	tr.TLSClientConfig = &tls.Config{RootCAs: pool}
	return oauthrs.NewHTTPClient(tr)
}

// prepareResourceServer runs the startup checks that need I/O, in order: load
// the keys, read Forgejo's version, and read the identity provider's metadata
// and keys. It fails on the first problem, before anything is bound.
func prepareResourceServer(ctx context.Context, cfg config.Config, factory forge.Factory) (*resourceServer, error) {
	a := cfg.Auth
	signing, err := jwtissuer.LoadSigningKey(a.ForgejoJWTSigningKeyFile)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -forgejo-jwt-signing-key-file: %w", err)
	}
	published := make([]*jwtissuer.Key, 0, len(a.ForgejoJWTPublishedKeyFiles))
	for _, path := range a.ForgejoJWTPublishedKeyFiles {
		k, err := jwtissuer.LoadPublishedKey(path)
		if err != nil {
			return nil, fmt.Errorf("refusing to start: -forgejo-jwt-published-key-files: %w", err)
		}
		published = append(published, k)
	}
	issuer, err := jwtissuer.New(a.ForgejoJWTIssuer, signing, published)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -forgejo-jwt-issuer: %w", err)
	}

	version, err := factory.ProbeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: cannot read the Forgejo version: %w", err)
	}
	major, err := forge.MajorVersion(version)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %w", err)
	}
	if major < minForgejoMajorForResourceServer {
		return nil, fmt.Errorf("refusing to start: -auth-mode resource-server requires Forgejo %d.0 or newer, but %s reports %s",
			minForgejoMajorForResourceServer, log.SanitizeURL(cfg.Forgejo.URL), version)
	}

	provider := providerClient(a.AuthorizationServerCAs)
	metadata, err := oauthrs.DiscoverProvider(ctx, provider, a.AuthorizationServer)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -authorization-server: %w", err)
	}
	keys, err := oauthrs.NewKeySet(ctx, metadata.JWKSURI, oauthrs.KeySetOptions{Client: provider})
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -authorization-server: %w", err)
	}
	audience := a.ResourceAudience
	if audience == "" {
		audience = a.Resource
	}
	validator, err := oauthrs.NewValidator(a.AuthorizationServer, audience, keys, nil)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %w", err)
	}

	log.Warn("The Forgejo signing key is a credential for every Forgejo user whose Authorized Integration trusts this issuer",
		log.StringField("kid", issuer.SigningKeyID()),
		log.StringField("issuer", issuer.URL()),
		log.StringField("forgejo_version", version),
	)

	return &resourceServer{
		issuer:        issuer,
		validator:     validator,
		resource:      a.Resource,
		authServer:    a.AuthorizationServer,
		scopes:        a.ScopesSupported,
		audienceClaim: a.ForgejoAudienceClaim,
	}, nil
}
