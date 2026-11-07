// SPDX-License-Identifier: GPL-3.0-or-later

package operation_test

import (
	"sort"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"
)

func builtTools(t *testing.T, cfg config.Config) []string {
	t.Helper()
	b, err := operation.Build(cfg, "test")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var names []string
	for n := range b.Server.ListTools() {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestBuild_Selection(t *testing.T) {
	issues := toolset.All()
	var issueTools []string
	for _, ts := range issues {
		if ts.Name == "issues" {
			issueTools = ts.Tools
		}
	}
	cases := []struct {
		name string
		cfg  config.Config
		want int
	}{
		{"everything", config.Config{}, len(toolset.ToolNames())},
		{"toolsets", config.Config{Toolsets: []string{"issues"}}, len(issueTools)},
		{"tools", config.Config{Tools: []string{"get_repo"}}, 1},
		{"union", config.Config{Toolsets: []string{"issues"}, Tools: []string{"get_repo"}}, len(issueTools) + 1},
		{"lean", config.Config{Toolsets: []string{"issues"}, Lean: true}, len(issueTools)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := builtTools(t, tc.cfg); len(got) != tc.want {
				t.Errorf("%d tools, want %d: %v", len(got), tc.want, got)
			}
		})
	}
}

func TestBuild_UnknownNameFails(t *testing.T) {
	for _, cfg := range []config.Config{
		{Toolsets: []string{"nope"}},
		{Tools: []string{"nope"}},
		{Preset: "nope"},
	} {
		if _, err := operation.Build(cfg, "test"); err == nil {
			t.Errorf("Build(%+v) = nil error, want one", cfg)
		}
	}
}

// A truncation note may name only tools in the registry; Build records it.
func TestBuild_RecordsRegistry(t *testing.T) {
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })
	resource.SetRegisteredTools(nil)
	builtTools(t, config.Config{Tools: []string{"get_repo"}})
	if got := resource.ToolRef("get_repo"); got != "get_repo" {
		t.Errorf("ToolRef(selected) = %q", got)
	}
	if got := resource.ToolRef("get_my_user_info"); got != "" {
		t.Errorf("ToolRef(excluded) = %q, want empty", got)
	}
}
