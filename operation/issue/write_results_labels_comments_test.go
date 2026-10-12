// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"encoding/json"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
)

// The label and comment writes return the compact shape by default, with the
// optional verbose argument restoring the full entity (see operation/compact).

func jsonInt(t *testing.T, raw json.RawMessage) int64 {
	t.Helper()
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("not an integer: %s: %v", raw, err)
	}
	return n
}

func TestLabelWrites_CompactByDefault(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newLabel(t, f, "bug")

	added := call(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "bug"})
	keys := mcptest.EnvelopeKeys(t, added)
	for _, k := range []string{"number", "url", "title", "state", "labels"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("add_issue_labels compact result misses %q: %s", k, added)
		}
	}
	if _, ok := keys["user"]; ok {
		t.Fatalf("add_issue_labels compact result carries the user object: %s", added)
	}
	if !strings.Contains(added, `"bug"`) || strings.Contains(added, "body of t") {
		t.Fatalf("add_issue_labels compact result wrong: %s", added)
	}

	removed := call(t, h.RemoveIssueLabelsFn, map[string]any{"index": idx, "labels": "bug"})
	keys = mcptest.EnvelopeKeys(t, removed)
	if _, ok := keys["user"]; ok || keys["number"] == nil {
		t.Fatalf("remove_issue_labels compact result wrong: %s", removed)
	}
	if strings.Contains(removed, `"bug"`) {
		t.Fatalf("removed label still listed: %s", removed)
	}
}

func TestLabelWrites_VerboseReturnsFullEntity(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newLabel(t, f, "bug")

	added := call(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "bug", "verbose": true})
	keys := mcptest.EnvelopeKeys(t, added)
	if _, ok := keys["body"]; !ok {
		t.Fatalf("add_issue_labels verbose has no body key: %s", added)
	}
	if _, ok := keys["user"]; !ok {
		t.Fatalf("add_issue_labels verbose has no user key: %s", added)
	}

	removed := call(t, h.RemoveIssueLabelsFn, map[string]any{"index": idx, "labels": "bug", "verbose": true})
	if _, ok := mcptest.EnvelopeKeys(t, removed)["body"]; !ok {
		t.Fatalf("remove_issue_labels verbose has no body key: %s", removed)
	}
}

func TestCommentWrites_CompactByDefault(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))

	created := call(t, h.CreateIssueCommentFn, map[string]any{"index": idx, "body": "COMMENT-BODY"})
	keys := mcptest.EnvelopeKeys(t, created)
	for _, k := range []string{"id", "url", "updated_at"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("create_issue_comment compact result misses %q: %s", k, created)
		}
	}
	if len(keys) != 3 || strings.Contains(created, "COMMENT-BODY") {
		t.Fatalf("create_issue_comment compact result wrong: %s", created)
	}

	id := mcptest.EnvelopeKeys(t, created)["id"]
	edited := call(t, h.EditIssueCommentFn, map[string]any{"comment_id": float64(jsonInt(t, id)), "body": "EDITED-BODY"})
	if len(mcptest.EnvelopeKeys(t, edited)) != 3 || strings.Contains(edited, "EDITED-BODY") {
		t.Fatalf("edit_issue_comment compact result wrong: %s", edited)
	}
}

func TestCommentWrites_VerboseReturnsFullEntity(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))

	created := call(t, h.CreateIssueCommentFn, map[string]any{"index": idx, "body": "VERBOSE-BODY", "verbose": true})
	keys := mcptest.EnvelopeKeys(t, created)
	if _, ok := keys["body"]; !ok || !strings.Contains(created, "VERBOSE-BODY") {
		t.Fatalf("create_issue_comment verbose must return the full comment: %s", created)
	}
	if _, ok := keys["user"]; !ok {
		t.Fatalf("create_issue_comment verbose has no user key: %s", created)
	}

	id := jsonInt(t, keys["id"])
	edited := call(t, h.EditIssueCommentFn, map[string]any{"comment_id": float64(id), "body": "VERBOSE-EDIT", "verbose": true})
	if !strings.Contains(edited, "VERBOSE-EDIT") {
		t.Fatalf("edit_issue_comment verbose must return the full comment: %s", edited)
	}
}
