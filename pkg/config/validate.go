// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer"
)

const (
	authModePassthrough    = "passthrough"
	authModeResourceServer = "resource-server"

	// mcpEndpointPath is where the http transport serves the MCP endpoint.
	mcpEndpointPath = "/mcp"
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
		mode = authModePassthrough
	}
	switch mode {
	case authModePassthrough:
		if len(c.suppliedResourceServer) > 0 {
			return fmt.Errorf("refusing to start: %s requires -auth-mode resource-server; remove it, or enable that mode",
				strings.Join(c.suppliedResourceServer, ", "))
		}
		return nil
	case authModeResourceServer:
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
		if !validScopeToken(scope) {
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
	// mcpEndpointPath. Any other path starts cleanly and then names a resource
	// this server does not serve, which clients must reject.
	if resourceURL.Path != mcpEndpointPath {
		return fmt.Errorf("refusing to start: the path of -resource (%q) must be %q, the MCP endpoint this server serves",
			resourceURL.Path, mcpEndpointPath)
	}
	issuerURL, err := url.Parse(a.ForgejoJWTIssuer)
	if err != nil || issuerURL.Scheme != "https" || issuerURL.Host == "" {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer %q must be an https URL, because Forgejo only fetches https issuers",
			a.ForgejoJWTIssuer)
	}
	if err := jwtissuer.CheckIssuerURL(a.ForgejoJWTIssuer); err != nil {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer: %w", err)
	}

	allowed := normalizeList(c.Listen.AllowedHosts)
	loopbackOnly := isLoopbackHostname(strings.TrimSpace(c.Listen.Host))
	for _, s := range []struct {
		name string
		u    *url.URL
	}{{"-resource", resourceURL}, {"-forgejo-jwt-issuer", issuerURL}} {
		if !hostAllowed(allowed, loopbackOnly, s.u.Host) {
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
	if !isLoopbackHostname(host) && len(normalizeList(l.AllowedHosts)) == 0 {
		return fmt.Errorf("refusing to start: %s transport is configured to bind %s, which the network can reach, "+
			"but no allowed hosts are declared; pass -allowed-hosts (or FORGEJO_MCP_ALLOWED_HOSTS) "+
			"with the host names clients will use, or bind to loopback instead",
			c.Transport, host)
	}
	for _, raw := range normalizeList(l.AllowedOrigins) {
		if !usableOrigin(raw) {
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
	case u.Scheme == "http" && isLoopbackHostname(u.Hostname()):
		return u, nil
	default:
		return nil, fmt.Errorf("refusing to start: %s %q must use https (http is accepted only for a loopback host)", name, raw)
	}
}

// validScopeToken reports whether s is an OAuth scope-token (RFC 6749 3.3).
func validScopeToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x21 || c > 0x7E || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func isLoopbackHostname(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

// hostAllowed mirrors the listener's Host policy: an entry with a port must
// match exactly, one without matches any port, and a loopback-only bind also
// answers to loopback names.
func hostAllowed(allowed []string, loopbackOnly bool, value string) bool {
	host, port, ok := splitHostPort(value)
	if !ok {
		return false
	}
	for _, entry := range allowed {
		eHost, ePort, eOK := splitHostPort(entry)
		if eOK && eHost == host && (ePort == "" || ePort == port) {
			return true
		}
	}
	return loopbackOnly && isLoopbackHostname(host)
}

// splitHostPort separates "host" or "host:port". A port that is not a plain
// number does not parse, so "127.0.0.1:8080@example.com" cannot reduce to a
// loopback host.
func splitHostPort(authority string) (host, port string, ok bool) {
	authority = strings.TrimSpace(strings.ToLower(authority))
	if authority == "" {
		return "", "", false
	}
	if h, p, err := net.SplitHostPort(authority); err == nil {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return "", "", false
		}
		return strings.Trim(h, "[]"), p, true
	}
	if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		return strings.Trim(authority, "[]"), "", true
	}
	if strings.Contains(authority, ":") {
		if _, err := netip.ParseAddr(authority); err == nil {
			return authority, "", true
		}
		return "", "", false
	}
	return authority, "", true
}

// usableOrigin reports whether value is a scheme, a host and an optional port,
// without user information.
func usableOrigin(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return false
	}
	if u.User != nil {
		return false
	}
	host, _, ok := splitHostPort(u.Host)
	return ok && host != ""
}
