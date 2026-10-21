// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Dependencies = (*Forgejo)(nil)

// Dependency edges are kept in issueState (they join issues across
// repositories, under the issue lock): from -> the issues it depends on.
type depKey struct {
	owner, repo string
	index       int64
}

func keyOf(owner, repo string, index int64) depKey { return depKey{owner, repo, index} }

// edges returns the issues related to k, in the direction selected: the issues
// k depends on, or the issues that depend on k. The caller holds f.issues.mu.
func (f *Forgejo) relatedIssues(k depKey, dependents bool) []*forgejo_sdk.Issue {
	var out []*forgejo_sdk.Issue
	for _, e := range f.issues.deps {
		from, to := e[0], e[1]
		if dependents {
			from, to = to, from
		}
		if from != k {
			continue
		}
		r, ok := f.issues.repos[to.owner+"/"+to.repo]
		if !ok {
			continue
		}
		if rec, err := r.issue(to.index); err == nil {
			out = append(out, copyIssue(rec.issue))
		}
	}
	return out
}

func (f *Forgejo) listRelated(owner, repo string, index int64, pg, limit int, dependents bool) ([]*forgejo_sdk.Issue, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	// Unknown issues list as empty, as Forgejo's 404 does through the adapter.
	all := f.relatedIssues(keyOf(owner, repo, index), dependents)
	lo, hi := page(forgejo_sdk.ListOptions{Page: pg, PageSize: limit}, len(all))
	out := all[lo:hi]
	if out == nil {
		out = []*forgejo_sdk.Issue{}
	}
	return out, nil
}

func (f *Forgejo) ListIssueDependencies(_ context.Context, owner, repo string, index int64, pg, limit int) ([]*forgejo_sdk.Issue, error) {
	return f.listRelated(owner, repo, index, pg, limit, false)
}

func (f *Forgejo) ListIssueDependents(_ context.Context, owner, repo string, index int64, pg, limit int) ([]*forgejo_sdk.Issue, error) {
	return f.listRelated(owner, repo, index, pg, limit, true)
}

func (f *Forgejo) AddIssueDependency(_ context.Context, owner, repo string, index int64, dep forge.IssueRef) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	if err := f.requireIssue(owner, repo, index); err != nil {
		return err
	}
	if err := f.requireIssue(dep.Owner, dep.Repo, dep.Index); err != nil {
		return err
	}
	from, to := keyOf(owner, repo, index), keyOf(dep.Owner, dep.Repo, dep.Index)
	if from == to {
		return statusErr(http.StatusBadRequest, "an issue cannot depend on itself")
	}
	for _, e := range f.issues.deps {
		if e == [2]depKey{from, to} {
			return statusErr(http.StatusConflict, "dependency already exists")
		}
	}
	f.issues.deps = append(f.issues.deps, [2]depKey{from, to})
	return nil
}

func (f *Forgejo) RemoveIssueDependency(_ context.Context, owner, repo string, index int64, dep forge.IssueRef) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	if err := f.requireIssue(owner, repo, index); err != nil {
		return err
	}
	want := [2]depKey{keyOf(owner, repo, index), keyOf(dep.Owner, dep.Repo, dep.Index)}
	for i, e := range f.issues.deps {
		if e == want {
			f.issues.deps = append(f.issues.deps[:i], f.issues.deps[i+1:]...)
			return nil
		}
	}
	return notFound("dependency")
}

// requireIssue is 404 unless owner/repo#index exists. The caller holds f.issues.mu.
func (f *Forgejo) requireIssue(owner, repo string, index int64) error {
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return err
	}
	_, err = r.issue(index)
	return err
}
