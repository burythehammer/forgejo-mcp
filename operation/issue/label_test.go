// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// ---- normalizeColor ----

func TestNormalizeColor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in  string
		out string
		err bool
	}{
		{"0088ff", "#0088ff", false},
		{"#0088ff", "#0088ff", false},
		{"#0088FF", "#0088ff", false},
		{"0088FF", "#0088ff", false},
		{"abc", "", true},    // 3-digit rejected
		{"#abc", "", true},   // 3-digit rejected
		{"gggggg", "", true}, // invalid hex
		{"", "", true},
	}
	for _, c := range cases {
		got, err := normalizeColor(c.in)
		if c.err {
			if err == nil {
				t.Errorf("normalizeColor(%q): want error, got %q", c.in, got)
			}
		} else {
			if err != nil {
				t.Errorf("normalizeColor(%q): unexpected error: %v", c.in, err)
			} else if got != c.out {
				t.Errorf("normalizeColor(%q): got %q, want %q", c.in, got, c.out)
			}
		}
	}
}

// ---- create_repo_label ----

func TestCreateRepoLabelFn_CreatesIt(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	out := call(t, h.CreateRepoLabelFn, map[string]any{"name": "bug", "color": "0088ff", "description": "a bug"})
	if !strings.Contains(out, `"name":"bug"`) {
		t.Fatalf("got %q", out)
	}
	labels, _, err := f.ListRepoLabels(context.Background(), testOwner, testRepo, forgejo_sdk.ListLabelsOptions{})
	if err != nil || len(labels) != 1 || labels[0].Description != "a bug" {
		t.Fatalf("labels = %+v err=%v", labels, err)
	}
}

func TestCreateRepoLabelFn_InvalidColor(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	callErr(t, h.CreateRepoLabelFn, map[string]any{"name": "bug", "color": "gg00ff"})
	if labels, _, _ := f.ListRepoLabels(context.Background(), testOwner, testRepo, forgejo_sdk.ListLabelsOptions{}); len(labels) != 0 {
		t.Fatalf("an invalid colour must create nothing, got %+v", labels)
	}
}

// ---- edit_repo_label ----

func TestEditRepoLabelFn_OnlyTheSuppliedFieldsChange(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "bug")

	call(t, h.EditRepoLabelFn, map[string]any{"id": float64(id), "name": "bug-renamed"})
	l, err := f.GetRepoLabel(context.Background(), testOwner, testRepo, id)
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "bug-renamed" || l.Color != "aabbcc" {
		t.Fatalf("got %+v", l)
	}
}

func TestEditRepoLabelFn_EmptyReject(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "bug")

	msg := callErr(t, h.EditRepoLabelFn, map[string]any{"id": float64(id)})
	if !strings.Contains(msg, "at least one of name, color, description") {
		t.Fatalf("got %q", msg)
	}
}

// ---- delete_repo_label ----

func TestDeleteRepoLabelFn_UnusedSuccess(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "unused")

	out := call(t, h.DeleteRepoLabelFn, map[string]any{"id": float64(id)})
	if !strings.Contains(out, `"deleted":true`) {
		t.Fatalf("got %q", out)
	}
	if _, err := f.GetRepoLabel(context.Background(), testOwner, testRepo, id); err == nil {
		t.Fatal("label still exists")
	}
}

func TestDeleteRepoLabelFn_InUseRefused(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "inuse")
	for _, title := range []string{"a", "b", "c"} {
		idx := openIssue(t, f, title)
		if err := f.AddIssueLabels(context.Background(), testOwner, testRepo, idx, []int64{id}); err != nil {
			t.Fatal(err)
		}
	}

	msg := callErr(t, h.DeleteRepoLabelFn, map[string]any{"id": float64(id)})
	if !strings.Contains(msg, "is used by 3") {
		t.Fatalf("expected in-use error, got: %q", msg)
	}
	if _, err := f.GetRepoLabel(context.Background(), testOwner, testRepo, id); err != nil {
		t.Fatalf("the label must survive a refused delete: %v", err)
	}
}

func TestDeleteRepoLabelFn_InUseForceSuccess(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "inuse")
	idx := openIssue(t, f, "a")
	if err := f.AddIssueLabels(context.Background(), testOwner, testRepo, idx, []int64{id}); err != nil {
		t.Fatal(err)
	}

	call(t, h.DeleteRepoLabelFn, map[string]any{"id": float64(id), "delete_mode": "force"})
	if got := issueLabelNames(t, f, idx); len(got) != 0 {
		t.Fatalf("deleting a label unlabels its issues, got %v", got)
	}
}

func TestDeleteRepoLabelFn_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	msg := callErr(t, h.DeleteRepoLabelFn, map[string]any{"id": float64(99), "delete_mode": "force"})
	if !strings.Contains(msg, "label 99 not found") {
		t.Fatalf("got %q", msg)
	}
}

// ---- get_repo_label ----

func TestGetRepoLabelFn_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "priority")

	out := call(t, h.GetRepoLabelFn, map[string]any{"id": float64(id)})
	if !strings.Contains(out, `"name":"priority"`) {
		t.Fatalf("got %q", out)
	}
}

func TestGetRepoLabelFn_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	callErr(t, h.GetRepoLabelFn, map[string]any{"id": float64(404)})
}

// ---- organization labels ----

func orgReq(args map[string]any) map[string]any {
	args["org"] = "acme"
	return args
}

func TestOrgLabelTools_RoundTrip(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	out := call(t, h.CreateOrgLabelFn, orgReq(map[string]any{"name": "prio", "color": "0088ff", "description": "d"}))
	var wrapped struct{ Result forgejo_sdk.Label }
	if err := json.Unmarshal([]byte(out), &wrapped); err != nil || wrapped.Result.Name != "prio" || wrapped.Result.ID == 0 {
		t.Fatalf("create: %q err=%v", out, err)
	}
	id := float64(wrapped.Result.ID)

	out = call(t, h.GetOrgLabelFn, orgReq(map[string]any{"id": id}))
	if !strings.Contains(out, `"name":"prio"`) {
		t.Fatalf("get: %q", out)
	}

	out = call(t, h.EditOrgLabelFn, orgReq(map[string]any{"id": id, "name": "prio2"}))
	if !strings.Contains(out, `"name":"prio2"`) || !strings.Contains(out, `"description":"d"`) {
		t.Fatalf("edit should change only the name: %q", out)
	}

	callErr(t, h.EditOrgLabelFn, orgReq(map[string]any{"id": id}))
	callErr(t, h.CreateOrgLabelFn, orgReq(map[string]any{"name": "x", "color": "zzz"}))

	call(t, h.DeleteOrgLabelFn, orgReq(map[string]any{"id": id}))
	if labels, _, _ := f.ListOrgLabels(context.Background(), "acme", 1, 10); len(labels) != 0 {
		t.Fatalf("label still exists: %+v", labels)
	}
	callErr(t, h.GetOrgLabelFn, orgReq(map[string]any{"id": id}))
}

func TestDeleteOrgLabelFn_RefusedWhileAnIssueInAnOrgRepoUsesIt(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddRepo("acme", "anvil")
	label, err := f.CreateOrgLabel(context.Background(), "acme", forgejo_sdk.CreateLabelOption{Name: "prio", Color: "#aabbcc"})
	if err != nil {
		t.Fatal(err)
	}
	iss, err := f.CreateIssue(context.Background(), "acme", "anvil", forgejo_sdk.CreateIssueOption{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.AddIssueLabels(context.Background(), "acme", "anvil", iss.Index, []int64{label.ID}); err != nil {
		t.Fatal(err)
	}

	msg := callErr(t, h.DeleteOrgLabelFn, orgReq(map[string]any{"id": float64(label.ID)}))
	if !strings.Contains(msg, "is used by 1 issue(s)/PR(s) in visible repos") {
		t.Fatalf("got %q", msg)
	}

	call(t, h.DeleteOrgLabelFn, orgReq(map[string]any{"id": float64(label.ID), "delete_mode": "force"}))
}

// ---- label resources ----

func TestParseLabel_InvalidID(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/owner/repo/label/abc"}}
	if _, err := h.repoLabelResourceHandler(context.Background(), req); err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestRepoLabelResource_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	id := newLabel(t, f, "bug")

	var p labelResourcePayload
	readJSON(t, h.repoLabelResourceHandler, fmt.Sprintf("forgejo://repo/%s/%s/label/%d", testOwner, testRepo, id), &p)
	if p.ID != id || p.Name != "bug" || p.Color != "aabbcc" {
		t.Fatalf("got %+v", p)
	}
}

func TestRepoLabelResource_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: fmt.Sprintf("forgejo://repo/%s/%s/label/77", testOwner, testRepo)}}
	if _, err := h.repoLabelResourceHandler(context.Background(), req); err == nil {
		t.Fatal("expected error for a missing label")
	}
}

func TestRepoLabelsResource_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, 2)

	var p labelsListPayload
	readJSON(t, h.repoLabelsResourceHandler, fmt.Sprintf("forgejo://repo/%s/%s/labels", testOwner, testRepo), &p)
	if len(p.Labels) != 2 || p.Truncated {
		t.Fatalf("got %+v", p)
	}
}

func TestRepoLabelsResource_OverCap(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, 31) // more than EmbeddedListCap (30)

	var p labelsListPayload
	readJSON(t, h.repoLabelsResourceHandler, fmt.Sprintf("forgejo://repo/%s/%s/labels", testOwner, testRepo), &p)
	if !p.Truncated {
		t.Error("expected truncated=true for 31 labels")
	}
	if p.ListTool != ListRepoLabelsToolName {
		t.Errorf("expected list_tool=%q got %q", ListRepoLabelsToolName, p.ListTool)
	}
}

func TestRepoLabelsResource_UnknownRepo(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/nobody/nothing/labels"}}
	if _, err := h.repoLabelsResourceHandler(context.Background(), req); err == nil {
		t.Fatal("expected error for an unknown repository")
	}
}
