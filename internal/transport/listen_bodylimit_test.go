// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/upload"
)

// bodyProbe stands in for the MCP handler behind the door. It reads the whole
// body, as mcp-go does, and records that it was reached.
type bodyProbe struct {
	reached       bool
	read          int
	contentLength int64
}

func (p *bodyProbe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.reached = true
	p.contentLength = r.ContentLength
	b, err := io.ReadAll(r.Body)
	p.read = len(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// bodyLimitDoor runs guardRequests with a small ceiling so the tests do not
// allocate the production default. authorized decides whether an Authorization
// header is present; nil means the door does not look at it.
func bodyLimitDoor(t *testing.T, probe http.Handler, limit int64, authorized bool) *httptest.Server {
	t.Helper()
	var authorize func(*http.Request) error
	if authorized {
		authorize = func(r *http.Request) error {
			if r.Header.Get("Authorization") == "" {
				return io.EOF
			}
			return nil
		}
	}
	h := guardRequests(probe, hostPolicy{loopbackOnly: true}, originPolicy{}, authorize, &refusalLimiter{}, limit)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

// chunkedReader hides its length, so the client sends the body chunked and the
// server sees ContentLength -1.
type chunkedReader struct{ r io.Reader }

func (c chunkedReader) Read(p []byte) (int, error) { return c.r.Read(p) }

func TestBodyLimit_DeclaredLength(t *testing.T) {
	const limit = 1024
	cases := []struct {
		name       string
		size       int
		auth       bool
		authHeader bool
		wantStatus int
		wantReach  bool
	}{
		{"just under, no auth header", limit - 1, false, false, http.StatusOK, true},
		{"exactly at the limit, no auth header", limit, false, false, http.StatusOK, true},
		{"just over, no auth header", limit + 1, false, false, http.StatusRequestEntityTooLarge, false},
		{"just over, with auth header", limit + 1, true, true, http.StatusRequestEntityTooLarge, false},
		{"just under, with auth header", limit - 1, true, true, http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := &bodyProbe{}
			ts := bodyLimitDoor(t, probe, limit, tc.auth)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader(make([]byte, tc.size)))
			if err != nil {
				t.Fatal(err)
			}
			if tc.authHeader {
				req.Header.Set("Authorization", "Bearer x")
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if probe.reached != tc.wantReach {
				t.Fatalf("handler reached = %v, want %v", probe.reached, tc.wantReach)
			}
			if tc.wantReach && probe.read != tc.size {
				t.Fatalf("handler read %d bytes, want %d", probe.read, tc.size)
			}
		})
	}
}

func TestBodyLimit_RefusalBodyNamesTheLimit(t *testing.T) {
	const limit = 1024
	ts := bodyLimitDoor(t, &bodyProbe{}, limit, false)
	resp, err := http.Post(ts.URL+"/sse", "text/plain", bytes.NewReader(make([]byte, limit+1)))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
	text := string(body)
	if !strings.Contains(strings.ToLower(text), "too large") || !strings.Contains(text, strconv.Itoa(limit)) {
		t.Fatalf("refusal body %q does not say the request was too large and the limit %d", text, limit)
	}
}

func TestBodyLimit_ChunkedBody(t *testing.T) {
	const limit = 1024
	cases := []struct {
		name       string
		size       int
		wantStatus int
		wantReach  bool
	}{
		{"just under", limit - 1, http.StatusOK, true},
		{"just over", limit + 1, http.StatusRequestEntityTooLarge, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := &bodyProbe{}
			ts := bodyLimitDoor(t, probe, limit, false)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp",
				chunkedReader{bytes.NewReader(make([]byte, tc.size))})
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			// The probe must have seen a chunked request, or the test proves
			// only the declared-length check.
			if !probe.reached || probe.contentLength != -1 {
				t.Fatalf("handler reached = %v with ContentLength %d; want reached and -1 (chunked)", probe.reached, probe.contentLength)
			}
		})
	}
}

// The default must clear the largest body a legitimate request can carry: a
// base64 upload at the upload cap, plus the JSON-RPC envelope and the other
// arguments. The envelope allowance is generous, so a future envelope change
// does not silently cut off uploads.
func TestBodyLimit_DefaultClearsLargestUpload(t *testing.T) {
	const envelopeAllowance = 1 << 20
	if maxRequestBodyBytes < int64(upload.MaxContentB64Bytes)+envelopeAllowance {
		t.Fatalf("maxRequestBodyBytes = %d, want at least upload cap %d plus %d bytes of envelope",
			maxRequestBodyBytes, upload.MaxContentB64Bytes, envelopeAllowance)
	}
}
