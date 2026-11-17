// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"net/http/httptest"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// TestNetworkTransportMarksContextRemote pins that every HTTP transport's
// per-request context carries the remote mark, so a tool that writes a file
// can refuse on sse and http, where the caller cannot read the file.
func TestNetworkTransportMarksContextRemote(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("POST", "/mcp", nil)
	ctx := requestTokenContextFunc(config.Config{})(context.Background(), req)
	if !forge.Remote(ctx) {
		t.Fatal("a network transport's request context is not marked remote")
	}
}
