// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func readResourceReq(uri string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}}
}

func resourceText(t *testing.T, contents []mcp.ResourceContents) string {
	t.Helper()
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	return tc.Text
}

func TestBranchProtectionsResource_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: "main", RequiredApprovals: 1})
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: "release/*"})

	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TotalCount != 2 || len(payload.BranchProtections) != 2 {
		t.Errorf("expected 2 rules, got total=%d shown=%d", payload.TotalCount, len(payload.BranchProtections))
	}
	if payload.Truncated {
		t.Error("did not expect truncation for 2 rules")
	}
}

func TestBranchProtectionsResource_Truncation(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: fmt.Sprintf("b-%d", i), RuleName: fmt.Sprintf("rule-%d", i)})
	}

	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if !payload.Truncated {
		t.Error("expected truncation when over EmbeddedListCap")
	}
	if len(payload.BranchProtections) != resource.EmbeddedListCap {
		t.Errorf("expected %d shown rules, got %d", resource.EmbeddedListCap, len(payload.BranchProtections))
	}
	if payload.ListTool != "list_branch_protections" {
		t.Errorf("expected list_tool sentinel, got %q", payload.ListTool)
	}
}

func TestBranchProtectionsResource_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/missing/branch_protections"))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var re *resource.ResourceError
	if !errors.As(err, &re) {
		t.Fatalf("expected *resource.ResourceError, got %T", err)
	}
	if re.Code != -32003 {
		t.Errorf("expected code -32003 (not found), got %d", re.Code)
	}
}

func TestBranchProtectionResource_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{
		BranchName: "main", StatusCheckContexts: []string{"ci/build"}, RequiredApprovals: 2,
	})

	contents, err := h.branchProtectionResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protection/main"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var bp forgejo_sdk.BranchProtection
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &bp); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if bp.RuleName != "main" || bp.RequiredApprovals != 2 || len(bp.StatusCheckContexts) != 1 {
		t.Errorf("unexpected protection state: %+v", bp)
	}
}

func TestBranchProtectionResource_MalformedURI(t *testing.T) {
	t.Parallel()
	// Parse fails before any port call.
	h, _ := newFixture(t)
	_, err := h.branchProtectionResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protection"))
	if err == nil {
		t.Fatal("expected error for malformed URI (missing rule)")
	}
	var re *resource.ResourceError
	if !errors.As(err, &re) {
		t.Fatalf("expected *resource.ResourceError, got %T", err)
	}
	if re.Code != -32602 {
		t.Errorf("expected code -32602 (invalid params), got %d", re.Code)
	}
}

// The truncation note carries the shared sentinel and names the list tool
// through the registry.
func TestBranchProtectionsResource_TruncationSentinelNamesRegisteredTool(t *testing.T) {
	resource.SetRegisteredTools([]string{"list_branch_protections"})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: fmt.Sprintf("b-%d", i), RuleName: fmt.Sprintf("rule-%d", i)})
	}
	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	want := "[truncated: 30 items shown, more remain. Use list_branch_protections tool to fetch more.]"
	if payload.Sentinel != want {
		t.Errorf("sentinel = %q, want %q", payload.Sentinel, want)
	}
}

func TestBranchProtectionsResource_TruncationOmitsUnregisteredListTool(t *testing.T) {
	resource.SetRegisteredTools([]string{"something_else"})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: fmt.Sprintf("b-%d", i), RuleName: fmt.Sprintf("rule-%d", i)})
	}
	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ListTool != "" || strings.Contains(payload.Sentinel, "list_branch_protections") {
		t.Errorf("list_tool=%q sentinel=%q: an unregistered tool must not be named", payload.ListTool, payload.Sentinel)
	}
}
