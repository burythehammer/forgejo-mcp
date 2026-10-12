// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"strings"
	"testing"
)

// withRegistry installs names as the registered-tool set for one test and
// restores the unset state afterwards, so tests cannot leak a registry.
func withRegistry(t *testing.T, names []string) {
	t.Helper()
	SetRegisteredTools(names)
	t.Cleanup(func() { SetRegisteredTools(nil) })
}

func thirtyFive() []string {
	items := make([]string, 35)
	for i := range items {
		items[i] = "x"
	}
	return items
}

func TestSentinel_NamesRegisteredTool_TextUnchanged(t *testing.T) {
	withRegistry(t, []string{"list_repo_issues"})
	got := Bounded(thirtyFive(), 30, "list_repo_issues").Sentinel()
	want := "[truncated: 30 of 35 items shown. Use list_repo_issues tool to fetch more.]"
	if got != want {
		t.Fatalf("sentinel = %q, want %q", got, want)
	}
}

func TestSentinel_UnknownTotal_TextUnchanged(t *testing.T) {
	withRegistry(t, []string{"list_repo_issues"})
	// Truncated but total not reported: total == cap takes the "more remain" branch.
	b := Bounded(thirtyFive(), 30, "list_repo_issues").WithMoreRemaining(0)
	got := b.Sentinel()
	want := "[truncated: 30 items shown, more remain. Use list_repo_issues tool to fetch more.]"
	if got != want {
		t.Fatalf("sentinel = %q, want %q", got, want)
	}
}

func TestSentinel_OmitsUnregisteredTool(t *testing.T) {
	withRegistry(t, []string{"list_repo_issues"})
	got := Bounded(thirtyFive(), 30, "list_removed_tool").Sentinel()
	if strings.Contains(got, "list_removed_tool") {
		t.Fatalf("sentinel names an unregistered tool: %q", got)
	}
	want := "[truncated: 30 of 35 items shown.]"
	if got != want {
		t.Fatalf("sentinel = %q, want %q", got, want)
	}
}

func TestToolRef_NoRegistryPassesThrough(t *testing.T) {
	SetRegisteredTools(nil)
	if got := ToolRef("list_anything"); got != "list_anything" {
		t.Fatalf("ToolRef without a registry = %q, want the name unchanged", got)
	}
}

func TestToolRef_RegistryGatesNames(t *testing.T) {
	withRegistry(t, []string{"get_wiki_page"})
	if got := ToolRef("get_wiki_page"); got != "get_wiki_page" {
		t.Fatalf("registered name = %q", got)
	}
	if got := ToolRef("get_wiki_gone"); got != "" {
		t.Fatalf("unregistered name = %q, want empty", got)
	}
}
