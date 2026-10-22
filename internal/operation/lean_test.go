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

// leanServer builds the server Run builds, applying lean mode when asked.
func leanServer(t *testing.T, leanOn bool) *server.MCPServer {
	t.Helper()
	s := selectedServer(t, toolset.Everything())
	if leanOn {
		lean.Apply(s)
	}
	return s
}

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
	full := byName(toolsListed(t, leanServer(t, false)))
	leanTools := byName(toolsListed(t, leanServer(t, true)))

	for name := range full {
		fProps := full[name]["inputSchema"].(map[string]any)["properties"].(map[string]any)
		lProps := leanTools[name]["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for arg := range fProps {
			if !lean.Keeps(arg) {
				continue
			}
			if _, ok := fProps[arg].(map[string]any)["description"]; !ok {
				t.Errorf("%s.%s is a marked argument with no description", name, arg)
			}
		}
		for arg, lv := range lProps {
			lp := lv.(map[string]any)
			fp := fProps[arg].(map[string]any)
			ld, hasL := lp["description"]
			if !hasL {
				continue
			}
			if !lean.Keeps(arg) {
				t.Errorf("%s.%s kept a description but is not a marked argument", name, arg)
			}
			if ld != fp["description"] {
				t.Errorf("%s.%s description changed: %v, want %v", name, arg, ld, fp["description"])
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
