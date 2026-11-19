// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"reflect"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/toolset"
)

func presetToolsOf(t *testing.T, name string) []string {
	t.Helper()
	tools, ok := toolset.PresetTools(name)
	if !ok {
		t.Fatalf("no preset %q", name)
	}
	return sortedCopy(tools)
}

// The tracker-core preset lists exactly its tools in tools/list, and no
// resource templates, as a tools-only selection does.
func TestTrackerCorePresetRegistersExactlyItsToolsAndNoTemplates(t *testing.T) {
	sel, err := toolset.ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	want := presetToolsOf(t, "tracker-core")
	if got := toolNamesOf(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools/list = %d tools %v, want the preset's %d", len(got), got, len(want))
	}
	if got := templatesOf(t, s); len(got) != 0 {
		t.Fatalf("preset registered templates %v", got)
	}
}

// A preset plus toolsets plus named tools registers the union.
func TestPresetPlusToolsetsAndToolsIsTheUnion(t *testing.T) {
	sel, err := toolset.ParseWithPreset("tracker-core", []string{"milestones"}, []string{"create_pull_request"})
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	want := sortedCopy(append(append(presetToolsOf(t, "tracker-core"), toolsOf(t, "milestones")...), "create_pull_request"))
	want = dedupe(want)
	if got := toolNamesOf(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("tools/list = %v, want %v", got, want)
	}
}

func dedupe(sorted []string) []string {
	var out []string
	for i, v := range sorted {
		if i == 0 || v != sorted[i-1] {
			out = append(out, v)
		}
	}
	return out
}
