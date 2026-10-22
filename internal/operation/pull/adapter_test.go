// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/sdk"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (internal/app/pull) and pinned on the wire by golden_test.go. What is left to check
// here is that each tool's argument struct agrees with its schema, that a tool
// reaches its use case through the real handler, and that the three failure
// paths (bad arguments, no credential, a use-case error) reach the client.

type toolFn = func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{GetPullRequestByIndexTool, getArgs{}},
		{ListRepoPullRequestsTool, listArgs{}},
		{CreatePullRequestTool, createArgs{}},
		{UpdatePullRequestTool, updateArgs{}},
		{ListPullReviewsTool, pagedArgs{}},
		{GetPullReviewTool, getReviewArgs{}},
		{ListPullReviewCommentsTool, reviewCommentsArgs{}},
		{MergePullRequestTool, mergeArgs{}},
		{ListPullRequestFilesTool, pagedArgs{}},
		{GetPullRequestDiffTool, diffArgs{}},
		{CreatePullReviewTool, createReviewArgs{}},
		{SubmitPullReviewTool, submitReviewArgs{}},
		{DismissPullReviewTool, dismissArgs{}},
		{DeletePullReviewTool, reviewArgs{}},
		{CreateReviewRequestsTool, reviewersArgs{}},
		{DeleteReviewRequestsTool, reviewersArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func(handlers) toolFn
		args map[string]any
		want string
	}{
		{"get", func(h handlers) toolFn { return h.GetPullRequestByIndexFn }, map[string]any{"index": float64(1)}, "seeded"},
		{"list", func(h handlers) toolFn { return h.ListRepoPullRequestsFn }, nil, "seeded"},
		{"files", func(h handlers) toolFn { return h.ListPullRequestFilesFn }, map[string]any{"index": float64(1)}, "Result"},
		{"diff", func(h handlers) toolFn { return h.GetPullRequestDiffFn }, map[string]any{"index": float64(1)}, "diff --git"},
		{"merge", func(h handlers) toolFn { return h.MergePullRequestFn }, map[string]any{"index": float64(1), "style": "merge"}, "merged successfully"},
		{"update", func(h handlers) toolFn { return h.UpdatePullRequestFn }, map[string]any{"index": float64(1), "title": "renamed"}, "renamed"},
		{"create review", func(h handlers) toolFn { return h.CreatePullReviewFn }, map[string]any{"index": float64(1), "state": "COMMENT", "body": "hmm"}, "hmm"},
		{"request reviews", func(h handlers) toolFn { return h.CreateReviewRequestsFn }, map[string]any{"index": float64(1), "reviewers": "bob"}, "created successfully"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h, f := newFixture(t)
			idx := openPR(t, f, "seeded")
			if err := f.SetPullRequestDiff(testOwner, testRepo, idx, "diff --git a/x b/x\n+x\n"); err != nil {
				t.Fatal(err)
			}
			if got := call(t, c.fn(h), c.args); !strings.Contains(got, c.want) {
				t.Fatalf("got %s, want it to contain %q", got, c.want)
			}
		})
	}
}

// A missing index is refused with the numeric-argument module's message
// before Forgejo is asked (#246, under #228).
func TestMissingRequiredIndexIsRefused(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	res, err := h.GetPullRequestByIndexFn(context.Background(), mcp.CallToolRequest{})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("want an error result, got err=%v res=%+v", err, res)
	}
	if text := res.Content[0].(mcp.TextContent).Text; !strings.Contains(text, "index is required") {
		t.Fatalf("got %q, want it to say index is required", text)
	}
}

// A numeric string is a number on every tool, so "1" resolves. A fraction is
// refused, not truncated into the pull request below it (#246, under #228).
func TestNonWholeIndexIsRefusedNumericStringResolves(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "five")
	if got := call(t, h.GetPullRequestByIndexFn, map[string]any{"index": "1"}); !strings.Contains(got, "five") {
		t.Fatalf("index \"1\": %s", got)
	}
	for _, v := range []any{float64(idx) + 0.5, "1.9"} {
		res, err := h.GetPullRequestByIndexFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"index": v}}})
		if err != nil || res == nil || !res.IsError {
			t.Fatalf("index %v: want an error result, got err=%v res=%+v", v, err, res)
		}
		if text := res.Content[0].(mcp.TextContent).Text; !strings.Contains(text, "index must be an integer between 1 and 9223372036854775807") {
			t.Fatalf("index %v: got %q", v, text)
		}
	}
}

func TestToolsFailWhenFactoryRefuses(t *testing.T) {
	t.Parallel()
	refusal := errors.New("no credential for this request")
	h := handlers{forge: fake.Factory{Err: refusal}}
	_, err := h.GetPullRequestByIndexFn(context.Background(), makeReq(map[string]any{"index": float64(1)}))
	if !errors.Is(err, refusal) {
		t.Fatalf("got %v, want the factory's refusal", err)
	}
}

func TestUseCaseErrorReachesTheClient(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.GetPullRequestByIndexFn(context.Background(), makeReq(map[string]any{"index": float64(99)}))
	if err == nil || !strings.Contains(err.Error(), "get pull request err: ") || !strings.Contains(err.Error(), "(code -32003)") {
		t.Fatalf("want the use case's error as a resource error, got %v", err)
	}
}

// The one smoke test that goes over HTTP: a registered tool, the real SDK
// adapter and a stub Forgejo, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo(testOwner, testRepo)
	port := sdk.NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL)

	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: port})
	call := func(tool string, args map[string]any) string {
		t.Helper()
		res, err := s.GetTool(tool).Handler(context.Background(), makeReq(args))
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", tool, err, res)
		}
		return textOf(res)
	}

	created := call(CreatePullRequestToolName, map[string]any{"head": "feat", "base": "main", "title": "over the wire"})
	if !strings.Contains(created, "over the wire") {
		t.Fatalf("create: %s", created)
	}
	if got := call(GetPullRequestByIndexToolName, map[string]any{"index": float64(1)}); !strings.Contains(got, "over the wire") {
		t.Fatalf("get: %s", got)
	}
	if got := call(MergePullRequestToolName, map[string]any{"index": float64(1), "style": "merge"}); got != "Pull request merged successfully" {
		t.Fatalf("merge: %s", got)
	}
}
