// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Issues = (*Forgejo)(nil)

type issueState struct {
	mu       sync.Mutex
	repos    map[string]*issueRepo // key "owner/repo"
	maxItems int                   // 0 = unknown
}

type issueRepo struct {
	owner, name string
	issues      []*issueRecord
	next        int64
	nextComment int64
	nextLabel   int64
	nextMS      int64
	labels      map[int64]*forgejo_sdk.Label
	milestones  []*forgejo_sdk.Milestone
}

type issueRecord struct {
	issue    *forgejo_sdk.Issue
	comments []*forgejo_sdk.Comment
}

// repoKnown reports whether AddRepo made owner/repo exist.
func (f *Forgejo) repoKnown(owner, repo string) bool {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	_, ok := f.pulls.repos[owner+"/"+repo]
	return ok
}

// issueRepo returns the repository's issue state, 404 if it does not exist.
// The caller holds f.issues.mu.
func (f *Forgejo) issueRepo(owner, repo string) (*issueRepo, error) {
	key := owner + "/" + repo
	if r, ok := f.issues.repos[key]; ok {
		return r, nil
	}
	if !f.repoKnown(owner, repo) {
		return nil, notFound("repository")
	}
	if f.issues.repos == nil {
		f.issues.repos = map[string]*issueRepo{}
	}
	r := &issueRepo{owner: owner, name: repo, labels: map[int64]*forgejo_sdk.Label{}}
	f.issues.repos[key] = r
	return r, nil
}

func (r *issueRepo) issue(index int64) (*issueRecord, error) {
	for _, i := range r.issues {
		if i.issue.Index == index {
			return i, nil
		}
	}
	return nil, notFound("issue")
}

func (r *issueRepo) comment(id int64) (*issueRecord, int, error) {
	for _, i := range r.issues {
		for n, c := range i.comments {
			if c.ID == id {
				return i, n, nil
			}
		}
	}
	return nil, 0, notFound("comment")
}

func copyIssue(i *forgejo_sdk.Issue) *forgejo_sdk.Issue {
	cp := *i
	cp.Labels = append([]*forgejo_sdk.Label(nil), i.Labels...)
	cp.Assignees = append([]*forgejo_sdk.User(nil), i.Assignees...)
	return &cp
}

func copyComment(c *forgejo_sdk.Comment) *forgejo_sdk.Comment {
	cp := *c
	return &cp
}

// SetMaxResponseItems sets the pagination ceiling MaxResponseItems reports;
// zero (the default) is "unknown".
func (f *Forgejo) SetMaxResponseItems(n int) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	f.issues.maxItems = n
}

// SeedIssueLabel makes a label with the given name exist in owner/repo and
// returns its ID, which ReplaceIssueLabels and CreateIssueOption.Labels take.
// The label catalogue itself is not on this port.
func (f *Forgejo) SeedIssueLabel(owner, repo, name string) int64 {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		panic(fmt.Sprintf("fake: SeedIssueLabel in unknown repository %s/%s", owner, repo))
	}
	r.nextLabel++
	r.labels[r.nextLabel] = &forgejo_sdk.Label{ID: r.nextLabel, Name: name}
	return r.nextLabel
}

// SeedMilestone makes an open milestone exist in owner/repo and returns its ID.
func (f *Forgejo) SeedMilestone(owner, repo, title string) int64 {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		panic(fmt.Sprintf("fake: SeedMilestone in unknown repository %s/%s", owner, repo))
	}
	r.nextMS++
	r.milestones = append(r.milestones, &forgejo_sdk.Milestone{ID: r.nextMS, Title: title, State: forgejo_sdk.StateOpen, Created: f.Now()})
	return r.nextMS
}

// CloseMilestone marks a seeded milestone closed.
func (f *Forgejo) CloseMilestone(owner, repo string, id int64) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return
	}
	for _, m := range r.milestones {
		if m.ID == id {
			m.State = forgejo_sdk.StateClosed
		}
	}
}

func (f *Forgejo) MaxResponseItems(context.Context) (int, bool) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	return f.issues.maxItems, f.issues.maxItems > 0
}

func (f *Forgejo) GetIssue(_ context.Context, owner, repo string, index int64) (*forgejo_sdk.Issue, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	rec, err := r.issue(index)
	if err != nil {
		return nil, err
	}
	return copyIssue(rec.issue), nil
}

func pageInfoFor(opt forgejo_sdk.ListOptions, total int) (lo, hi int, info forge.PageInfo) {
	lo, hi = page(opt, total)
	return lo, hi, forge.PageInfo{HasNext: hi < total, TotalCount: total, TotalKnown: true}
}

func matchState(want, have forgejo_sdk.StateType) bool {
	if want == "" {
		want = forgejo_sdk.StateOpen
	}
	return want == forgejo_sdk.StateAll || want == have
}

func hasLabel(i *forgejo_sdk.Issue, name string) bool {
	for _, l := range i.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

func sortIssues(out []*forgejo_sdk.Issue, by string) {
	less := func(a, b *forgejo_sdk.Issue) bool { return a.Index > b.Index } // latest
	far := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	due := func(i *forgejo_sdk.Issue) time.Time {
		if i.Deadline == nil {
			return far
		}
		return *i.Deadline
	}
	switch by {
	case "oldest":
		less = func(a, b *forgejo_sdk.Issue) bool { return a.Index < b.Index }
	case "recentupdate":
		less = func(a, b *forgejo_sdk.Issue) bool { return a.Updated.After(b.Updated) }
	case "leastupdate":
		less = func(a, b *forgejo_sdk.Issue) bool { return a.Updated.Before(b.Updated) }
	case "mostcomment":
		less = func(a, b *forgejo_sdk.Issue) bool { return a.Comments > b.Comments }
	case "leastcomment":
		less = func(a, b *forgejo_sdk.Issue) bool { return a.Comments < b.Comments }
	case "nearduedate":
		less = func(a, b *forgejo_sdk.Issue) bool { return due(a).Before(due(b)) }
	case "farduedate":
		less = func(a, b *forgejo_sdk.Issue) bool { return due(a).After(due(b)) }
	}
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
}

func (f *Forgejo) ListRepoIssues(_ context.Context, owner, repo string, opt forge.ListIssuesOptions) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	// Forgejo drops label names it does not know, so a filter naming only
	// unknown labels is no filter at all.
	known := map[string]bool{}
	for _, l := range r.labels {
		known[l.Name] = true
	}
	var out []*forgejo_sdk.Issue
	for _, rec := range r.issues {
		i := rec.issue
		if !matchState(opt.State, i.State) || opt.Type == forgejo_sdk.IssueTypePull {
			continue
		}
		skip := false
		for _, name := range opt.Labels {
			if known[name] && !hasLabel(i, name) {
				skip = true
			}
		}
		if skip || !matchMilestones(i, opt.Milestones) || !matchKeyword(rec, opt.KeyWord) {
			continue
		}
		out = append(out, copyIssue(i))
	}
	sortIssues(out, opt.Sort)
	lo, hi, info := pageInfoFor(opt.ListOptions, len(out))
	return out[lo:hi], info, nil
}

func matchMilestones(i *forgejo_sdk.Issue, want []string) bool {
	if len(want) == 0 {
		return true
	}
	if i.Milestone == nil {
		return false
	}
	for _, w := range want {
		if w == i.Milestone.Title || w == strconv.FormatInt(i.Milestone.ID, 10) {
			return true
		}
	}
	return false
}

func matchKeyword(rec *issueRecord, kw string) bool {
	if kw == "" {
		return true
	}
	kw = strings.ToLower(kw)
	if strings.Contains(strings.ToLower(rec.issue.Title), kw) || strings.Contains(strings.ToLower(rec.issue.Body), kw) {
		return true
	}
	for _, c := range rec.comments {
		if strings.Contains(strings.ToLower(c.Body), kw) {
			return true
		}
	}
	return false
}

func (f *Forgejo) SearchIssues(_ context.Context, opt forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	var out []*forgejo_sdk.Issue
	for _, r := range f.issues.repos {
		if r.owner != opt.Owner {
			continue
		}
		for _, rec := range r.issues {
			i := rec.issue
			if !matchState(opt.State, i.State) || opt.Type == forgejo_sdk.IssueTypePull {
				continue
			}
			// Search labels have OR semantics.
			if len(opt.Labels) > 0 {
				any := false
				for _, name := range opt.Labels {
					any = any || hasLabel(i, name)
				}
				if !any {
					continue
				}
			}
			if !matchMilestones(i, opt.Milestones) || !matchKeyword(rec, opt.KeyWord) {
				continue
			}
			if opt.CreatedBy != "" && (i.Poster == nil || i.Poster.UserName != opt.CreatedBy) {
				continue
			}
			if opt.AssignedBy != "" {
				found := false
				for _, a := range i.Assignees {
					found = found || a.UserName == opt.AssignedBy
				}
				if !found {
					continue
				}
			}
			out = append(out, copyIssue(i))
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Repository.FullName != out[b].Repository.FullName {
			return out[a].Repository.FullName < out[b].Repository.FullName
		}
		return out[a].Index > out[b].Index
	})
	lo, hi, info := pageInfoFor(opt.ListOptions, len(out))
	return out[lo:hi], info, nil
}

func (f *Forgejo) CreateIssue(_ context.Context, owner, repo string, opt forgejo_sdk.CreateIssueOption) (*forgejo_sdk.Issue, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opt.Title) == "" {
		return nil, fmt.Errorf("title is empty")
	}
	r.next++
	now := f.Now()
	iss := &forgejo_sdk.Issue{
		ID:         r.next,
		Index:      r.next,
		Poster:     user(Poster),
		Title:      opt.Title,
		Body:       opt.Body,
		Ref:        opt.Ref,
		State:      forgejo_sdk.StateOpen,
		Created:    now,
		Updated:    now,
		Deadline:   opt.Deadline,
		Repository: &forgejo_sdk.RepositoryMeta{Owner: owner, Name: repo, FullName: owner + "/" + repo},
	}
	for _, id := range opt.Labels {
		if l, ok := r.labels[id]; ok {
			cp := *l
			iss.Labels = append(iss.Labels, &cp)
		}
	}
	for _, a := range opt.Assignees {
		iss.Assignees = append(iss.Assignees, user(a))
	}
	iss.Milestone = r.milestone(opt.Milestone)
	if opt.Closed {
		iss.State = forgejo_sdk.StateClosed
		iss.Closed = &now
	}
	r.issues = append(r.issues, &issueRecord{issue: iss})
	return copyIssue(iss), nil
}

func (r *issueRepo) milestone(id int64) *forgejo_sdk.Milestone {
	for _, m := range r.milestones {
		if m.ID == id {
			cp := *m
			return &cp
		}
	}
	return nil
}

func (f *Forgejo) EditIssue(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.EditIssueOption) (*forgejo_sdk.Issue, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	if opt.Title != "" && strings.TrimSpace(opt.Title) == "" {
		return nil, fmt.Errorf("title is empty")
	}
	rec, err := r.issue(index)
	if err != nil {
		return nil, err
	}
	i := rec.issue
	now := f.Now()
	if opt.Title != "" {
		i.Title = opt.Title
	}
	if opt.Body != nil {
		i.Body = *opt.Body
	}
	if opt.Ref != nil {
		i.Ref = *opt.Ref
	}
	if opt.Assignees != nil {
		i.Assignees = nil
		for _, a := range opt.Assignees {
			i.Assignees = append(i.Assignees, user(a))
		}
	}
	if opt.Milestone != nil {
		i.Milestone = r.milestone(*opt.Milestone)
	}
	if opt.RemoveDeadline != nil && *opt.RemoveDeadline {
		i.Deadline = nil
	} else if opt.Deadline != nil {
		d := *opt.Deadline
		i.Deadline = &d
	}
	if opt.State != nil && *opt.State != i.State {
		i.State = *opt.State
		if i.State == forgejo_sdk.StateClosed {
			i.Closed = &now
		} else {
			i.Closed = nil
		}
	}
	i.Updated = now
	return copyIssue(i), nil
}

func (f *Forgejo) ReplaceIssueLabels(_ context.Context, owner, repo string, index int64, labelIDs []int64) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return err
	}
	rec, err := r.issue(index)
	if err != nil {
		return err
	}
	var labels []*forgejo_sdk.Label
	for _, id := range labelIDs {
		l, ok := r.labels[id]
		if !ok {
			return notFound("label")
		}
		cp := *l
		labels = append(labels, &cp)
	}
	rec.issue.Labels = labels
	rec.issue.Updated = f.Now()
	return nil
}

func (f *Forgejo) ClearIssueLabels(_ context.Context, owner, repo string, index int64) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return err
	}
	rec, err := r.issue(index)
	if err != nil {
		return err
	}
	rec.issue.Labels = nil
	rec.issue.Updated = f.Now()
	return nil
}

func (f *Forgejo) ListIssueComments(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	f.issues.mu.Lock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		f.issues.mu.Unlock()
		return nil, forge.PageInfo{}, err
	}
	rec, err := r.issue(index)
	if err != nil {
		f.issues.mu.Unlock()
		// A pull request's thread lives on the same endpoint.
		pulls, perr := f.ListPullRequestComments(ctx, owner, repo, index, forgejo_sdk.ListOptions{Page: -1})
		if perr != nil {
			return nil, forge.PageInfo{}, err
		}
		lo, hi, info := pageInfoFor(opt.ListOptions, len(pulls))
		return pulls[lo:hi], info, nil
	}
	defer f.issues.mu.Unlock()
	var out []*forgejo_sdk.Comment
	for _, c := range rec.comments {
		if !opt.Since.IsZero() && c.Created.Before(opt.Since) {
			continue
		}
		if !opt.Before.IsZero() && c.Created.After(opt.Before) {
			continue
		}
		out = append(out, copyComment(c))
	}
	lo, hi, info := pageInfoFor(opt.ListOptions, len(out))
	return out[lo:hi], info, nil
}

func (f *Forgejo) GetIssueComment(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Comment, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	rec, n, err := r.comment(id)
	if err != nil {
		return nil, err
	}
	return copyComment(rec.comments[n]), nil
}

func (f *Forgejo) CreateIssueComment(_ context.Context, owner, repo string, index int64, body string) (*forgejo_sdk.Comment, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	rec, err := r.issue(index)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, fmt.Errorf("body is empty")
	}
	r.nextComment++
	now := f.Now()
	c := &forgejo_sdk.Comment{
		ID: r.nextComment, Poster: user(Poster), Body: body, Created: now, Updated: now,
		HTMLURL: fmt.Sprintf("/%s/%s/issues/%d#issuecomment-%d", owner, repo, index, r.nextComment),
	}
	rec.comments = append(rec.comments, c)
	rec.issue.Comments = len(rec.comments)
	rec.issue.Updated = now
	return copyComment(c), nil
}

func (f *Forgejo) EditIssueComment(_ context.Context, owner, repo string, id int64, body string) (*forgejo_sdk.Comment, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	rec, n, err := r.comment(id)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, fmt.Errorf("body is empty")
	}
	c := rec.comments[n]
	c.Body = body
	c.Updated = f.Now()
	return copyComment(c), nil
}

func (f *Forgejo) DeleteIssueComment(_ context.Context, owner, repo string, id int64) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return err
	}
	rec, n, err := r.comment(id)
	if err != nil {
		return err
	}
	rec.comments = append(rec.comments[:n], rec.comments[n+1:]...)
	rec.issue.Comments = len(rec.comments)
	return nil
}

func (f *Forgejo) ListRepoMilestones(_ context.Context, owner, repo string, opt forgejo_sdk.ListMilestoneOption) ([]*forgejo_sdk.Milestone, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	var out []*forgejo_sdk.Milestone
	for _, m := range r.milestones {
		if !matchState(opt.State, m.State) {
			continue
		}
		cp := *m
		out = append(out, &cp)
	}
	lo, hi := page(opt.ListOptions, len(out))
	return out[lo:hi], nil
}
