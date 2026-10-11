// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func resolve(t *testing.T, h handlers, csv string) ([]int64, error) {
	t.Helper()
	return h.resolveIssueLabelIDs(context.Background(), testOwner, testRepo, csv)
}

func TestResolveIssueLabelIDs_RepoName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	bug, triage := newLabel(t, f, "bug"), newLabel(t, f, "triage")

	ids, err := resolve(t, h, "bug, triage")
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, bug, triage)
}

func TestResolveIssueLabelIDs_OrgName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "bug")
	dogfood, err := f.CreateOrgLabel(context.Background(), testOwner, forgejo_sdk.CreateLabelOption{Name: "dogfood", Color: "#aabbcc"})
	if err != nil {
		t.Fatal(err)
	}

	ids, err := resolve(t, h, "dogfood")
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, dogfood.ID)
}

func TestResolveIssueLabelIDs_NumericTokenStillWorks(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	bug, triage := newLabel(t, f, "bug"), newLabel(t, f, "triage")

	ids, err := resolve(t, h, itoa(bug)+","+itoa(triage))
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, bug, triage)
}

// A label literally named "2" must win over the label whose ID is 2, otherwise
// it is unreachable through an argument documented to take names.
func TestResolveIssueLabelIDs_NameBeatsID(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	named := newLabel(t, f, "2") // the first label created has ID 1
	other := newLabel(t, f, "other")
	if other != 2 || named == 2 {
		t.Fatalf("fixture assumption broken: named=%d other=%d", named, other)
	}

	ids, err := resolve(t, h, "2")
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, named)
}

// A full first page plus one more row: the label under test only appears once
// enumeration asks for page two.
func TestResolveIssueLabelIDs_SecondPage(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedRepoLabels(t, f, labelCatalogPageSize)
	late := newLabel(t, f, "late")

	ids, err := resolve(t, h, "late")
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, late)
}

func TestResolveIssueLabelIDs_DuplicatesCollapse(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	bug := newLabel(t, f, "bug")

	ids, err := resolve(t, h, "bug,bug,"+itoa(bug))
	if err != nil {
		t.Fatal(err)
	}
	wantIndices(t, ids, bug)
}

func TestResolveIssueLabelIDs_UnknownName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "bug")

	_, err := resolve(t, h, "bug,frction")
	if err == nil || !strings.Contains(err.Error(), "frction") {
		t.Fatalf("error should name the unresolved token, got: %v", err)
	}
}

// A numeric token is honoured only when the catalogue has that ID, so a stale ID
// fails here rather than 404-ing mid-write.
func TestResolveIssueLabelIDs_UnknownNumericID(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newLabel(t, f, "bug")

	_, err := resolve(t, h, "4242")
	if err == nil || !strings.Contains(err.Error(), "4242") {
		t.Fatalf("error should name the unresolved token, got: %v", err)
	}
}

func TestResolveIssueLabelIDs_AmbiguousAcrossScopes(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	repoBug := newLabel(t, f, "bug")
	orgBug, err := f.CreateOrgLabel(context.Background(), testOwner, forgejo_sdk.CreateLabelOption{Name: "bug", Color: "#aabbcc"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = resolve(t, h, "bug")
	if err == nil {
		t.Fatal("expected an error when a name matches in two scopes")
	}
	want := []string{fmt.Sprintf("repository label %d", repoBug), fmt.Sprintf("organization label %d", orgBug.ID)}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("error should report both scopes and IDs (%q), got: %v", w, err)
		}
	}
}

func TestResolveIssueLabelIDs_EmptyIsAnError(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	reads := 0
	h := withPort(statusPort{Forgejo: f, repoLabelReads: &reads})

	if _, err := resolve(t, h, "  ,  "); err == nil {
		t.Fatal("expected an error when the argument names no label")
	}
	if reads != 0 {
		t.Fatalf("expected no catalogue read before rejecting an empty argument, got %d", reads)
	}
}

// A 403 on the org endpoint would narrow the catalogue silently, turning a real
// org label into "unknown label". It has to fail loudly instead.
func TestResolveIssueLabelIDs_OrgLabelsUnauthorized(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	newLabel(t, f, "bug")
	h := withPort(statusPort{Forgejo: f, orgLabelsStatus: http.StatusForbidden})

	if _, err := resolve(t, h, "bug"); err == nil {
		t.Fatal("expected an error when organization labels cannot be read")
	}
}
