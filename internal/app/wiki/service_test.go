// SPDX-License-Identifier: GPL-3.0-or-later

package wiki_test

import (
	"context"
	"strings"
	"testing"

	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/wikiadapter"
)

// The use cases run against the in-memory fake through the wiki adapter: no
// MCP, no HTTP. The fake is built per test, so tests run in parallel.

const (
	owner = "octo"
	repo  = "hello"
)

func newService(t *testing.T) *appwiki.Service {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return appwiki.New(wikiadapter.New(f))
}

func create(t *testing.T, s *appwiki.Service, title, content string) {
	t.Helper()
	if _, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: title, Content: content}); err != nil {
		t.Fatalf("creating %s: %v", title, err)
	}
}

func TestListPagesAndFollowsTheProbe(t *testing.T) {
	t.Parallel()
	s := newService(t)
	for _, title := range []string{"Home", "Guides", "FAQ"} {
		create(t, s, title, "x")
	}
	first, err := s.List(context.Background(), appwiki.ListRequest{Owner: owner, Repo: repo, Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || !first.HasNext || first.Page != 1 {
		t.Fatalf("page 1: got %+v", first)
	}
	last, err := s.List(context.Background(), appwiki.ListRequest{Owner: owner, Repo: repo, Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.HasNext {
		t.Fatalf("page 2: got %+v", last)
	}
}

func TestListDefaultsToPageOne(t *testing.T) {
	t.Parallel()
	got, err := newService(t).List(context.Background(), appwiki.ListRequest{Owner: owner, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 1 || got.HasNext || len(got.Items) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestGetRoundTripsContentAndCutsLines(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Notes", "a\nb\nc")
	start, end := 2, 3
	got, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "Notes", StartLine: &start, EndLine: &end})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "b\nc" || got.TotalLines != 3 || got.StartLine != 2 || got.EndLine != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestGetCountsLinesTheSameForCRLFAndLF(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Crlf", "a\r\nb\r\n")
	create(t, s, "Lf", "a\nb\n")
	for _, name := range []string{"Crlf", "Lf"} {
		got, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: name})
		if err != nil {
			t.Fatal(err)
		}
		if got.TotalLines != 3 {
			t.Fatalf("%s: total lines %d, want 3", name, got.TotalLines)
		}
	}
}

func TestGetInvertedRangeIsReportedRaw(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Notes", "a\nb\nc")
	start, end := 3, 1
	_, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "Notes", StartLine: &start, EndLine: &end})
	if err == nil || !strings.HasPrefix(err.Error(), "start_line (3) is after end_line (1)") {
		t.Fatalf("got %v", err)
	}
}

func TestGetUnknownIsWrapped(t *testing.T) {
	t.Parallel()
	_, err := newService(t).Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "Nope"})
	if err == nil || !strings.HasPrefix(err.Error(), "get wiki page: ") {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateKeepsTheTitleWhenNoneIsGiven(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Old Title", "body")
	got, err := s.Update(context.Background(), appwiki.UpdateRequest{Owner: owner, Repo: repo, PageName: "Old Title", Content: "new body"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Old Title" {
		t.Fatalf("title changed to %q", got.Title)
	}
}

func TestUpdateUnknownPageIsRefusedWithOrWithoutATitle(t *testing.T) {
	t.Parallel()
	for _, title := range []string{"", "Nope"} {
		s := newService(t)
		_, err := s.Update(context.Background(), appwiki.UpdateRequest{Owner: owner, Repo: repo, PageName: "Nope", Title: title, Content: "x"})
		if err == nil || !strings.Contains(err.Error(), "use create_wiki_page") || !forge.IsNotFound(err) {
			t.Fatalf("title %q: got %v", title, err)
		}
		// The refusal must leave nothing behind: Forgejo's own edit would have
		// created the page.
		if _, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "Nope"}); !forge.IsNotFound(err) {
			t.Fatalf("title %q: page was created: %v", title, err)
		}
	}
}

func TestDeleteRemovesThePage(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Gone", "x")
	got, err := s.Delete(context.Background(), appwiki.PageRef{Owner: owner, Repo: repo, PageName: "Gone"})
	if err != nil || !got.Deleted || got.PageName != "Gone" {
		t.Fatalf("delete: got %+v, %v", got, err)
	}
	if _, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "Gone"}); err == nil {
		t.Fatal("page still readable after delete")
	}
}

func TestRevisionsReportTheTotalAndPaging(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Notes", "v1")
	if _, err := s.Update(context.Background(), appwiki.UpdateRequest{Owner: owner, Repo: repo, PageName: "Notes", Content: "v2"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Revisions(context.Background(), appwiki.RevisionsRequest{Owner: owner, Repo: repo, PageName: "Notes", Page: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || !got.HasNext || got.Total == nil || *got.Total != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestListEnvelopeOnAnExactMultiple(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Home", "x")
	create(t, s, "Guides", "x")
	got, err := s.List(context.Background(), appwiki.ListRequest{Owner: owner, Repo: repo, Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 1 || got.Limit != 2 || got.Count != 2 || got.HasNext {
		t.Fatalf("exact multiple, last page: got %+v", got)
	}
}

func TestCreateRefusesAnExistingHyphenatedTitle(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "delivery-workflow", "one")
	_, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: "delivery-workflow", Content: "two"})
	if err == nil {
		t.Fatal("a second create of a hyphenated title must be refused")
	}
	// Forgejo stores the page as "delivery-workflow.-", so the guard's read of
	// the plain title may not see it; the refusal then comes from Forgejo. Either
	// way the first content must survive.
	got, gerr := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: "delivery-workflow.-"})
	if gerr != nil || got.Content != "one" {
		t.Fatalf("content %+v, err %v", got, gerr)
	}
}
