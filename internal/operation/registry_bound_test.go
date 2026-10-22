// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pull"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// boundItems is how many rows each list fixture seeds. It is above every default
// limit a list tool or template takes (the tree endpoint's 1000 is the largest),
// so a default call always leaves rows for the next page.
const boundItems = 1001

// collectionSegments name the path segment of a list resource template that takes
// its window from the URI. A collection read is a list even when its URI names no
// page or limit, so it is held to the same bound.
var collectionSegments = map[string]bool{
	"issues":             true,
	"labels":             true,
	"comments":           true,
	"hooks":              true,
	"branch_protections": true,
}

// textShape describes a list tool whose output is text rather than a JSON
// envelope. Each row is a line starting with rowPrefix, and a page that leaves
// rows behind carries marker.
type textShape struct {
	rowPrefix string
	marker    string
}

var textListTools = map[string]textShape{
	pull.GetPullRequestDiffToolName: {rowPrefix: "diff --git ", marker: "[diff truncated:"},
}

// skippedListTools are list tools the bound cannot hold yet. Each runs as a
// subtest that skips with its reason, so the gap shows in every run.
var skippedListTools = map[string]string{}

// listResourceFixture is one list resource template: a concrete URI that reads
// it and the seed that fills the fake behind that URI.
type listResourceFixture struct {
	uri  string
	seed func(t *testing.T, f *fake.Forgejo, n int)
}

// wireTemplate is one resource template as resources/templates/list names it.
type wireTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Description string `json:"description"`
}

// TestRegistryBoundsEveryListTool enumerates every registered tool that returns a
// list and holds it to the bound against the fake with boundItems rows: a
// default call is a full page, the payload says whether rows follow, paging walks
// every row once, and page and limit are documented. A new list tool needs one
// seed row in pageToolsTable, and the test names it when that row is missing.
func TestRegistryBoundsEveryListTool(t *testing.T) {
	t.Parallel()
	for name, st := range newListServer(fake.New()).ListTools() {
		if !isListTool(name, st.Tool) {
			continue
		}
		if reason, ok := skippedListTools[name]; ok {
			t.Run(name, func(t *testing.T) { t.Skip(reason) })
			continue
		}
		pt, ok := pageToolFor(name)
		if !ok {
			t.Errorf("list tool %q has no seed row in pageToolsTable; add one so the registry test can fill it past the default limit", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkListTool(t, pt, st.Tool)
		})
	}
}

// TestRegistryBoundsEveryListResource enumerates every registered resource
// template that returns a list and reads it against boundItems rows: the payload
// is cut short with a marker, and a template that takes page and limit hands back
// the window it names.
func TestRegistryBoundsEveryListResource(t *testing.T) {
	t.Parallel()
	fixtures := listResourceFixtures()
	for _, tpl := range listTemplates(t, newListServer(fake.New())) {
		if !isListTemplate(tpl.URITemplate) {
			continue
		}
		fx, ok := fixtures[tpl.URITemplate]
		if !ok {
			t.Errorf("list resource template %q has no fixture in listResourceFixtures; add one", tpl.URITemplate)
			continue
		}
		t.Run(tpl.URITemplate, func(t *testing.T) {
			t.Parallel()
			checkListResource(t, fx, tpl)
		})
	}
}

// isListTool reports whether a registered tool returns a list: its name says so,
// or it takes a page or limit, which only a list does.
func isListTool(name string, tool mcp.Tool) bool {
	if strings.HasPrefix(name, "list_") || strings.HasPrefix(name, "search_") {
		return true
	}
	_, page := tool.InputSchema.Properties["page"]
	_, limit := tool.InputSchema.Properties["limit"]
	return page || limit
}

// isListTemplate reports whether a resource template returns a list: its query
// takes page or limit, or its path ends in a collection segment.
func isListTemplate(uriTemplate string) bool {
	path, query, _ := strings.Cut(uriTemplate, "{?")
	if strings.Contains(query, "page") || strings.Contains(query, "limit") {
		return true
	}
	return collectionSegments[path[strings.LastIndex(path, "/")+1:]]
}

// pageToolFor returns the seed row of a list tool, if it has one.
func pageToolFor(name string) (pageTool, bool) {
	for _, pt := range pageToolsTable {
		if pt.tool == name {
			return pt, true
		}
	}
	return pageTool{}, false
}

// listResourceFixtures maps each list template to a concrete URI and a seed.
// Where a template reads the rows of a list tool, it reuses that tool's seed.
func listResourceFixtures() map[string]listResourceFixture {
	seedOf := func(tool string) func(*testing.T, *fake.Forgejo, int) {
		pt, ok := pageToolFor(tool)
		if !ok {
			panic("no seed row for " + tool)
		}
		return pt.seed
	}
	return map[string]listResourceFixture{
		"forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}": {
			uri: "forgejo://repo/octo/hello/issues", seed: seedOf(issue.ListRepoIssuesToolName),
		},
		"forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}": {
			uri: "forgejo://repo/octo/hello/issue/1/comments", seed: seedOf(issue.ListIssueCommentsToolName),
		},
		"forgejo://repo/{owner}/{repo}/labels{?page,limit}": {
			uri: "forgejo://repo/octo/hello/labels", seed: seedOf(issue.ListRepoLabelsToolName),
		},
		"forgejo://org/{org}/labels{?page,limit}": {
			uri: "forgejo://org/acme/labels", seed: seedOf(issue.ListOrgLabelsToolName),
		},
		"forgejo://repo/{owner}/{repo}/hooks{?page,limit}": {
			uri: "forgejo://repo/octo/hello/hooks", seed: seedOf(hook.ListRepoHooksToolName),
		},
		"forgejo://repo/{owner}/{repo}/branch_protections{?page,limit}": {
			uri: "forgejo://repo/octo/hello/branch_protections", seed: seedOf(branchprotection.ListBranchProtectionsToolName),
		},
		"forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page}": {
			uri: "forgejo://repo/octo/hello/commit/" + pageSHA, seed: seedCommitFiles,
		},
	}
}

// seedCommitFiles creates one commit that touches n files, for the commit
// resource's paged file list.
func seedCommitFiles(t *testing.T, f *fake.Forgejo, n int) {
	t.Helper()
	seedRepo(t, f)
	paths := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		paths = append(paths, fmt.Sprintf("file-%d.txt", i))
	}
	must(t, f.AddCommit(pageOwner, pageRepo, pageSHA, "many files", paths...))
}

// newListServer returns a server with every tool and core resource registered
// over f. Resources are enabled, so resources/templates/list and resources/read
// are served.
func newListServer(f *fake.Forgejo) *server.MCPServer {
	s := server.NewMCPServer("test", "0", server.WithResourceCapabilities(true, true))
	factory := fake.Factory{Forgejo: f}
	RegisterTool(s, "test", factory)
	RegisterCoreResources(s, factory)
	return s
}

// boundedServer returns newListServer over a fake that seed has filled with
// boundItems rows.
func boundedServer(t *testing.T, seed func(*testing.T, *fake.Forgejo, int)) *server.MCPServer {
	t.Helper()
	f := fake.New()
	f.Now = func() time.Time { return pageNow }
	seed(t, f, boundItems)
	return newListServer(f)
}

// checkListTool holds one list tool to the bound.
func checkListTool(t *testing.T, pt pageTool, tool mcp.Tool) {
	t.Helper()
	for _, p := range []string{"page", "limit"} {
		if _, ok := tool.InputSchema.Properties[p]; !ok {
			t.Fatalf("list tool takes no %q parameter, so a caller cannot choose the window", p)
		}
		if !strings.Contains(tool.Description, p) {
			t.Errorf("tool description does not document %q", p)
		}
	}

	want := boundItems
	if pt.rows != nil {
		want = pt.rows(boundItems)
	}
	limit := schemaDefaultLimit(t, tool)
	if limit < 1 || limit >= want {
		t.Fatalf("default limit %d is not below the %d rows the fixture holds; raise boundItems", limit, want)
	}
	s := boundedServer(t, pt.seed)
	if shape, ok := textListTools[tool.Name]; ok {
		checkTextTool(t, s, pt, tool.Name, shape, limit, want)
		return
	}
	checkEnvelopeTool(t, s, pt, tool.Name, limit, want)
}

// checkEnvelopeTool holds a tool whose output is a JSON payload to the bound.
func checkEnvelopeTool(t *testing.T, s *server.MCPServer, pt pageTool, name string, limit, want int) {
	t.Helper()
	first := invokeTool(t, s, name, withArgs(pt.args, nil))
	_, rows := rowsOf(t, first)
	if got := optionalNumber(first, "limit"); got != 0 && got != limit {
		t.Errorf("default call reports limit %d, the schema default is %d", got, limit)
	}
	if len(rows) != limit {
		t.Errorf("default call returned %d rows, want a full page of %d", len(rows), limit)
	}
	if !follows(t, name, first, len(rows), limit, 0) {
		t.Errorf("default call over %d rows says nothing follows", want)
	}

	seen := map[string]bool{}
	for page := 1; ; page++ {
		body := invokeTool(t, s, name, withArgs(pt.args, map[string]any{"page": float64(page), "limit": float64(limit)}))
		_, rows := rowsOf(t, body)
		for _, r := range rows {
			key, err := json.Marshal(r)
			must(t, err)
			if seen[string(key)] {
				t.Errorf("page %d repeats a row already read: %s", page, key)
			}
			seen[string(key)] = true
		}
		if !follows(t, name, body, len(rows), limit, len(seen)) {
			break
		}
		if page > want {
			t.Fatalf("paging never ends: still rows after page %d", page)
		}
	}
	if len(seen) != want {
		t.Errorf("paging read %d distinct rows, want all %d", len(seen), want)
	}
}

// checkTextTool holds a tool whose output is text to the bound: a page is a
// limit of rows, and a page that leaves rows behind carries the marker.
func checkTextTool(t *testing.T, s *server.MCPServer, pt pageTool, name string, shape textShape, limit, want int) {
	t.Helper()
	first := callTool(t, s, name, withArgs(pt.args, nil))
	if got := textRows(first, shape.rowPrefix); got != limit {
		t.Errorf("default call returned %d rows, want a full page of %d", got, limit)
	}
	if !strings.Contains(first, shape.marker) {
		t.Errorf("default call over %d rows carries no truncation marker", want)
	}

	seen := map[string]bool{}
	for page := 1; ; page++ {
		out := callTool(t, s, name, withArgs(pt.args, map[string]any{"page": float64(page), "limit": float64(limit)}))
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, shape.rowPrefix) {
				if seen[line] {
					t.Errorf("page %d repeats a row already read: %s", page, line)
				}
				seen[line] = true
			}
		}
		if !strings.Contains(out, shape.marker) {
			break
		}
		if page > want {
			t.Fatalf("paging never ends: still carries the marker after page %d", page)
		}
	}
	if len(seen) != want {
		t.Errorf("paging read %d distinct rows, want all %d", len(seen), want)
	}
}

// checkListResource holds one list resource template to the bound.
func checkListResource(t *testing.T, fx listResourceFixture, tpl wireTemplate) {
	t.Helper()
	if !strings.Contains(tpl.URITemplate, "page") || !strings.Contains(tpl.URITemplate, "limit") {
		t.Fatalf("list resource takes no page or limit, so a caller cannot choose the window; the payload is capped at a fixed size")
	}

	s := boundedServer(t, fx.seed)
	full := readResource(t, s, fx.uri)
	_, rows := rowsOf(t, full)
	if len(rows) >= boundItems {
		t.Errorf("default read returned all %d rows, which is no bound", len(rows))
	}
	if !continuationMarked(full) {
		t.Errorf("default read is cut short without a truncation marker or next-page signal")
	}

	window := readResource(t, s, fx.uri+"?page=2&limit=10")
	_, windowRows := rowsOf(t, window)
	if len(windowRows) != 10 {
		t.Errorf("page 2 at limit 10 returned %d rows, want 10", len(windowRows))
		return
	}
	a, _ := json.Marshal(rows[0])
	b, _ := json.Marshal(windowRows[0])
	if string(a) == string(b) {
		t.Errorf("page 2 starts with the same row as page 1")
	}
}

// follows reports whether rows follow a page. A payload that names has_next
// decides; one that names only total_count is read against the rows seen so far.
// A payload with neither cannot say, which the test reports.
func follows(t *testing.T, name string, body map[string]any, pageRows, limit, seen int) bool {
	t.Helper()
	if v, ok := body["has_next"].(bool); ok {
		return v
	}
	if total, ok := body["total_count"].(float64); ok {
		return seen < int(total)
	}
	t.Fatalf("payload has neither has_next nor total_count; a caller cannot tell whether rows follow")
	return false
}

// schemaDefaultLimit is the limit the tool's schema names as its default.
func schemaDefaultLimit(t *testing.T, tool mcp.Tool) int {
	t.Helper()
	prop, ok := tool.InputSchema.Properties["limit"].(map[string]any)
	if !ok {
		t.Fatalf("limit has no schema property")
	}
	switch def := prop["default"].(type) {
	case int:
		return def
	case int64:
		return int(def)
	case float64:
		return int(def)
	}
	t.Fatalf("limit names no numeric default in its schema")
	return 0
}

// callTool calls a registered tool and returns its text content.
func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) string {
	t.Helper()
	st := s.GetTool(name)
	if st == nil {
		t.Fatalf("tool %s not registered", name)
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}})
	must(t, err)
	text := res.Content[0].(mcp.TextContent).Text
	if res.IsError {
		t.Fatalf("%s returned an error: %s", name, text)
	}
	return text
}

// invokeTool calls a tool whose output is a JSON payload and decodes it.
func invokeTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) map[string]any {
	t.Helper()
	return payloadOf(t, callTool(t, s, name, args))
}

// listTemplates returns the server's resource templates as the wire lists them.
func listTemplates(t *testing.T, s *server.MCPServer) []wireTemplate {
	t.Helper()
	raw := rpcResult(t, s, "resources/templates/list", nil)
	var out struct {
		ResourceTemplates []wireTemplate `json:"resourceTemplates"`
	}
	must(t, json.Unmarshal(raw, &out))
	return out.ResourceTemplates
}

// rpcResult sends one JSON-RPC request to the server and returns its result.
func rpcResult(t *testing.T, s *server.MCPServer, method string, params any) json.RawMessage {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		req["params"] = params
	}
	raw, err := json.Marshal(req)
	must(t, err)
	resp, err := json.Marshal(s.HandleMessage(context.Background(), raw))
	must(t, err)
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	must(t, json.Unmarshal(resp, &env))
	if env.Error != nil {
		t.Fatalf("%s failed: %s", method, env.Error)
	}
	return env.Result
}

// payloadOf decodes a tool body. Tool output wraps its payload in Result;
// resource output is the payload itself.
func payloadOf(t *testing.T, text string) map[string]any {
	t.Helper()
	var body map[string]any
	must(t, json.Unmarshal([]byte(text), &body))
	if inner, ok := body["Result"].(map[string]any); ok {
		return inner
	}
	return body
}

// rowsOf returns the one list in a payload. Two arrays make the list ambiguous,
// so the check refuses rather than guessing.
func rowsOf(t *testing.T, body map[string]any) (string, []any) {
	t.Helper()
	var key string
	var rows []any
	for k, v := range body {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		if key != "" {
			t.Fatalf("payload has two arrays (%s and %s); the bound check cannot tell which is the list", key, k)
		}
		key, rows = k, arr
	}
	if key == "" {
		t.Fatalf("payload carries no list")
	}
	return key, rows
}

// optionalNumber is the numeric field name, or 0 when the payload has none.
func optionalNumber(body map[string]any, name string) int {
	if v, ok := body[name].(float64); ok {
		return int(v)
	}
	return 0
}

// textRows counts the lines of a text output that start with prefix.
func textRows(text, prefix string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

// continuationMarked reports whether a cut-short resource payload says so: a
// truncated flag, a has_next, a sentinel, or a *_next pointer.
func continuationMarked(body map[string]any) bool {
	for k, v := range body {
		switch {
		case k == "truncated" || k == "has_next":
			if b, ok := v.(bool); ok && b {
				return true
			}
		case k == "sentinel":
			if s, ok := v.(string); ok && s != "" {
				return true
			}
		case strings.HasSuffix(k, "_next"):
			if v != nil && v != false && v != "" {
				return true
			}
		}
	}
	return false
}

// withArgs returns base with extra laid over it, as a fresh map.
func withArgs(base, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
