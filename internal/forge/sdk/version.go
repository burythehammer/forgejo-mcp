// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// maxVersionBody bounds the /version response read at startup.
const maxVersionBody = 64 << 10

// ProbeServerVersion implements forge.Factory. It asks the configured Forgejo
// instance for its version without sending any credential: /api/v1/version
// needs none, and in resource-server mode there is no credential to send before
// a caller arrives. The version is recorded, so every SDK client built
// afterwards skips the version request the SDK otherwise sends each time a
// client is constructed.
func (f *Factory) ProbeServerVersion(ctx context.Context) (string, error) {
	r := &rawClient{base: f.cfg.URL, ua: f.ua, hc: f.hc}
	u, err := r.resolveURL("/version")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("build version request: %w", err)
	}
	req.Header.Set("User-Agent", f.ua)
	req.Header.Set("Accept", "application/json")

	resp, err := f.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %s", u, resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxVersionBody)).Decode(&body); err != nil {
		return "", fmt.Errorf("GET %s: %w", u, err)
	}
	if body.Version == "" {
		return "", fmt.Errorf("GET %s: response carries no version", u)
	}
	f.serverVersion.Store(body.Version)
	return body.Version, nil
}
