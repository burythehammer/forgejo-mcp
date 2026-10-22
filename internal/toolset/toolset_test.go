// SPDX-License-Identifier: GPL-3.0-or-later

package toolset

import (
	"reflect"
	"strings"
	"testing"
)

func TestNoSelectionSelectsEverything(t *testing.T) {
	sel, err := Parse(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.Full() || !sel.HasTool("get_my_user_info") || !sel.HasToolset("wiki") {
		t.Fatalf("empty selection is not the full catalogue: %+v", sel)
	}
}

func TestToolsetsSelectTheirToolsAndTemplates(t *testing.T) {
	sel, err := Parse([]string{"labels"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Full() {
		t.Fatal("a toolset selection reported as full")
	}
	if !sel.HasTool("create_repo_label") || sel.HasTool("create_pull_request") {
		t.Fatal("toolset did not select exactly its tools")
	}
	if !sel.HasToolset("labels") || sel.HasToolset("pulls") {
		t.Fatal("toolset did not select exactly its resource templates")
	}
}

func TestToolsOnlyHoldsNoToolset(t *testing.T) {
	sel, err := Parse(nil, []string{"list_repo_labels"})
	if err != nil {
		t.Fatal(err)
	}
	if !sel.HasTool("list_repo_labels") || sel.HasTool("create_repo_label") {
		t.Fatal("tools-only selection did not hold exactly the named tool")
	}
	if sel.HasToolset("labels") {
		t.Fatal("tools-only selection registers resource templates")
	}
}

func TestUnknownToolsetNamesTheValidOnes(t *testing.T) {
	_, err := Parse([]string{"isssues"}, nil)
	if err == nil {
		t.Fatal("unknown toolset accepted")
	}
	for _, name := range []string{`"isssues"`, "issues", "labels", "user"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestUnknownToolNamesTheValidOnes(t *testing.T) {
	_, err := Parse(nil, []string{"list_repo_labelz"})
	if err == nil {
		t.Fatal("unknown tool accepted")
	}
	if !strings.Contains(err.Error(), `"list_repo_labelz"`) || !strings.Contains(err.Error(), "list_repo_labels") {
		t.Errorf("error %q does not name the bad tool and the valid ones", err)
	}
}

func TestUnionAndDuplicates(t *testing.T) {
	sel, err := Parse([]string{"milestones", "milestones"}, []string{"get_repo_label", "get_repo_label"})
	if err != nil {
		t.Fatal(err)
	}
	if !sel.HasTool("list_repo_milestones") || !sel.HasTool("get_repo_label") || sel.HasTool("list_repo_labels") {
		t.Fatal("union of a toolset and a named tool is wrong")
	}
}

func TestOfNamesTheToolset(t *testing.T) {
	if got := Of("add_issue_dependency"); got != "dependencies" {
		t.Fatalf("Of = %q, want dependencies", got)
	}
	if got := Of("no_such_tool"); got != "" {
		t.Fatalf("Of unknown = %q, want empty", got)
	}
}

func TestFlagHelpListsEveryToolset(t *testing.T) {
	help := FlagHelp()
	for _, name := range Names() {
		if !strings.Contains(help, "\n  "+name+": ") {
			t.Errorf("flag help lacks toolset %q", name)
		}
	}
}

func TestNamesAreInCatalogueOrder(t *testing.T) {
	if got := Names(); len(got) == 0 || !reflect.DeepEqual(got[:2], []string{"user", "repos"}) {
		t.Fatalf("Names = %v", got)
	}
}
