// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Mock-server helpers for the label, label-resolution and dependency tests,
// which still run against the legacy client. The labels ticket deletes this
// file with them.

type recordedReq struct {
	method  string
	path    string
	query   string
	rawBody []byte
}

func newPatchBackend(t *testing.T, respBody string) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 2)
	// Serve the SDK's startup version probe so NewClient succeeds.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			query:   r.URL.RawQuery,
			rawBody: body,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	forgejo.ConfigureForTesting(srv.URL, "tkn", "test")

	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("failed to build SDK client for test: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return srv, &records
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// newLabelsBackend serves /api/v1/repos/{owner}/{repo}/labels and
// /api/v1/orgs/{owner}/labels with caller-supplied status codes and bodies.
// Caller passes maps keyed by exact path. Status defaults to 200 if 0.
func newLabelsBackend(
	t *testing.T,
	repoLabelsBody string, repoStatus int,
	orgLabelsBody string, orgStatus int,
) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/repos/") && strings.HasSuffix(r.URL.Path, "/labels"):
			if repoStatus == 0 {
				repoStatus = http.StatusOK
			}
			w.WriteHeader(repoStatus)
			_, _ = w.Write([]byte(repoLabelsBody))
		case strings.HasPrefix(r.URL.Path, "/api/v1/orgs/") && strings.HasSuffix(r.URL.Path, "/labels"):
			if orgStatus == 0 {
				orgStatus = http.StatusOK
			}
			w.WriteHeader(orgStatus)
			_, _ = w.Write([]byte(orgLabelsBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	forgejo.ConfigureForTesting(srv.URL, "tkn", "test")

	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("failed to build SDK client for test: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return srv, &records
}

// textOf flattens a CallToolResult into a single string for substring assertions.
func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// addLabelsBackend serves the assignment endpoints for a label catalogue of
// bug=5 and triage=6: POST .../labels answers with a label list, anything else
// with the refreshed issue.
func addLabelsBackend(t *testing.T) (*[]recordedReq, *int) {
	t.Helper()
	return newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}, {ID: 6, Name: "triage"}},
		orgStatus:  http.StatusNotFound,
		next: func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`[{"id":5},{"id":6}]`))
				return
			}
			_, _ = w.Write([]byte(`{"id":1,"number":1,"labels":[{"id":5},{"id":6}]}`))
		},
	})
}
