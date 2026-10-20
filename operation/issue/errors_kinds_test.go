// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// Each domain error kind, as the port reports it, renders through the tool and
// through the resource: the tool's text names the kind and carries the status,
// and the resource's JSON-RPC code is the one the kind maps to. The port error
// is a real forge.StatusError from the fake's seam, so only the status varies.
func TestErrorKindsRenderThroughToolAndResource(t *testing.T) {
	t.Parallel()
	const uri = "forgejo://repo/octo/hello/issue/7"
	kinds := []struct {
		name     string
		status   int
		toolText string // substring of the tool's error text
		code     int    // the resource error's JSON-RPC code
	}{
		{"unauthorized 401", http.StatusUnauthorized, "access denied: 401 get issue err: status", -32002},
		{"forbidden 403", http.StatusForbidden, "access denied: 403 get issue err: status", -32002},
		{"not found 404", http.StatusNotFound, "not found: 404 get issue err: status", -32003},
		{"validation 422", http.StatusUnprocessableEntity, "invalid params: 422 get issue err: status", -32602},
		{"upstream 502", http.StatusBadGateway, "internal error: 502 get issue err: status", -32603},
	}
	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			t.Parallel()

			stub := newGoldenStub()
			stub.failOn["GetIssue"] = goldenStatus(k.status)
			out := goldenInvoke(t, stub, GetIssueByIndexToolName, map[string]any{"index": float64(7)})
			if !strings.Contains(out, k.toolText) {
				t.Fatalf("tool text lacks %q:\n%s", k.toolText, out)
			}

			stub = newGoldenStub()
			stub.failOn["GetIssue"] = goldenStatus(k.status)
			h := handlers{forge: fake.Factory{Forgejo: stub}}
			_, err := h.issueResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
			var re *resource.ResourceError
			if !errors.As(err, &re) {
				t.Fatalf("resource error = %v, want a *resource.ResourceError", err)
			}
			if re.Code != k.code {
				t.Fatalf("resource code = %d, want %d (%v)", re.Code, k.code, err)
			}
		})
	}
}
