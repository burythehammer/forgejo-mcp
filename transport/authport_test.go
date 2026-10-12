// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// The authentication port is tested at its own seam: an inbound request and a
// Config go in, and a credential or a refusal comes out. The transport tests in
// listen_test.go and the race tests cover the same behaviour end to end.

func passthroughConfig(fallback bool) config.Config {
	return config.Config{
		Transport: "http",
		Listen:    config.Listen{Host: "localhost", AllowOperatorTokenFallback: fallback},
	}
}

func requestWithAuth(t *testing.T, auth string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func TestAuthenticatePassthrough(t *testing.T) {
	tests := []struct {
		name     string
		auth     string
		fallback bool
		want     credential
		refused  bool
	}{
		// A request carrying its own credential is accepted, whatever the policy.
		{"bearer token accepted", "Bearer per-request", false, "per-request", false},
		{"token scheme accepted", "token per-request", false, "per-request", false},
		{"scheme is case-insensitive", "BEARER per-request", false, "per-request", false},
		{"credential accepted with fallback allowed", "Bearer per-request", true, "per-request", false},

		// No usable credential: refused when the operator's credential may not
		// stand in, and passed through with no credential when it may.
		{"no header refused", "", false, "", true},
		{"no header with fallback passes empty", "", true, "", false},
		{"bare token refused", "per-request", false, "", true},
		{"bare token with fallback passes empty", "per-request", true, "", false},
		{"unrecognised scheme refused", "Basic per-request", false, "", true},
		{"unrecognised scheme with fallback passes empty", "Basic per-request", true, "", false},
		{"empty credential after scheme refused", "Bearer ", false, "", true},
		{"empty credential after scheme with fallback passes empty", "Bearer ", true, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := authenticatePassthrough(requestWithAuth(t, tt.auth), passthroughConfig(tt.fallback))
			if tt.refused {
				if !errors.Is(err, errCredentialRequired) {
					t.Fatalf("got (%q, %v), want refusal errCredentialRequired", got, err)
				}
				if got != "" {
					t.Fatalf("refused request carried credential %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("got refusal %v, want credential %q", err, tt.want)
			}
			if got != tt.want {
				t.Fatalf("got credential %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttachCredentialCarriesOnlyAPresentCredential(t *testing.T) {
	ctx := attachCredential(context.Background(), "per-request")
	if got := forge.TokenFromContext(ctx); got != "per-request" {
		t.Fatalf("handler saw credential %q, want %q", got, "per-request")
	}

	// An absent credential attaches nothing, so the factory's own policy
	// decides, exactly as when no header was sent.
	bare := attachCredential(context.Background(), "")
	if got := forge.TokenFromContext(bare); got != "" {
		t.Fatalf("empty credential attached %q", got)
	}
}

// The fallback policy is the port's: a passthrough request yields the caller's
// own credential, else the operator's when the flag allows it, else a refusal.
// The factory only sees the result as a context value.

func TestAttachPassthroughCarriesTheRequestCredential(t *testing.T) {
	ctx := attachPassthroughCredential(context.Background(), "per-request")
	if got := forge.TokenFromContext(ctx); got != "per-request" {
		t.Fatalf("handler saw credential %q, want %q", got, "per-request")
	}
	if forge.UsesOperatorCredential(ctx) {
		t.Fatal("a request carrying its own credential was marked for the operator's")
	}
}

func TestAttachPassthroughMarksTheOperatorWhenNoCredentialIsGiven(t *testing.T) {
	ctx := attachPassthroughCredential(context.Background(), "")
	if !forge.UsesOperatorCredential(ctx) {
		t.Fatal("a request the port passed with no credential was not marked for the operator's")
	}
	if got := forge.TokenFromContext(ctx); got != "" {
		t.Fatalf("operator marking also attached request credential %q", got)
	}
}

func TestPassthroughFallbackCases(t *testing.T) {
	tests := []struct {
		name     string
		auth     string
		fallback bool
		// want is what the handler context must carry: a request credential,
		// the operator's credential, or nothing (refused before the handler).
		wantToken    string
		wantOperator bool
	}{
		{"bearer credential, fallback off", "Bearer per-request", false, "per-request", false},
		{"bearer credential, fallback on", "Bearer per-request", true, "per-request", false},
		{"token scheme credential, fallback off", "token per-request", false, "per-request", false},
		{"no header, fallback off: refused", "", false, "", false},
		{"no header, fallback on: operator", "", true, "", true},
		{"bare token, fallback off: refused", "per-request", false, "", false},
		{"bare token, fallback on: operator", "per-request", true, "", true},
		{"unrecognised scheme, fallback off: refused", "Basic per-request", false, "", false},
		{"unrecognised scheme, fallback on: operator", "Basic per-request", true, "", true},
		{"empty credential after scheme, fallback off: refused", "Bearer ", false, "", false},
		{"empty credential after scheme, fallback on: operator", "Bearer ", true, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := requestWithAuth(t, tt.auth)
			ctx := withRequestToken(passthroughConfig(tt.fallback), r.Context(), r)
			if got := forge.TokenFromContext(ctx); got != tt.wantToken {
				t.Fatalf("handler credential %q, want %q", got, tt.wantToken)
			}
			if got := forge.UsesOperatorCredential(ctx); got != tt.wantOperator {
				t.Fatalf("operator marking %v, want %v", got, tt.wantOperator)
			}
		})
	}
}

func TestStdioAlwaysFallsBackToTheOperator(t *testing.T) {
	// The client launched this process, so there is no per-request identity to
	// carry and the operator's credential is the only one. The flag does not
	// apply to stdio.
	ctx := attachStdioCredential(context.Background())
	if !forge.UsesOperatorCredential(ctx) {
		t.Fatal("stdio request was not marked for the operator's credential")
	}
	if got := forge.TokenFromContext(ctx); got != "" {
		t.Fatalf("stdio request carried request credential %q", got)
	}
}
