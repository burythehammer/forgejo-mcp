package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeStatusResourceRequest(sha string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/" + filesOwner + "/" + filesRepo + "/commit/" + sha + "/status",
		},
	}
}

// seedStatuses puts one status per state on testSHA.
func seedStatuses(t *testing.T, f *fake.Forgejo, states ...forgejo_sdk.StatusState) {
	t.Helper()
	for i, s := range states {
		if err := f.AddCommitStatus(filesOwner, filesRepo, testSHA, fmt.Sprintf("ci/test-%d", i), s, "test context "+string(s)); err != nil {
			t.Fatalf("seeding status: %v", err)
		}
	}
}

func statusPayload(t *testing.T, contents []mcp.ResourceContents) statusResourcePayload {
	t.Helper()
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	block, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("content block must be TextResourceContents")
	}
	if block.MIMEType != "application/json" {
		t.Errorf("MIME type: got %q, want application/json", block.MIMEType)
	}
	var payload statusResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	return payload
}

func TestStatusResourceHandler_HappyPath_UnderCap(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedStatuses(t, f, forgejo_sdk.StatusSuccess, forgejo_sdk.StatusSuccess, forgejo_sdk.StatusPending)

	contents, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload := statusPayload(t, contents)
	if payload.State != "pending" {
		t.Errorf("expected aggregate state=pending (has one pending), got %q", payload.State)
	}
	if payload.TotalCount != 3 {
		t.Errorf("expected total_count=3, got %d", payload.TotalCount)
	}
	if payload.Truncated {
		t.Error("expected truncated=false for 3 statuses under cap")
	}
	if len(payload.Statuses) != 3 {
		t.Errorf("expected 3 status items, got %d", len(payload.Statuses))
	}
}

func TestStatusResourceHandler_OverCap_Truncated(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	capPlus1 := resource.EmbeddedListCap + 1
	states := make([]forgejo_sdk.StatusState, capPlus1)
	for i := range states {
		states[i] = forgejo_sdk.StatusSuccess
	}
	seedStatuses(t, f, states...)

	contents, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload := statusPayload(t, contents)
	if !payload.Truncated {
		t.Errorf("expected truncated=true for %d statuses (EmbeddedListCap+1)", capPlus1)
	}
	if len(payload.Statuses) != resource.EmbeddedListCap {
		t.Errorf("expected %d capped statuses, got %d", resource.EmbeddedListCap, len(payload.Statuses))
	}
	if payload.ListTool != "get_commit_statuses" {
		t.Errorf("expected list_tool=get_commit_statuses, got %q", payload.ListTool)
	}
	if payload.TotalCount != capPlus1 {
		t.Errorf("expected total_count=%d, got %d", capPlus1, payload.TotalCount)
	}
}

func TestStatusResourceHandler_ExactlyAtCap_NotTruncated(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	states := make([]forgejo_sdk.StatusState, resource.EmbeddedListCap)
	for i := range states {
		states[i] = forgejo_sdk.StatusSuccess
	}
	seedStatuses(t, f, states...)

	contents, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload := statusPayload(t, contents); payload.Truncated || len(payload.Statuses) != resource.EmbeddedListCap {
		t.Errorf("exactly at cap must not truncate: truncated=%v len=%d", payload.Truncated, len(payload.Statuses))
	}
}

func TestStatusResourceHandler_EmptyStatuses_StateUnknown(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "a commit with no checks"); err != nil {
		t.Fatal(err)
	}

	contents, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload := statusPayload(t, contents); payload.State != "unknown" {
		t.Errorf("expected state=unknown for empty statuses, got %q", payload.State)
	}
}

func TestStatusResourceHandler_ShortSHA(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	_, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest("abc123"))
	if err == nil {
		t.Fatal("expected error for short sha")
	}
}

func TestStatusResourceHandler_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	_, err := h.statusResourceHandler(context.Background(), makeStatusResourceRequest(testSHA))
	var resErr *resource.ResourceError
	if err == nil {
		t.Fatal("expected error for an unknown commit")
	}
	if e, ok := err.(*resource.ResourceError); ok {
		resErr = e
	}
	if resErr == nil || resErr.Code != -32003 {
		t.Fatalf("want a not-found resource error (-32003), got %T %v", err, err)
	}
}

func TestComputeAggregateState_AllSuccess(t *testing.T) {
	s := forgejo_sdk.StatusSuccess
	statuses := []*forgejo_sdk.Status{{State: s}, {State: s}}
	if got := computeAggregateState(statuses); got != "success" {
		t.Errorf("expected success, got %q", got)
	}
}

func TestComputeAggregateState_AnyFailure(t *testing.T) {
	statuses := []*forgejo_sdk.Status{
		{State: forgejo_sdk.StatusSuccess},
		{State: forgejo_sdk.StatusFailure},
	}
	if got := computeAggregateState(statuses); got != "failure" {
		t.Errorf("expected failure, got %q", got)
	}
}

func TestComputeAggregateState_NilOnlyIsUnknown(t *testing.T) {
	if got := computeAggregateState([]*forgejo_sdk.Status{nil, nil}); got != "unknown" {
		t.Errorf("got %q, want unknown", got)
	}
}

func TestToStatusItem_Nil(t *testing.T) {
	item, ok := toStatusItem(nil)
	if ok {
		t.Fatalf("nil must be skipped, got %+v", item)
	}
	if item != (statusItem{}) {
		t.Errorf("zero item: got %+v", item)
	}
}

func TestToStatusItem_MapsFields(t *testing.T) {
	created := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	got, ok := toStatusItem(&forgejo_sdk.Status{
		Context:     "ci/woodpecker",
		State:       forgejo_sdk.StatusSuccess,
		TargetURL:   "https://ci.example/1",
		Description: "ok",
		Created:     created,
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Context != "ci/woodpecker" || got.State != "success" || got.TargetURL != "https://ci.example/1" || got.Description != "ok" {
		t.Errorf("item: %+v", got)
	}
	if got.CreatedAt != "2026-08-01T12:00:00Z" {
		t.Errorf("created_at: got %q", got.CreatedAt)
	}
}
