// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"crypto/tls"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/config"
)

// tlsTransport is the one transport every Forgejo client shares when the
// operator set a CA bundle or skip-verify. It returns nil when neither is set,
// so the default transport stays in use. Nothing outside this package builds a
// transport.
func tlsTransport(t config.TLS) *http.Transport {
	if t.CAPool == nil && !t.SkipVerify {
		return nil
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	tr := base.Clone()
	// A nil RootCAs means the system roots; CAPool already holds them plus the
	// bundle, so the bundle is added to, not substituted for, the system trust.
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            t.CAPool,
		InsecureSkipVerify: t.SkipVerify, //nolint:gosec // opt-in, off by default, warned at startup
	}
	return tr
}
