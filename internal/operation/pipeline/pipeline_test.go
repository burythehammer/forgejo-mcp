// SPDX-License-Identifier: GPL-3.0-or-later

package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type echoArgs struct {
	Name string  `json:"name" required:"true"`
	N    float64 `json:"n"`
}

// checkedArgs has a Validate method, which the pipeline runs after decoding.
type checkedArgs struct {
	Name string `json:"name"`
}

func (a checkedArgs) Validate() error {
	if a.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func invoke(h server.ToolHandlerFunc, args map[string]any) (*mcp.CallToolResult, error) {
	return h(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "echo", Arguments: args}})
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("no result content: %#v", res)
	}
	return res.Content[0].(mcp.TextContent).Text
}

func TestValidateRefusesBeforeBind(t *testing.T) {
	t.Parallel()
	bound := 0
	bind := func(context.Context) (string, error) { bound++; return "svc", nil }
	do := func(context.Context, string, checkedArgs) (*mcp.CallToolResult, error) {
		t.Fatal("do ran for a request that failed validation")
		return nil, nil
	}
	res, err := invoke(Handle(bind, do), map[string]any{})
	if err != nil {
		t.Fatalf("a bad request must be a tool error, got Go error %v", err)
	}
	if !res.IsError || !strings.Contains(resultText(t, res), "name is required") {
		t.Fatalf("got %#v, want the validation message as a tool error", res)
	}
	if bound != 0 {
		t.Fatalf("bind ran %d times for a refused request, want 0", bound)
	}
}

func TestBindErrorIsGoErrorAndSkipsDo(t *testing.T) {
	t.Parallel()
	bind := func(context.Context) (string, error) { return "", errors.New("no credential") }
	do := func(context.Context, string, checkedArgs) (*mcp.CallToolResult, error) {
		t.Fatal("do ran without a bound use case")
		return nil, nil
	}
	res, err := invoke(Handle(bind, do), map[string]any{"name": "x"})
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got res=%v err=%v, want the bind error as a Go error", res, err)
	}
}

func TestSuccessReturnsDoResult(t *testing.T) {
	t.Parallel()
	bind := func(context.Context) (string, error) { return "svc", nil }
	do := func(_ context.Context, s string, a checkedArgs) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(s + ":" + a.Name), nil
	}
	res, err := invoke(Handle(bind, do), map[string]any{"name": "x"})
	if err != nil || res.IsError {
		t.Fatalf("got res=%#v err=%v", res, err)
	}
	if got := resultText(t, res); got != "svc:x" {
		t.Fatalf("got %q, want svc:x", got)
	}
}

func TestLaxIsTheDefault(t *testing.T) {
	t.Parallel()
	var got echoArgs
	bind := func(context.Context) (struct{}, error) { return struct{}{}, nil }
	do := func(_ context.Context, _ struct{}, a echoArgs) (*mcp.CallToolResult, error) {
		got = a
		return mcp.NewToolResultText("ran"), nil
	}
	// A missing required field and a wrongly typed number are not refused by
	// default: the use case sees the zero value, as the lax handlers did.
	res, err := invoke(Handle(bind, do, Lax()), map[string]any{"n": "many"})
	if err != nil || res.IsError {
		t.Fatalf("lax refused: res=%#v err=%v", res, err)
	}
	if got.Name != "" || got.N != 0 {
		t.Fatalf("got %+v, want zero values", got)
	}
}

func TestStrictRefusesMissingAndWrongTyped(t *testing.T) {
	t.Parallel()
	bind := func(context.Context) (struct{}, error) { return struct{}{}, nil }
	do := func(context.Context, struct{}, echoArgs) (*mcp.CallToolResult, error) {
		t.Fatal("do ran for a strict refusal")
		return nil, nil
	}
	h := Handle(bind, do, Strict())
	res, err := invoke(h, map[string]any{"n": "many"})
	if err != nil {
		t.Fatalf("strict refusal must be a tool error, got Go error %v", err)
	}
	text := resultText(t, res)
	if !res.IsError || !strings.Contains(text, "name is required") || !strings.Contains(text, "n has the wrong type") {
		t.Fatalf("got %q, want both problems in one tool error", text)
	}
}

func TestPackageDefaultCanBeFlipped(t *testing.T) {
	old := defaultStrict
	defaultStrict = true
	t.Cleanup(func() { defaultStrict = old })
	bind := func(context.Context) (struct{}, error) { return struct{}{}, nil }
	do := func(context.Context, struct{}, echoArgs) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ran"), nil
	}
	// The package default is strict now, so a tool with no explicit option refuses...
	if res, _ := invoke(Handle(bind, do), map[string]any{}); !res.IsError {
		t.Fatal("package default strict did not refuse a missing required field")
	}
	// ...and a tool that asks for Lax keeps the forgiving behaviour.
	if res, _ := invoke(Handle(bind, do, Lax()), map[string]any{}); res.IsError {
		t.Fatalf("explicit Lax refused: %s", resultText(t, res))
	}
}

func TestRawIsGivenOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	bind := func(context.Context) (struct{}, error) { return struct{}{}, nil }
	var sawRaw, sawPlain bool
	withRaw := func(ctx context.Context, _ struct{}, _ checkedArgs) (*mcp.CallToolResult, error) {
		req, ok := Raw(ctx)
		sawRaw = ok && req.Params.Name == "echo"
		return mcp.NewToolResultText("ok"), nil
	}
	plain := func(ctx context.Context, _ struct{}, _ checkedArgs) (*mcp.CallToolResult, error) {
		_, sawPlain = Raw(ctx)
		return mcp.NewToolResultText("ok"), nil
	}
	args := map[string]any{"name": "x"}
	invoke(Handle(bind, withRaw, WithRaw()), args)
	invoke(Handle(bind, plain), args)
	if !sawRaw {
		t.Fatal("WithRaw did not give the tool the raw request")
	}
	if sawPlain {
		t.Fatal("a tool without WithRaw was given the raw request")
	}
}
