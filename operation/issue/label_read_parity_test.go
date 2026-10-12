// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// labelFacts is what a caller learns about one label. The tool and the
// resource encode it differently, but the facts must agree.
type labelFacts = labelResourcePayload

// labelListFacts reads a tool's label list as the facts a caller learns: the
// scope field is a tool encoding and is dropped here.
func labelListFacts(t *testing.T, text string) []labelFacts {
	t.Helper()
	var listed []labelFacts
	if err := unmarshalResult(text, &listed); err != nil {
		t.Fatal(err)
	}
	return listed
}

func parityLabelURI(id int64) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/label/%d", testOwner, testRepo, id)
}

func parityRepoLabelsURI(page, limit int) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/labels?page=%d&limit=%d", testOwner, testRepo, page, limit)
}

func parityOrgLabelsURI(org string, page, limit int) string {
	return fmt.Sprintf("forgejo://org/%s/labels?page=%d&limit=%d", org, page, limit)
}

// TestLabelReadParity_RepoLabel: get_repo_label and forgejo://repo/{o}/{r}/label/{id}
// return the same facts for the same label.
func TestLabelReadParity_RepoLabel(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	ctx := context.Background()
	created, err := f.CreateRepoLabel(ctx, testOwner, testRepo, forgejo_sdk.CreateLabelOption{
		Name: "bug", Color: "#ff0000", Description: "broken",
	})
	if err != nil {
		t.Fatal(err)
	}

	var fromTool labelFacts
	if err := unmarshalResult(call(t, h.GetRepoLabelFn, map[string]any{
		"owner": testOwner, "repo": testRepo, "id": float64(created.ID),
	}), &fromTool); err != nil {
		t.Fatal(err)
	}
	var fromResource labelResourcePayload
	readJSON(t, h.repoLabelResourceHandler, parityLabelURI(created.ID), &fromResource)

	if !reflect.DeepEqual(fromResource, fromTool) {
		t.Fatalf("label facts differ\nresource: %+v\ntool:     %+v", fromResource, fromTool)
	}
}

// TestLabelReadParity_RepoLabelsList: list_repo_labels with include_org_labels
// false and the repo-labels resource return the same labels in the same order,
// page by page.
func TestLabelReadParity_RepoLabelsList(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, 7)
	seedOrgLabels(t, f, testOwner, 2) // not in either read: the repo list leaves org labels out

	for _, page := range []int{1, 2} {
		fromTool := labelListFacts(t, call(t, h.ListRepoLabelsFn, map[string]any{
			"owner": testOwner, "repo": testRepo, "page": float64(page), "limit": float64(3), "include_org_labels": false,
		}))
		var fromResource labelsListPayload
		readJSON(t, h.repoLabelsResourceHandler, parityRepoLabelsURI(page, 3), &fromResource)
		if !reflect.DeepEqual(fromResource.Labels, fromTool) {
			t.Fatalf("page %d differs\nresource: %+v\ntool:     %+v", page, fromResource.Labels, fromTool)
		}
	}
}

// TestLabelReadParity_OrgLabelsList: list_org_labels and the org-labels resource
// return the same labels in the same order.
func TestLabelReadParity_OrgLabelsList(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedOrgLabels(t, f, "o", 4)

	fromTool := labelListFacts(t, call(t, h.ListOrgLabelsFn, map[string]any{
		"org": "o", "page": float64(1), "limit": float64(10),
	}))
	var fromResource orgLabelsListPayload
	readJSON(t, h.orgLabelsResourceHandler, parityOrgLabelsURI("o", 1, 10), &fromResource)
	if !reflect.DeepEqual(fromResource.Labels, fromTool) {
		t.Fatalf("org labels differ\nresource: %+v\ntool:     %+v", fromResource.Labels, fromTool)
	}
}

// TestLabelReadParity_ErrorsMapTheSameCode: a missing label fails both reads with
// the same resource error code, since both paths use the resource-side mapping.
func TestLabelReadParity_ErrorsMapTheSameCode(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	ctx := context.Background()

	_, toolErr := h.GetRepoLabelFn(ctx, repoReq(map[string]any{"id": float64(99)}))
	_, resErr := h.repoLabelResourceHandler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: parityLabelURI(99)}})

	var tr, rr *resource.ResourceError
	if !errors.As(toolErr, &tr) || !errors.As(resErr, &rr) {
		t.Fatalf("want *resource.ResourceError from both paths, got tool=%v resource=%v", toolErr, resErr)
	}
	if tr.Code != rr.Code {
		t.Fatalf("codes: tool=%d resource=%d", tr.Code, rr.Code)
	}
}

// TestLabelResourceGolden_RepoLabelsAreRepoOnly pins the intentional difference
// the tool does not share: the repo-labels resource leaves the organization's
// labels out, where list_repo_labels merges them by default.
func TestLabelResourceGolden_RepoLabelsAreRepoOnly(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, 2)
	seedOrgLabels(t, f, testOwner, 2)

	var b strings.Builder
	for _, c := range readURI(t, h.repoLabelsResourceHandler, parityRepoLabelsURI(1, 30)) {
		b.WriteString(c.(mcp.TextResourceContents).Text)
		b.WriteString("\n---\n")
	}
	mcptest.Golden(t, "resource_repo_labels_repo_only", []byte(b.String()))
}

// TestLabelResourceGolden_RepoLabelsTruncate pins the resource's ceiling: a page
// of 30 over 31 repository labels carries the truncation marker, which names
// list_repo_labels as the way to fetch the rest.
func TestLabelResourceGolden_RepoLabelsTruncate(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, 31)

	var b strings.Builder
	for _, c := range readURI(t, h.repoLabelsResourceHandler, parityRepoLabelsURI(1, 30)) {
		b.WriteString(c.(mcp.TextResourceContents).Text)
		b.WriteString("\n---\n")
	}
	mcptest.Golden(t, "resource_repo_labels_truncated", []byte(b.String()))
}
