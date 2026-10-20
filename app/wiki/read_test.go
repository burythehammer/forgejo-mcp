// SPDX-License-Identifier: GPL-3.0-or-later

package wiki_test

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/wikiadapter"
)

// editPage rewrites a page so that its history grows by one revision.
func editPage(t *testing.T, s *appwiki.Service, name, content string) {
	t.Helper()
	if _, err := s.Update(context.Background(), appwiki.UpdateRequest{Owner: owner, Repo: repo, PageName: name, Content: content}); err != nil {
		t.Fatalf("editing %s: %v", name, err)
	}
}

func TestReadPage_ReturnsTitleNameCommitAndContent(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Start", "hello\nworld")

	got, err := s.ReadPage(context.Background(), owner, repo, "Start", 30)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Start" || got.PageName != "Start" || got.CommitSHA == "" || got.Content != "hello\nworld" {
		t.Errorf("got title=%q page=%q sha=%q content=%q", got.Title, got.PageName, got.CommitSHA, got.Content)
	}
	if len(got.Revisions) != 1 || got.RevisionsTruncated {
		t.Errorf("revisions=%d truncated=%v, want 1 and false", len(got.Revisions), got.RevisionsTruncated)
	}
}

func TestReadPage_CapsRevisionsAndFlagsMore(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Start", "v1")
	editPage(t, s, "Start", "v2")
	editPage(t, s, "Start", "v3")
	editPage(t, s, "Start", "v4")

	got, err := s.ReadPage(context.Background(), owner, repo, "Start", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Revisions) != 2 || !got.RevisionsTruncated || got.RevisionCount != 4 {
		t.Errorf("revisions=%d truncated=%v count=%d, want 2, true, 4", len(got.Revisions), got.RevisionsTruncated, got.RevisionCount)
	}
}

func TestReadPage_ExactlyTheCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Start", "v1")
	editPage(t, s, "Start", "v2")

	got, err := s.ReadPage(context.Background(), owner, repo, "Start", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Revisions) != 2 || got.RevisionsTruncated {
		t.Errorf("revisions=%d truncated=%v, want 2 and false", len(got.Revisions), got.RevisionsTruncated)
	}
}

func TestReadPage_UnknownPageIsWrapped(t *testing.T) {
	t.Parallel()
	s := newService(t)
	if _, err := s.ReadPage(context.Background(), owner, repo, "Nope", 30); err == nil {
		t.Fatal("want an error for a missing page")
	}
}

// failingRevisions is a port whose revision listing fails.
type failingRevisions struct{ appwiki.Port }

func (failingRevisions) GetWikiPageRevisions(context.Context, string, string, string, int, int) (*model.WikiRevisions, error) {
	return nil, errors.New("500 internal error")
}

// A failed revision listing degrades the read to no history: the page is still
// returned, and nothing claims the history was truncated.
func TestReadPage_RevisionFailureDegradesToNoHistory(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(owner, repo)
	create(t, appwiki.New(wikiadapter.New(f)), "Start", "v1")
	s := appwiki.New(failingRevisions{wikiadapter.New(f)})

	got, err := s.ReadPage(context.Background(), owner, repo, "Start", 30)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "v1" || len(got.Revisions) != 0 || got.RevisionsTruncated {
		t.Errorf("content=%q revisions=%d truncated=%v, want the page with no history", got.Content, len(got.Revisions), got.RevisionsTruncated)
	}
}
