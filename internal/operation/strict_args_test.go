// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// countingFactory counts the credential lookups a call makes. The pipeline
// binds a use case, and so looks up a credential, only for a request it has
// accepted; a count of zero means the call made no forge request.
type countingFactory struct {
	forge.Factory
	calls *int
}

func (c countingFactory) For(ctx context.Context) (forge.Forgejo, error) {
	*c.calls++
	return c.Factory.For(ctx)
}

// setLaxArgs sets the startup relax option for one test and restores strict
// refusal afterwards. Tests that call it are not parallel.
func setLaxArgs(t *testing.T, on bool) {
	t.Helper()
	pipeline.SetLaxArgs(on)
	t.Cleanup(func() { pipeline.SetLaxArgs(false) })
}

// registeredWithCount returns a server holding the full registry, with a fake
// forge behind a factory that counts credential lookups.
func registeredWithCount(t *testing.T) (*server.MCPServer, *int) {
	t.Helper()
	calls := new(int)
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", countingFactory{Factory: fake.Factory{Forgejo: fake.New()}, calls: calls})
	return s, calls
}

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("%s is not registered", name)
	}
	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}})
	if err != nil {
		t.Fatalf("%s returned a Go error: %v", name, err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("%s returned no result", name)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result content is %T, want text", res.Content[0])
	}
	return text.Text
}

// TestSearchIssuesRefusesMisspelledKeyBeforeForge is the case from #341: a
// search that names `query` for `q` must be refused, not run unfiltered.
func TestSearchIssuesRefusesMisspelledKeyBeforeForge(t *testing.T) {
	setLaxArgs(t, false)
	s, calls := registeredWithCount(t)

	res := callTool(t, s, issue.SearchIssuesToolName, map[string]any{"owner": "octo", "query": "crash"})
	if !res.IsError {
		t.Fatalf("search_issues with unknown key query was not refused: %q", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, `"query"`) {
		t.Errorf("refusal does not name the unknown key: %q", text)
	}
	for _, valid := range []string{"owner", "q", "state", "verbose", "mentioned_by"} {
		if !strings.Contains(text, valid) {
			t.Errorf("refusal does not list valid argument %q: %q", valid, text)
		}
	}
	if *calls != 0 {
		t.Errorf("refused call made %d credential lookups, want 0", *calls)
	}
}

// TestSearchIssuesRefusalNamesEveryUnknownKey checks that one response names
// all the unknown keys, not only the first.
func TestSearchIssuesRefusalNamesEveryUnknownKey(t *testing.T) {
	setLaxArgs(t, false)
	s, calls := registeredWithCount(t)

	res := callTool(t, s, issue.SearchIssuesToolName, map[string]any{"owner": "octo", "query": "crash", "stat": "open"})
	if !res.IsError {
		t.Fatalf("search_issues with two unknown keys was not refused: %q", resultText(t, res))
	}
	text := resultText(t, res)
	for _, key := range []string{`"query"`, `"stat"`} {
		if !strings.Contains(text, key) {
			t.Errorf("refusal does not name unknown key %s: %q", key, text)
		}
	}
	if *calls != 0 {
		t.Errorf("refused call made %d credential lookups, want 0", *calls)
	}
}

// TestSearchIssuesValidCallStillRuns checks that strictness costs a correct
// call nothing: a call with only declared arguments reaches the forge.
func TestSearchIssuesValidCallStillRuns(t *testing.T) {
	setLaxArgs(t, false)
	s, calls := registeredWithCount(t)

	res := callTool(t, s, issue.SearchIssuesToolName, map[string]any{"owner": "octo", "q": "crash"})
	if res.IsError {
		t.Fatalf("valid search_issues call was refused: %q", resultText(t, res))
	}
	if *calls != 1 {
		t.Errorf("valid call made %d credential lookups, want 1", *calls)
	}
}

// TestSearchIssuesLaxArgsRestoresOldBehaviour checks the relax option: the same
// misspelled call is served as before, and it reaches the forge.
func TestSearchIssuesLaxArgsRestoresOldBehaviour(t *testing.T) {
	setLaxArgs(t, true)
	s, calls := registeredWithCount(t)

	res := callTool(t, s, issue.SearchIssuesToolName, map[string]any{"owner": "octo", "query": "crash"})
	if res.IsError {
		t.Fatalf("with lax args on, unknown key was refused: %q", resultText(t, res))
	}
	if *calls != 1 {
		t.Errorf("with lax args on, call made %d credential lookups, want 1", *calls)
	}
}

// TestUnknownKeyStillIgnoredOutsideSearchIssues checks that the strict switch
// is per tool: list_repo_issues keeps the lax decoding it has always had.
func TestUnknownKeyStillIgnoredOutsideSearchIssues(t *testing.T) {
	setLaxArgs(t, false)
	s, _ := registeredWithCount(t)

	res := callTool(t, s, issue.ListRepoIssuesToolName, map[string]any{"owner": "octo", "repo": "hello", "query": "crash"})
	if strings.Contains(resultText(t, res), "unknown argument") {
		t.Fatalf("list_repo_issues refused an unknown key; only search_issues is strict: %q", resultText(t, res))
	}
}

// TestBuildLaxArgsReachesSearchIssues checks the startup wiring: Build passes
// the configured option to the pipeline, so the flag decides the behaviour.
func TestBuildLaxArgsReachesSearchIssues(t *testing.T) {
	for _, on := range []bool{false, true} {
		b, err := Build(config.Config{LaxArgs: on}, "test")
		if err != nil {
			t.Fatalf("Build(LaxArgs=%v): %v", on, err)
		}
		t.Cleanup(func() { pipeline.SetLaxArgs(false) })
		res := callTool(t, b.Server, issue.SearchIssuesToolName, map[string]any{"owner": "octo", "query": "crash"})
		refused := strings.Contains(resultText(t, res), "unknown argument")
		if refused == on {
			t.Errorf("Build(LaxArgs=%v): refused=%v, want %v", on, refused, !on)
		}
	}
}
