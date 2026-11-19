// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
)

func settingsServer(t *testing.T, requests *int32, settingsStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		case "/api/v1/settings/api":
			atomic.AddInt32(requests, 1)
			if settingsStatus != http.StatusOK {
				w.WriteHeader(settingsStatus)
				_, _ = w.Write([]byte(`{"message":"forbidden"}`))
				return
			}
			_, _ = w.Write([]byte(`{"max_response_items":50,"default_paging_num":30,"default_git_trees_per_page":1000,"default_max_blob_size":10485760}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func portFor(t *testing.T, f *Factory, ctx context.Context) forge.Forgejo {
	t.Helper()
	port, err := f.For(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// A second call for the same instance does not re-hit the settings endpoint,
// even through a different adapter (a different request's credential): the
// ceiling is cached per instance, on the factory.
func TestMaxResponseItems_CachesAcrossCalls(t *testing.T) {
	t.Parallel()
	var requests int32
	srv := settingsServer(t, &requests, http.StatusOK)
	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}, "test")

	// The port marks an operator-served request; these tests use one.
	ctx := forge.WithOperatorCredential(context.Background())
	max1, ok1 := portFor(t, f, ctx).MaxResponseItems(ctx)
	if !ok1 || max1 != 50 {
		t.Fatalf("first call: got %d/%v, want 50/true", max1, ok1)
	}
	other := forge.WithToken(ctx, "request-token")
	max2, ok2 := portFor(t, f, other).MaxResponseItems(other)
	if !ok2 || max2 != 50 {
		t.Fatalf("expected cached value 50/true, got %d/%v", max2, ok2)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected exactly 1 settings request (cache hit on 2nd call), got %d", got)
	}
}

// A non-2xx response from the settings endpoint (e.g. 403, or the endpoint
// missing on older/restricted instances) surfaces as ok=false ("unknown"),
// never as a ceiling of 0 -- a zero ceiling would reject every request.
func TestMaxResponseItems_FailureIsUnknownNotZero(t *testing.T) {
	t.Parallel()
	var requests int32
	srv := settingsServer(t, &requests, http.StatusForbidden)
	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}, "test")

	// The port marks an operator-served request; these tests use one.
	ctx := forge.WithOperatorCredential(context.Background())
	max, ok := portFor(t, f, ctx).MaxResponseItems(ctx)
	if ok {
		t.Fatalf("expected ok=false on 403, got ok=true with max=%d", max)
	}
	if max != 0 {
		t.Fatalf("expected max=0 (unused) when ok=false, got %d", max)
	}
}

// Two factories (two instances, or two tests) never share a cached ceiling.
func TestMaxResponseItems_IsPerFactory(t *testing.T) {
	t.Parallel()
	var requests int32
	srv := settingsServer(t, &requests, http.StatusOK)
	cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}

	// The port marks an operator-served request; these tests use one.
	ctx := forge.WithOperatorCredential(context.Background())
	for range 2 {
		if _, ok := portFor(t, NewFactory(cfg, "test"), ctx).MaxResponseItems(ctx); !ok {
			t.Fatal("expected ok=true")
		}
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected a settings request per factory, got %d", got)
	}
}
