// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// The decoy below is never a real credential and never reaches a network: these
// tests assert which value is selected, not that it authenticates.
const decoyOperatorCredential = "decoy-operator-value-not-a-credential"

func policyFactory(require bool, operatorValue string) *Factory {
	return NewFactory(config.Forgejo{Token: config.NewSecret(operatorValue)}, "test", WithRequireRequestToken(require))
}

func TestRequestCredentialAlwaysWins(t *testing.T) {
	t.Parallel()
	f := policyFactory(true, decoyOperatorCredential)
	ctx := forge.WithToken(context.Background(), "per-request-value")
	got, err := f.tokenFor(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "per-request-value" {
		t.Fatalf("got %q, want the per-request value", got)
	}
}

func TestOperatorCredentialStandsInWhenTheFallbackIsAllowed(t *testing.T) {
	// This is the stdio behaviour, and the loopback-listener behaviour. It is
	// the control: if this fails, the change has broken the supported case
	// rather than the attack.
	t.Parallel()
	f := policyFactory(false, decoyOperatorCredential)
	got, err := f.tokenFor(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != decoyOperatorCredential {
		t.Fatalf("got %q, want the operator value", got)
	}
}

func TestAnonymousRequestIsRefusedWhenTheFallbackIsOff(t *testing.T) {
	t.Parallel()
	f := policyFactory(true, decoyOperatorCredential)
	got, err := f.tokenFor(context.Background())
	if !errors.Is(err, forge.ErrNoRequestToken) {
		t.Fatalf("got (%q, %v), want ErrNoRequestToken", got, err)
	}
	if got != "" {
		t.Fatalf("a refused request must yield no credential, got %q", got)
	}
}

func TestEmptyRequestCredentialIsNotAnIdentity(t *testing.T) {
	// An Authorization header present but empty must not be treated as an
	// identity that then falls through to the operator's own value.
	t.Parallel()
	f := policyFactory(true, decoyOperatorCredential)
	ctx := forge.WithToken(context.Background(), "")
	if _, err := f.tokenFor(ctx); !errors.Is(err, forge.ErrNoRequestToken) {
		t.Fatalf("empty per-request credential was accepted: %v", err)
	}
}

func TestFactoryRefusesRatherThanReturningTheOperatorPort(t *testing.T) {
	// The refusal lives at the single fallback site, For, so a caller cannot
	// reach an adapter bound to the operator's credential around the policy.
	// No server is involved: the refusal comes before any client is built.
	t.Parallel()
	f := policyFactory(true, decoyOperatorCredential)
	port, err := f.For(context.Background())
	if !errors.Is(err, forge.ErrNoRequestToken) {
		t.Fatalf("For did not refuse: %v", err)
	}
	if port != nil {
		t.Fatal("a refused request must yield no port")
	}
}

// A factory built with no policy is a stdio factory: the network transports
// are the only ones that select the stricter policy. So the default must be
// "fall back to the operator's credential".
func TestOperatorFallbackIsAvailableUntilANetworkTransportSwitchesItOff(t *testing.T) {
	t.Parallel()
	f := NewFactory(config.Forgejo{Token: config.NewSecret(decoyOperatorCredential)}, "test")
	if f.RequireRequestToken() {
		t.Fatal("the policy requires a per-request token before any transport selected it")
	}
	got, err := f.tokenFor(context.Background())
	if err != nil || got != decoyOperatorCredential {
		t.Fatalf("got (%q, %v), want the operator credential on the initial policy", got, err)
	}
}
