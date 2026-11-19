// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"fmt"
	"testing"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/pull"
)

func TestCreateResolvesLabelNamesAndIDsBeforeOpening(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	f.SeedIssueLabel(owner, repo, "bug")
	docs := f.SeedIssueLabel(owner, repo, "docs")
	labels := fmt.Sprintf("bug,%d", docs)
	assignees := "octo, hubot"
	pr, err := s.Create(context.Background(), apppull.CreateRequest{
		Owner: owner, Repo: repo, Head: "feature", Base: "main", Title: "t", Body: "b",
		Labels: &labels, Assignees: &assignees,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Labels) != 2 || pr.Labels[0].Name != "bug" || pr.Labels[1].Name != "docs" {
		t.Fatalf("labels %+v, want bug and docs", pr.Labels)
	}
	if len(pr.Assignees) != 2 || pr.Assignees[0].UserName != "octo" || pr.Assignees[1].UserName != "hubot" {
		t.Fatalf("assignees %+v, want octo and hubot", pr.Assignees)
	}
}

func TestCreateWithAnUnknownLabelOpensNothing(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	f.SeedIssueLabel(owner, repo, "bug")
	labels := "bug,nope"
	_, err := s.Create(context.Background(), apppull.CreateRequest{
		Owner: owner, Repo: repo, Head: "feature", Base: "main", Title: "t", Body: "b", Labels: &labels,
	})
	if err == nil {
		t.Fatal("create with an unknown label succeeded")
	}
	page, lerr := s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, State: "all"})
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(page.Items) != 0 {
		t.Fatalf("a failed create left %d pull request(s)", len(page.Items))
	}
}
