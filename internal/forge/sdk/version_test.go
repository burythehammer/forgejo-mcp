// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// versionServer serves /api/v1/version and counts requests to it and requests
// that carried an Authorization header.
type versionServer struct {
	*httptest.Server
	versionRequests atomic.Int64
	authorized      atomic.Int64
	status          int
	version         string
}

func newVersionServer(t *testing.T, version string, status int) *versionServer {
	t.Helper()
	vs := &versionServer{version: version, status: status}
	vs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			vs.authorized.Add(1)
		}
		if r.URL.Path != "/api/v1/version" {
			http.NotFound(w, r)
			return
		}
		vs.versionRequests.Add(1)
		w.WriteHeader(vs.status)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": vs.version})
	}))
	t.Cleanup(vs.Close)
	return vs
}

func TestProbeServerVersionSendsNoCredential(t *testing.T) {
	t.Parallel()
	vs := newVersionServer(t, "16.0.3", http.StatusOK)
	// A configured operator value must not leak into the probe either.
	f := NewFactory(config.Forgejo{URL: vs.URL, Token: config.NewSecret(decoyOperatorCredential)}, "test")

	got, err := f.ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if got != "16.0.3" {
		t.Fatalf("version = %q, want 16.0.3", got)
	}
	if vs.authorized.Load() != 0 {
		t.Fatal("the version probe sent an Authorization header")
	}
}

func TestProbeServerVersionRefusesAnErrorStatus(t *testing.T) {
	t.Parallel()
	vs := newVersionServer(t, "16.0.3", http.StatusBadGateway)
	f := NewFactory(config.Forgejo{URL: vs.URL}, "test")
	if _, err := f.ProbeServerVersion(context.Background()); err == nil {
		t.Fatal("ProbeServerVersion accepted a 502")
	}
}

// Once the version is recorded at startup, building a client per request must
// not ask Forgejo for its version again.
func TestRecordedServerVersionSkipsThePerClientVersionRequest(t *testing.T) {
	t.Parallel()
	vs := newVersionServer(t, "16.0.3", http.StatusOK)
	f := NewFactory(config.Forgejo{URL: vs.URL}, "test")
	ctx := forge.WithToken(context.Background(), "per-request-value")

	// The control: without a recorded version the SDK asks on construction,
	// which is the request recording removes.
	if _, err := f.For(ctx); err != nil {
		t.Fatalf("For: %v", err)
	}
	if got := vs.versionRequests.Load(); got != 1 {
		t.Fatalf("version requests without a recorded version = %d, want 1", got)
	}

	if _, err := f.ProbeServerVersion(context.Background()); err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	before := vs.versionRequests.Load()
	for range 2 {
		if _, err := f.For(ctx); err != nil {
			t.Fatalf("For: %v", err)
		}
	}
	if got := vs.versionRequests.Load(); got != before {
		t.Fatalf("version requests with a recorded version = %d, want none after the probe's %d", got, before)
	}
}
