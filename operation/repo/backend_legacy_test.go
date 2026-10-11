// SPDX-License-Identifier: GPL-3.0-or-later

package repo

// Shared mock-server helper kept for the statuses tests until the RepoFiles
// migration (#51) moves them onto the fake; delete this file then.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

type recordedReq struct {
	method  string
	path    string
	rawBody []byte
}

func newRepoBackend(t *testing.T, muxFn func(*http.ServeMux)) *[]recordedReq {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	if muxFn != nil {
		muxFn(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			rawBody: body,
		})
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	forgejo.ConfigureForTesting(srv.URL, "tkn", "test")
	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("sdk client: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return &records
}
