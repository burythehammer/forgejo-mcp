// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

// The fallback policy is the authentication port's and is pinned in
// transport/authport_test.go. These tests pin the factory's half: it serves the
// credential a request is given and refuses everything else.

// The decoy below is never a real credential and never reaches a network: these
// tests assert which value is selected, not that it authenticates.
const decoyOperatorCredential = "decoy-operator-value-not-a-credential"

func TestAnonymousRequestIsRefusedWithoutACredential(t *testing.T) {
	// The factory holds an operator credential, and still refuses a request that
	// has neither a credential nor the port's mark. It decides nothing about
	// whether the operator's credential may stand in.
	t.Parallel()
	f := NewFactory(config.Forgejo{Token: config.NewSecret(decoyOperatorCredential)}, "test")
	port, err := f.For(context.Background())
	if !errors.Is(err, forge.ErrNoRequestToken) {
		t.Fatalf("got (%v, %v), want ErrNoRequestToken", port, err)
	}
	if port != nil {
		t.Fatal("a refused request must yield no port")
	}
}

func TestEmptyRequestCredentialIsNotAnIdentity(t *testing.T) {
	// A context carrying an empty request credential is no identity. The port
	// never attaches one, and the factory must not serve it with the operator's.
	t.Parallel()
	f := NewFactory(config.Forgejo{Token: config.NewSecret(decoyOperatorCredential)}, "test")
	if _, err := f.For(forge.WithToken(context.Background(), "")); !errors.Is(err, forge.ErrNoRequestToken) {
		t.Fatalf("empty request credential was accepted: %v", err)
	}
}

func TestMarkedRequestIsServedWhenTheOperatorCredentialIsEmpty(t *testing.T) {
	// An anonymous instance served over stdio is configured with no operator
	// credential. The mark is what lets that request through, and it must still
	// be served rather than refused for want of a value.
	t.Parallel()
	srv := newStub(t)
	f := NewFactory(config.Forgejo{URL: srv.URL}, "test")
	port, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil || port == nil {
		t.Fatalf("marked request with no operator credential refused: port=%v err=%v", port, err)
	}
	if got := srv.credentials(); strings.Trim(got, "|") != "" {
		t.Fatalf("anonymous request carried credential %q", got)
	}
}

func TestMarkedRequestReusesTheSharedOperatorClient(t *testing.T) {
	// Stdio serves every tool call through the operator's mark, so those calls
	// must share one client as they did before the port took the policy.
	t.Parallel()
	srv := newStub(t)
	f := NewFactory(config.Forgejo{URL: srv.URL, Token: config.NewSecret(decoyOperatorCredential)}, "test")
	a, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.For(forge.WithOperatorCredential(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	if a.(*Adapter).client != b.(*Adapter).client {
		t.Fatal("operator-marked requests built separate clients; want the shared one")
	}
}
