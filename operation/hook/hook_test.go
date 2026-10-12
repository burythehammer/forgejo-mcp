// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	testOwner = "goern"
	testRepo  = "forgejo-mcp"
	// A decoy, never a real secret.
	decoySecret = "decoy-hmac-secret"
)

func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func newHookCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	base := map[string]interface{}{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func hookToolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	content, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatal("not text content")
	}
	return content.Text
}

// seedHook creates a hook with a secret through the fake's port.
func seedHook(t *testing.T, f *fake.Forgejo, url string) *forgejo_sdk.Hook {
	t.Helper()
	hk, err := f.CreateRepoHook(context.Background(), testOwner, testRepo, forgejo_sdk.CreateHookOption{
		Type:   forgejo_sdk.HookTypeForgejo,
		Config: map[string]string{"url": url, "content_type": "json", "secret": decoySecret},
		Events: []string{"push"},
		Active: true,
	})
	if err != nil {
		t.Fatalf("seeding hook: %v", err)
	}
	return hk
}

func listArgs() map[string]interface{} {
	return map[string]interface{}{"page": float64(1), "limit": float64(30)}
}

func TestListRepoHooksFn_CountAndPageEcho(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedHook(t, f, "https://example.test/hook")

	res, err := h.ListRepoHooksFn(context.Background(), newHookCallToolRequest(listArgs()))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	text := hookToolText(t, res)
	if !strings.Contains(text, `"count":1`) || !strings.Contains(text, `"page":1`) {
		t.Fatalf("expected count+page echo in result, got: %s", text)
	}
}

func TestListRepoHooksFn_TotalCountIsReported(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for _, u := range []string{"a", "b", "c", "d"} {
		seedHook(t, f, "https://example.test/"+u)
	}
	res, err := h.ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"page": float64(1), "limit": float64(2),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); !strings.Contains(text, `"count":2`) || !strings.Contains(text, `"total_count":4`) {
		t.Fatalf("expected count 2 of total_count 4, got: %s", text)
	}
}

// Confirmed-zero and unknown are different answers: a total of 0 must marshal
// as "total_count":0 rather than being swallowed by `omitempty`.
func TestListRepoHooksFn_TotalCountZeroIsEmittedNotOmitted(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	res, err := h.ListRepoHooksFn(context.Background(), newHookCallToolRequest(listArgs()))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); !strings.Contains(text, `"total_count":0`) {
		t.Fatalf("expected a confirmed-zero total_count to be emitted, got: %s", text)
	}
}

// noTotal is a port whose list answers carry no total, as when Forgejo sends
// no (or an unparsable) X-Total-Count; the adapter's own handling of the
// header is covered in pkg/forge/sdk.
type noTotal struct{ forge.Forgejo }

func (n noTotal) ListRepoHooks(ctx context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	l, err := n.Forgejo.ListRepoHooks(ctx, owner, repo, opt)
	l.TotalCount = nil
	return l, err
}

func TestListRepoHooksFn_TotalCountOmittedWhenUnknown(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	seedHook(t, f, "https://example.test/hook")
	h := handlers{forge: fake.Factory{Forgejo: noTotal{f}}}

	res, err := h.ListRepoHooksFn(context.Background(), newHookCallToolRequest(listArgs()))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); strings.Contains(text, "total_count") {
		t.Fatalf("expected total_count to be omitted, got: %s", text)
	}
}

// The secret is write-only: whatever the port holds, no tool or resource
// returns it.
func TestSecretIsNeverReturned(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedHook(t, f, "https://example.test/hook")
	ctx := context.Background()

	check := func(name, text string) {
		t.Helper()
		if strings.Contains(text, decoySecret) || strings.Contains(strings.ToLower(text), "secret") {
			t.Errorf("%s leaked the secret: %s", name, text)
		}
	}

	res, err := h.ListRepoHooksFn(ctx, newHookCallToolRequest(listArgs()))
	if err != nil {
		t.Fatal(err)
	}
	check("list", hookToolText(t, res))

	res, err = h.GetRepoHookFn(ctx, newHookCallToolRequest(map[string]interface{}{"id": float64(existing.ID)}))
	if err != nil {
		t.Fatal(err)
	}
	check("get", hookToolText(t, res))

	res, err = h.CreateRepoHookFn(ctx, newHookCallToolRequest(map[string]interface{}{
		"url": "https://example.test/new", "secret": decoySecret + "-new",
	}))
	if err != nil {
		t.Fatal(err)
	}
	check("create", hookToolText(t, res))

	res, err = h.EditRepoHookFn(ctx, newHookCallToolRequest(map[string]interface{}{
		"id": float64(existing.ID), "secret": decoySecret + "-edited",
	}))
	if err != nil {
		t.Fatal(err)
	}
	check("edit", hookToolText(t, res))

	for _, uri := range []string{
		"forgejo://repo/goern/forgejo-mcp/hooks",
		"forgejo://repo/goern/forgejo-mcp/hook/1",
	} {
		handler := h.hookResourceHandler
		if strings.HasSuffix(uri, "/hooks") {
			handler = h.hooksResourceHandler
		}
		contents, err := handler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
		if err != nil {
			t.Fatal(err)
		}
		check(uri, contents[0].(mcp.TextResourceContents).Text)
	}
}

func TestCreateRepoHookFn_StoresWhatWasAsked(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	res, err := h.CreateRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"url": "https://example.test/hook", "events": "push, pull_request", "branch_filter": "main", "secret": decoySecret,
	}))
	if err != nil {
		t.Fatalf("CreateRepoHookFn err: %v", err)
	}
	text := hookToolText(t, res)
	for _, want := range []string{`"url":"https://example.test/hook"`, `"content_type":"json"`, `"branch_filter":"main"`, `"active":true`} {
		if !strings.Contains(text, want) {
			t.Errorf("result lacks %s: %s", want, text)
		}
	}
	stored, err := f.GetRepoHook(context.Background(), testOwner, testRepo, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Events) != 2 || stored.Events[1] != "pull_request" || stored.Config["secret"] != decoySecret {
		t.Errorf("stored hook = %+v", stored)
	}
}

func TestEditRepoHookFn_MergesConfigAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedHook(t, f, "https://example.test/old")

	res, err := h.EditRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"id": float64(existing.ID), "url": "https://example.test/new", "active": false,
	}))
	if err != nil {
		t.Fatalf("EditRepoHookFn err: %v", err)
	}
	var wrapped struct{ Result hookPayload }
	if err := json.Unmarshal([]byte(hookToolText(t, res)), &wrapped); err != nil {
		t.Fatal(err)
	}
	got := wrapped.Result
	if got.URL != "https://example.test/new" || got.Active || got.ContentType != "json" || len(got.Events) != 1 {
		t.Errorf("after edit: %+v", got)
	}
	stored, _ := f.GetRepoHook(context.Background(), testOwner, testRepo, existing.ID)
	if stored.Config["secret"] != decoySecret {
		t.Errorf("an edit that does not pass the secret must keep it, got %q", stored.Config["secret"])
	}
}

func TestEditRepoHookFn_UnknownHook(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if _, err := h.EditRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{"id": float64(9)})); err == nil {
		t.Fatal("expected an error for an unknown hook")
	}
}

func TestDeleteRepoHookFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedHook(t, f, "https://example.test/hook")
	res, err := h.DeleteRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{"id": float64(existing.ID)}))
	if err != nil {
		t.Fatalf("DeleteRepoHookFn err: %v", err)
	}
	if !strings.Contains(hookToolText(t, res), "Deleted webhook") {
		t.Errorf("expected delete confirmation, got %s", hookToolText(t, res))
	}
	if _, err := f.GetRepoHook(context.Background(), testOwner, testRepo, existing.ID); !forge.IsNotFound(err) {
		t.Errorf("expected the hook to be gone, got %v", err)
	}
}

func TestTestRepoHookFn_TriggersADelivery(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedHook(t, f, "https://example.test/hook")
	res, err := h.TestRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{"id": float64(existing.ID)}))
	if err != nil {
		t.Fatalf("TestRepoHookFn err: %v", err)
	}
	if !strings.Contains(hookToolText(t, res), `"triggered":true`) {
		t.Errorf("got %s", hookToolText(t, res))
	}
	if n := f.HookTestDeliveries(existing.ID); n != 1 {
		t.Errorf("deliveries = %d, want 1", n)
	}
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

// seedFilteredHook seeds a hook limited to a branch, as a plain edit must not widen.
func seedFilteredHook(t *testing.T, f *fake.Forgejo) *forgejo_sdk.Hook {
	t.Helper()
	hk, err := f.CreateRepoHook(context.Background(), testOwner, testRepo, forgejo_sdk.CreateHookOption{
		Type:         forgejo_sdk.HookTypeForgejo,
		Config:       map[string]string{"url": "https://example.test/old", "content_type": "json"},
		Events:       []string{"push"},
		Active:       true,
		BranchFilter: "main",
	})
	if err != nil {
		t.Fatalf("seeding filtered hook: %v", err)
	}
	return hk
}

func TestEditRepoHookFn_UrlOnlyKeepsBranchFilterEventsAndActive(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedFilteredHook(t, f)

	if _, err := h.EditRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"id": float64(existing.ID), "url": "https://example.test/new",
	})); err != nil {
		t.Fatalf("EditRepoHookFn err: %v", err)
	}
	stored, err := f.GetRepoHook(context.Background(), testOwner, testRepo, existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Config["url"] != "https://example.test/new" || stored.Config["branch_filter"] != "main" ||
		!stored.Active || len(stored.Events) != 1 || stored.Events[0] != "push" {
		t.Errorf("url-only edit changed other fields: %+v", stored)
	}
}

func TestEditRepoHookFn_ExplicitEmptyBranchFilterClears(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	existing := seedFilteredHook(t, f)

	if _, err := h.EditRepoHookFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"id": float64(existing.ID), "branch_filter": "",
	})); err != nil {
		t.Fatalf("EditRepoHookFn err: %v", err)
	}
	stored, err := f.GetRepoHook(context.Background(), testOwner, testRepo, existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := stored.Config["branch_filter"]; ok && v != "" {
		t.Errorf("branch_filter = %q after explicit empty, want cleared", v)
	}
	if stored.Config["url"] != "https://example.test/old" || !stored.Active || stored.Events[0] != "push" {
		t.Errorf("clearing the branch filter changed other fields: %+v", stored)
	}
}

// An argument that is present but empty is a value the caller provided, so the
// edit must send it. Only an absent argument is left untouched.
func TestEditRepoHookFn_PresentEmptyArgumentsAreSent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		args  map[string]interface{}
		check func(t *testing.T, stored *forgejo_sdk.Hook)
	}{
		{"empty events clears the events", map[string]interface{}{"events": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if len(stored.Events) != 0 {
				t.Errorf("events = %v after explicit empty, want none", stored.Events)
			}
		}},
		{"empty content_type is stored", map[string]interface{}{"content_type": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["content_type"]; !ok || v != "" {
				t.Errorf("content_type = %q (present %v) after explicit empty, want empty", v, ok)
			}
		}},
		{"empty http_method is stored", map[string]interface{}{"http_method": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["http_method"]; !ok || v != "" {
				t.Errorf("http_method = %q (present %v) after explicit empty, want empty", v, ok)
			}
		}},
		{"empty secret is stored", map[string]interface{}{"secret": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v, ok := stored.Config["secret"]; !ok || v != "" {
				t.Errorf("secret present %v = %q after explicit empty, want empty", ok, v)
			}
		}},
		{"empty url is stored", map[string]interface{}{"url": ""}, func(t *testing.T, stored *forgejo_sdk.Hook) {
			if v := stored.Config["url"]; v != "" {
				t.Errorf("url = %q after explicit empty, want empty", v)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, f := newFixture(t)
			existing := seedHook(t, f, "https://example.test/old")
			args := map[string]interface{}{"id": float64(existing.ID)}
			for k, v := range tc.args {
				args[k] = v
			}
			if _, err := h.EditRepoHookFn(context.Background(), newHookCallToolRequest(args)); err != nil {
				t.Fatalf("EditRepoHookFn err: %v", err)
			}
			stored, err := f.GetRepoHook(context.Background(), testOwner, testRepo, existing.ID)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, stored)
		})
	}
}
