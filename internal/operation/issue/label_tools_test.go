// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Tests of the label-assignment and label-listing tools.

func newLabel(t *testing.T, f *fake.Forgejo, name string) int64 {
	t.Helper()
	l, err := f.CreateRepoLabel(context.Background(), testOwner, testRepo, forgejo_sdk.CreateLabelOption{Name: name, Color: "#aabbcc"})
	if err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func TestListOrgLabels_Success(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedOrgLabels(t, f, "codeberg", 2)

	out := call(t, h.ListOrgLabelsFn, map[string]any{"org": "codeberg"})
	if !strings.Contains(out, `"scope":"org"`) || !strings.Contains(out, `"label-1"`) || !strings.Contains(out, `"label-2"`) {
		t.Fatalf("expected both labels with scope=org, got %q", out)
	}
}

// A page below 1 reads as page 1: the rendered window names page 1, not the
// page the caller sent.
func TestListOrgLabels_NegativePageReadsAsPageOne(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedOrgLabels(t, f, "codeberg", 2)

	out := call(t, h.ListOrgLabelsFn, map[string]any{"org": "codeberg", "page": float64(-1)})
	if !strings.Contains(out, `"page":1,`) {
		t.Fatalf("page -1 must render as page 1, got %q", out)
	}
}

// A missing org is a not-found, not an empty list: an empty answer would read as
// "no labels" for a mistyped org name.
func TestListOrgLabels_UnknownOrgIsNotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	out := callErr(t, h.ListOrgLabelsFn, map[string]any{"org": "ghost"})
	if !strings.Contains(out, "not found") {
		t.Fatalf("expected a not-found error, got %q", out)
	}
}

func TestListOrgLabels_UnauthorizedSurfaces(t *testing.T) {
	t.Parallel()
	f := fake.New()
	h := withPort(statusPort{Forgejo: f, orgLabelsStatus: http.StatusUnauthorized})
	callErr(t, h.ListOrgLabelsFn, map[string]any{"org": "codeberg"})
}

func TestListRepoLabels_MergeWithOrgLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "good-first-issue")
	if _, err := f.CreateOrgLabel(context.Background(), testOwner, forgejo_sdk.CreateLabelOption{Name: "security", Color: "#d73a4a"}); err != nil {
		t.Fatal(err)
	}

	out := call(t, h.ListRepoLabelsFn, nil)
	if !strings.Contains(out, `"scope":"repo"`) || !strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected both scopes in result, got %q", out)
	}
	if !strings.Contains(out, `"good-first-issue"`) || !strings.Contains(out, `"security"`) {
		t.Fatalf("expected both label names in merged result, got %q", out)
	}
}

func TestListRepoLabels_IncludeOrgFalseSkipsOrgLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "good-first-issue")
	if _, err := f.CreateOrgLabel(context.Background(), testOwner, forgejo_sdk.CreateLabelOption{Name: "security", Color: "#d73a4a"}); err != nil {
		t.Fatal(err)
	}

	out := call(t, h.ListRepoLabelsFn, map[string]any{"include_org_labels": false})
	if strings.Contains(out, `"scope":"org"`) || strings.Contains(out, `"security"`) {
		t.Fatalf("expected no org-scoped labels in result, got %q", out)
	}
	if !strings.Contains(out, `"good-first-issue"`) {
		t.Fatalf("expected the repo label, got %q", out)
	}
}

func TestListRepoLabels_UserOwnerHasNoOrgLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "bug")

	out := call(t, h.ListRepoLabelsFn, nil)
	if !strings.Contains(out, `"scope":"repo"`) || strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected only repo-scoped labels, got %q", out)
	}
}

func TestAddIssueLabelsFn_ByName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	newLabel(t, f, "bug")
	newLabel(t, f, "triage")

	out := call(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "bug,triage"})
	// The result is the refreshed issue, labels included.
	if !strings.Contains(out, `"bug"`) || !strings.Contains(out, `"triage"`) {
		t.Fatalf("expected the refreshed issue with both labels, got %q", out)
	}
	if got := issueLabelNames(t, f, idx); strings.Join(got, ",") != "bug,triage" {
		t.Fatalf("labels on the issue: %v", got)
	}
}

func TestAddIssueLabelsFn_KeepsExistingLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	newLabel(t, f, "bug")
	newLabel(t, f, "triage")

	call(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "bug"})
	call(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "triage"})
	if got := issueLabelNames(t, f, idx); strings.Join(got, ",") != "bug,triage" {
		t.Fatalf("labels on the issue: %v", got)
	}
}

func TestAddIssueLabelsFn_ByID(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	bug := newLabel(t, f, "bug")
	triage := newLabel(t, f, "triage")

	call(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": itoa(bug) + "," + itoa(triage)})
	if got := issueLabelNames(t, f, idx); strings.Join(got, ",") != "bug,triage" {
		t.Fatalf("numeric IDs should keep working, got %v", got)
	}
}

func TestAddIssueLabelsFn_UnknownLabelWritesNothing(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	newLabel(t, f, "bug")

	callErr(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "bug,not-a-label"})
	if got := issueLabelNames(t, f, idx); len(got) != 0 {
		t.Fatalf("expected no write when a label cannot be resolved, got %v", got)
	}
}

func TestAddIssueLabelsFn_EmptyLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	callErr(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": ""})
}

func TestRemoveIssueLabelsFn_ByName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	newLabel(t, f, "bug")
	newLabel(t, f, "triage")
	call(t, h.AddIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "bug,triage"})

	out := call(t, h.RemoveIssueLabelsFn, map[string]any{"index": float64(idx), "labels": "bug"})
	if strings.Contains(out, `"name":"bug"`) {
		t.Fatalf("expected the refreshed issue without the label, got %q", out)
	}
	if got := issueLabelNames(t, f, idx); strings.Join(got, ",") != "triage" {
		t.Fatalf("labels on the issue: %v", got)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
