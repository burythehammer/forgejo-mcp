// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"net/http"
	"strings"
	"testing"
)

// rsRefusalMessage is the protected route's debug line for a refused request.
const rsRefusalMessage = "Refused a request to the MCP endpoint"

// TestResourceServerRefusalsShareTheListenersBudget pins that, in
// resource-server mode, the protected route's refusal lines and the door's
// Host lines draw on the listener's one budget. The handler is built and served
// the way Run builds it. A flood of refused tokens that spends the burst must
// also silence the door's next refusal line.
func TestResourceServerRefusalsShareTheListenersBudget(t *testing.T) {
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
	addr := ln.Addr().String()
	logs := observeLogs(t)

	for i := 0; i < refusalLogBurst; i++ {
		if got := get(t, addr, streamableHTTPEndpointPath, "Host: "+rsHost, "Authorization: Bearer not-a-jwt"); !strings.Contains(got, "401") {
			t.Fatalf("refused token was not refused: %q", got)
		}
	}
	if n := logs.FilterMessage(rsRefusalMessage).Len(); n != refusalLogBurst {
		t.Fatalf("the protected route logged %d refusal lines, want the burst of %d", n, refusalLogBurst)
	}

	before := logs.FilterMessage(hostRefusalMessage).Len()
	if got := get(t, addr, streamableHTTPEndpointPath, "Host: attacker.example.com"); !strings.Contains(got, "403") {
		t.Fatalf("forged Host was not rejected: %q", got)
	}
	if after := logs.FilterMessage(hostRefusalMessage).Len(); after != before {
		t.Fatalf("the door logged a Host refusal after the route spent the listener's budget (%d before, %d after): "+
			"the route and the door do not share one budget", before, after)
	}
}
