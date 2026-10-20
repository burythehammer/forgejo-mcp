// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/server"
)

// noteToolNames is every tool a truncation note can point at: the list tools
// that Bounded callers pass, the wiki marker's tool, and the template
// sentinels. A rename or removal makes this list stale, and the test below
// then fails rather than letting a note name a dead tool.
var noteToolNames = []string{
	issue.ListRepoIssuesToolName,
	issue.ListIssueCommentsToolName,
	issue.ListRepoLabelsToolName,
	issue.ListOrgLabelsToolName,
	wiki.GetWikiPageToolName,
	wiki.GetWikiRevisionsToolName,
	repo.GetCommitStatusesToolName,
	branchprotection.ListBranchProtectionsToolName,
	hook.ListRepoHooksToolName,
	pull.ListPullReviewsToolName,
}

func newRegisteredServer(t *testing.T) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", fake.Factory{Forgejo: fake.New()})
	return s
}

func TestRegisterTool_EveryNoteToolIsRegistered(t *testing.T) {
	s := newRegisteredServer(t)
	registered := s.ListTools()
	for _, name := range noteToolNames {
		if _, ok := registered[name]; !ok {
			t.Errorf("truncation note names %q, which is not a registered tool", name)
		}
		if got := resource.ToolRef(name); got != name {
			t.Errorf("ToolRef(%q) = %q after registration, want the name unchanged", name, got)
		}
	}
}

func TestRegisterTool_NoteNeverNamesRemovedTool(t *testing.T) {
	newRegisteredServer(t)
	if got := resource.ToolRef("list_tool_that_was_removed"); got != "" {
		t.Fatalf("ToolRef named an unregistered tool: %q", got)
	}
}
