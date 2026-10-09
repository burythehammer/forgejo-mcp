// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs"
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

// prepareResourceServer runs the startup checks that need I/O, in order: load
// the keys, read Forgejo's version, and read the identity provider's metadata
// and keys. It fails on the first problem, before anything is bound.
func prepareResourceServer(ctx context.Context, cfg config.Config) (*resourceServer, error) {
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

	version, err := forgejo.ProbeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: cannot read the Forgejo version: %w", err)
	}
	major, err := forgejo.MajorVersion(version)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %w", err)
	}
	if major < minForgejoMajorForResourceServer {
		return nil, fmt.Errorf("refusing to start: -auth-mode resource-server requires Forgejo %d.0 or newer, but %s reports %s",
			minForgejoMajorForResourceServer, log.SanitizeURL(cfg.Forgejo.URL), version)
	}
	forgejo.SetServerVersion(version)

	metadata, err := oauthrs.DiscoverProvider(ctx, nil, a.AuthorizationServer)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -authorization-server: %w", err)
	}
	keys, err := oauthrs.NewKeySet(ctx, metadata.JWKSURI, oauthrs.KeySetOptions{})
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
