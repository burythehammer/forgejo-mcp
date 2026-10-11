// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// A pointer field says whether an argument was given. A value of the wrong
// type must read as not given, not as a pointer to the zero value.
func TestLaxPointerFieldsReadWrongTypesAsAbsent(t *testing.T) {
	t.Parallel()
	type args struct {
		Labels *string `json:"labels"`
	}
	cases := []struct {
		name    string
		value   any
		wantNil bool
		want    string
	}{
		{"absent", nil, true, ""},
		{"given empty", "", false, ""},
		{"given", "bug", false, "bug"},
		{"wrong type", float64(5), true, ""},
		{"wrong type bool", true, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			arguments := map[string]any{}
			if c.value != nil {
				arguments["labels"] = c.value
			}
			req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: arguments}}
			var got args
			if err := Lax(req, &got); err != nil {
				t.Fatalf("Lax: %v", err)
			}
			if (got.Labels == nil) != c.wantNil {
				t.Fatalf("labels nil = %v, want %v", got.Labels == nil, c.wantNil)
			}
			if got.Labels != nil && *got.Labels != c.want {
				t.Fatalf("labels = %q, want %q", *got.Labels, c.want)
			}
		})
	}
}
