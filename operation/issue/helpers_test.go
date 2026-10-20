// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	testOwner = "octo"
	testRepo  = "hello"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return newHandlers(fake.Factory{Forgejo: f}), f
}

// withPort serves the handlers from a custom port (a fake with one method
// overridden, say).
func withPort(p forge.Forgejo) handlers { return newHandlers(fake.Factory{Forgejo: p}) }

func openIssue(t *testing.T, f *fake.Forgejo, title string) int64 {
	t.Helper()
	iss, err := f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: title, Body: "body of " + title})
	if err != nil {
		t.Fatalf("seeding issue: %v", err)
	}
	return iss.Index
}

func comment(t *testing.T, f *fake.Forgejo, index int64, body string) int64 {
	t.Helper()
	c, err := f.CreateIssueComment(context.Background(), testOwner, testRepo, index, body)
	if err != nil {
		t.Fatalf("seeding comment: %v", err)
	}
	return c.ID
}

// callErrNoRepo runs a tool with no arguments and returns its error text.
func callErrNoRepo(t *testing.T, fn toolFn) string {
	t.Helper()
	res, err := fn(context.Background(), makeReq(map[string]any{}))
	if err != nil {
		return err.Error()
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected an error, got %+v", res)
	}
	return textOf(res)
}

func repoReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return makeReq(base)
}

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// call runs a tool against testOwner/testRepo and returns its text, failing
// the test on an error result.
func call(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), repoReq(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("tool failed: err=%v res=%+v", err, res)
	}
	return textOf(res)
}

// callErr runs a tool expected to return an error result and returns its text.
func callErr(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), repoReq(args))
	if err != nil {
		return err.Error()
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected an error result, got %+v", res)
	}
	return textOf(res)
}
