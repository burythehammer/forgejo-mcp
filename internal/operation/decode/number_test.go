// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// An optional number tagged numeric:"string" is read like a number: a numeric string is accepted, as
// it is for a plain number field. A field tagged exact:"true" refuses a
// string, for a tool whose handler asserted float64 and would have ignored it.
func TestNumberFieldsAcceptStringsUnlessExact(t *testing.T) {
	t.Parallel()
	type args struct {
		Plain       float64  `json:"plain"`
		Optional    *float64 `json:"optional" numeric:"string"`
		StrictPlain float64  `json:"strict_plain" exact:"true"`
		StrictOpt   *float64 `json:"strict_optional" numeric:"string" exact:"true"`
	}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"plain": "7", "optional": "7", "strict_plain": "7", "strict_optional": "7",
	}}}
	var got args
	if err := Lax(req, &got); err != nil {
		t.Fatalf("Lax: %v", err)
	}
	if got.Plain != 7 {
		t.Errorf("plain = %v, want 7", got.Plain)
	}
	if got.Optional == nil || *got.Optional != 7 {
		t.Errorf("optional = %v, want 7", got.Optional)
	}
	if got.StrictPlain != 0 {
		t.Errorf("strict plain = %v, want the string refused (0)", got.StrictPlain)
	}
	if got.StrictOpt != nil {
		t.Errorf("strict optional = %v, want the string refused (nil)", *got.StrictOpt)
	}
}

func TestExactNumberStillAcceptsANumber(t *testing.T) {
	t.Parallel()
	type args struct {
		Page *float64 `json:"page" exact:"true"`
	}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"page": float64(3)}}}
	var got args
	if err := Lax(req, &got); err != nil {
		t.Fatalf("Lax: %v", err)
	}
	if got.Page == nil || *got.Page != 3 {
		t.Fatalf("page = %v, want 3", got.Page)
	}
}
