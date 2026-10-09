// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"net/url"
	"sync/atomic"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
)

// target is the Forgejo instance this process talks to, as the composition root
// configured it.
type target struct {
	url       string
	token     string
	userAgent string
}

var current atomic.Pointer[target]

// Configure points the typed client and the raw-HTTP helpers at cfg. version
// is the build version, used for the default user agent. Call it once at
// startup, before serving.
func Configure(cfg config.Forgejo, version string) {
	ua := cfg.UserAgent
	if ua == "" {
		ua = "forgejo-mcp/" + version
	}
	if cfg.URL != "" {
		if _, err := url.Parse(cfg.URL); err != nil {
			log.Errorf("Forgejo URL is not a valid URL: %v", err)
		}
	}
	current.Store(&target{url: cfg.URL, token: cfg.Token.Reveal(), userAgent: ua})
}

func baseURL() string {
	if t := current.Load(); t != nil {
		return t.url
	}
	return ""
}

func operatorToken() string {
	if t := current.Load(); t != nil {
		return t.token
	}
	return ""
}

// userAgent returns the configured UA, falling back to forgejo-mcp/<version>.
func userAgent() string {
	if t := current.Load(); t != nil {
		return t.userAgent
	}
	return "forgejo-mcp/"
}
