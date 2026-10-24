// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"errors"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// Sub-resources of a missing issue are the shared not-found outcome. The same
// lists under an issue that exists with nothing in them are empty.

func TestComments_MissingIssueIsNotFound(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	_, err := s.Comments(context.Background(), apppissue.CommentsInput{Target: target(99), Paging: apppissue.Paging{Page: 1, Limit: 10}})
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}

func TestComments_EmptyIssueIsEmptyList(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "quiet", Body: "b"})
	out, err := s.Comments(context.Background(), apppissue.CommentsInput{Target: target(iss.Index), Paging: apppissue.Paging{Page: 1, Limit: 10}})
	if err != nil {
		t.Fatalf("got %v, want an empty list", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("got %d comments, want none", len(out.Items))
	}
}

func TestDependencies_MissingIssueIsNotFound(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	_, err := s.Dependencies(context.Background(), target(99), apppissue.Paging{Page: 1, Limit: 10})
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}

func TestDependencies_EmptyIssueIsEmptyList(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "lonely", Body: "b"})
	out, err := s.Dependencies(context.Background(), target(iss.Index), apppissue.Paging{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("got %v, want an empty list", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("got %d dependencies, want none", len(out.Items))
	}
}
