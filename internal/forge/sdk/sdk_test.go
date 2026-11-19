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

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/forgetest"
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

func TestFactoryServesTheCredentialItIsGiven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		token      string // a request credential, as the port attaches it
		operator   bool   // the port's mark for the operator's credential
		wantErr    error
		wantSecret string // credential that must reach Forgejo; "" when refused
	}{
		{"request credential is used", requestValue, false, nil, requestValue},
		{"operator mark is served with the operator's credential", "", true, nil, operatorValue},
		{"unmarked request with no credential is refused", "", false, forge.ErrNoRequestToken, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newStub(t)
			cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret(operatorValue)}

			ctx := context.Background()
			if tc.token != "" {
				ctx = forge.WithToken(ctx, tc.token)
			}
			if tc.operator {
				ctx = forge.WithOperatorCredential(ctx)
			}
			port, err := NewFactory(cfg, "test").For(ctx)

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
	t.Parallel()
	srv := newStub(t)
	cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret(operatorValue)}

	f := NewFactory(cfg, "test")
	if _, err := f.For(forge.WithToken(context.Background(), requestValue)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.For(forge.WithOperatorCredential(context.Background())); err != nil {
		t.Fatal(err)
	}
	creds := srv.credentials()
	if strings.Count(creds, requestValue) != 1 || !strings.Contains(creds, operatorValue) {
		t.Fatalf("credentials %q: want request token once, then the operator's", creds)
	}
}
