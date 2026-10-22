// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"net/http"
	"strings"
	"testing"
)

// The refusal reason embeds the token's kid, which a stranger chooses. In
// resource-server mode the logged reason must be cut to the same bound as the
// other attacker-influenced fields, and the cut must show.
func TestResourceServerRefusalLogBoundsAnOversizedReason(t *testing.T) {
	rs, env := portResourceServer(t)
	cfg := resolveTransportConfig(resourceServerConfig())

	handler, err := rs.handlerWithRefusals(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), cfg.refusals)
	if err != nil {
		t.Fatalf("handlerWithRefusals: %v", err)
	}
	ln := mustListenLoopback(t)
	srv := newMCPHTTPServer(handler, cfg)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	addr := ln.Addr().String()
	logs := observeLogs(t)

	kid := strings.Repeat("k", 8<<10)
	token := env.signedAccessToken(t, kid, nil)
	if got := get(t, addr, streamableHTTPEndpointPath, "Host: "+rsHost, "Authorization: Bearer "+token); !strings.Contains(got, "401") {
		t.Fatalf("refused token was not refused: %q", got)
	}

	entries := logs.FilterMessage(rsRefusalMessage).All()
	if len(entries) != 1 {
		t.Fatalf("logged %d refusal lines, want 1", len(entries))
	}
	reason, _ := entries[0].ContextMap()["reason"].(string)
	if reason == "" {
		t.Fatalf("refusal line has no reason field: %v", entries[0].ContextMap())
	}
	if len(reason) > maxLoggedHeaderLen+len("…(truncated)") {
		t.Fatalf("logged reason is %d bytes, want at most the %d-byte bound", len(reason), maxLoggedHeaderLen+len("…(truncated)"))
	}
	if !strings.HasSuffix(reason, "…(truncated)") {
		t.Fatalf("an oversized reason was cut without the marker: %q", reason)
	}
	if !strings.HasPrefix(reason, "invalid bearer token: ") || !strings.Contains(reason, `key "kkk`) {
		t.Fatalf("the cut reason lost its start: %q", reason)
	}
}

// A short reason is logged unchanged, with no cut marker.
func TestResourceServerRefusalLogKeepsAShortReason(t *testing.T) {
	rs, _ := portResourceServer(t)
	cfg := resolveTransportConfig(resourceServerConfig())

	handler, err := rs.handlerWithRefusals(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), cfg.refusals)
	if err != nil {
		t.Fatalf("handlerWithRefusals: %v", err)
	}
	ln := mustListenLoopback(t)
	srv := newMCPHTTPServer(handler, cfg)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	logs := observeLogs(t)

	if got := get(t, ln.Addr().String(), streamableHTTPEndpointPath, "Host: "+rsHost, "Authorization: Bearer not-a-jwt"); !strings.Contains(got, "401") {
		t.Fatalf("refused token was not refused: %q", got)
	}
	entries := logs.FilterMessage(rsRefusalMessage).All()
	if len(entries) != 1 {
		t.Fatalf("logged %d refusal lines, want 1", len(entries))
	}
	reason, _ := entries[0].ContextMap()["reason"].(string)
	if reason == "" || strings.Contains(reason, "(truncated)") {
		t.Fatalf("a short reason was altered or lost: %q", reason)
	}
}
