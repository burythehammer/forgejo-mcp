// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// The resource-server authentication port is tested at its own seam, as the
// passthrough port is: an inbound request and the resource-server configuration
// go in, and a credential or a refusal comes out. The HTTP tests in
// resource_server_http_test.go cover the same cases end to end.

// portResourceServer prepares resource-server mode against a fake Forgejo and
// identity provider, without serving anything. It returns the resourceServer
// and a rsEnv whose accessToken signs tokens from the fake provider.
func portResourceServer(t *testing.T) (*resourceServer, *rsEnv) {
	t.Helper()
	app := resourceServerConfig()
	fixture := newFixture(t, &app, "16.0.3", mustP256(t), "")
	app.Auth.ScopesSupported = []string{"openid", "profile"}
	app.Auth.ResourceAudience = rsClientID
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	rs, err := prepareResourceServer(context.Background(), app, newFactory(app, "test"))
	if err != nil {
		t.Fatalf("prepareResourceServer: %v", err)
	}
	return rs, &rsEnv{fixture: fixture, rs: rs}
}

// portChallenge is the challenge the transport passes the port: the same value
// rs.handler builds for the protected route.
func portChallenge(rs *resourceServer) string {
	return bearerChallenge(rsMetadataURL, rs.scopes)
}

func TestAuthenticateResourceServerAcceptsAValidTokenAndMintsForForgejo(t *testing.T) {
	rs, env := portResourceServer(t)
	r := requestWithAuth(t, "Bearer "+env.accessToken(t, nil))

	got, err := authenticateResourceServer(r, rs, portChallenge(rs))
	if err != nil {
		t.Fatalf("refused a valid token: %v", err)
	}
	if got.subject != rsSubject || got.audience != rsForgejoAud {
		t.Fatalf("subject, audience = %q, %q; want %q, %q", got.subject, got.audience, rsSubject, rsForgejoAud)
	}
	if got.credential == "" {
		t.Fatal("a valid token produced no credential")
	}
	if got.credential == credential(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
		t.Fatal("the provider's token reached the handler as the credential")
	}
}

func TestAuthenticateResourceServerMintedTokenCarriesTheForgejoAudience(t *testing.T) {
	rs, env := portResourceServer(t)
	got, err := authenticateResourceServer(requestWithAuth(t, "Bearer "+env.accessToken(t, nil)), rs, portChallenge(rs))
	if err != nil {
		t.Fatalf("refused a valid token: %v", err)
	}
	claims := jwtClaims(t, string(got.credential))
	if claims["sub"] != rsSubject {
		t.Fatalf("minted sub = %v, want %q", claims["sub"], rsSubject)
	}
	if aud, _ := claims["aud"].(string); aud != rsForgejoAud {
		t.Fatalf("minted aud = %v, want %q", claims["aud"], rsForgejoAud)
	}
	if iss, _ := claims["iss"].(string); iss != rs.issuer.URL() {
		t.Fatalf("minted iss = %v, want %q", claims["iss"], rs.issuer.URL())
	}
}

func TestAuthenticateResourceServerRefusesAMissingTokenWithTheBareChallenge(t *testing.T) {
	rs, env := portResourceServer(t)
	// A bare token with no scheme is no bearer token at all, so it is a missing
	// token, not an invalid one.
	bare := env.accessToken(t, nil)
	for _, auth := range []string{"", "Basic abc", "Bearer ", bare} {
		_, err := authenticateResourceServer(requestWithAuth(t, auth), rs, portChallenge(rs))
		ref := requireResourceRefusal(t, err)
		if ref.status != http.StatusUnauthorized {
			t.Fatalf("Authorization %q: status = %d, want 401", auth, ref.status)
		}
		if ref.challenge != portChallenge(rs) {
			t.Fatalf("Authorization %q: challenge = %q, want the bare challenge %q", auth, ref.challenge, portChallenge(rs))
		}
		if strings.Contains(ref.challenge, "invalid_token") {
			t.Fatalf("Authorization %q: a missing token was reported as invalid: %q", auth, ref.challenge)
		}
		if ref.body != "" {
			t.Fatalf("Authorization %q: 401 carries a body %q", auth, ref.body)
		}
	}
}

func TestAuthenticateResourceServerRefusesAnInvalidTokenWithInvalidTokenChallenge(t *testing.T) {
	rs, env := portResourceServer(t)
	tokens := map[string]string{
		"not a JWT":      "Bearer not-a-jwt",
		"wrong audience": "Bearer " + env.accessToken(t, func(c map[string]any) { c["aud"] = []string{"someone-else"} }),
		"expired":        "Bearer " + env.accessToken(t, func(c map[string]any) { c["exp"] = 1 }),
		"no subject":     "Bearer " + env.accessToken(t, func(c map[string]any) { delete(c, "sub") }),
	}
	for name, auth := range tokens {
		t.Run(name, func(t *testing.T) {
			_, err := authenticateResourceServer(requestWithAuth(t, auth), rs, portChallenge(rs))
			ref := requireResourceRefusal(t, err)
			if ref.status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", ref.status)
			}
			want := portChallenge(rs) + `, error="invalid_token"`
			if ref.challenge != want {
				t.Fatalf("challenge = %q, want %q", ref.challenge, want)
			}
		})
	}
}

func TestAuthenticateResourceServerRefusesATokenWithoutAUsableAudienceWithoutAChallenge(t *testing.T) {
	rs, env := portResourceServer(t)
	auth := "Bearer " + env.accessToken(t, func(c map[string]any) { delete(c, "forgejo_aud") })

	_, err := authenticateResourceServer(requestWithAuth(t, auth), rs, portChallenge(rs))
	ref := requireResourceRefusal(t, err)
	if ref.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", ref.status)
	}
	if ref.challenge != "" {
		t.Fatalf("403 carries a challenge %q; it must not invite a re-authorization loop", ref.challenge)
	}
	if !strings.Contains(ref.body, "forgejo_aud") {
		t.Fatalf("403 body does not name the claim: %q", ref.body)
	}
}

// requireResourceRefusal asserts that err is a refusal and returns it.
func requireResourceRefusal(t *testing.T, err error) *resourceRefusal {
	t.Helper()
	var ref *resourceRefusal
	if !errors.As(err, &ref) {
		t.Fatalf("got error %v, want a refusal", err)
	}
	return ref
}
