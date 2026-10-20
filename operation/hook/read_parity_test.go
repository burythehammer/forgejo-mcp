// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// decoySecret is the webhook secret the parity fixtures carry. No path may
// return it, tool or resource.
const decoySecret = "decoy-hmac-secret"

// seedSecretHooks creates n webhooks, each with the decoy secret in its config.
func seedSecretHooks(t *testing.T, f *fake.Forgejo, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := f.CreateRepoHook(context.Background(), testOwner, testRepo, forgejo_sdk.CreateHookOption{
			Type: forgejo_sdk.HookTypeForgejo,
			Config: map[string]string{
				"url": "https://example.test/h", "content_type": "json", "secret": decoySecret,
			},
			Events: []string{"push"},
			Active: true,
		})
		if err != nil {
			t.Fatalf("seeding hook: %v", err)
		}
	}
}

func readHooksCollection(t *testing.T, h handlers) (hooksCollectionPayload, string) {
	t.Helper()
	contents, err := h.hooksResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + testOwner + "/" + testRepo + "/hooks"}})
	if err != nil {
		t.Fatal(err)
	}
	text := contents[0].(mcp.TextResourceContents).Text
	var p hooksCollectionPayload
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		t.Fatal(err)
	}
	return p, text
}

func readHook(t *testing.T, h handlers, id int64) (model.HookPayload, string) {
	t.Helper()
	uri := "forgejo://repo/" + testOwner + "/" + testRepo + "/hook/" + strconv.FormatInt(id, 10)
	contents, err := h.hookResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		t.Fatal(err)
	}
	text := contents[0].(mcp.TextResourceContents).Text
	var p model.HookPayload
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		t.Fatal(err)
	}
	return p, text
}

// TestHooksReadParity_ToolAndResourceAgree drives the list and get tools and
// the two webhook resources over the same fake hooks. The facts they return
// must agree: the collection resource is the first cap hooks of the list tool,
// and the single-hook resource is the get tool's hook.
func TestHooksReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedSecretHooks(t, f, resource.EmbeddedListCap+1)

	var listed model.HookListResult
	listRes, err := h.ListRepoHooksFn(context.Background(), readReq(map[string]any{"page": float64(1), "limit": float64(resource.EmbeddedListCap + 1)}))
	if err != nil {
		t.Fatal(err)
	}
	if err := unmarshalResult(textOf(listRes), &listed); err != nil {
		t.Fatal(err)
	}

	collection, _ := readHooksCollection(t, h)
	if !collection.Truncated || collection.ListTool != "list_repo_hooks" {
		t.Fatalf("collection truncated=%v list_tool=%q, want truncated with list_repo_hooks", collection.Truncated, collection.ListTool)
	}
	if !reflect.DeepEqual(collection.Hooks, listed.Hooks[:resource.EmbeddedListCap]) {
		t.Fatalf("collection facts differ from the list tool's first %d hooks", resource.EmbeddedListCap)
	}

	want := listed.Hooks[0]
	getRes, err := h.GetRepoHookFn(context.Background(), readReq(map[string]any{"id": float64(want.ID)}))
	if err != nil {
		t.Fatal(err)
	}
	var fromTool model.HookPayload
	if err := unmarshalResult(textOf(getRes), &fromTool); err != nil {
		t.Fatal(err)
	}
	fromResource, _ := readHook(t, h, want.ID)
	if !reflect.DeepEqual(fromResource, fromTool) {
		t.Fatalf("hook %d differs\nresource: %+v\ntool:     %+v", want.ID, fromResource, fromTool)
	}
}

// readReq is a tool request for the test repository with extra arguments.
func readReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

// textOf is the text of a tool result, which the tests decode as JSON.
func textOf(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// TestHooksReadParity_NeverReturnTheSecret checks redaction on every read path.
func TestHooksReadParity_NeverReturnTheSecret(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedSecretHooks(t, f, 2)

	listRes, err := h.ListRepoHooksFn(context.Background(), readReq(map[string]any{"page": float64(1), "limit": float64(30)}))
	if err != nil {
		t.Fatal(err)
	}
	getRes, err := h.GetRepoHookFn(context.Background(), readReq(map[string]any{"id": float64(1)}))
	if err != nil {
		t.Fatal(err)
	}
	_, collectionText := readHooksCollection(t, h)
	_, hookText := readHook(t, h, 1)
	for name, out := range map[string]string{
		"list tool":           textOf(listRes),
		"get tool":            textOf(getRes),
		"collection resource": collectionText,
		"hook resource":       hookText,
	} {
		if strings.Contains(out, decoySecret) {
			t.Errorf("%s leaks the webhook secret: %s", name, out)
		}
	}
}

// unmarshalResult decodes the Result envelope a tool wraps its output in.
func unmarshalResult(text string, v any) error {
	var env struct {
		Result json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		return err
	}
	return json.Unmarshal(env.Result, v)
}

// TestHookResource_ZeroIDIsInvalidParams: the get tool refuses an id of zero
// as a missing argument, so the resource refuses it as invalid params rather
// than asking Forgejo for hook 0.
func TestHookResource_ZeroIDIsInvalidParams(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.hookResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + testOwner + "/" + testRepo + "/hook/0"}})
	var re *resource.ResourceError
	if !errors.As(err, &re) || re.Code != -32602 {
		t.Fatalf("want invalid-params resource error, got %v", err)
	}
}
