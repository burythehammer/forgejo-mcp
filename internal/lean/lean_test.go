// SPDX-License-Identifier: GPL-3.0-or-later

package lean

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func demoTool() mcp.Tool {
	return mcp.NewTool("demo_tool",
		mcp.WithDescription("Lists things. Pages through the result."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner")),
		mcp.WithString("state", mcp.Description("Filter by state: open|closed")),
		mcp.WithString("verbose", mcp.Description("Return the full entity instead of the compact summary.")),
		mcp.WithArray("files", mcp.Description("File operations applied atomically"),
			mcp.Items(map[string]any{
				"type":        "object",
				"description": "one operation",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "File path"},
				},
			})),
	)
}

func TestToolKeepsNamesRequiredAndSchemaShape(t *testing.T) {
	full := demoTool()
	got := Tool(full)

	if got.Name != full.Name {
		t.Errorf("Name = %q, want %q", got.Name, full.Name)
	}
	if !reflect.DeepEqual(got.InputSchema.Required, full.InputSchema.Required) {
		t.Errorf("Required = %v, want %v", got.InputSchema.Required, full.InputSchema.Required)
	}
	for name := range full.InputSchema.Properties {
		if _, ok := got.InputSchema.Properties[name]; !ok {
			t.Errorf("argument %q dropped", name)
		}
	}
	if len(got.InputSchema.Properties) != len(full.InputSchema.Properties) {
		t.Errorf("argument count = %d, want %d", len(got.InputSchema.Properties), len(full.InputSchema.Properties))
	}
}

func TestToolCutsDescriptionToItsFirstSentence(t *testing.T) {
	if got, want := Tool(demoTool()).Description, "Lists things."; got != want {
		t.Errorf("Description = %q, want %q", got, want)
	}
}

func TestToolDropsPropertyDescriptionsExceptKeptArguments(t *testing.T) {
	props := Tool(demoTool()).InputSchema.Properties

	if _, ok := props["owner"].(map[string]any)["description"]; ok {
		t.Error("owner kept a description; owner is never kept")
	}
	if _, ok := props["verbose"].(map[string]any)["description"]; ok {
		t.Error("verbose kept a description; verbose is never kept")
	}
	if got := props["state"].(map[string]any)["description"]; got != "Filter by state: open|closed" {
		t.Errorf("state description = %v, want it kept", got)
	}
	if _, ok := props["files"].(map[string]any)["description"]; !ok {
		t.Error("files lost its description; files is a kept argument")
	}
	items := props["files"].(map[string]any)["items"].(map[string]any)
	if _, ok := items["description"]; ok {
		t.Error("nested items kept a description")
	}
	inner := items["properties"].(map[string]any)["path"].(map[string]any)
	if _, ok := inner["description"]; ok {
		t.Error("nested property kept a description")
	}
}

func TestToolAnnotationsAreEmptyObject(t *testing.T) {
	raw, err := json.Marshal(Tool(demoTool()))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Annotations json.RawMessage `json:"annotations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc.Annotations) != "{}" {
		t.Errorf("annotations = %s, want {}", doc.Annotations)
	}
}

func TestKeepsMarksFormatAndEnumArgumentsOnly(t *testing.T) {
	for _, name := range []string{"state", "sort", "since", "labels", "sha", "content"} {
		if !Keeps(name) {
			t.Errorf("Keeps(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"owner", "repo", "index", "page", "limit", "verbose", "title", "body"} {
		if Keeps(name) {
			t.Errorf("Keeps(%q) = true, want false", name)
		}
	}
}

func TestFirstSentenceStopsAtTheEndOfTheFirstSentence(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Get a thing. More text here.", "Get a thing."},
		{"Single sentence with no full stop", "Single sentence with no full stop"},
		{"Only one sentence.", "Only one sentence."},
		{"Get file content by default. Set `with_metadata=true` for binary files.", "Get file content by default."},
		// A full stop inside brackets before a lowercase word is not the end.
		{"Create a branch protection rule (e.g. require status checks before merge)", "Create a branch protection rule (e.g. require status checks before merge)"},
		{"Filter by state (e.g. open or closed). Defaults to open.", "Filter by state (e.g. open or closed)."},
		{"First line.\nSecond line.", "First line."},
	}
	for _, c := range cases {
		if got := FirstSentence(c.in); got != c.want {
			t.Errorf("FirstSentence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
