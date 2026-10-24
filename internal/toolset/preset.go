// SPDX-License-Identifier: GPL-3.0-or-later

package toolset

import (
	"fmt"
	"strings"
)

// Preset is a named selection of individual tools that may span domains. It is
// not a toolset: toolsets stay one per domain and each tool stays in exactly
// one of them, while a preset only names tools. A preset registers no resource
// templates, as a tools-only selection does.
type Preset struct {
	Name  string
	Tools []string
}

// presets lists every preset in help order. Each tool a preset names must be a
// registered tool; the preset tests fail the build otherwise.
var presets = []Preset{
	{
		Name: "tracker-core",
		Tools: []string{
			"get_my_user_info",
			"get_issue_by_index", "list_repo_issues", "search_issues", "create_issue", "update_issue",
			"replace_issue_body_line", "issue_state_change",
			"create_issue_comment", "list_issue_comments",
			"add_issue_labels", "remove_issue_labels", "list_repo_labels", "get_repo_label",
			"create_repo_label", "edit_repo_label",
			"list_repo_milestones", "get_milestone", "create_milestone", "edit_milestone",
			"add_issue_dependency", "remove_issue_dependency", "list_issue_dependencies", "list_issue_dependents",
		},
	},
}

// PresetNames returns the preset names in help order.
func PresetNames() []string {
	out := make([]string, len(presets))
	for i, p := range presets {
		out[i] = p.Name
	}
	return out
}

// PresetTools returns a copy of the tools the named preset holds, and whether
// the preset exists.
func PresetTools(name string) ([]string, bool) {
	for _, p := range presets {
		if p.Name == name {
			return append([]string(nil), p.Tools...), true
		}
	}
	return nil, false
}

// PresetFlagHelp is the help text for the preset flag: every preset and the
// tools it holds, one preset per line.
func PresetFlagHelp() string {
	var b strings.Builder
	b.WriteString("Named selection of tools to load, across domains. Combines with -toolsets and -tools as a union. Presets and the tools each holds:")
	for _, p := range presets {
		fmt.Fprintf(&b, "\n  %s: %s", p.Name, strings.Join(p.Tools, ", "))
	}
	return b.String()
}
