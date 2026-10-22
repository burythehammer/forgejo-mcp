// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

// countFault is a port whose label-usage count fails for one repository.
type countFault struct {
	apppissue.Port
	repo string
	err  error
}

func (c countFault) CountIssuesLabelled(ctx context.Context, o, r, label string) (*int, error) {
	if r == c.repo {
		return nil, c.err
	}
	return c.Port.CountIssuesLabelled(ctx, o, r, label)
}

func orgLabelFixture(t *testing.T, fault error) (*apppissue.Service, int64) {
	t.Helper()
	f := fake.New()
	f.AddRepo("acme", "a")
	f.AddRepo("acme", "b")
	port := issueadapter.New(f)
	if fault != nil {
		port = countFault{Port: port, repo: "b", err: fault}
	}
	s := apppissue.New(port)
	l, err := s.CreateOrgLabel(context.Background(), "acme", apppissue.LabelNew{Name: "infra", Color: "#0000ff"})
	if err != nil {
		t.Fatal(err)
	}
	return s, l.ID
}

func TestDeleteOrgLabel_FailedCountRefusesAndNamesTheRepo(t *testing.T) {
	t.Parallel()
	s, id := orgLabelFixture(t, fmt.Errorf("boom"))
	err := s.DeleteOrgLabel(context.Background(), "acme", id, false)
	if err == nil || !strings.Contains(err.Error(), "acme/b") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want it to name acme/b and boom", errText(err))
	}
	if _, gerr := s.GetOrgLabel(context.Background(), "acme", id); gerr != nil {
		t.Fatalf("label was deleted: %v", gerr)
	}
}

func TestDeleteOrgLabel_InaccessibleRepoIsSkipped(t *testing.T) {
	t.Parallel()
	s, id := orgLabelFixture(t, &apppissue.InaccessibleError{Err: fmt.Errorf("404")})
	if err := s.DeleteOrgLabel(context.Background(), "acme", id, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeleteOrgLabel_UnusedLabelDeletes(t *testing.T) {
	t.Parallel()
	s, id := orgLabelFixture(t, nil)
	if err := s.DeleteOrgLabel(context.Background(), "acme", id, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// noTotal is a port whose label-usage count reports no total.
type noTotal struct{ apppissue.Port }

func (noTotal) CountIssuesLabelled(context.Context, string, string, string) (*int, error) {
	return nil, nil
}

func TestDeleteOrgLabel_UnreportedTotalRefuses(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", "a")
	s := apppissue.New(noTotal{issueadapter.New(f)})
	l, err := s.CreateOrgLabel(context.Background(), "acme", apppissue.LabelNew{Name: "infra", Color: "#0000ff"})
	if err != nil {
		t.Fatal(err)
	}
	err = s.DeleteOrgLabel(context.Background(), "acme", l.ID, false)
	if err == nil || !strings.Contains(err.Error(), "acme/a") {
		t.Fatalf("error = %q, want a refusal naming acme/a", errText(err))
	}
}

func TestDeleteOrgLabel_UsedInSecondRepoIsRefused(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", "a")
	f.AddRepo("acme", "b")
	s := apppissue.New(issueadapter.New(f))
	ctx := context.Background()
	l, err := s.CreateOrgLabel(ctx, "acme", apppissue.LabelNew{Name: "infra", Color: "#0000ff"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, apppissue.CreateInput{Owner: "acme", Repo: "b", Title: "t", Labels: str("infra")}); err != nil {
		t.Fatal(err)
	}
	err = s.DeleteOrgLabel(ctx, "acme", l.ID, false)
	if err == nil || !strings.Contains(err.Error(), "used by 1 issue(s)/PR(s)") {
		t.Fatalf("error = %q, want a usage refusal", errText(err))
	}
}
