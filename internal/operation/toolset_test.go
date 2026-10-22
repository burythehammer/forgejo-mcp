// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"

	"github.com/mark3labs/mcp-go/server"
)

// selectedServer builds a server the way Run does for the given selection.
func selectedServer(t *testing.T, sel toolset.Selection) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	f := fake.Factory{Forgejo: fake.New()}
	RegisterSelectedTool(s, "test", f, sel)
	RegisterSelectedResources(s, f, sel)
	return s
}

// listed sends a JSON-RPC list request to the server and returns the names of
// the entries under the result key result (tools or resourceTemplates).
func listed(t *testing.T, s *server.MCPServer, method, key, field string) []string {
	t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method})
	raw, err := json.Marshal(s.HandleMessage(context.Background(), req))
	if err != nil {
		t.Fatalf("marshal %s response: %v", method, err)
	}
	var resp struct {
		Result map[string]json.RawMessage `json:"result"`
		Error  json.RawMessage            `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if resp.Error != nil && string(resp.Error) != "null" {
		// With no template registered the server advertises no resources
		// capability, so a template list is refused rather than empty.
		if strings.Contains(string(resp.Error), "resources not supported") {
			return nil
		}
		t.Fatalf("%s returned error: %s", method, resp.Error)
	}
	if _, ok := resp.Result["nextCursor"]; ok {
		t.Fatalf("%s paginated; the selection tests expect one page", method)
	}
	var items []map[string]any
	if err := json.Unmarshal(resp.Result[key], &items); err != nil {
		t.Fatalf("decode %s.%s: %v", method, key, err)
	}
	var names []string
	for _, it := range items {
		names = append(names, it[field].(string))
	}
	sort.Strings(names)
	return names
}

func toolNamesOf(t *testing.T, s *server.MCPServer) []string {
	t.Helper()
	return listed(t, s, "tools/list", "tools", "name")
}

func templatesOf(t *testing.T, s *server.MCPServer) []string {
	t.Helper()
	return listed(t, s, "resources/templates/list", "resourceTemplates", "uriTemplate")
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func toolsOf(t *testing.T, sets ...string) []string {
	t.Helper()
	var out []string
	for _, name := range sets {
		found := false
		for _, ts := range toolset.All() {
			if ts.Name == name {
				out = append(out, ts.Tools...)
				found = true
			}
		}
		if !found {
			t.Fatalf("no toolset %q", name)
		}
	}
	return sortedCopy(out)
}

// Every tool the server registers belongs to exactly one toolset, and every
// toolset names only tools the server registers.
func TestEveryRegisteredToolBelongsToOneToolset(t *testing.T) {
	registered := toolNamesOf(t, selectedServer(t, toolset.Everything()))

	seen := map[string]string{}
	for _, ts := range toolset.All() {
		for _, name := range ts.Tools {
			if prev, dup := seen[name]; dup {
				t.Errorf("tool %q is in toolsets %q and %q", name, prev, ts.Name)
			}
			seen[name] = ts.Name
		}
	}
	for _, name := range registered {
		if _, ok := seen[name]; !ok {
			t.Errorf("registered tool %q is in no toolset; add it to internal/toolset", name)
		}
	}
	if want, got := len(registered), len(seen); want != got {
		t.Errorf("toolsets name %d tools, server registers %d", got, want)
	}
	for name := range seen {
		if !contains(registered, name) {
			t.Errorf("toolset names %q, which the server does not register", name)
		}
	}
}

func TestDefaultSelectionIsTheFullCatalogue(t *testing.T) {
	s := selectedServer(t, toolset.Everything())
	if got, want := toolNamesOf(t, s), toolsOf(t, toolsetNames()...); !reflect.DeepEqual(got, want) {
		t.Fatalf("default tools/list has %d tools, want the whole catalogue of %d", len(got), len(want))
	}
	if len(templatesOf(t, s)) == 0 {
		t.Fatal("default selection registers no resource templates")
	}
}

func TestToolsetsOnlyOffersTheirTools(t *testing.T) {
	sel, err := toolset.Parse([]string{"issues", "labels", "milestones", "dependencies", "user"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	want := toolsOf(t, "issues", "labels", "milestones", "dependencies", "user")
	if got := toolNamesOf(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools/list = %v, want %v", got, want)
	}
}

func TestToolsOnlyRegistersNoTemplates(t *testing.T) {
	sel, err := toolset.Parse(nil, []string{"list_repo_labels", "get_my_user_info"})
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	if got, want := toolNamesOf(t, s), []string{"get_my_user_info", "list_repo_labels"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tools/list = %v, want %v", got, want)
	}
	if got := templatesOf(t, s); len(got) != 0 {
		t.Fatalf("tools-only selection registered templates %v", got)
	}
}

func TestUnionOfToolsetsAndNamedTools(t *testing.T) {
	sel, err := toolset.Parse([]string{"milestones"}, []string{"get_repo_label"})
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	want := sortedCopy(append(toolsOf(t, "milestones"), "get_repo_label"))
	if got := toolNamesOf(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools/list = %v, want %v", got, want)
	}
}

// A resource template registers with its domain's toolset, so the labels
// templates come with labels and not with pulls.
func TestTemplatesFollowTheirToolset(t *testing.T) {
	sel, err := toolset.Parse([]string{"labels"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(templatesOf(t, selectedServer(t, sel)), "\n")
	if !strings.Contains(got, "label") {
		t.Fatalf("labels toolset registered no label template:\n%s", got)
	}
	if strings.Contains(got, "/pull") {
		t.Fatalf("labels toolset registered a pull template:\n%s", got)
	}
}

func TestSelectionRegistersNoUnselectedTemplates(t *testing.T) {
	full := templatesOf(t, selectedServer(t, toolset.Everything()))
	sel, err := toolset.Parse([]string{"wiki"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	narrow := templatesOf(t, selectedServer(t, sel))
	if len(narrow) == 0 || len(narrow) >= len(full) {
		t.Fatalf("wiki selection has %d templates of %d; want some but fewer", len(narrow), len(full))
	}
}

func toolsetNames() []string { return toolset.Names() }

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
