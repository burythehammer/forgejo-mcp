// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
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
	base, err := forge.APIPath("repos", owner, repo, "issues")
	if err != nil {
		t.Fatalf("APIPath: %v", err)
	}
	path := base + fmt.Sprintf("?page=%d&limit=%d", 1, 20)

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

// TestDeleteFile_DotSegmentNeverSent is the regression test for the file-delete
// traversal: a dot segment anywhere in filePath is refused before a request is
// made, so the stub sees nothing.
func TestDeleteFile_DotSegmentNeverSent(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	a := &Adapter{raw: newRawClient(srv.URL, "test-token", "forgejo-mcp-test/0.0.1")}

	for _, p := range []string{"../../../../user/keys/5", "docs/../x", "."} {
		err := a.DeleteFile(context.Background(), "o", "r", p, forgejo_sdk.DeleteFileOptions{})
		if !errors.Is(err, forge.ErrDotSegment) {
			t.Fatalf("DeleteFile(%q) error = %v, want ErrDotSegment", p, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d requests reached the stub, want none", n)
	}
}
