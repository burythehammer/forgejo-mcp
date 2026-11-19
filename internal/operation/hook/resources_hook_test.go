// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// The forgejo://…/hooks resources still read through the SDK-typed port; these
// tests pin their bounding and not-found behaviour. The tools are pinned by
// golden_test.go.

const (
	testOwner = "goern"
	testRepo  = "forgejo-mcp"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

// seedHook creates a hook through the fake's port.
func seedHook(t *testing.T, f *fake.Forgejo, url string) *forgejo_sdk.Hook {
	t.Helper()
	hk, err := f.CreateRepoHook(context.Background(), testOwner, testRepo, forgejo_sdk.CreateHookOption{
		Type:   forgejo_sdk.HookTypeForgejo,
		Config: map[string]string{"url": url, "content_type": "json"},
		Events: []string{"push"},
		Active: true,
	})
	if err != nil {
		t.Fatalf("seeding hook: %v", err)
	}
	return hk
}

func TestHooksResource_BoundedToTheCap(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedHook(t, f, "https://example.test/h")
	}
	contents, err := h.hooksResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/goern/forgejo-mcp/hooks"}})
	if err != nil {
		t.Fatal(err)
	}
	var payload hooksCollectionPayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Truncated || len(payload.Hooks) != resource.EmbeddedListCap || payload.ListTool != "list_repo_hooks" {
		t.Errorf("truncated=%v shown=%d list_tool=%q", payload.Truncated, len(payload.Hooks), payload.ListTool)
	}
}

func TestHookResource_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.hookResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/goern/forgejo-mcp/hook/7"}})
	var re *resource.ResourceError
	if !errors.As(err, &re) || re.Code != -32003 {
		t.Fatalf("want not-found resource error, got %v", err)
	}
}

// The truncation note names the list tool through the registry and carries the
// shared sentinel, as every other embedded list resource does.
func TestHooksResource_TruncationCarriesSentinelNamingRegisteredTool(t *testing.T) {
	resource.SetRegisteredTools([]string{"list_repo_hooks"})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedHook(t, f, "https://example.test/h")
	}
	contents, err := h.hooksResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/goern/forgejo-mcp/hooks"}})
	if err != nil {
		t.Fatal(err)
	}
	var payload hooksCollectionPayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload); err != nil {
		t.Fatal(err)
	}
	want := "[truncated: 30 items shown, more remain. Use list_repo_hooks tool to fetch more.]"
	if payload.Sentinel != want {
		t.Errorf("sentinel = %q, want %q", payload.Sentinel, want)
	}
}

func TestHooksResource_TruncationOmitsUnregisteredListTool(t *testing.T) {
	resource.SetRegisteredTools([]string{"something_else"})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedHook(t, f, "https://example.test/h")
	}
	contents, err := h.hooksResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/goern/forgejo-mcp/hooks"}})
	if err != nil {
		t.Fatal(err)
	}
	var payload hooksCollectionPayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ListTool != "" || strings.Contains(payload.Sentinel, "list_repo_hooks") {
		t.Errorf("list_tool=%q sentinel=%q: an unregistered tool must not be named", payload.ListTool, payload.Sentinel)
	}
}
