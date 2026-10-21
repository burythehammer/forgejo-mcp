// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"sync"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// settings caches the instance's pagination ceiling. One value serves one
// instance: the factory shares it between the adapters it hands out, and a
// bare adapter has its own, so no ceiling cached against one instance can leak
// into another.
type settings struct {
	mu    sync.Mutex
	entry *settingsEntry
}

// settingsEntry is a resolved pagination ceiling. ok is false when the ceiling
// could not be determined (endpoint unreachable, 403, old server, ...);
// callers must treat that as "unknown", never as a ceiling of zero.
type settingsEntry struct {
	maxResponseItems int
	ok               bool
}

// maxResponseItems returns the instance's configured max_response_items
// pagination ceiling (GET /api/v1/settings/api), fetching and caching it on
// first use. ok is false when the ceiling is unknown -- e.g. the endpoint
// errored, returned a non-2xx status (older servers, or instances that
// restrict the settings endpoint), or is otherwise unreachable.
func (s *settings) maxResponseItems(c *forgejo_sdk.Client) (int, bool) {
	s.mu.Lock()
	if s.entry != nil {
		e := *s.entry
		s.mu.Unlock()
		return e.maxResponseItems, e.ok
	}
	s.mu.Unlock()

	e := fetchMaxResponseItems(c)

	s.mu.Lock()
	s.entry = &e
	s.mu.Unlock()
	return e.maxResponseItems, e.ok
}

func fetchMaxResponseItems(c *forgejo_sdk.Client) settingsEntry {
	got, resp, err := c.GetGlobalAPISettings()
	if err != nil {
		return settingsEntry{}
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return settingsEntry{}
	}
	if got == nil {
		return settingsEntry{}
	}
	return settingsEntry{maxResponseItems: got.MaxResponseItems, ok: true}
}

// MaxResponseItems reads the instance setting, cached per instance.
func (a *Adapter) MaxResponseItems(context.Context) (int, bool) {
	return a.settings.maxResponseItems(a.client)
}
