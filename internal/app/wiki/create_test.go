// SPDX-License-Identifier: GPL-3.0-or-later

package wiki_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/wikiadapter"
)

// newFake is a fake Forgejo with the test repository and no wiki pages.
func newFake() *fake.Forgejo {
	f := fake.New()
	f.AddRepo(owner, repo)
	return f
}

// countingPort wraps a port: GetWikiPage can be made to fail, and CreateWikiPage
// calls are counted so a test can see whether a write happened.
type countingPort struct {
	appwiki.Port
	getErr  error
	creates *int
}

func (p countingPort) GetWikiPage(ctx context.Context, o, r, name string) (*model.WikiPage, error) {
	if p.getErr != nil {
		return nil, p.getErr
	}
	return p.Port.GetWikiPage(ctx, o, r, name)
}

func (p countingPort) CreateWikiPage(ctx context.Context, o, r, title, b64, msg string) (*model.WikiPage, error) {
	*p.creates++
	return p.Port.CreateWikiPage(ctx, o, r, title, b64, msg)
}

func readContent(t *testing.T, s *appwiki.Service, name string) string {
	t.Helper()
	got, err := s.Get(context.Background(), appwiki.GetRequest{Owner: owner, Repo: repo, PageName: name})
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return got.Content
}

func TestCreateNewTitleSucceeds(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Fresh", "body")
	if got := readContent(t, s, "Fresh"); got != "body" {
		t.Fatalf("content %q, want body", got)
	}
}

func TestCreateFirstPageInRepoWithoutWikiSucceeds(t *testing.T) {
	t.Parallel()
	s := newService(t) // a repository with no wiki pages yet
	got, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: "Home", Content: "first"})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if got.PageName != "Home" {
		t.Fatalf("page name %q", got.PageName)
	}
}

func TestCreateRefusesExistingTitleAndKeepsContent(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Home", "one")
	_, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: "Home", Content: "two"})
	if err == nil {
		t.Fatal("create of an existing title succeeded")
	}
	if !strings.Contains(err.Error(), "update_wiki_page") || !strings.Contains(err.Error(), "Home") {
		t.Fatalf("error %q should name the page and update_wiki_page", err)
	}
	if got := readContent(t, s, "Home"); got != "one" {
		t.Fatalf("content %q after refused create, want the original", got)
	}
}

func TestCreateRefusesNormalizedVariants(t *testing.T) {
	t.Parallel()
	cases := []struct{ existing, again string }{
		{"Guides/Setup", "Guides%2FSetup"},
		{"Guides/Setup", "Guides/Setup"},
		{"My Page", "My-Page"},
		{"My Page", "My%20Page"},
	}
	for _, c := range cases {
		s := newService(t)
		create(t, s, c.existing, "one")
		_, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: c.again, Content: "two"})
		if err == nil {
			t.Fatalf("create %q over existing %q succeeded", c.again, c.existing)
		}
		if got := readContent(t, s, c.existing); got != "one" {
			t.Fatalf("%q overwritten by %q: content %q", c.existing, c.again, got)
		}
	}
}

func TestCreateFailingCheckIsReportedNotReadAsExists(t *testing.T) {
	t.Parallel()
	creates := 0
	boom := &forge.StatusError{StatusCode: http.StatusInternalServerError, Err: errors.New("boom")}
	s := appwiki.New(countingPort{Port: wikiadapter.New(newFake()), getErr: boom, creates: &creates})
	_, err := s.Create(context.Background(), appwiki.CreateRequest{Owner: owner, Repo: repo, Title: "Home", Content: "x"})
	if err == nil {
		t.Fatal("a failed existence check was treated as a create")
	}
	if strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a failed check was reported as an existing page: %v", err)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("error %v does not wrap the check failure", err)
	}
	if creates != 0 {
		t.Fatalf("CreateWikiPage called %d times after a failed check", creates)
	}
}

func TestUpdateStillReplacesAnExistingPage(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Home", "one")
	if _, err := s.Update(context.Background(), appwiki.UpdateRequest{Owner: owner, Repo: repo, PageName: "Home", Content: "two"}); err != nil {
		t.Fatal(err)
	}
	if got := readContent(t, s, "Home"); got != "two" {
		t.Fatalf("content %q after update, want two", got)
	}
}
