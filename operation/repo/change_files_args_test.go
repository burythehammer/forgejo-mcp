// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestChangeFilesArgsMatchSchema keeps the batch's argument struct in agreement
// with the top-level tool schema.
func TestChangeFilesArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, ChangeFilesTool, changeFilesArgs{})
}

// TestChangeFileArgsMatchSchema checks one batch entry's struct against the
// schema of the files items, which mcptest cannot reach from the top level.
func TestChangeFileArgsMatchSchema(t *testing.T) {
	t.Parallel()
	items := ChangeFilesTool.InputSchema.Properties["files"].(map[string]any)["items"].(map[string]any)
	entry := mcp.Tool{
		Name: "change_files_entry",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: items["properties"].(map[string]any),
			Required:   items["required"].([]string),
		},
	}
	mcptest.ArgsMatchSchema(t, entry, changeFileArgs{})
}

// contentPair holds the two public content selectors of a write, for tests
// that exercise the selection on a plain argument map.
type contentPair struct {
	Content       decode.Arg `json:"content" schema:"string"`
	ContentBase64 decode.Arg `json:"content_base64" schema:"string"`
}

// selectFromMap runs the content selection on a plain argument map.
func selectFromMap(args map[string]any) (string, error) {
	var p contentPair
	if err := decode.Map(args, &p); err != nil {
		return "", err
	}
	return selectRepositoryWriteContent(p.Content, p.ContentBase64)
}

// TestParseChangeFilesNullIsSentNotAbsent pins the texts for a null argument.
// The legacy parser read a null as present but not a string, so it reports
// "must be a string" where an absent argument reports "is required". Golden
// tests do not send nulls, so this table is the check.
func TestParseChangeFilesNullIsSentNotAbsent(t *testing.T) {
	t.Parallel()
	base := func(files []any) map[string]any {
		return map[string]any{"owner": "o", "repo": "r", "message": "m", "branch_name": "main", "files": files}
	}
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "null message", args: map[string]any{"owner": "o", "repo": "r", "message": nil, "branch_name": "main", "files": []any{}}, want: "message must be a string"},
		{name: "absent message", args: map[string]any{"owner": "o", "repo": "r", "branch_name": "main", "files": []any{}}, want: "message is required"},
		{name: "null new_branch_name", args: map[string]any{"owner": "o", "repo": "r", "message": "m", "branch_name": "main", "new_branch_name": nil, "files": []any{}}, want: "new_branch_name must be a string"},
		{name: "null operation", args: base([]any{map[string]any{"operation": nil, "path": "a"}}), want: "files[0]: operation must be a string"},
		{name: "null sha on update", args: base([]any{map[string]any{"operation": "update", "path": "a", "content": "x", "sha": nil}}), want: "files[0]: sha must be a string"},
		{name: "null sha on create", args: base([]any{map[string]any{"operation": "create", "path": "a", "content": "x", "sha": nil}}), want: "files[0]: sha must be a string"},
		{name: "null content on create", args: base([]any{map[string]any{"operation": "create", "path": "a", "content": nil}}), want: "files[0]: content must be a string"},
		{name: "null content on delete", args: base([]any{map[string]any{"operation": "delete", "path": "a", "sha": "abc", "content": nil}}), want: "files[0]: content must be omitted for delete"},
		{name: "null content_base64 on delete", args: base([]any{map[string]any{"operation": "delete", "path": "a", "sha": "abc", "content_base64": nil}}), want: "files[0]: content_base64 must be omitted for delete"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var a changeFilesArgs
			if err := decode.Map(tt.args, &a); err != nil {
				t.Fatal(err)
			}
			_, _, _, err := parseChangeFiles(a)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error %v, want %q", err, tt.want)
			}
		})
	}
}
