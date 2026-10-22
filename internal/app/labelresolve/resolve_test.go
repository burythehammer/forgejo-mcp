// SPDX-License-Identifier: GPL-3.0-or-later

package labelresolve_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/labelresolve"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

// These tests run the resolver over the in-memory fake through the port
// bridge. They assert on the IDs a caller gets back and on the error text, not
// on how the catalogue was paged.

const (
	owner = "octo"
	repo  = "hello"
)

func resolver(t *testing.T) (*labelresolve.Resolver, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return labelresolve.New(catalogOf{port: issueadapter.New(f)}), f
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestResolve_NamesAndIDsResolve(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	bug := f.SeedIssueLabel(owner, repo, "bug")
	ui := f.SeedIssueLabel(owner, repo, "ui")

	ids, err := r.Resolve(context.Background(), owner, repo, fmt.Sprintf("bug,%d", ui))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 2 || ids[0] != bug || ids[1] != ui {
		t.Fatalf("ids = %v, want [%d %d]", ids, bug, ui)
	}
}

func TestResolve_DuplicatesCollapse(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	bug := f.SeedIssueLabel(owner, repo, "bug")

	ids, err := r.Resolve(context.Background(), owner, repo, "bug, bug ,bug")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != bug {
		t.Fatalf("ids = %v, want one [%d]", ids, bug)
	}
}

func TestResolve_NameBeatsID(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	f.SeedIssueLabel(owner, repo, "x")
	f.SeedIssueLabel(owner, repo, "y") // ID 2 belongs to "y"
	named := f.SeedIssueLabel(owner, repo, "2")

	ids, err := r.Resolve(context.Background(), owner, repo, "2")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != named {
		t.Fatalf("ids = %v, want the label named 2 (id %d)", ids, named)
	}
}

func TestResolve_RejectsWithTheSharedErrors(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	f.SeedIssueLabel(owner, repo, "bug")

	cases := []struct{ csv, want string }{
		{"bug,nope", `unknown label "nope"`},
		{"", "labels names nothing"},
		{" , ", "labels names nothing"},
		{"99", `label "99" is neither a label name nor the ID`},
	}
	for _, c := range cases {
		_, err := r.Resolve(context.Background(), owner, repo, c.csv)
		if !strings.Contains(errText(err), c.want) {
			t.Errorf("csv %q: error %q, want it to contain %q", c.csv, errText(err), c.want)
		}
	}
}

func TestResolve_AmbiguousAcrossScopesNamesBoth(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", repo)
	port := issueadapter.New(f)
	r := labelresolve.New(catalogOf{port: port})
	ctx := context.Background()
	f.SeedIssueLabel("acme", repo, "ui")
	if _, err := port.CreateOrgLabel(ctx, "acme", apppissue.LabelNew{Name: "ui", Color: "#00aaaa"}); err != nil {
		t.Fatal(err)
	}

	_, err := r.Resolve(ctx, "acme", repo, "ui")
	want := `label "ui" is ambiguous: it matches organization label`
	if !strings.Contains(errText(err), want) || !strings.Contains(errText(err), "repository label") {
		t.Fatalf("error = %q, want an ambiguity naming both scopes (%s...)", errText(err), want)
	}
}

// The catalogue is paged 100 at a time. A label on a later page must still
// resolve, or it would be reported as unknown.
func TestResolve_LabelOnALaterPage(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	for i := 0; i < 150; i++ {
		f.SeedIssueLabel(owner, repo, "filler-"+strconv.Itoa(i))
	}
	late := f.SeedIssueLabel(owner, repo, "late")

	ids, err := r.Resolve(context.Background(), owner, repo, "late")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != late {
		t.Fatalf("ids = %v, want [%d]", ids, late)
	}
}

// A server may clamp a page below the size the catalogue asks for (Forgejo's
// max_response_items). A short page is then not the last one, so a label past
// the ceiling must still resolve: the catalogue pages until a page comes back
// empty.
func TestResolve_LabelBeyondACeilingBelowThePageSize(t *testing.T) {
	t.Parallel()
	r, f := resolver(t)
	f.SetMaxResponseItems(10)
	for i := 0; i < 25; i++ {
		f.SeedIssueLabel(owner, repo, "filler-"+strconv.Itoa(i))
	}
	late := f.SeedIssueLabel(owner, repo, "late")

	ids, err := r.Resolve(context.Background(), owner, repo, "late")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != late {
		t.Fatalf("ids = %v, want [%d]", ids, late)
	}
}

// catalogOf presents the issue port's label reads in the resolver's paging
// type, as the owning use case does in production.
type catalogOf struct{ port apppissue.Port }

func (c catalogOf) ListRepoLabels(ctx context.Context, owner, repo string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListRepoLabels(ctx, owner, repo, apppissue.Paging{Page: p.Page, Limit: p.Limit})
}

func (c catalogOf) MaxResponseItems(ctx context.Context) (int, bool) {
	return c.port.MaxResponseItems(ctx)
}

func (c catalogOf) ListOrgLabels(ctx context.Context, org string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListOrgLabels(ctx, org, apppissue.Paging{Page: p.Page, Limit: p.Limit})
}
