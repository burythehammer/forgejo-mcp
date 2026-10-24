// SPDX-License-Identifier: GPL-3.0-or-later

package toolset

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// trackerCore is the tracker-core list from the spec (#303), in the order the
// ticket gives it. The test compares it as a set, so the order is not checked.
var trackerCore = []string{
	"get_my_user_info",
	"get_issue_by_index", "list_repo_issues", "search_issues", "create_issue", "update_issue",
	"replace_issue_body_line", "issue_state_change",
	"create_issue_comment", "list_issue_comments",
	"add_issue_labels", "remove_issue_labels", "list_repo_labels", "get_repo_label",
	"create_repo_label", "edit_repo_label",
	"list_repo_milestones", "get_milestone", "create_milestone", "edit_milestone",
	"add_issue_dependency", "remove_issue_dependency", "list_issue_dependencies", "list_issue_dependents",
}

func sortedOf(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestTrackerCorePresetHoldsTheSpecTools(t *testing.T) {
	got, ok := PresetTools("tracker-core")
	if !ok {
		t.Fatal("no tracker-core preset")
	}
	if want := sortedOf(trackerCore); !reflect.DeepEqual(sortedOf(got), want) {
		t.Fatalf("tracker-core = %d tools %v, want the spec's %d", len(got), sortedOf(got), len(want))
	}
}

func TestEveryPresetToolIsARegisteredTool(t *testing.T) {
	known := map[string]bool{}
	for _, name := range ToolNames() {
		known[name] = true
	}
	for _, name := range PresetNames() {
		tools, _ := PresetTools(name)
		for _, tool := range tools {
			if !known[tool] {
				t.Errorf("preset %q names %q, which is not a registered tool", name, tool)
			}
		}
	}
}

func TestPresetAloneSelectsItsToolsAndNoTemplates(t *testing.T) {
	sel, err := ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Full() {
		t.Fatal("a preset selection reported as full")
	}
	for _, tool := range trackerCore {
		if !sel.HasTool(tool) {
			t.Errorf("preset does not select %q", tool)
		}
	}
	if sel.HasTool("create_pull_request") {
		t.Error("preset selects a tool outside its list")
	}
	for _, ts := range Names() {
		if sel.HasToolset(ts) {
			t.Errorf("preset registers resource templates of toolset %q", ts)
		}
	}
}

func TestPresetUnionWithToolsetsAndTools(t *testing.T) {
	sel, err := ParseWithPreset("tracker-core", []string{"wiki"}, []string{"create_pull_request"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range append([]string{"create_pull_request"}, trackerCore...) {
		if !sel.HasTool(tool) {
			t.Errorf("union lacks %q", tool)
		}
	}
	for _, ts := range All() {
		if ts.Name != "wiki" {
			continue
		}
		for _, name := range ts.Tools {
			if !sel.HasTool(name) {
				t.Errorf("union lacks wiki tool %q", name)
			}
		}
	}
	if !sel.HasToolset("wiki") {
		t.Error("the named toolset's resource templates are not registered in the union")
	}
	if sel.HasToolset("labels") {
		t.Error("the preset pulled in the templates of a toolset it does not name")
	}
}

func TestPresetAloneLeavesEverythingElseOut(t *testing.T) {
	sel, err := ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sel.HasTool("create_wiki_page") {
		t.Error("preset alone selected a wiki tool")
	}
}

func TestUnknownPresetNamesTheValidOnes(t *testing.T) {
	_, err := ParseWithPreset("trackr-core", nil, nil)
	if err == nil {
		t.Fatal("unknown preset accepted")
	}
	if !strings.Contains(err.Error(), `"trackr-core"`) || !strings.Contains(err.Error(), "tracker-core") {
		t.Errorf("error %q does not name the bad preset and the valid ones", err)
	}
}

func TestEmptyPresetKeepsTheDefaultSelection(t *testing.T) {
	sel, err := ParseWithPreset("", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.Full() {
		t.Fatal("no preset and no selection is not the full catalogue")
	}
}

func TestPresetFlagHelpListsEachPresetAndItsTools(t *testing.T) {
	help := PresetFlagHelp()
	for _, name := range PresetNames() {
		if !strings.Contains(help, "\n  "+name+": ") {
			t.Errorf("preset help lacks %q", name)
		}
	}
	for _, tool := range trackerCore {
		if !strings.Contains(help, tool) {
			t.Errorf("preset help lacks tool %q", tool)
		}
	}
}
