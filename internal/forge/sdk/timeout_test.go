// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// stalledServer accepts each connection and never answers it, until the test
// ends. It is the "accepts, then goes silent" Forgejo from the spec.
func stalledServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv
}

// newTimedFactory builds a factory whose requests give up after timeout. The
// version is preset, so no probe request is sent before the test's own call.
func newTimedFactory(t *testing.T, url string, timeout time.Duration) *Factory {
	t.Helper()
	f := NewFactory(config.Forgejo{
		URL:            url,
		Token:          config.NewSecret("decoy-token"),
		RequestTimeout: timeout,
	}, "test")
	f.serverVersion.Store("11.0.0")
	return f
}

// An SDK-backed call against a stalled Forgejo returns a timeout error once the
// configured timeout passes, instead of hanging.
func TestSDKCallTimesOutWhenForgejoStalls(t *testing.T) {
	t.Parallel()
	srv := stalledServer(t)
	f := newTimedFactory(t, srv.URL, 200*time.Millisecond)

	port, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	start := time.Now()
	_, err = port.GetRepo(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatal("expected a timeout error from a stalled Forgejo")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("call returned after %s, want it bounded by the 200ms timeout", elapsed)
	}
	if !strings.Contains(err.Error(), "did not respond") {
		t.Fatalf("timeout error should say Forgejo did not respond in time, got: %v", err)
	}
}

// A raw-HTTP call against a stalled Forgejo is bounded by the same timeout.
func TestRawCallTimesOutWhenForgejoStalls(t *testing.T) {
	t.Parallel()
	srv := stalledServer(t)
	f := newTimedFactory(t, srv.URL, 200*time.Millisecond)

	port, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	start := time.Now()
	if _, err := port.GetIssueAttachment(context.Background(), "owner", "repo", 1, 1); err == nil {
		t.Fatal("expected a timeout error from a stalled Forgejo")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("raw call returned after %s, want it bounded by the 200ms timeout", elapsed)
	}
}

// An upload that takes longer than the shared request timeout is not cut off
// by it: uploads have their own, larger, documented bound.
func TestUploadOutlivesTheSharedRequestTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"name":"x.bin"}`))
	}))
	t.Cleanup(srv.Close)
	f := newTimedFactory(t, srv.URL, 100*time.Millisecond)

	port, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	up := forge.Upload{Filename: "x.bin", Content: strings.NewReader("payload")}
	if _, err := port.CreateIssueAttachment(context.Background(), "owner", "repo", 1, up); err != nil {
		t.Fatalf("upload cut off by the request timeout: %v", err)
	}
}

// Cancelling the request context stops an SDK-backed call against a stalled
// Forgejo promptly, long before the request timeout would. Both credential
// paths are covered: the operator's client and a per-request token's client.
func TestSDKCallStopsWhenRequestIsCancelled(t *testing.T) {
	t.Parallel()
	cases := map[string]func(context.Context) context.Context{
		"operator": forge.WithOperatorCredential,
		"request token": func(ctx context.Context) context.Context {
			return forge.WithToken(ctx, "decoy-request-token")
		},
	}
	for name, bind := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := stalledServer(t)
			f := newTimedFactory(t, srv.URL, time.Minute)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			port, err := f.For(bind(ctx))
			if err != nil {
				t.Fatalf("For: %v", err)
			}
			time.AfterFunc(100*time.Millisecond, cancel)

			start := time.Now()
			_, err = port.GetRepo(ctx, "owner", "repo")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled, got %v", err)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("cancelled call returned after %s, want it prompt", elapsed)
			}
		})
	}
}
