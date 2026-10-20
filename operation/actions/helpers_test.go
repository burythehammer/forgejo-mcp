// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	testOwner = "o"
	testRepo  = "r"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func repoArgs(extra map[string]any) map[string]any {
	args := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// callRaw runs a tool and fails the test unless it succeeded.
func callRaw(t *testing.T, fn toolFn, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := fn(context.Background(), newCallToolRequest(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return res
}

func call(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	return textOf(callRaw(t, fn, args))
}

// callErr runs a tool and returns its error, failing the test if it succeeded.
func callErr(t *testing.T, fn toolFn, args map[string]any) error {
	t.Helper()
	res, err := fn(context.Background(), newCallToolRequest(args))
	if err == nil {
		t.Fatalf("expected an error, got result %+v", res)
	}
	return err
}

// resultOf runs a tool and decodes the {"Result": ...} envelope of its output.
func resultOf[T any](t *testing.T, fn toolFn, args map[string]any) T {
	t.Helper()
	var envelope struct {
		Result T `json:"Result"`
	}
	text := call(t, fn, args)
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, text)
	}
	return envelope.Result
}
