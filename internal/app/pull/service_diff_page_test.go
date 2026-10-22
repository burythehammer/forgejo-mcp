// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"strings"
	"testing"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/pull"
)

const threeFiles = "diff --git a/a.go b/a.go\n+a\ndiff --git a/b.go b/b.go\n+b\ndiff --git a/c.go b/c.go\n+c\n"

func TestDiffPagesWholeFilesWithNoGapOrOverlap(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "paged-diff")
	if err := f.SetPullRequestDiff(owner, repo, i, threeFiles); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, err := s.Diff(ctx, apppull.DiffInput{PR: pr(i), Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasNext || first.Files != 3 || first.Count != 2 || strings.Contains(first.Text, "c.go") {
		t.Fatalf("page 1: %+v", first)
	}
	second, err := s.Diff(ctx, apppull.DiffInput{PR: pr(i), Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.HasNext || second.Count != 1 || strings.Contains(second.Text, "a.go") {
		t.Fatalf("page 2: %+v", second)
	}
	if first.Text+"\n"+second.Text != threeFiles {
		t.Fatalf("pages do not rejoin to the diff:\n%q\n%q", first.Text, second.Text)
	}
}

func TestDiffPageBeyondTheEndIsEmpty(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "paged-diff-end")
	if err := f.SetPullRequestDiff(owner, repo, i, threeFiles); err != nil {
		t.Fatal(err)
	}
	got, err := s.Diff(context.Background(), apppull.DiffInput{PR: pr(i), Page: 9, Limit: 2})
	if err != nil || got.Text != "" || got.Count != 0 || got.HasNext || got.Files != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestDiffDefaultPageIsTheWholeSmallDiff(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "small-diff")
	if err := f.SetPullRequestDiff(owner, repo, i, threeFiles); err != nil {
		t.Fatal(err)
	}
	got, err := s.Diff(context.Background(), apppull.DiffInput{PR: pr(i)})
	if err != nil || got.Text != threeFiles || got.HasNext {
		t.Fatalf("got %+v, %v", got, err)
	}
}
