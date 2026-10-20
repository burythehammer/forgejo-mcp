// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// These tests drive edit_repo_hook through its handler against the in-memory
// fake. They pin which arguments change a stored webhook: an argument that is
// absent leaves its field alone, and one the caller sent, empty or not, is
// applied.

const editOwner, editRepo = "octo", "hello"

// seedEditHook stores a webhook limited to one branch, with two events, so an
// edit that widened or reset it would show.
func seedEditHook(t *testing.T, f *fake.Forgejo) *forgejo_sdk.Hook {
	t.Helper()
	hk, err := f.CreateRepoHook(context.Background(), editOwner, editRepo, forgejo_sdk.CreateHookOption{
		Type:         forgejo_sdk.HookTypeForgejo,
		Config:       map[string]string{"url": "https://hooks.example/old", "content_type": "json", "secret": "stored-secret"},
		Events:       []string{"push", "create"},
		Active:       true,
		BranchFilter: "main",
	})
	if err != nil {
		t.Fatalf("seeding hook: %v", err)
	}
	return hk
}

// editCall runs edit_repo_hook with args on the hook and returns the stored
// hook afterwards.
func editCall(t *testing.T, f *fake.Forgejo, id int64, args map[string]any) *forgejo_sdk.Hook {
	t.Helper()
	h := handlers{forge: fake.Factory{Forgejo: f}}
	base := map[string]any{"owner": editOwner, "repo": editRepo, "id": float64(id)}
	for k, v := range args {
		base[k] = v
	}
	res, err := h.EditRepoHookFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}})
	if err != nil {
		t.Fatalf("EditRepoHookFn err: %v", err)
	}
	if res.IsError {
		t.Fatalf("edit returned an error result: %v", res.Content)
	}
	stored, err := f.GetRepoHook(context.Background(), editOwner, editRepo, id)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestEditRepoHookFn_UrlOnlyKeepsBranchFilterEventsAndActive(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(editOwner, editRepo)
	hk := seedEditHook(t, f)

	stored := editCall(t, f, hk.ID, map[string]any{"url": "https://hooks.example/new"})
	if stored.Config["url"] != "https://hooks.example/new" || stored.Config["branch_filter"] != "main" ||
		!stored.Active || len(stored.Events) != 2 || stored.Events[0] != "push" {
		t.Errorf("url-only edit changed other fields: %+v", stored)
	}
	if stored.Config["secret"] != "stored-secret" {
		t.Errorf("url-only edit dropped the stored secret: %q", stored.Config["secret"])
	}
}

func TestEditRepoHookFn_ExplicitEmptyBranchFilterClears(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(editOwner, editRepo)
	hk := seedEditHook(t, f)

	stored := editCall(t, f, hk.ID, map[string]any{"branch_filter": ""})
	if v := stored.Config["branch_filter"]; v != "" {
		t.Errorf("branch_filter = %q after explicit empty, want cleared", v)
	}
	if stored.Config["url"] != "https://hooks.example/old" || !stored.Active || len(stored.Events) != 2 {
		t.Errorf("clearing the branch filter changed other fields: %+v", stored)
	}
}

// A present argument is sent even when empty; only an absent one is left alone.
func TestEditRepoHookFn_PresentEmptyArgumentsAreSent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		args  map[string]any
		check func(t *testing.T, stored *forgejo_sdk.Hook)
	}{
		{"empty events clears the events", map[string]any{"events": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if len(stored.Events) != 0 {
				t.Errorf("events = %v after explicit empty, want none", stored.Events)
			}
		}},
		{"empty content_type is stored", map[string]any{"content_type": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["content_type"]; !ok || v != "" {
				t.Errorf("content_type = %q (present %v), want empty", v, ok)
			}
		}},
		{"empty http_method is stored", map[string]any{"http_method": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["http_method"]; !ok || v != "" {
				t.Errorf("http_method = %q (present %v), want empty", v, ok)
			}
		}},
		{"empty secret is stored", map[string]any{"secret": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["secret"]; !ok || v != "" {
				t.Errorf("secret = %q (present %v), want empty", v, ok)
			}
		}},
		{"empty url is stored", map[string]any{"url": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v := stored.Config["url"]; v != "" {
				t.Errorf("url = %q, want empty", v)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := fake.New()
			f.AddRepo(editOwner, editRepo)
			hk := seedEditHook(t, f)
			tc.check(t, editCall(t, f, hk.ID, tc.args))
		})
	}
}

// A wrongly typed string argument is treated as absent, as the hand-written
// handler did: the stored value stays.
func TestEditRepoHookFn_WrongTypedStringIsIgnored(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(editOwner, editRepo)
	hk := seedEditHook(t, f)

	stored := editCall(t, f, hk.ID, map[string]any{"branch_filter": float64(5), "url": nil})
	if stored.Config["branch_filter"] != "main" || stored.Config["url"] != "https://hooks.example/old" {
		t.Errorf("wrong-typed or null argument changed a field: %+v", stored)
	}
}
