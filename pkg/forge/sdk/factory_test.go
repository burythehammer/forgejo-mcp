// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

func versionOnlyServer(t *testing.T) *httptest.Server {
	t.Helper()
	// forgejo_sdk.NewClient makes a test request, so the server must answer.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFactory_WithContextToken(t *testing.T) {
	t.Parallel()
	srv := versionOnlyServer(t)
	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}, "test")

	// 1. A request the port marked for the operator's credential is served by
	// the operator's shared client.
	p1, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	p1b, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p1.(*Adapter).client != p1b.(*Adapter).client {
		t.Fatal("expected the operator's shared client to be reused")
	}

	// 2. A request with a token gets its own ephemeral client.
	p2, err := f.For(forge.WithToken(context.Background(), "request-token"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p1.(*Adapter).client == p2.(*Adapter).client {
		t.Fatal("expected a new ephemeral client instance when a token is present, got the shared one")
	}
}

// When a context token is present but client construction fails (here: an
// unreachable URL, so the SDK's version probe errors), For returns an error and
// no port rather than silently downgrading to the operator's client. Spec:
// "Failed ephemeral construction MUST NOT fall through to the shared client."
func TestFactory_EphemeralConstructionError(t *testing.T) {
	t.Parallel()
	// Port 1 refuses connections, so the SDK's server-version probe fails.
	f := NewFactory(config.Forgejo{URL: "http://127.0.0.1:1", Token: config.NewSecret("global-token")}, "test")

	port, err := f.For(forge.WithToken(context.Background(), "request-token"))
	if err == nil {
		t.Fatal("expected error when ephemeral client construction fails, got nil")
	}
	if port != nil {
		t.Fatal("expected no port on construction failure (silent shared-client downgrade)")
	}
}

func TestFactory_AuthorizationHeader(t *testing.T) {
	t.Parallel()
	tokensSeen := make(map[string]bool)
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			mu.Lock()
			tokensSeen[auth] = true
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		} else {
			_, _ = w.Write([]byte(`{"login":"test"}`))
		}
	}))
	defer srv.Close()

	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}, "test")

	// Concurrent requests with different tokens.
	var wg sync.WaitGroup
	const numRequests = 10
	for i := range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := forge.WithToken(context.Background(), fmt.Sprintf("token-%d", i))
			port, err := f.For(ctx)
			if err != nil {
				t.Errorf("failed to get port: %v", err)
				return
			}
			_, _ = port.GetMyUserInfo(ctx)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	for i := range numRequests {
		if expected := fmt.Sprintf("token token-%d", i); !tokensSeen[expected] {
			t.Errorf("expected to see token %q, but didn't", expected)
		}
	}
}

func TestVerifyConnectionAndHealthCheck(t *testing.T) {
	t.Parallel()
	srv := versionOnlyServer(t)
	// The startup check uses the operator's own client, whatever credential a
	// request is given, as it always has.
	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret("global-token")}, "test")
	if err := f.VerifyConnection(context.Background()); err != nil {
		t.Fatalf("VerifyConnection: %v", err)
	}
	if err := f.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	down := NewFactory(config.Forgejo{URL: "http://127.0.0.1:1"}, "test")
	if err := down.VerifyConnection(context.Background()); err == nil {
		t.Fatal("VerifyConnection succeeded against an unreachable instance")
	}
	if err := down.HealthCheck(context.Background()); err == nil {
		t.Fatal("HealthCheck succeeded against an unreachable instance")
	}
}
