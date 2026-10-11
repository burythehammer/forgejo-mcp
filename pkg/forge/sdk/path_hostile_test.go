// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// TestAPIPath_HostileOwnerStaysOnEndpoint is the regression test for the
// raw-HTTP path-injection defect: an owner containing a traversal and a repo
// containing "?" must not move the request off /repos/{owner}/{repo}/issues.
func TestAPIPath_HostileOwnerStaysOnEndpoint(t *testing.T) {
	var requestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	rc := newRawClient(srv.URL, "test-token", "forgejo-mcp-test/0.0.1")

	const owner = "o/x/../../admin"
	const repo = "r?state=all&"
	path := forge.APIPath("repos", owner, repo, "issues") + fmt.Sprintf("?page=%d&limit=%d", 1, 20)

	var out []any
	if err := rc.doJSONList(context.Background(), http.MethodGet, path, &out); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// The traversal must arrive escaped, not collapsed by the URL parser.
	if !strings.Contains(requestURI, "%2F") {
		t.Fatalf("owner slashes were not escaped, got RequestURI %q", requestURI)
	}
	// The literal "?" in repo must not have terminated the path and pushed
	// "/issues" out of it.
	if !strings.HasSuffix(strings.SplitN(requestURI, "?", 2)[0], "/issues") {
		t.Fatalf("request left the /issues endpoint, got RequestURI %q", requestURI)
	}
	if strings.Contains(requestURI, "/repos/admin/") {
		t.Fatalf("traversal collapsed to another owner, got RequestURI %q", requestURI)
	}
	if want := "/api/v1/repos/o%2Fx%2F..%2F..%2Fadmin/r%3Fstate=all&/issues?page=1&limit=20"; requestURI != want {
		t.Fatalf("RequestURI = %q, want %q", requestURI, want)
	}
}
