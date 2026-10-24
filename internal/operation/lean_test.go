// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"

	"github.com/mark3labs/mcp-go/server"
)

// leanServer builds the server Run builds, in the mode asked for.
func leanServer(t *testing.T, leanOn bool) *server.MCPServer {
	t.Helper()
	s := selectedServer(t, toolset.Everything())
	lean.Finish(s, leanOn)
	return s
}

// markedArgs returns, for each registered tool, its marked arguments and their
// clauses, read from the definitions before lean.Finish strips the markers.
func markedArgs(t *testing.T, sel toolset.Selection) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for name, st := range selectedServer(t, sel).ListTools() {
		out[name] = lean.Marked(st.Tool)
	}
	return out
}

// maxClauseLen bounds a keep clause to one short clause.
const maxClauseLen = 120

// neverMarked are the arguments whose descriptions are never kept under lean.
var neverMarked = []string{"owner", "repo", "index", "page", "limit", "verbose"}

// toolsListed returns the tools/list entries as the wire carries them.
func toolsListed(t *testing.T, s *server.MCPServer) []map[string]any {
	t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	raw, err := json.Marshal(s.HandleMessage(context.Background(), req))
	if err != nil {
		t.Fatalf("marshal tools/list response: %v", err)
	}
	var resp struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode tools/list response: %v", err)
	}
	if len(resp.Result.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	return resp.Result.Tools
}

func byName(tools []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(tools))
	for _, tool := range tools {
		out[tool["name"].(string)] = tool
	}
	return out
}

// argNames returns a tool's argument names and its required list, sorted.
func argNames(tool map[string]any) ([]string, []string) {
	schema := tool["inputSchema"].(map[string]any)
	var names []string
	for name := range schema["properties"].(map[string]any) {
		names = append(names, name)
	}
	sort.Strings(names)
	var required []string
	if r, ok := schema["required"].([]any); ok {
		for _, v := range r {
			required = append(required, v.(string))
		}
	}
	sort.Strings(required)
	return names, required
}

func TestLeanKeepsEveryToolNameArgumentNameAndRequiredList(t *testing.T) {
	full := byName(toolsListed(t, leanServer(t, false)))
	leanTools := byName(toolsListed(t, leanServer(t, true)))

	if len(leanTools) != len(full) {
		t.Fatalf("lean lists %d tools, full lists %d", len(leanTools), len(full))
	}
	for name, ft := range full {
		lt, ok := leanTools[name]
		if !ok {
			t.Errorf("tool %q missing under lean", name)
			continue
		}
		fArgs, fReq := argNames(ft)
		lArgs, lReq := argNames(lt)
		if !reflect.DeepEqual(fArgs, lArgs) {
			t.Errorf("%s: lean argument names %v, full %v", name, lArgs, fArgs)
		}
		if !reflect.DeepEqual(fReq, lReq) {
			t.Errorf("%s: lean required %v, full %v", name, lReq, fReq)
		}
	}
}

func TestLeanDescriptionIsTheFirstSentenceOfTheFullOne(t *testing.T) {
	full := byName(toolsListed(t, leanServer(t, false)))
	leanTools := byName(toolsListed(t, leanServer(t, true)))

	for name, ft := range full {
		fd := ft["description"].(string)
		ld, _ := leanTools[name]["description"].(string)
		if ld == "" {
			t.Errorf("%s: lean description is empty", name)
			continue
		}
		if !strings.HasPrefix(fd, ld) {
			t.Errorf("%s: lean description %q is not the start of the full one", name, ld)
			continue
		}
		if ld != fd && !strings.HasSuffix(ld, ".") {
			t.Errorf("%s: lean description %q does not end at a full stop", name, ld)
		}
		if rest := fd[len(ld):]; rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\n") {
			t.Errorf("%s: lean description %q cuts a word in the full one", name, ld)
		}
	}
}

func TestLeanPropertyDescriptionsOnlyForKeptArguments(t *testing.T) {
	marked := markedArgs(t, toolset.Everything())
	leanTools := byName(toolsListed(t, leanServer(t, true)))

	for name, lt := range leanTools {
		lProps := lt["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for arg, lv := range lProps {
			clause, isMarked := marked[name][arg]
			ld, hasL := lv.(map[string]any)["description"]
			if hasL != isMarked {
				t.Errorf("%s.%s: lean has a description %v, marked %v", name, arg, hasL, isMarked)
				continue
			}
			if hasL && ld != clause {
				t.Errorf("%s.%s: lean description %v, want the marker's clause %q", name, arg, ld, clause)
			}
		}
	}
}

func TestEveryMarkedArgumentHasAFullDescriptionAndAShortClause(t *testing.T) {
	for _, st := range selectedServer(t, toolset.Everything()).ListTools() {
		props := st.Tool.InputSchema.Properties
		for arg, clause := range lean.Marked(st.Tool) {
			full, _ := props[arg].(map[string]any)["description"].(string)
			if full == "" {
				t.Errorf("%s.%s is marked but has no description", st.Tool.Name, arg)
			}
			if clause == "" || len(clause) > maxClauseLen {
				t.Errorf("%s.%s clause %q is empty or longer than %d bytes", st.Tool.Name, arg, clause, maxClauseLen)
			}
		}
	}
}

func TestNeverMarkedArgumentsStayOut(t *testing.T) {
	for name, args := range markedArgs(t, toolset.Everything()) {
		for _, never := range neverMarked {
			if _, ok := args[never]; ok {
				t.Errorf("%s.%s is marked; %s is never marked", name, never, never)
			}
		}
	}
}

func TestDefaultOutputCarriesNoKeepMarker(t *testing.T) {
	raw, err := json.Marshal(toolsListed(t, leanServer(t, false)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "x-lean-keep") {
		t.Error("the default tools/list carries a keep marker")
	}
}

func TestTrackerCoreLeanDescribesOnlyMarkedArguments(t *testing.T) {
	sel, err := toolset.ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	lean.Finish(s, true)
	marked := markedArgs(t, sel)
	for _, tool := range toolsListed(t, s) {
		name := tool["name"].(string)
		props := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for arg, v := range props {
			_, hasL := v.(map[string]any)["description"]
			_, isMarked := marked[name][arg]
			if hasL != isMarked {
				t.Errorf("%s.%s: lean has a description %v, marked %v", name, arg, hasL, isMarked)
			}
		}
	}
}

// formatArgNames are the argument names that carry a format, an allowed-value
// list or a rule (comma-separated lists, RFC3339 times, state, sort, type). On
// every tracker-core tool that takes one, the argument must be marked.
var formatArgNames = []string{
	"state", "type", "sort", "labels", "set_labels", "assignees", "milestones",
	"due_date", "clear_due_date", "since", "before", "color", "exclusive", "ref",
	"old", "new", "depends_on_owner", "depends_on_repo", "dependency_owner", "dependency_repo",
}

func TestTrackerCoreMarksEveryFormatArgument(t *testing.T) {
	sel, err := toolset.ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	marked := markedArgs(t, sel)
	for _, st := range selectedServer(t, sel).ListTools() {
		props := st.Tool.InputSchema.Properties
		for _, arg := range formatArgNames {
			if _, has := props[arg]; !has {
				continue
			}
			if _, ok := marked[st.Tool.Name][arg]; !ok {
				t.Errorf("%s.%s carries a format or enum but is not marked with lean.Keep", st.Tool.Name, arg)
			}
		}
	}
}

func TestLeanAnnotationsAreEmptyOnEveryTool(t *testing.T) {
	for _, tool := range toolsListed(t, leanServer(t, true)) {
		ann, ok := tool["annotations"].(map[string]any)
		if !ok || len(ann) != 0 {
			t.Errorf("%v: annotations = %v, want {}", tool["name"], tool["annotations"])
		}
	}
}
