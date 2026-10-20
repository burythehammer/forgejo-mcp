// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every wiki tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port with a
// five-page wiki and a three-revision history, so paging and the next-page
// probe are exercised for real. The wiki resources are pinned elsewhere.

var allTools = []mcp.Tool{
	ListWikiPagesTool, GetWikiPageTool, GetWikiRevisionsTool,
	CreateWikiPageTool, UpdateWikiPageTool, DeleteWikiPageTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

func fixedWikiPage(title, subURL, contentB64 string) *forge.WikiPage {
	return &forge.WikiPage{
		WikiPageMeta: forge.WikiPageMeta{
			Title: title, HTMLURL: "https://f.example/octo/hello/wiki/" + subURL, SubURL: subURL,
			LastCommit: forge.WikiCommit{SHA: "abc123", Author: forge.WikiPerson{Name: "octo", Email: "o@example.com", Date: "2026-01-02"}, Message: "edit"},
		},
		ContentBase64: contentB64, CommitCount: 3, Sidebar: "side", Footer: "foot",
	}
}

// stubWiki is a forge.Forgejo whose wiki domain returns fixed data and records
// the calls it receives. Any other domain would panic on the nil embed.
type stubWiki struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	// fail is returned by every call. probeFail is returned for a page after
	// the first; getFail by GetWikiPage only.
	fail, probeFail, getFail error
	noTotal                  bool
	// content is the page body, base64 encoded as Forgejo sends it, unless
	// badBase64 is set.
	content   string
	badBase64 bool
}

var stubPages = []forge.WikiPageMeta{
	{Title: "Home", SubURL: "Home"}, {Title: "Guides", SubURL: "Guides"}, {Title: "Setup", SubURL: "Guides%2FSetup"},
	{Title: "FAQ", SubURL: "FAQ"}, {Title: "Changelog", SubURL: "Changelog"},
}

var stubRevisions = []forge.WikiCommit{
	{SHA: "r1", Author: forge.WikiPerson{Name: "octo"}, Message: "first"},
	{SHA: "r2", Author: forge.WikiPerson{Name: "bob"}, Message: "second"},
	{SHA: "r3", Author: forge.WikiPerson{Name: "alice"}, Message: "third"},
}

func (s *stubWiki) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func window[T any](all []T, page, limit int) []T {
	start := (page - 1) * limit
	if start < 0 {
		start = 0
	}
	if start > len(all) {
		start = len(all)
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

func (s *stubWiki) ListWikiPages(_ context.Context, o, r string, page, limit int) (forge.WikiPageList, error) {
	if err := s.rec("ListWikiPages", o, r, page, limit); err != nil {
		return forge.WikiPageList{}, err
	}
	if page > 1 && s.probeFail != nil {
		return forge.WikiPageList{}, s.probeFail
	}
	list := forge.WikiPageList{Pages: window(stubPages, page, limit)}
	if !s.noTotal {
		n := len(stubPages)
		list.TotalCount = &n
	}
	return list, nil
}

func (s *stubWiki) GetWikiPage(_ context.Context, o, r, name string) (*forge.WikiPage, error) {
	if err := s.rec("GetWikiPage", o, r, name); err != nil {
		return nil, err
	}
	if s.getFail != nil {
		return nil, s.getFail
	}
	if name == "missing" {
		return nil, &forge.StatusError{StatusCode: 404, Err: errors.New("wiki page does not exist")}
	}
	content := s.content
	if s.badBase64 {
		return fixedWikiPage("Home", name, "!!!not-base64!!!"), nil
	}
	return fixedWikiPage("Home", name, base64.StdEncoding.EncodeToString([]byte(content))), nil
}

func (s *stubWiki) GetWikiPageRevisions(_ context.Context, o, r, name string, page, limit int) (*forge.WikiRevisions, error) {
	if err := s.rec("GetWikiPageRevisions", o, r, name, page, limit); err != nil {
		return nil, err
	}
	if page > 1 && s.probeFail != nil {
		return nil, s.probeFail
	}
	if name == "missing" {
		return nil, &forge.StatusError{StatusCode: 404, Err: errors.New("wiki page does not exist")}
	}
	return &forge.WikiRevisions{Commits: window(stubRevisions, page, limit), Count: len(stubRevisions)}, nil
}

func (s *stubWiki) CreateWikiPage(_ context.Context, o, r, title, contentB64, message string) (*forge.WikiPage, error) {
	return fixedWikiPage(title, title, contentB64), s.rec("CreateWikiPage", o, r, title, contentB64, message)
}

func (s *stubWiki) EditWikiPage(_ context.Context, o, r, name, title, contentB64, message string) (*forge.WikiPage, error) {
	return fixedWikiPage(title, name, contentB64), s.rec("EditWikiPage", o, r, name, title, contentB64, message)
}

func (s *stubWiki) DeleteWikiPage(_ context.Context, o, r, name string) error {
	return s.rec("DeleteWikiPage", o, r, name)
}

// invokeWiki calls a registered tool the way the server does and renders what
// the client would see: the text, or the error, then the port calls made.
func invokeWiki(t *testing.T, stub *stubWiki, tool string, args map[string]any) string {
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

type wikiGoldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubWiki)
}

var wikiGoldenCases = []wikiGoldenCase{
	// list: page and limit default to 1 and 30; a non-float or non-positive value
	// is the default; the next-page probe runs only on a full page.
	{name: "list_defaults", tool: ListWikiPagesToolName},
	{name: "list_probe_has_next", tool: ListWikiPagesToolName, args: map[string]any{"page": float64(1), "limit": float64(2)}},
	{name: "list_last_page", tool: ListWikiPagesToolName, args: map[string]any{"page": float64(3), "limit": float64(2)}},
	{name: "list_string_page_is_default", tool: ListWikiPagesToolName, args: map[string]any{"page": "2", "limit": "2"}},
	{name: "list_fractional_page_truncates", tool: ListWikiPagesToolName, args: map[string]any{"page": 0.5, "limit": 2.9}},
	{name: "list_negative_limit_is_default", tool: ListWikiPagesToolName, args: map[string]any{"limit": float64(-4)}},
	{name: "list_no_total", tool: ListWikiPagesToolName, stub: func(s *stubWiki) { s.noTotal = true }},
	{name: "list_probe_error", tool: ListWikiPagesToolName, args: map[string]any{"page": float64(1), "limit": float64(2)},
		stub: func(s *stubWiki) { s.probeFail = errors.New("boom") }},
	{name: "list_port_error", tool: ListWikiPagesToolName, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},

	{name: "get_ok", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home"}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_lines", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "start_line": float64(2), "end_line": float64(3)}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_start_only", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "start_line": float64(2)}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_end_only", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "end_line": float64(2)}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_fractional_lines", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "start_line": 1.9, "end_line": 2.9}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_string_lines_are_ignored", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "start_line": "2", "end_line": "3"}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_inverted_lines", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home", "start_line": float64(3), "end_line": float64(1)}, stub: func(s *stubWiki) { s.content = "one\ntwo\nthree" }},
	{name: "get_bad_base64", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home"}, stub: func(s *stubWiki) { s.badBase64 = true }},
	{name: "get_missing", tool: GetWikiPageToolName, args: map[string]any{"page_name": "missing"}},
	{name: "get_port_error", tool: GetWikiPageToolName, args: map[string]any{"page_name": "Home"}, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},

	{name: "revisions_defaults", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "Home"}},
	{name: "revisions_probe_has_next", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "Home", "page": float64(1), "limit": float64(2)}},
	{name: "revisions_string_page_is_default", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "Home", "page": "2", "limit": "2"}},
	{name: "revisions_probe_error", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "Home", "page": float64(1), "limit": float64(2)},
		stub: func(s *stubWiki) { s.probeFail = errors.New("boom") }},
	{name: "revisions_missing", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "missing"}},
	{name: "revisions_port_error", tool: GetWikiRevisionsToolName, args: map[string]any{"page_name": "Home"}, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},

	{name: "create_default_message", tool: CreateWikiPageToolName, args: map[string]any{"title": "New", "content": "body"}},
	{name: "create_custom_message", tool: CreateWikiPageToolName, args: map[string]any{"title": "New", "content": "body", "message": "add new"}},
	{name: "create_port_error", tool: CreateWikiPageToolName, args: map[string]any{"title": "New", "content": "body"}, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},

	{name: "update_with_title", tool: UpdateWikiPageToolName, args: map[string]any{"page_name": "Home", "title": "Start", "content": "body", "message": "reword"}},
	{name: "update_default_message", tool: UpdateWikiPageToolName, args: map[string]any{"page_name": "Home", "title": "Start", "content": "body"}},
	{name: "update_without_title_keeps_it", tool: UpdateWikiPageToolName, args: map[string]any{"page_name": "Home", "content": "body"}},
	{name: "update_read_error", tool: UpdateWikiPageToolName, args: map[string]any{"page_name": "Home", "content": "body"}, stub: func(s *stubWiki) { s.getFail = errors.New("boom") }},
	{name: "update_port_error", tool: UpdateWikiPageToolName, args: map[string]any{"page_name": "Home", "title": "Start", "content": "body"}, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},

	{name: "delete_ok", tool: DeleteWikiPageToolName, args: map[string]any{"page_name": "Home"}},
	{name: "delete_port_error", tool: DeleteWikiPageToolName, args: map[string]any{"page_name": "Home"}, stub: func(s *stubWiki) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range wikiGoldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubWiki{content: "one\ntwo\nthree"}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invokeWiki(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the wiki goldens pin the paging
// contract without a ceiling, as they always have.
func (s *stubWiki) MaxResponseItems(context.Context) (int, bool) { return 0, false }
