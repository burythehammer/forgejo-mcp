// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every branch protection tool: its tool
// definition (schema snapshot) and its output and error text (golden), together
// with the call the tool made on the Forgejo port. They run against a stub port
// that returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	ListBranchProtectionsTool, GetBranchProtectionTool, CreateBranchProtectionTool,
	EditBranchProtectionTool, DeleteBranchProtectionTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	bpT0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	bpT1 = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
)

func fixedRule(name string) *forgejo_sdk.BranchProtection {
	return &forgejo_sdk.BranchProtection{
		BranchName: name, RuleName: "rule-" + name,
		EnablePush: true, EnablePushWhitelist: true, PushWhitelistUsernames: []string{"octo"},
		PushWhitelistTeams: []string{"core"}, PushWhitelistDeployKeys: true,
		EnableMergeWhitelist: true, MergeWhitelistUsernames: []string{"bob"}, MergeWhitelistTeams: []string{"ops"},
		EnableStatusCheck: true, StatusCheckContexts: []string{"ci/build", "lint"}, RequiredApprovals: 2,
		EnableApprovalsWhitelist: true, ApprovalsWhitelistUsernames: []string{"alice"}, ApprovalsWhitelistTeams: []string{"leads"},
		BlockOnRejectedReviews: true, BlockOnOfficialReviewRequests: true, BlockOnOutdatedBranch: true,
		DismissStaleApprovals: true, RequireSignedCommits: true,
		ProtectedFilePatterns: "*.go", UnprotectedFilePatterns: "docs/*",
		Created: bpT0, Updated: bpT1,
	}
}

// stubBP is a forge.Forgejo whose branch protection domain returns fixed data
// and records the calls it receives. Any other domain would panic on the nil
// embed.
type stubBP struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
}

func (s *stubBP) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubBP) ListBranchProtections(_ context.Context, o, r string, opt forgejo_sdk.ListBranchProtectionsOptions) ([]*forgejo_sdk.BranchProtection, error) {
	return []*forgejo_sdk.BranchProtection{fixedRule("main"), fixedRule("dev")}, s.rec("ListBranchProtections", o, r, opt)
}

func (s *stubBP) GetBranchProtection(_ context.Context, o, r, rule string) (*forgejo_sdk.BranchProtection, error) {
	return fixedRule("main"), s.rec("GetBranchProtection", o, r, rule)
}

func (s *stubBP) CreateBranchProtection(_ context.Context, o, r string, opt forgejo_sdk.CreateBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	return fixedRule(opt.BranchName), s.rec("CreateBranchProtection", o, r, opt)
}

func (s *stubBP) EditBranchProtection(_ context.Context, o, r, rule string, opt forgejo_sdk.EditBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	return fixedRule(rule), s.rec("EditBranchProtection", o, r, rule, opt)
}

func (s *stubBP) DeleteBranchProtection(_ context.Context, o, r, rule string) error {
	return s.rec("DeleteBranchProtection", o, r, rule)
}

// invokeBP calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made.
func invokeBP(t *testing.T, stub *stubBP, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type bpGoldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubBP)
}

var bpGoldenCases = []bpGoldenCase{
	// list: page and limit default to 1 and 100; fractions truncate; strings parse.
	{name: "list_defaults", tool: ListBranchProtectionsTool.Name},
	{name: "list_paged", tool: ListBranchProtectionsTool.Name, args: map[string]any{"page": float64(2), "limit": float64(5)}},
	{name: "list_fractional_paging", tool: ListBranchProtectionsTool.Name, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "list_string_paging", tool: ListBranchProtectionsTool.Name, args: map[string]any{"page": "2", "limit": "5"}},
	{name: "list_missing_owner", tool: ListBranchProtectionsTool.Name, args: map[string]any{"owner": nil}},
	{name: "list_port_error", tool: ListBranchProtectionsTool.Name, stub: func(s *stubBP) { s.fail = errors.New("boom") }},

	{name: "get_ok", tool: GetBranchProtectionTool.Name, args: map[string]any{"rule": "main"}},
	{name: "get_missing_rule", tool: GetBranchProtectionTool.Name},
	{name: "get_missing_repo", tool: GetBranchProtectionTool.Name, args: map[string]any{"repo": nil, "rule": "main"}},
	{name: "get_port_error", tool: GetBranchProtectionTool.Name, args: map[string]any{"rule": "main"}, stub: func(s *stubBP) { s.fail = errors.New("boom") }},

	// create: contexts split and trimmed, approvals truncate, wrong types read as absent.
	{name: "create_full", tool: CreateBranchProtectionTool.Name, args: map[string]any{
		"branch_name": "main", "rule_name": "main-rule", "enable_push": true, "enable_status_check": true,
		"status_check_contexts": "ci/build, lint,,", "required_approvals": float64(2), "block_on_outdated_branch": true,
		"require_signed_commits": true, "dismiss_stale_approvals": true,
	}},
	{name: "create_minimal", tool: CreateBranchProtectionTool.Name, args: map[string]any{"branch_name": "dev"}},
	{name: "create_fractional_approvals", tool: CreateBranchProtectionTool.Name, args: map[string]any{"branch_name": "dev", "required_approvals": 2.7}},
	{name: "create_string_approvals", tool: CreateBranchProtectionTool.Name, args: map[string]any{"branch_name": "dev", "required_approvals": "3"}},
	{name: "create_wrong_type_bool", tool: CreateBranchProtectionTool.Name, args: map[string]any{"branch_name": "dev", "enable_push": "yes"}},
	{name: "create_missing_branch", tool: CreateBranchProtectionTool.Name},
	{name: "create_port_error", tool: CreateBranchProtectionTool.Name, args: map[string]any{"branch_name": "dev"}, stub: func(s *stubBP) { s.fail = errors.New("boom") }},

	// edit: only the fields passed are sent, and an explicit false is still sent.
	{name: "edit_only_passed", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main", "enable_push": false, "required_approvals": float64(0)}},
	{name: "edit_none_passed", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main"}},
	{name: "edit_empty_contexts", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main", "status_check_contexts": ""}},
	{name: "edit_whitelists", tool: EditBranchProtectionTool.Name, args: map[string]any{
		"rule": "main", "enable_push_whitelist": true, "push_whitelist_usernames": "a, b",
		"enable_merge_whitelist": false, "merge_whitelist_usernames": "c",
		"enable_approvals_whitelist": true, "approvals_whitelist_usernames": " d ,",
	}},
	{name: "edit_wrong_type_approvals", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main", "required_approvals": "many"}},
	{name: "edit_string_approvals", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main", "required_approvals": "7"}},
	{name: "edit_missing_rule", tool: EditBranchProtectionTool.Name},
	{name: "edit_port_error", tool: EditBranchProtectionTool.Name, args: map[string]any{"rule": "main"}, stub: func(s *stubBP) { s.fail = errors.New("boom") }},

	{name: "delete_ok", tool: DeleteBranchProtectionTool.Name, args: map[string]any{"rule": "main"}},
	{name: "delete_missing_rule", tool: DeleteBranchProtectionTool.Name},
	{name: "delete_port_error", tool: DeleteBranchProtectionTool.Name, args: map[string]any{"rule": "main"}, stub: func(s *stubBP) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range bpGoldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubBP{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invokeBP(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the branch protection goldens
// pin the paging contract without a ceiling, as they always have.
func (s *stubBP) MaxResponseItems(context.Context) (int, bool) { return 0, false }
