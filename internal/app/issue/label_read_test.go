// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

// labelOrg owns the organization labels these tests seed. It is not the
// repository owner, so its labels reach a read only through IncludeOrg.
const labelOrg = "acme"

// labelsFail is a port whose label-page listings fail. A read that lists labels
// must report that failure, not an empty catalogue.
type labelsFail struct{ apppissue.Port }

func (labelsFail) ListRepoLabelsPage(context.Context, string, string, apppissue.Paging) (apppissue.LabelPage, error) {
	return apppissue.LabelPage{}, errors.New("labels boom")
}

func (labelsFail) ListOrgLabelsPage(context.Context, string, apppissue.Paging) (apppissue.LabelPage, error) {
	return apppissue.LabelPage{}, errors.New("labels boom")
}

// seedRepoLabelNames creates repository labels prefix1..prefixN in owner/repo
// through the label use case.
func seedRepoLabelNames(t *testing.T, s *apppissue.Service, owner, repo, prefix string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		in := apppissue.LabelNew{Name: fmt.Sprintf("%s%d", prefix, i), Color: "#aabbcc"}
		if _, err := s.CreateRepoLabel(context.Background(), owner, repo, in); err != nil {
			t.Fatalf("seed repo label: %v", err)
		}
	}
}

// seedOrgLabelNames creates organization labels prefix1..prefixN in labelOrg
// through the label use case.
func seedOrgLabelNames(t *testing.T, s *apppissue.Service, prefix string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		in := apppissue.LabelNew{Name: fmt.Sprintf("%s%d", prefix, i), Color: "#aabbcc"}
		if _, err := s.CreateOrgLabel(context.Background(), labelOrg, in); err != nil {
			t.Fatalf("seed org label: %v", err)
		}
	}
}

func labelNamesOf(ls []model.ScopedLabel) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Name)
	}
	return out
}

func TestReadRepoLabels_RepoOnlyUnlessIncludeOrg(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	seedRepoLabelNames(t, s, owner, repo, "repo-", 2)
	seedOrgLabelNames(t, s, "org-", 1)

	repoOnly, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: owner, Repo: repo, Paging: apppissue.Paging{Page: 1, Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"repo-1", "repo-2"}; fmt.Sprint(labelNamesOf(repoOnly.Items)) != fmt.Sprint(want) {
		t.Fatalf("repo only: got %v, want %v", labelNamesOf(repoOnly.Items), want)
	}
	for _, l := range repoOnly.Items {
		if l.Scope != "repo" {
			t.Fatalf("repo-only read carries scope %q", l.Scope)
		}
	}
}

func TestReadRepoLabels_IncludeOrgAppendsOrgLabelsWithScope(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	ctx := context.Background()
	f.AddRepo(labelOrg, repo)
	seedRepoLabelNames(t, s, labelOrg, repo, "repo-", 1)
	seedOrgLabelNames(t, s, "org-", 1)

	got, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: labelOrg, Repo: repo, IncludeOrg: true, Paging: apppissue.Paging{Page: 1, Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"repo-1", "org-1"}; fmt.Sprint(labelNamesOf(got.Items)) != fmt.Sprint(want) {
		t.Fatalf("merged names: got %v, want %v", labelNamesOf(got.Items), want)
	}
	if got.Items[0].Scope != "repo" || got.Items[1].Scope != "org" {
		t.Fatalf("scopes: %q %q", got.Items[0].Scope, got.Items[1].Scope)
	}
}

func TestReadRepoLabels_ReportsMoreRowsOfTheRepository(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	seedRepoLabelNames(t, s, owner, repo, "r", 5)

	first, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: owner, Repo: repo, Paging: apppissue.Paging{Page: 1, Limit: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || !first.HasNext {
		t.Fatalf("page 1: %d labels, more=%v; want 2, true", len(first.Items), first.HasNext)
	}
	if first.Total == nil || *first.Total != 5 {
		t.Fatalf("page 1 total: %v; want known 5", first.Total)
	}

	last, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: owner, Repo: repo, Paging: apppissue.Paging{Page: 3, Limit: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.HasNext {
		t.Fatalf("page 3: %d labels, more=%v; want 1, false", len(last.Items), last.HasNext)
	}
}

func TestReadOrgLabels_PagesAndReportsMore(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	seedOrgLabelNames(t, s, "o", 3)

	page, err := s.ReadOrgLabels(ctx, labelOrg, apppissue.Paging{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasNext {
		t.Fatalf("org page 1: %d labels, more=%v; want 2, true", len(page.Items), page.HasNext)
	}
	for _, l := range page.Items {
		if l.Scope != "org" {
			t.Fatalf("org read carries scope %q", l.Scope)
		}
	}
}

func TestReadLabels_PortErrorsAreReported(t *testing.T) {
	t.Parallel()
	_, f := fixture(t)
	s := apppissue.New(labelsFail{issueadapter.New(f)})
	ctx := context.Background()

	if _, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: owner, Repo: repo, Paging: apppissue.Paging{Page: 1, Limit: 10},
	}); err == nil {
		t.Fatal("repo labels: want the port failure, got nil")
	}
	if _, err := s.ReadOrgLabels(ctx, labelOrg, apppissue.Paging{Page: 1, Limit: 10}); err == nil {
		t.Fatal("org labels: want the port failure, got nil")
	}
}
