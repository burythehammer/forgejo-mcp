// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The authorization header is write-only: the fake records it for the edit
// that set it, and the port's Hook cannot read it back.
func TestEditRepoHookRecordsAuthorizationHeader(t *testing.T) {
	t.Parallel()
	f := New()
	f.AddRepo("o", "r")
	hk, err := f.CreateRepoHook(context.Background(), "o", "r", forgejo_sdk.CreateHookOption{
		Type:   forgejo_sdk.HookTypeForgejo,
		Config: map[string]string{"url": "https://example.test/a"},
		Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	header := "Bearer decoy-token"
	if err := f.EditRepoHook(context.Background(), "o", "r", hk.ID, forge.HookEdit{AuthorizationHeader: &header}); err != nil {
		t.Fatal(err)
	}
	if got := f.HookAuthorizationHeader(hk.ID); got != header {
		t.Fatalf("authorization header = %q, want %q", got, header)
	}

	// An edit that does not pass it keeps the stored one.
	off := false
	if err := f.EditRepoHook(context.Background(), "o", "r", hk.ID, forge.HookEdit{Active: &off}); err != nil {
		t.Fatal(err)
	}
	if got := f.HookAuthorizationHeader(hk.ID); got != header {
		t.Fatalf("authorization header = %q after unrelated edit, want %q", got, header)
	}
}
