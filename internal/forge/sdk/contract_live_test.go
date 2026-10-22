// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Environment variables that switch on the live contract driver. The token is
// supplied by a human at run time; nothing reads it from a file.
const (
	liveURLEnv   = "FORGEJO_URL"
	liveTokenEnv = "FORGEJO_ACCESS_TOKEN"
)

// TestHooksContractLive runs the hook contract against a real Forgejo through
// the SDK adapter. It skips unless both environment variables are set, so
// go test ./... stays offline. Each case works in a throwaway repository that
// is deleted when the case ends, including when the case fails.
func TestHooksContractLive(t *testing.T) {
	url, token := os.Getenv(liveURLEnv), os.Getenv(liveTokenEnv)
	if url == "" || token == "" {
		t.Skipf("set %s and %s to run the contract against a real Forgejo", liveURLEnv, liveTokenEnv)
	}
	t.Parallel()
	forgetest.RunHooks(t, liveHarness(url, token))
}

// liveHarness builds a fresh adapter and throwaway repository for each case,
// owned by the token's user.
func liveHarness(url, token string) func(t *testing.T) forgetest.Harness {
	return func(t *testing.T) forgetest.Harness {
		t.Helper()
		ctx := forge.WithToken(context.Background(), token)
		f := NewFactory(config.Forgejo{URL: url, Token: config.NewSecret(token)}, "live-contract")
		port, err := f.For(ctx)
		if err != nil {
			t.Fatalf("building adapter for %s: %v", url, err)
		}
		me, err := port.GetMyUserInfo(ctx)
		if err != nil {
			t.Fatalf("reading the token's user: %v", err)
		}
		client, err := forgejo_sdk.NewClient(url, forgejo_sdk.SetToken(token))
		if err != nil {
			t.Fatalf("building SDK client for %s: %v", url, err)
		}
		name := fmt.Sprintf("contract-%d", time.Now().UnixNano())
		return forgetest.Harness{
			Port:  port,
			Seed:  &liveSeed{t: t, client: client, owner: me.UserName, name: name},
			Owner: me.UserName,
			Repo:  name,
		}
	}
}

// liveSeed creates the case's repository on the first AddRepo and deletes it
// in the case's cleanup. The hook cases need no other seeding. Any other
// Seeder method reaches the nil embedded interface and panics, which is the
// signal that a case needs a live seeder this driver does not have yet.
type liveSeed struct {
	forgetest.Seeder
	t       *testing.T
	client  *forgejo_sdk.Client
	owner   string
	name    string
	created bool
}

func (s *liveSeed) AddRepo(owner, repo string) {
	s.t.Helper()
	if owner != s.owner || repo != s.name {
		s.t.Fatalf("live case works in %s/%s, not %s/%s", s.owner, s.name, owner, repo)
	}
	if s.created {
		return
	}
	if _, _, err := s.client.CreateRepo(forgejo_sdk.CreateRepoOption{Name: s.name, Private: true}); err != nil {
		s.t.Fatalf("creating throwaway repo %s/%s: %v", s.owner, s.name, err)
	}
	s.created = true
	s.t.Cleanup(func() {
		if _, err := s.client.DeleteRepo(s.owner, s.name); err != nil {
			s.t.Errorf("deleting throwaway repo %s/%s: %v", s.owner, s.name, err)
		}
	})
}
