// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"fmt"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
)

// envelope is the continuation signal every paged issue-domain list returns.
type envelope struct {
	Page, Limit, Count int
	HasNext            bool
}

// envelopeFamily is one paged list: seed fills the fake with n rows, and read
// returns the envelope of the page asked for.
type envelopeFamily struct {
	name string
	seed func(t *testing.T, s *apppissue.Service, f fixtureFake, n int)
	read func(t *testing.T, s *apppissue.Service, page, limit int) envelope
}

// fixtureFake is the part of the fake these seeders use.
type fixtureFake interface {
	SeedMilestone(owner, repo, title string) int64
}

// envelopeCases are the page shapes every family must report exactly: a full
// page with rows after it, a middle page, a short last page, an exact-multiple
// last page, and a page past the end.
var envelopeCases = []struct {
	name       string
	n          int
	page, size int
	want       envelope
}{
	{"full page with more", 5, 1, 2, envelope{Page: 1, Limit: 2, Count: 2, HasNext: true}},
	{"middle full page", 5, 2, 2, envelope{Page: 2, Limit: 2, Count: 2, HasNext: true}},
	{"short last page", 5, 3, 2, envelope{Page: 3, Limit: 2, Count: 1, HasNext: false}},
	{"exact multiple last page", 4, 2, 2, envelope{Page: 2, Limit: 2, Count: 2, HasNext: false}},
	{"past the end", 4, 9, 2, envelope{Page: 9, Limit: 2, Count: 0, HasNext: false}},
}

// seedIssues creates issues 1..n in owner/repo and returns nothing: index 1 is
// the subject of the list, the rest are the rows it refers to.
func seedIssues(t *testing.T, s *apppissue.Service, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: fmt.Sprintf("issue %d", i)})
	}
}

var envelopeFamilies = []envelopeFamily{
	{
		name: "comments",
		seed: func(t *testing.T, s *apppissue.Service, _ fixtureFake, n int) {
			seedIssues(t, s, 1)
			for i := 1; i <= n; i++ {
				if _, err := s.CreateComment(context.Background(), target(1), fmt.Sprintf("c%d", i)); err != nil {
					t.Fatalf("seed comment: %v", err)
				}
			}
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.Comments(context.Background(), apppissue.CommentsInput{
				Target: target(1), Paging: apppissue.Paging{Page: page, Limit: limit},
			})
			if err != nil {
				t.Fatal(err)
			}
			return envelope{Page: got.Page, Limit: got.Limit, Count: got.Count, HasNext: got.HasNext}
		},
	},
	{
		name: "dependencies",
		seed: func(t *testing.T, s *apppissue.Service, _ fixtureFake, n int) {
			seedIssues(t, s, n+1)
			for i := 2; i <= n+1; i++ {
				if err := s.AddDependency(context.Background(), target(1), target(int64(i))); err != nil {
					t.Fatalf("seed dependency: %v", err)
				}
			}
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.Dependencies(context.Background(), target(1), apppissue.Paging{Page: page, Limit: limit})
			if err != nil {
				t.Fatal(err)
			}
			return envelope{Page: got.Page, Limit: got.Limit, Count: got.Count, HasNext: got.HasNext}
		},
	},
	{
		name: "dependents",
		seed: func(t *testing.T, s *apppissue.Service, _ fixtureFake, n int) {
			seedIssues(t, s, n+1)
			for i := 2; i <= n+1; i++ {
				if err := s.AddDependency(context.Background(), target(int64(i)), target(1)); err != nil {
					t.Fatalf("seed dependent: %v", err)
				}
			}
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.Dependents(context.Background(), target(1), apppissue.Paging{Page: page, Limit: limit})
			if err != nil {
				t.Fatal(err)
			}
			return envelope{Page: got.Page, Limit: got.Limit, Count: got.Count, HasNext: got.HasNext}
		},
	},
	{
		name: "milestones",
		seed: func(t *testing.T, _ *apppissue.Service, f fixtureFake, n int) {
			for i := 1; i <= n; i++ {
				f.SeedMilestone(owner, repo, fmt.Sprintf("m%d", i))
			}
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.Milestones(context.Background(), owner, repo, apppissue.MilestoneList{
				Paging: apppissue.Paging{Page: page, Limit: limit},
			})
			if err != nil {
				t.Fatal(err)
			}
			return envelope{Page: got.Page, Limit: got.Limit, Count: got.Count, HasNext: got.HasNext}
		},
	},
	{
		name: "repo labels",
		seed: func(t *testing.T, s *apppissue.Service, _ fixtureFake, n int) {
			seedRepoLabelNames(t, s, owner, repo, "r", n)
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.ReadRepoLabels(context.Background(), apppissue.RepoLabelsInput{
				Owner: owner, Repo: repo, Paging: apppissue.Paging{Page: page, Limit: limit},
			})
			if err != nil {
				t.Fatal(err)
			}
			env := got
			return envelope{Page: env.Page, Limit: env.Limit, Count: env.Count, HasNext: env.HasNext}
		},
	},
	{
		name: "org labels",
		seed: func(t *testing.T, s *apppissue.Service, _ fixtureFake, n int) {
			seedOrgLabelNames(t, s, "o", n)
		},
		read: func(t *testing.T, s *apppissue.Service, page, limit int) envelope {
			got, err := s.ReadOrgLabels(context.Background(), labelOrg, apppissue.Paging{Page: page, Limit: limit})
			if err != nil {
				t.Fatal(err)
			}
			env := got
			return envelope{Page: env.Page, Limit: env.Limit, Count: env.Count, HasNext: env.HasNext}
		},
	},
}

// TestPagedIssueLists_Envelope pins the page, limit, count and has_next of
// every paged issue-domain list on the shapes that matter: has_next is true on
// a full page with rows after it, false on a short page, false on an
// exact-multiple last page, and false past the end.
func TestPagedIssueLists_Envelope(t *testing.T) {
	t.Parallel()
	for _, fam := range envelopeFamilies {
		for _, c := range envelopeCases {
			t.Run(fam.name+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				s, f := fixture(t)
				fam.seed(t, s, f, c.n)
				if got := fam.read(t, s, c.page, c.size); got != c.want {
					t.Fatalf("envelope = %+v, want %+v", got, c.want)
				}
			})
		}
	}
}
