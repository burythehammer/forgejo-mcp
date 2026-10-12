// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// An optional number reads a JSON number, and reads a string as absent, unless
// it is tagged numeric:"string", when it reads a numeric string too (and a
// malformed one as absent).
func TestLaxOptionalNumberTakesStringNumbersOnlyWhenTagged(t *testing.T) {
	t.Parallel()
	type args struct {
		Seconds *float64 `json:"seconds"`
		TimeID  *float64 `json:"time_id" numeric:"string"`
	}
	cases := []struct {
		name    string
		field   string
		value   any
		wantNil bool
		want    float64
	}{
		{"number", "seconds", 90.0, false, 90},
		{"string number refused", "seconds", "90", true, 0},
		{"tagged number", "time_id", 90.0, false, 90},
		{"tagged string number", "time_id", "90", false, 90},
		{"tagged empty string", "time_id", "", true, 0},
		{"tagged malformed string", "time_id", "soon", true, 0},
		{"tagged bool", "time_id", true, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{c.field: c.value}}}
			var got args
			if err := Lax(req, &got); err != nil {
				t.Fatalf("Lax: %v", err)
			}
			v := got.Seconds
			if c.field == "time_id" {
				v = got.TimeID
			}
			if (v == nil) != c.wantNil || (v != nil && *v != c.want) {
				t.Fatalf("%s = %v, want nil=%v value %v", c.field, v, c.wantNil, c.want)
			}
		})
	}
}

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
