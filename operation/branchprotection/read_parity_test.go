// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// seedRules creates the named rules through the fake, each on a branch of the
// same name.
func seedRules(t *testing.T, f *fake.Forgejo, names ...string) {
	t.Helper()
	for _, n := range names {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: n, RuleName: n, RequiredApprovals: 1})
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

func toolText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func parityReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

// readCollectionPayload reads the rules collection resource.
func readCollectionPayload(t *testing.T, h handlers) bpCollectionPayload {
	t.Helper()
	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + testOwner + "/" + testRepo + "/branch_protections"}})
	if err != nil {
		t.Fatal(err)
	}
	var p bpCollectionPayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// readRulePayload reads one rule resource.
func readRulePayload(t *testing.T, h handlers, rule string) (*model.BranchProtection, string) {
	t.Helper()
	contents, err := h.branchProtectionResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + testOwner + "/" + testRepo + "/branch_protection/" + rule}})
	if err != nil {
		t.Fatal(err)
	}
	text := contents[0].(mcp.TextResourceContents).Text
	var bp model.BranchProtection
	if err := json.Unmarshal([]byte(text), &bp); err != nil {
		t.Fatal(err)
	}
	return &bp, text
}

// TestBranchProtectionReadParity_ToolAndResourceAgree drives the list and get
// tools and the two rule resources over the same fake rules. The collection is
// the first cap rules of the list tool, and the single-rule resource is the get
// tool's rule.
func TestBranchProtectionReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	names := make([]string, resource.EmbeddedListCap+1)
	for i := range names {
		names[i] = "b" + strings.Repeat("x", i)
	}
	seedRules(t, f, names...)

	listRes, err := h.ListBranchProtectionsFn(context.Background(), parityReq(map[string]any{"page": float64(1), "limit": float64(len(names))}))
	if err != nil {
		t.Fatal(err)
	}
	var listed model.BranchProtectionList
	if err := unmarshalResult(toolText(listRes), &listed); err != nil {
		t.Fatal(err)
	}

	collection := readCollectionPayload(t, h)
	if !collection.Truncated || collection.ListTool != "list_branch_protections" {
		t.Fatalf("collection truncated=%v list_tool=%q, want truncated with list_branch_protections", collection.Truncated, collection.ListTool)
	}
	if !reflect.DeepEqual(collection.BranchProtections, listed.BranchProtections[:resource.EmbeddedListCap]) {
		t.Fatalf("collection facts differ from the list tool's first %d rules", resource.EmbeddedListCap)
	}

	want := listed.BranchProtections[0]
	getRes, err := h.GetBranchProtectionFn(context.Background(), parityReq(map[string]any{"rule": want.RuleName}))
	if err != nil {
		t.Fatal(err)
	}
	var fromTool model.BranchProtection
	if err := unmarshalResult(toolText(getRes), &fromTool); err != nil {
		t.Fatal(err)
	}
	fromResource, _ := readRulePayload(t, h, want.RuleName)
	if !reflect.DeepEqual(fromResource, &fromTool) {
		t.Fatalf("rule %q differs\nresource: %+v\ntool:     %+v", want.RuleName, fromResource, fromTool)
	}
}

// TestBranchProtectionReadParity_MissingRuleIsNotFoundOnBothPaths: a rule
// that does not exist is not found through the tool or the resource.
func TestBranchProtectionReadParity_MissingRuleIsNotFoundOnBothPaths(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	ctx := context.Background()

	_, resErr := h.branchProtectionResourceHandler(ctx,
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + testOwner + "/" + testRepo + "/branch_protection/nope"}})

	var rr *resource.ResourceError
	if !errors.As(resErr, &rr) || rr.Code != -32003 {
		t.Fatalf("resource: want not-found, got %v", resErr)
	}
	toolRes, toolErr := h.GetBranchProtectionFn(ctx, parityReq(map[string]any{"rule": "nope"}))
	if toolErr == nil && (toolRes == nil || !toolRes.IsError) {
		t.Fatalf("tool: want an error result for a missing rule, got err=%v res=%+v", toolErr, toolRes)
	}
}
