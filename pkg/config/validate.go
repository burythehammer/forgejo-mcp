// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer"
)

const (
	// AuthModePassthrough forwards the caller's Authorization header to Forgejo.
	AuthModePassthrough = "passthrough"
	// AuthModeResourceServer validates identity-provider tokens and signs
	// Forgejo Authorized Integration tokens.
	AuthModeResourceServer = "resource-server"

	// MCPEndpointPath is where the http transport serves the MCP endpoint.
	MCPEndpointPath = "/mcp"
)

// Validate refuses a Config the server must not start with. It does no I/O and
// reads no process state: connection tests, the Forgejo version check and
// key-file loading stay in startup. Load calls it before returning.
func (c Config) Validate() error {
	if err := c.validateAuth(); err != nil {
		return err
	}
	if c.Mode == ModeServe && (c.Transport == "sse" || c.Transport == "http") {
		return c.validateListen()
	}
	return nil
}

func (c Config) validateAuth() error {
	a := c.Auth
	mode := a.Mode
	if mode == "" {
		mode = AuthModePassthrough
	}
	switch mode {
	case AuthModePassthrough:
		if len(c.suppliedResourceServer) > 0 {
			return fmt.Errorf("refusing to start: %s requires -auth-mode resource-server; remove it, or enable that mode",
				strings.Join(c.suppliedResourceServer, ", "))
		}
		return nil
	case AuthModeResourceServer:
	default:
		return fmt.Errorf("refusing to start: -auth-mode %q is not valid; use passthrough or resource-server", mode)
	}

	if c.Mode == ModeCLI {
		return errors.New("refusing to start: -auth-mode resource-server requires the http transport and cannot be used with --cli")
	}
	if c.Transport != "http" {
		return fmt.Errorf("refusing to start: -auth-mode resource-server requires the http transport, not %q", c.Transport)
	}
	if c.Forgejo.Token.Reveal() != "" {
		return errors.New("refusing to start: -auth-mode resource-server takes no operator token; " +
			"unset -token and FORGEJO_ACCESS_TOKEN (and GITEA_ACCESS_TOKEN)")
	}
	if c.Listen.AllowOperatorTokenFallback {
		return errors.New("refusing to start: -allow-operator-token-fallback cannot be combined with -auth-mode resource-server")
	}

	var missing []string
	for _, s := range []struct{ name, value string }{
		{"-authorization-server", a.AuthorizationServer},
		{"-resource", a.Resource},
		{"-forgejo-jwt-issuer", a.ForgejoJWTIssuer},
		{"-forgejo-jwt-signing-key-file", a.ForgejoJWTSigningKeyFile},
	} {
		if strings.TrimSpace(s.value) == "" {
			missing = append(missing, s.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("refusing to start: -auth-mode resource-server requires %s", strings.Join(missing, ", "))
	}
	for _, scope := range a.ScopesSupported {
		if !ValidScopeToken(scope) {
			return fmt.Errorf("refusing to start: -scopes-supported contains %q, which is not a valid OAuth scope", scope)
		}
	}

	if _, err := httpsOrLoopbackURL("-authorization-server", a.AuthorizationServer); err != nil {
		return err
	}
	resourceURL, err := httpsOrLoopbackURL("-resource", a.Resource)
	if err != nil {
		return err
	}
	// The metadata publishes -resource as the resource and derives its own
	// location from its path, while the endpoint is always served at
	// MCPEndpointPath. Any other path starts cleanly and then names a resource
	// this server does not serve, which clients must reject.
	if resourceURL.Path != MCPEndpointPath {
		return fmt.Errorf("refusing to start: the path of -resource (%q) must be %q, the MCP endpoint this server serves",
			resourceURL.Path, MCPEndpointPath)
	}
	issuerURL, err := url.Parse(a.ForgejoJWTIssuer)
	if err != nil || issuerURL.Scheme != "https" || issuerURL.Host == "" {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer %q must be an https URL, because Forgejo only fetches https issuers",
			a.ForgejoJWTIssuer)
	}
	if err := jwtissuer.CheckIssuerURL(a.ForgejoJWTIssuer); err != nil {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer: %w", err)
	}

	allowed := NormalizeList(c.Listen.AllowedHosts)
	loopbackOnly := IsLoopbackHostname(strings.TrimSpace(c.Listen.Host))
	for _, s := range []struct {
		name string
		u    *url.URL
	}{{"-resource", resourceURL}, {"-forgejo-jwt-issuer", issuerURL}} {
		if !HostAllowed(allowed, loopbackOnly, s.u.Host) {
			return fmt.Errorf("refusing to start: the host of %s (%s) is not one this server answers to; add it to -allowed-hosts",
				s.name, s.u.Host)
		}
	}
	return nil
}

// validateListen holds the fail-closed rules for the network transports.
func (c Config) validateListen() error {
	l := c.Listen
	host := strings.TrimSpace(l.Host)
	if host == "" {
		return fmt.Errorf("refusing to start: %s transport has an empty bind address; "+
			"pass -host with an address to listen on", c.Transport)
	}
	if !IsLoopbackHostname(host) && len(NormalizeList(l.AllowedHosts)) == 0 {
		return fmt.Errorf("refusing to start: %s transport is configured to bind %s, which the network can reach, "+
			"but no allowed hosts are declared; pass -allowed-hosts (or FORGEJO_MCP_ALLOWED_HOSTS) "+
			"with the host names clients will use, or bind to loopback instead",
			c.Transport, host)
	}
	for _, raw := range NormalizeList(l.AllowedOrigins) {
		if _, ok := NormalizeOrigin(raw); !ok {
			return fmt.Errorf("refusing to start: %q is not a usable origin for -allowed-origins; "+
				"an origin is a scheme, a host and an optional port, for example https://console.example.org", raw)
		}
	}
	return nil
}

// httpsOrLoopbackURL parses an absolute URL that must use https, or http on a
// loopback host, which only local development needs.
func httpsOrLoopbackURL(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("refusing to start: %s %q is not an absolute URL", name, raw)
	}
	switch {
	case u.Scheme == "https":
		return u, nil
	case u.Scheme == "http" && IsLoopbackHostname(u.Hostname()):
		return u, nil
	default:
		return nil, fmt.Errorf("refusing to start: %s %q must use https (http is accepted only for a loopback host)", name, raw)
	}
}
