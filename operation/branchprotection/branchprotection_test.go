package branchprotection

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

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

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	base := map[string]interface{}{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

// seedRule creates a rule through the fake's port.
func seedRule(t *testing.T, f *fake.Forgejo, opt forgejo_sdk.CreateBranchProtectionOption) {
	t.Helper()
	if _, err := f.CreateBranchProtection(context.Background(), testOwner, testRepo, opt); err != nil {
		t.Fatalf("seeding rule: %v", err)
	}
}

func getRule(t *testing.T, f *fake.Forgejo, rule string) *forgejo_sdk.BranchProtection {
	t.Helper()
	bp, err := f.GetBranchProtection(context.Background(), testOwner, testRepo, rule)
	if err != nil {
		t.Fatalf("reading rule %q: %v", rule, err)
	}
	return bp
}

func TestListBranchProtectionsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{
		BranchName: "main", EnableStatusCheck: true, StatusCheckContexts: []string{"ci/build"}, RequiredApprovals: 1,
	})

	res, err := h.ListBranchProtectionsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"page": float64(1), "limit": float64(50),
	}))
	if err != nil {
		t.Fatalf("ListBranchProtectionsFn err: %v", err)
	}
	text := toolText(t, res)
	if !strings.Contains(text, `"count":1`) || !strings.Contains(text, `"page":1`) {
		t.Errorf("expected count+page echo in result, got: %s", text)
	}
	if !strings.Contains(text, "ci/build") {
		t.Errorf("expected rule contexts in result, got: %s", text)
	}
}

func TestListBranchProtectionsFn_PagesThroughRules(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for _, n := range []string{"a", "b", "c"} {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: n})
	}
	res, err := h.ListBranchProtectionsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"page": float64(2), "limit": float64(2),
	}))
	if err != nil {
		t.Fatalf("ListBranchProtectionsFn err: %v", err)
	}
	text := toolText(t, res)
	if !strings.Contains(text, `"count":1`) || !strings.Contains(text, `"rule_name":"c"`) {
		t.Errorf("expected the third rule alone on page 2, got: %s", text)
	}
}

func TestGetBranchProtectionFn_OK(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: "main"})

	res, err := h.GetBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{"rule": "main"}))
	if err != nil {
		t.Fatalf("GetBranchProtectionFn err: %v", err)
	}
	if !strings.Contains(toolText(t, res), `"rule_name":"main"`) {
		t.Errorf("expected rule in result")
	}
}

func TestGetBranchProtectionFn_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.GetBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{"rule": "missing"}))
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCreateBranchProtectionFn_StatusCheckRoundTrip(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	res, err := h.CreateBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"branch_name":         "main",
		"enable_status_check": true, "status_check_contexts": "ci/build, ci/test",
	}))
	if err != nil {
		t.Fatalf("CreateBranchProtectionFn err: %v", err)
	}
	bp := getRule(t, f, "main")
	if !bp.EnableStatusCheck {
		t.Errorf("expected enable_status_check stored")
	}
	if got := bp.StatusCheckContexts; len(got) != 2 || got[0] != "ci/build" || got[1] != "ci/test" {
		t.Errorf("status_check_contexts not stored: %v", got)
	}
	if !strings.Contains(toolText(t, res), "ci/test") {
		t.Errorf("expected contexts echoed in result")
	}
}

func TestCreateBranchProtectionFn_MissingBranchName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	_, err := h.CreateBranchProtectionFn(context.Background(), newCallToolRequest(nil))
	if err == nil {
		t.Fatal("expected error when branch_name is missing")
	}
	list, _ := f.ListBranchProtections(context.Background(), testOwner, testRepo, forgejo_sdk.ListBranchProtectionsOptions{})
	if len(list) != 0 {
		t.Errorf("no rule may be created without a branch_name, got %d", len(list))
	}
}

// An edit changes only what the caller passed. An unpassed boolean must never
// be written as false (which would silently relax protection), nor an unpassed
// whitelist as empty (which would wipe it).
func TestEditBranchProtectionFn_OnlyPassedFields(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{
		BranchName: "main", EnableStatusCheck: true, RequiredApprovals: 1,
		EnableMergeWhitelist: true, MergeWhitelistUsernames: []string{"alice"},
	})

	_, err := h.EditBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"rule": "main", "required_approvals": float64(2),
	}))
	if err != nil {
		t.Fatalf("EditBranchProtectionFn err: %v", err)
	}
	bp := getRule(t, f, "main")
	if bp.RequiredApprovals != 2 {
		t.Errorf("expected required_approvals=2, got %d", bp.RequiredApprovals)
	}
	if !bp.EnableStatusCheck || !bp.EnableMergeWhitelist {
		t.Errorf("unpassed booleans must be left unchanged: %+v", bp)
	}
	if len(bp.MergeWhitelistUsernames) != 1 || bp.MergeWhitelistUsernames[0] != "alice" {
		t.Errorf("unpassed whitelist must be left unchanged: %v", bp.MergeWhitelistUsernames)
	}
}

func TestEditBranchProtectionFn_ContextsRoundTrip(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: "main"})

	_, err := h.EditBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"rule": "main", "status_check_contexts": "ci/build,ci/lint",
	}))
	if err != nil {
		t.Fatalf("EditBranchProtectionFn err: %v", err)
	}
	got := getRule(t, f, "main").StatusCheckContexts
	if len(got) != 2 || got[0] != "ci/build" || got[1] != "ci/lint" {
		t.Errorf("status_check_contexts not stored: %v", got)
	}
}

func TestEditBranchProtectionFn_PushWhitelistRoundTrip(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{
		BranchName: "main", MergeWhitelistUsernames: []string{"carol"},
	})

	_, err := h.EditBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"rule":                     "main",
		"enable_push_whitelist":    true,
		"push_whitelist_usernames": "goern,op1st-gitops",
	}))
	if err != nil {
		t.Fatalf("EditBranchProtectionFn err: %v", err)
	}
	bp := getRule(t, f, "main")
	if !bp.EnablePushWhitelist {
		t.Errorf("expected enable_push_whitelist=true")
	}
	if got := bp.PushWhitelistUsernames; len(got) != 2 || got[0] != "goern" || got[1] != "op1st-gitops" {
		t.Errorf("push_whitelist_usernames not stored: %v", got)
	}
	if got := bp.MergeWhitelistUsernames; len(got) != 1 || got[0] != "carol" {
		t.Errorf("unpassed merge whitelist must be left unchanged: %v", got)
	}
}

func TestDeleteBranchProtectionFn_OK(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: "main"})

	res, err := h.DeleteBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{"rule": "main"}))
	if err != nil {
		t.Fatalf("DeleteBranchProtectionFn err: %v", err)
	}
	if _, err := f.GetBranchProtection(context.Background(), testOwner, testRepo, "main"); err == nil {
		t.Error("expected the rule to be deleted")
	}
	if !strings.Contains(toolText(t, res), "Deleted branch protection rule") {
		t.Errorf("expected delete confirmation, got: %s", toolText(t, res))
	}
}

func TestSplitContexts(t *testing.T) {
	t.Parallel()
	got := splitContexts("ci/build, ci/test ,, ci/lint")
	want := []string{"ci/build", "ci/test", "ci/lint"}
	if len(got) != len(want) {
		t.Fatalf("splitContexts: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitContexts[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if n := len(splitContexts("")); n != 0 {
		t.Errorf("empty string should split to 0 contexts, got %d", n)
	}
}

// toolText extracts the text payload from a tool result.
func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("nil tool result")
	}
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatal("no text content in tool result")
	return ""
}
