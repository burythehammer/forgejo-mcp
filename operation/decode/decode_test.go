// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

type sample struct {
	Owner string  `json:"owner" required:"true"`
	Index float64 `json:"index" required:"true"`
	Draft bool    `json:"draft"`
}

func req(args map[string]any) mcp.CallToolRequest {
	var r mcp.CallToolRequest
	r.Params.Arguments = args
	return r
}

func TestArgsFillsStruct(t *testing.T) {
	var a sample
	if err := Args(req(map[string]any{"owner": "o", "index": 3, "draft": true}), &a); err != nil {
		t.Fatal(err)
	}
	if a.Owner != "o" || a.Index != 3 || !a.Draft {
		t.Fatalf("got %+v", a)
	}
}

func TestArgsReportsEveryMissingRequiredField(t *testing.T) {
	var a sample
	err := Args(req(map[string]any{}), &a)
	if err == nil || !strings.Contains(err.Error(), "owner") || !strings.Contains(err.Error(), "index") {
		t.Fatalf("want error naming owner and index, got %v", err)
	}
}

func TestArgsRejectsWrongType(t *testing.T) {
	var a sample
	err := Args(req(map[string]any{"owner": 5, "index": 1}), &a)
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("want type error naming owner, got %v", err)
	}
}

func TestArgsIgnoresUnknownAndAllowsNoArguments(t *testing.T) {
	var none struct{}
	if err := Args(mcp.CallToolRequest{}, &none); err != nil {
		t.Fatal(err)
	}
	var a sample
	if err := Args(req(map[string]any{"owner": "o", "index": 1, "extra": 1}), &a); err != nil {
		t.Fatal(err)
	}
}

func TestLaxForwardsMissingAndTruncatesFloats(t *testing.T) {
	t.Parallel()
	var a struct {
		Owner string  `json:"owner" required:"true"`
		Index float64 `json:"index" required:"true"`
		Draft bool    `json:"draft"`
	}
	if err := Lax(req(map[string]any{"index": 5.5, "draft": "nope"}), &a); err != nil {
		t.Fatal(err)
	}
	if a.Owner != "" || int64(a.Index) != 5 || a.Draft {
		t.Fatalf("got %+v", a)
	}
}

type numbers struct {
	ID    int64   `json:"id"`
	Limit int     `json:"limit"`
	Ratio float64 `json:"ratio"`
}

func TestArgsAcceptsStringEncodedNumbers(t *testing.T) {
	t.Parallel()
	var a numbers
	if err := Args(req(map[string]any{"id": "7", "limit": "20", "ratio": "1.5"}), &a); err != nil {
		t.Fatal(err)
	}
	if a.ID != 7 || a.Limit != 20 || a.Ratio != 1.5 {
		t.Fatalf("got %+v", a)
	}
}

// Whitespace around a numeric string is not a number, as pkg/to.Float64 has
// always read it: " 20 " is a wrong type, not 20.
func TestArgsRejectsWhitespaceAroundStringNumber(t *testing.T) {
	t.Parallel()
	var a numbers
	err := Args(req(map[string]any{"limit": " 20 "}), &a)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("want type error naming limit, got %v", err)
	}
	if a.Limit != 0 {
		t.Fatalf("a wrongly typed field must stay zero, got %+v", a)
	}
}

func TestArgsEmptyStringNumberLeavesFieldAlone(t *testing.T) {
	t.Parallel()
	a := numbers{ID: 9}
	if err := Args(req(map[string]any{"id": ""}), &a); err != nil {
		t.Fatal(err)
	}
	if a.ID != 9 {
		t.Fatalf("got %+v", a)
	}
}

func TestArgsRejectsNonNumericStringForNumber(t *testing.T) {
	t.Parallel()
	var a numbers
	err := Args(req(map[string]any{"id": "abc"}), &a)
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("want type error naming id, got %v", err)
	}
}
