// SPDX-License-Identifier: GPL-3.0-or-later

package repoadminadapter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/repoadminadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/sdk"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const (
	owner = "octo"
	repo  = "hello"
	sha   = "0123456789abcdef0123456789abcdef01234567"
)

// The one smoke test that goes over HTTP for this domain: the use cases, the
// bridge, the real SDK adapter and a stub Forgejo, end to end.
func TestSmokeAdminOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo(owner, repo)
	port := repoadminadapter.New(sdk.NewAdapter(stub.NewClient(t)))
	s := apprepo.NewAdmin(port)
	ctx := context.Background()

	fresh, err := s.CreateRepo(ctx, apprepo.CreateRepoInput{Name: ptr("fresh"), Description: "d"})
	if err != nil || fresh.Name != "fresh" || fresh.Description != "d" {
		t.Fatalf("create: %+v, %v", fresh, err)
	}
	if _, err := s.SetRepoTopics(ctx, owner, repo, ptr("go, ci")); err != nil {
		t.Fatalf("set topics: %v", err)
	}
	v, err := s.GetRepo(ctx, owner, repo)
	if err != nil || len(v.Topics) != 2 {
		t.Fatalf("get: %+v, %v", v, err)
	}
	if _, err := s.EditRepo(ctx, apprepo.EditRepoInput{Owner: owner, Repo: repo, Edit: apprepo.RepoEdit{Archived: ptrBool(true)}}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	page, err := s.ListRepoTopics(ctx, owner, repo, apprepo.AdminPaging{Page: 1, Limit: 100})
	if err != nil || page.Count != 2 {
		t.Fatalf("list topics: %+v, %v", page, err)
	}
}

func TestSmokeCommitStatusesOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo(owner, repo)
	if err := stub.Fake.AddCommit(owner, repo, sha, "msg"); err != nil {
		t.Fatal(err)
	}
	if err := stub.Fake.AddCommitStatus(owner, repo, sha, "ci/build", forgejo_sdk.StatusSuccess, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := stub.Fake.AddCommitStatus(owner, repo, sha, "ci/lint", forgejo_sdk.StatusFailure, "no"); err != nil {
		t.Fatal(err)
	}
	port := repoadminadapter.New(sdk.NewAdapter(stub.NewClient(t)))

	page, err := port.ListCommitStatuses(context.Background(), owner, repo, sha, apprepo.AdminPaging{Page: 1, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items: %+v", page.Items)
	}
	states := map[string]string{}
	for _, it := range page.Items {
		states[it.Context] = it.State
		if _, err := time.Parse(time.RFC3339, it.CreatedAt); err != nil {
			t.Fatalf("created_at %q is not RFC 3339: %v", it.CreatedAt, err)
		}
	}
	if states["ci/build"] != "success" || states["ci/lint"] != "failure" {
		t.Fatalf("states: %v", states)
	}
}

func TestSmokeUnknownTopicListIsNotFound(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	port := repoadminadapter.New(sdk.NewAdapter(stub.NewClient(t)))
	_, err := port.ListRepoTopics(context.Background(), "nobody", "nothing", apprepo.AdminPaging{Page: 1, Limit: 25})
	var nf *apprepo.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("got %v, want a NotFoundError", err)
	}
}

func ptr(s string) *string { return &s }
func ptrBool(b bool) *bool { return &b }
