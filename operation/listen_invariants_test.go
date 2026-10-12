// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type bindCase struct {
	name, host string
	allowed    []string
}

// bindCases is one row per kind of bind address the listener accepts. The test
// listener always sits on loopback; what varies is the configured address and
// so the policy resolved from it.
func bindCases() []bindCase {
	return []bindCase{
		{"loopback name", "localhost", nil},
		{"loopback IPv4", "127.0.0.1", nil},
		{"loopback IPv6", "::1", nil},
		{"all IPv4 interfaces", "0.0.0.0", []string{"mcp.example.org"}},
		{"all IPv6 interfaces", "::", []string{"mcp.example.org"}},
		{"named interface", "mcp.example.org", []string{"mcp.example.org"}},
	}
}

// do sends a real request to addr with the given Host header and returns the
// status, headers and (for a refusal) body, so a test can assert on all three.
func do(t *testing.T, addr, path, host string, headers map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Close the connection so an event stream cannot hold the read open: the
	// door answers before any stream starts, which is all these tests look at.
	req.Close = true
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return resp.StatusCode, resp.Header, ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, resp.Header, string(b)
}

// TestBindAddressNeverSkipsAuthentication pins the invariant that where the
// listener is bound is not a credential: an anonymous request is refused on
// every kind of bind address, and a credentialed one is let through.
func TestBindAddressNeverSkipsAuthentication(t *testing.T) {
	for _, tr := range transports() {
		for _, bc := range bindCases() {
			t.Run(tr.name+"/"+bc.name, func(t *testing.T) {
				app := defaultListenConfig()
				app.Listen.Host = bc.host
				app.Listen.AllowedHosts = bc.allowed
				if err := app.Validate(); err != nil {
					t.Fatalf("%s: %v", bc.host, err)
				}
				addr := startGuarded(t, tr, app)
				host := addr
				if len(bc.allowed) > 0 {
					host = bc.allowed[0]
				}

				status, hdr, body := do(t, addr, tr.path, host, nil)
				if status != http.StatusUnauthorized {
					t.Fatalf("anonymous request on %s: status %d, want 401 (body %q)", bc.host, status, body)
				}
				if hdr.Get("WWW-Authenticate") == "" {
					t.Errorf("anonymous refusal on %s carries no WWW-Authenticate challenge", bc.host)
				}

				status, _, body = do(t, addr, tr.path, host, map[string]string{"Authorization": tr.authorization(t)})
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					t.Fatalf("credentialed request on %s was refused: %d %q", bc.host, status, body)
				}
			})
		}
	}
}

// TestHostAndOriginRefusalsCarryStatusAndBody pins the response a refused
// request gets, not only that it is refused.
func TestHostAndOriginRefusalsCarryStatusAndBody(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			app := defaultListenConfig()
			addr := startGuarded(t, tr, app)
			auth := tr.authorization(t)

			status, _, body := do(t, addr, tr.path, "evil.example.com", map[string]string{"Authorization": auth})
			if status != http.StatusForbidden && status != http.StatusBadRequest {
				t.Errorf("forged Host: status %d, want a refusal", status)
			}
			if strings.TrimSpace(body) == "" {
				t.Error("forged Host: refusal has an empty body")
			}

			cross := map[string]string{"Authorization": auth, "Origin": "https://evil.example.com"}
			status, _, body = do(t, addr, tr.path, addr, cross)
			if status != http.StatusForbidden {
				t.Errorf("cross-site Origin: status %d, want 403", status)
			}
			if strings.TrimSpace(body) == "" {
				t.Error("cross-site Origin: refusal has an empty body")
			}
		})
	}
}
