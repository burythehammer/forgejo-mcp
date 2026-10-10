// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// Decoys, never real credentials.
const (
	operatorValue = "decoy-operator-value"
	requestValue  = "decoy-request-value"
)

// stub stands in for Forgejo's version endpoint, which the SDK probes when a
// client is built, and records the credential each probe carried.
type stub struct {
	*httptest.Server
	mu   sync.Mutex
	auth []string
}

func newStub(t *testing.T) *stub {
	t.Helper()
	s := &stub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		s.mu.Unlock()
		forgetest.WriteVersion(w)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *stub) credentials() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.auth, "|")
}

func TestFactoryAppliesCredentialPolicy(t *testing.T) {
	tests := []struct {
		name       string
		require    bool // network transport without operator opt-in
		token      string
		wantErr    error
		wantSecret string // credential that must reach Forgejo; "" when refused
	}{
		{"request token on stdio", false, requestValue, nil, requestValue},
		{"request token on network", true, requestValue, nil, requestValue},
		{"stdio falls back to operator", false, "", nil, operatorValue},
		{"network refuses without token", true, "", forgejo.ErrNoRequestToken, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newStub(t)
			cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret(operatorValue)}
			forgejo.Configure(cfg, "test")
			forgejo.ResetClientForTesting()
			prev := forgejo.RequireRequestToken()
			forgejo.SetRequireRequestToken(tc.require)
			t.Cleanup(func() {
				forgejo.SetRequireRequestToken(prev)
				forgejo.ResetClientForTesting()
			})

			ctx := context.Background()
			if tc.token != "" {
				ctx = forgejo.WithToken(ctx, tc.token)
			}
			port, err := NewFactory(cfg).For(ctx)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got err %v, want %v", err, tc.wantErr)
				}
				if port != nil {
					t.Fatalf("a refused request must yield no port, got %v", port)
				}
				if got := srv.credentials(); got != "" {
					t.Fatalf("a refused request must not reach Forgejo, saw %q", got)
				}
				return
			}
			if err != nil || port == nil {
				t.Fatalf("got (%v, %v), want a port", port, err)
			}
			if got := srv.credentials(); !strings.Contains(got, tc.wantSecret) {
				t.Fatalf("credential seen by Forgejo %q does not carry %q", got, tc.wantSecret)
			}
		})
	}
}

func TestRequestTokenIsForThatRequestOnly(t *testing.T) {
	srv := newStub(t)
	cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret(operatorValue)}
	forgejo.Configure(cfg, "test")
	forgejo.ResetClientForTesting()
	t.Cleanup(forgejo.ResetClientForTesting)

	f := NewFactory(cfg)
	if _, err := f.For(forgejo.WithToken(context.Background(), requestValue)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.For(context.Background()); err != nil {
		t.Fatal(err)
	}
	creds := srv.credentials()
	if strings.Count(creds, requestValue) != 1 || !strings.Contains(creds, operatorValue) {
		t.Fatalf("credentials %q: want request token once, then the operator's", creds)
	}
}
