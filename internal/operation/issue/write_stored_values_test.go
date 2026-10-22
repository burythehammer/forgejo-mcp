// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The issue writes report what the server stored, taken from its response
// (see internal/operation/compact). These tests compute the expected hash with
// crypto/sha256 directly, not through the helper under test.

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// driftPort stores something other than what the request asked for: the
// server keeps a different body and drops the assignees. The override is on
// the write responses, which is where a result must read from.
type driftPort struct {
	forge.Forgejo
	storedBody string
}

func (p driftPort) EditIssue(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.EditIssueOption) (*forgejo_sdk.Issue, error) {
	iss, err := p.Forgejo.EditIssue(ctx, owner, repo, index, opt)
	if err != nil {
		return nil, err
	}
	iss.Body = p.storedBody
	iss.Assignees = nil
	return iss, nil
}

func (p driftPort) CreateIssue(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateIssueOption) (*forgejo_sdk.Issue, error) {
	iss, err := p.Forgejo.CreateIssue(ctx, owner, repo, opt)
	if err != nil {
		return nil, err
	}
	iss.Body = p.storedBody
	iss.Assignees = nil
	return iss, nil
}

func TestUpdateIssue_ReportsStoredMilestoneAndBodyHash(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	mid := f.SeedMilestone(testOwner, testRepo, "v1")

	got := result(t, call(t, h.UpdateIssueFn, map[string]any{
		"index": idx, "body": "NEW BODY", "milestone": fmt.Sprint(mid),
	}))
	if got["body_sha256"] != sha256Hex("NEW BODY") {
		t.Fatalf("update_issue body_sha256 = %v, want hash of the stored body", got["body_sha256"])
	}
	if ms, _ := got["milestone"].(map[string]any); ms == nil || ms["id"] != float64(mid) {
		t.Fatalf("update_issue milestone = %v, want id %d", got["milestone"], mid)
	}
}

func TestUpdateIssue_NoBodySentNoHash(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))

	got := result(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "title": "renamed"}))
	if _, ok := got["body_sha256"]; ok {
		t.Fatalf("update_issue without a body must not report body_sha256: %v", got)
	}
}

func TestCreateIssue_ReportsStoredMilestoneAndBodyHash(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	mid := f.SeedMilestone(testOwner, testRepo, "v1")

	got := result(t, call(t, h.CreateIssueFn, map[string]any{
		"title": "new", "body": "SPEC BODY", "milestone": fmt.Sprint(mid),
	}))
	if got["body_sha256"] != sha256Hex("SPEC BODY") {
		t.Fatalf("create_issue body_sha256 = %v, want hash of the stored body", got["body_sha256"])
	}
	if ms, _ := got["milestone"].(map[string]any); ms == nil || ms["id"] != float64(mid) {
		t.Fatalf("create_issue milestone = %v, want id %d", got["milestone"], mid)
	}
}

func TestCreateIssue_NoBodySentNoHash(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	got := result(t, call(t, h.CreateIssueFn, map[string]any{"title": "bare"}))
	if _, ok := got["body_sha256"]; ok {
		t.Fatalf("create_issue without a body must not report body_sha256: %v", got)
	}
}

func TestWriteResults_ShowWhatTheServerStored(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	h := withPort(driftPort{Forgejo: f, storedBody: "STORED BODY"})

	created := result(t, call(t, h.CreateIssueFn, map[string]any{
		"title": "drift", "body": "REQUESTED BODY", "assignees": "alice",
	}))
	if created["body_sha256"] != sha256Hex("STORED BODY") {
		t.Fatalf("create_issue reported %v, want the hash of what the server stored", created["body_sha256"])
	}
	if a, _ := created["assignees"].([]any); len(a) != 0 {
		t.Fatalf("create_issue reported dropped assignee as stored: %v", created["assignees"])
	}

	idx := float64(openIssue(t, f, "u"))
	updated := result(t, call(t, h.UpdateIssueFn, map[string]any{
		"index": idx, "body": "REQUESTED BODY", "assignees": "bob",
	}))
	if updated["body_sha256"] != sha256Hex("STORED BODY") {
		t.Fatalf("update_issue reported %v, want the hash of what the server stored", updated["body_sha256"])
	}
	if a, _ := updated["assignees"].([]any); len(a) != 0 {
		t.Fatalf("update_issue reported dropped assignee as stored: %v", updated["assignees"])
	}
}
