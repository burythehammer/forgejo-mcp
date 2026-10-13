// SPDX-License-Identifier: GPL-3.0-or-later

package mcpresult

import (
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) != 1 {
		t.Fatalf("result = %+v, want one content item", res)
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content = %T, want mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestRenderWrapsResultAsJSON(t *testing.T) {
	t.Parallel()
	res, err := Render(map[string]int{"n": 1}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := text(t, res); got != `{"Result":{"n":1}}` {
		t.Fatalf("text = %s", got)
	}
}

func TestRenderSendsErrorAsGoError(t *testing.T) {
	t.Parallel()
	want := errors.New("forge said no")
	res, err := Render("ignored", want)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if res != nil {
		t.Fatalf("result = %+v, want nil", res)
	}
}

func TestFailReturnsErrorUnchanged(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	res, err := Fail(want)
	if err != want { //nolint:errorlint // identity, not errors.Is, is what is under test
		t.Fatalf("err = %v, want the same error", err)
	}
	if res != nil {
		t.Fatalf("result = %+v, want nil", res)
	}
}
