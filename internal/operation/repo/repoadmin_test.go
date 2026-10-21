// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests for the RepoAdmin tools (settings, topics, branches) and the
// repo resource run against the in-memory fake, one per test, in parallel.

const adminOwner, adminRepoName = "octo", "hello"

func adminFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(adminOwner, adminRepoName)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func adminReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": adminOwner, "repo": adminRepoName}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func adminText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// adminOK calls a tool and fails the test unless it succeeded.
func adminOK(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), adminReq(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return adminText(res)
}

// adminFails calls a tool and fails the test unless it reported an error.
func adminFails(t *testing.T, fn toolFn, args map[string]any) {
	t.Helper()
	res, err := fn(context.Background(), adminReq(args))
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected an error, got %+v", res)
	}
}

func adminTopics(t *testing.T, f *fake.Forgejo) []string {
	t.Helper()
	got, err := f.ListRepoTopics(context.Background(), adminOwner, adminRepoName, forgejo_sdk.ListRepoTopicsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestEditRepoFn_DescriptionOnlyLeavesTheRestAlone(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Private: ptr.To(true)})
	adminOK(t, h.EditRepoFn, map[string]any{"description": "only desc"})
	r, _ := f.GetRepo(context.Background(), adminOwner, adminRepoName)
	if r.Description != "only desc" || !r.Private || r.Archived {
		t.Fatalf("got description=%q private=%v archived=%v", r.Description, r.Private, r.Archived)
	}
}

func TestEditRepoFn_FalseAndTrueAreApplied(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Private: ptr.To(true)})
	adminOK(t, h.EditRepoFn, map[string]any{"private": false, "archived": true})
	r, _ := f.GetRepo(context.Background(), adminOwner, adminRepoName)
	if r.Private || !r.Archived {
		t.Fatalf("got private=%v archived=%v", r.Private, r.Archived)
	}
}

func TestEditRepoFn_EmptyDescriptionClears(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Description: ptr.To("old")})
	adminOK(t, h.EditRepoFn, map[string]any{"description": ""})
	r, _ := f.GetRepo(context.Background(), adminOwner, adminRepoName)
	if r.Description != "" {
		t.Fatalf("description %q", r.Description)
	}
}

func TestEditRepoFn_EmptyEditRejected(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	_, err := h.EditRepoFn(context.Background(), adminReq(nil))
	if err == nil {
		t.Fatal("expected error when no setting fields are provided")
	}
}

func TestEditRepoFn_UnknownRepoIsAnError(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	res, err := h.EditRepoFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"owner": "nobody", "repo": "nothing", "description": "x",
	}}})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected error, got %+v", res)
	}
}

func TestGetRepoFn_ReturnsWebsiteAndTopics(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Website: ptr.To("https://nesvet.dev")})
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"go", "mcp"})
	text := adminOK(t, h.GetRepoFn, nil)
	var out struct {
		Result struct {
			Name    string   `json:"name"`
			Website string   `json:"website"`
			Topics  []string `json:"topics"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%v: %s", err, text)
	}
	if out.Result.Name != adminRepoName || out.Result.Website != "https://nesvet.dev" || fmt.Sprint(out.Result.Topics) != "[go mcp]" {
		t.Fatalf("got %+v", out.Result)
	}
}

func TestGetRepoFn_NoTopicsIsEmptyList(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	text := adminOK(t, h.GetRepoFn, nil)
	if !strings.Contains(text, `"topics":[]`) && !strings.Contains(text, `"topics": []`) {
		t.Fatalf("want empty topics list in %s", text)
	}
}

func TestGetRepoFn_UnknownRepoIsAnError(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	res, err := h.GetRepoFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"owner": "nobody", "repo": "nothing",
	}}})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected error, got %+v", res)
	}
}

func TestListRepoTopicsFn_Envelope(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"go", "mcp"})
	var envelope struct {
		Result model.TopicPage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(adminOK(t, h.ListRepoTopicsFn, nil)), &envelope); err != nil {
		t.Fatal(err)
	}
	r := envelope.Result
	if r.Page != 1 || r.Limit != 100 || r.Count != 2 || fmt.Sprint(r.Topics) != "[go mcp]" {
		t.Fatalf("got %+v", r)
	}
}

func TestListRepoTopicsFn_Paginates(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"a", "b", "c"})
	var envelope struct {
		Result model.TopicPage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(adminOK(t, h.ListRepoTopicsFn, map[string]any{"page": 2, "limit": 2})), &envelope); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(envelope.Result.Topics) != "[c]" || envelope.Result.Page != 2 || envelope.Result.Limit != 2 {
		t.Fatalf("got %+v", envelope.Result)
	}
}

func TestSetRepoTopicsFn_NormalizesCSV(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	adminOK(t, h.SetRepoTopicsFn, map[string]any{"topics": "go, MCP, go"})
	if got := adminTopics(t, f); fmt.Sprint(got) != "[go mcp]" {
		t.Fatalf("got %v", got)
	}
}

func TestSetRepoTopicsFn_EmptyClears(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"old"})
	text := adminOK(t, h.SetRepoTopicsFn, map[string]any{"topics": ""})
	if got := adminTopics(t, f); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if !strings.Contains(text, `[]`) {
		t.Fatalf("topics must be [] not null: %s", text)
	}
}

func TestSetRepoTopicsFn_MissingKeyLeavesTopics(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"keep"})
	adminFails(t, h.SetRepoTopicsFn, nil)
	if got := adminTopics(t, f); fmt.Sprint(got) != "[keep]" {
		t.Fatalf("got %v", got)
	}
}

func TestSetRepoTopicsFn_TooManyLeavesTopics(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	names := make([]string, 26)
	for i := range names {
		names[i] = fmt.Sprintf("t%d", i)
	}
	adminFails(t, h.SetRepoTopicsFn, map[string]any{"topics": strings.Join(names, ",")})
	if got := adminTopics(t, f); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestAddAndDeleteRepoTopic(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	adminOK(t, h.AddRepoTopicFn, map[string]any{"topic": "CI"})
	if got := adminTopics(t, f); fmt.Sprint(got) != "[ci]" {
		t.Fatalf("after add: %v", got)
	}
	adminOK(t, h.DeleteRepoTopicFn, map[string]any{"topic": "ci"})
	if got := adminTopics(t, f); len(got) != 0 {
		t.Fatalf("after delete: %v", got)
	}
	adminFails(t, h.DeleteRepoTopicFn, map[string]any{"topic": "ci"})
}

func TestAddRepoTopicFn_InvalidNameChangesNothing(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	adminFails(t, h.AddRepoTopicFn, map[string]any{"topic": "Bad Name"})
	if got := adminTopics(t, f); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestBranchTools(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	if text := adminOK(t, h.CreateBranchFn, map[string]any{"branch": "feat", "old_branch": "main"}); text != "Branch Created" {
		t.Fatalf("got %q", text)
	}
	if text := adminOK(t, h.ListBranchesFn, map[string]any{"page": 1, "limit": 100}); !strings.Contains(text, `"feat"`) || !strings.Contains(text, `"main"`) {
		t.Fatalf("list: %s", text)
	}
	if text := adminOK(t, h.DeleteBranchFn, map[string]any{"branch": "feat"}); !strings.Contains(text, "Delete Branch Success") {
		t.Fatalf("got %q", text)
	}
	bs, _ := f.ListBranches(context.Background(), adminOwner, adminRepoName, forgejo_sdk.ListRepoBranchesOptions{})
	if len(bs) != 1 || bs[0].Name != "main" {
		t.Fatalf("got %+v", bs)
	}
}

func TestBranchTools_Failures(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	adminFails(t, h.CreateBranchFn, map[string]any{"branch": "x", "old_branch": "ghost"})
	adminFails(t, h.CreateBranchFn, map[string]any{"branch": "main", "old_branch": "main"})
	adminFails(t, h.DeleteBranchFn, map[string]any{"branch": "ghost"})
	adminFails(t, h.DeleteBranchFn, map[string]any{"branch": "main"})
	adminFails(t, h.ListBranchesFn, map[string]any{"owner": "nobody"})
}

func TestBranchTools_UsePerRequestFactory(t *testing.T) {
	t.Parallel()
	want := errors.New("no credential")
	h := handlers{forge: fake.Factory{Err: want}}
	adminFails(t, h.ListBranchesFn, nil)
}

func repoResourceRequest(owner, repo string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/" + owner + "/" + repo}}
}

func TestRepoResourceHandler_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Description: ptr.To("MCP server")})
	contents, err := h.repoResourceHandler(context.Background(), repoResourceRequest(adminOwner, adminRepoName))
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 {
		t.Fatalf("want 1 block, got %d", len(contents))
	}
	block, ok := contents[0].(mcp.TextResourceContents)
	if !ok || block.MIMEType != "application/json" {
		t.Fatalf("block %+v", contents[0])
	}
	var p repoResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &p); err != nil {
		t.Fatal(err)
	}
	if p.Owner != adminOwner || p.Name != adminRepoName || p.FullName != "octo/hello" || p.Description != "MCP server" || p.DefaultBranch != "main" || !p.HasIssues {
		t.Fatalf("got %+v", p)
	}
}

func TestRepoResourceHandler_UnknownRepoIsNotFound(t *testing.T) {
	t.Parallel()
	h, _ := adminFixture(t)
	_, err := h.repoResourceHandler(context.Background(), repoResourceRequest("nobody", "nothing"))
	if err == nil {
		t.Fatal("expected error")
	}
	var re *resource.ResourceError
	if !errors.As(err, &re) || re.Code != -32003 {
		t.Fatalf("want not-found resource error (-32003), got %v", err)
	}
}

func TestRepoResourceHandler_FactoryErrorIsReported(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: errors.New("no credential")}}
	_, err := h.repoResourceHandler(context.Background(), repoResourceRequest(adminOwner, adminRepoName))
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v", err)
	}
}
