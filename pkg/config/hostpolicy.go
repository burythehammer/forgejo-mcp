// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// NormalizeList lower-cases entries and drops empty ones.
func NormalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// IsLoopbackHostname reports whether host names a loopback interface. host may
// carry brackets ("[::1]") but not a port.
func IsLoopbackHostname(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

// IsIPLiteral reports whether host is an address rather than a name. Brackets
// around an IPv6 literal are accepted, as in "[::1]".
func IsIPLiteral(host string) bool {
	_, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil
}

// SplitHostPort separates a "host" or "host:port" authority. It returns the
// bare host (no brackets, lower-cased), the port ("" when absent), and whether
// the value parsed at all.
//
// A value whose port is not a plain number does not parse. net.SplitHostPort
// splits on the last colon without looking at what follows it, so
// "127.0.0.1:8080@example.com" would otherwise reduce to a loopback host.
func SplitHostPort(authority string) (host, port string, ok bool) {
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
	// No port. A bracketed literal is an IPv6 address; an unbracketed value
	// containing a colon is either a bare IPv6 address or malformed.
	if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		return strings.Trim(authority, "[]"), "", true
	}
	if strings.Contains(authority, ":") {
		// A bare IPv6 address is legitimate in an allow-list entry even though
		// it is not legal in a Host header. Accept it only if it really parses
		// as one.
		if _, err := netip.ParseAddr(authority); err == nil {
			return authority, "", true
		}
		return "", "", false
	}
	return authority, "", true
}

// HostAllowed reports whether a Host value is one the listener answers to: an
// allowed entry with a port must match exactly, one without matches any port,
// and a loopback-only bind also answers to loopback names.
func HostAllowed(allowed []string, loopbackOnly bool, value string) bool {
	host, port, ok := SplitHostPort(value)
	if !ok {
		return false
	}
	for _, entry := range allowed {
		eHost, ePort, eOK := SplitHostPort(entry)
		if eOK && eHost == host && (ePort == "" || ePort == port) {
			return true
		}
	}
	return loopbackOnly && IsLoopbackHostname(host)
}

// NormalizeOrigin reduces an origin to "scheme://host[:port]", dropping the
// port when it is the default for the scheme so that "https://x" and
// "https://x:443" compare equal. Anything that is not a hierarchical http or
// https URL (an opaque value, a bare hostname, a file URL) does not normalise.
func NormalizeOrigin(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if parsed.User != nil {
		// Userinfo has no meaning in an origin and is a classic way to make a
		// hostile authority read as a friendly one.
		return "", false
	}
	host, port, ok := SplitHostPort(parsed.Host)
	if !ok || host == "" {
		return "", false
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		return scheme + "://" + host, true
	}
	return scheme + "://" + host + ":" + port, true
}

// ValidScopeToken reports whether s is an OAuth 2.0 scope token (RFC 6749
// section 3.3): one or more of %x21 / %x23-5B / %x5D-7E. Scopes are placed
// into a quoted header value, so a quote or backslash must never reach it.
func ValidScopeToken(s string) bool {
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
