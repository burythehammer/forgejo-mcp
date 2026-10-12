// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

// bindProbe stands in for the MCP handler behind the policy stack. It records
// how many requests got past the door and which Authorization value the last
// one carried, which are the two things the trust invariant is about.
type bindProbe struct {
	mu            sync.Mutex
	calls         int
	authorization string
}

func (p *bindProbe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.calls++
	p.authorization = r.Header.Get("Authorization")
	p.mu.Unlock()
	_, _ = io.WriteString(w, "reached the handler")
}

func (p *bindProbe) snapshot() (int, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls, p.authorization
}

// bindType is one way an operator can ask for the listener to bind.
type bindType struct {
	name string
	// host returns the -host value, or "" when this machine cannot offer it.
	host func() string
	// loopback is what the bind is expected to resolve to: true for the loopback
	// names and addresses, false for anything a network peer can reach.
	loopback bool
	// optional marks a bind that depends on the machine (IPv6 enabled, a non-
	// loopback interface present); it is skipped, not failed, where absent.
	optional bool
}

func literalHost(host string) func() string { return func() string { return host } }

// lanIPv4 returns an address of a non-loopback interface, or "" when there is
// none. It is what a LAN-reachable bind looks like from the machine itself.
func lanIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}
		return ipNet.IP.String()
	}
	return ""
}

// bindTypes lists every bind type the configuration can produce: the loopback
// name (which binds both families), each loopback address family, the
// unspecified address (what a bare ":port" produces) and a network address.
func bindTypes() []bindType {
	return []bindType{
		{name: "loopback name", host: literalHost("localhost"), loopback: true},
		{name: "loopback IPv4 address", host: literalHost("127.0.0.1"), loopback: true},
		{name: "loopback IPv6 address", host: literalHost("::1"), loopback: true, optional: true},
		{name: "unspecified IPv4 address", host: literalHost("0.0.0.0")},
		{name: "network interface address", host: lanIPv4, optional: true},
	}
}

// probeRequest sends one GET to the Streamable HTTP endpoint on dial, with the
// given Host header and extra headers, and returns the response and its body.
func probeRequest(t *testing.T, dial, host string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	t.Cleanup(client.CloseIdleConnections)

	req, err := http.NewRequest(http.MethodGet, "http://"+dial+streamableHTTPEndpointPath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request to %s as Host %q: %v", dial, host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

// TestBindAddressNeverGrantsTrust starts the real policy stack on the listeners
// each bind type produces, and checks that the bind decides only the Host
// policy, never the credential policy. A request with no Authorization header
// is refused on every bind type, and the handler behind the door never sees it.
func TestBindAddressNeverGrantsTrust(t *testing.T) {
	const declared = "mcp.example.org"
	for _, bt := range bindTypes() {
		t.Run(bt.name, func(t *testing.T) {
			host := bt.host()
			if host == "" {
				t.Skip("this machine has no address for this bind type")
			}
			app := defaultListenConfig()
			app.Listen.Host = host
			app.Listen.AllowedHosts = []string{declared}
			if err := app.Validate(); err != nil {
				t.Fatalf("configuration for %q: %v", host, err)
			}
			cfg := resolveTransportConfig(app)
			if cfg.loopbackOnly != bt.loopback {
				t.Fatalf("%q resolved loopbackOnly=%v, want %v", host, cfg.loopbackOnly, bt.loopback)
			}

			listeners, err := bindListeners("http", cfg, 0, net.Listen)
			if err != nil {
				if bt.optional {
					t.Skipf("cannot bind %q here: %v", host, err)
				}
				t.Fatalf("bind %q: %v", host, err)
			}
			t.Cleanup(func() {
				for _, ln := range listeners {
					_ = ln.Close()
				}
			})

			probe := &bindProbe{}
			srv := newMCPHTTPServer(probe, cfg)
			for _, ln := range listeners {
				go func(ln net.Listener) { _ = srv.Serve(ln) }(ln)
			}
			t.Cleanup(func() { _ = srv.Close() })

			for _, ln := range listeners {
				dial := ln.Addr().String()
				if tcp, ok := ln.Addr().(*net.TCPAddr); ok && tcp.IP.IsUnspecified() {
					// A client on this machine reaches an unspecified bind via loopback.
					dial = net.JoinHostPort("127.0.0.1", strconv.Itoa(tcp.Port))
				}
				checkBindTrust(t, bt, probe, dial, declared)
			}
		})
	}
}

// checkBindTrust runs the request matrix against one bound listener. The
// subtests are sequential and each one states what the bind is allowed to change.
func checkBindTrust(t *testing.T, bt bindType, probe *bindProbe, dial, declared string) {
	t.Helper()
	const decoy = "Bearer decoy-bind-trust-value"

	t.Run("anonymous request on "+dial, func(t *testing.T) {
		before, _ := probe.snapshot()
		resp, body := probeRequest(t, dial, declared, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("anonymous request got %d, want 401: %q", resp.StatusCode, body)
		}
		if got := resp.Header.Get("WWW-Authenticate"); got != `Bearer realm="forgejo-mcp"` {
			t.Errorf("WWW-Authenticate = %q", got)
		}
		if after, _ := probe.snapshot(); after != before {
			t.Fatal("the handler ran for an anonymous request")
		}
	})

	t.Run("credentialed request on "+dial, func(t *testing.T) {
		resp, body := probeRequest(t, dial, declared, map[string]string{"Authorization": decoy})
		if resp.StatusCode != http.StatusOK || body != "reached the handler" {
			t.Fatalf("credentialed request got %d %q, want 200 from the handler", resp.StatusCode, body)
		}
		if _, auth := probe.snapshot(); auth != decoy {
			t.Fatalf("the handler saw Authorization %q, want the one the client sent", auth)
		}
	})

	t.Run("loopback Host on "+dial, func(t *testing.T) {
		// Loopback names are accepted only where the listener cannot be reached
		// from the network. Off loopback, the declared list is the whole answer.
		before, _ := probe.snapshot()
		resp, _ := probeRequest(t, dial, dial, map[string]string{"Authorization": decoy})
		after, _ := probe.snapshot()
		if bt.loopback {
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("loopback Host on a loopback bind got %d, want 200", resp.StatusCode)
			}
			return
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("loopback Host on a network bind got %d, want 403", resp.StatusCode)
		}
		if after != before {
			t.Fatal("the handler ran for a refused Host")
		}
	})

	t.Run("undeclared Host on "+dial, func(t *testing.T) {
		before, _ := probe.snapshot()
		resp, _ := probeRequest(t, dial, "attacker.example.com", map[string]string{"Authorization": decoy})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("undeclared Host got %d, want 403", resp.StatusCode)
		}
		if after, _ := probe.snapshot(); after != before {
			t.Fatal("the handler ran for an undeclared Host")
		}
	})

	t.Run("cross-site Origin on "+dial, func(t *testing.T) {
		before, _ := probe.snapshot()
		resp, _ := probeRequest(t, dial, declared, map[string]string{
			"Authorization": decoy,
			"Origin":        "https://attacker.example.com",
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("cross-site Origin got %d, want 403", resp.StatusCode)
		}
		if after, _ := probe.snapshot(); after != before {
			t.Fatal("the handler ran for a refused Origin")
		}
	})
}
