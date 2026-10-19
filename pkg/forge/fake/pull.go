// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Pulls = (*Forgejo)(nil)

// Poster is the user the fake attributes everything to.
const Poster = "fake-user"

type pullState struct {
	mu    sync.Mutex
	repos map[string]*repoState // key "owner/repo"
}

type repoState struct {
	pulls []*pullRecord
	next  int64
}

type pullRecord struct {
	pr       *forgejo_sdk.PullRequest
	diff     string
	files    []*forgejo_sdk.ChangedFile
	comments []*forgejo_sdk.Comment
	reviews  []*reviewRecord
	nextID   int64
	// decline, when set, is the answer every merge of this pull request gets
	// in place of the fake's own (see SetMergeDecline).
	decline *mergeDecline
}

// mergeDecline is a declined merge's answer: the status and the message Forgejo
// gives with it. An empty message stands for a body that is not JSON.
type mergeDecline struct {
	statusCode int
	message    string
}

type reviewRecord struct {
	review   *forgejo_sdk.PullReview
	comments []*forgejo_sdk.PullReviewComment
}

func (s *pullState) repo(owner, repo string) (*repoState, error) {
	r, ok := s.repos[owner+"/"+repo]
	if !ok {
		return nil, notFound("repository")
	}
	return r, nil
}

func (s *pullState) pull(owner, repo string, index int64) (*pullRecord, error) {
	r, err := s.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	for _, p := range r.pulls {
		if p.pr.Index == index {
			return p, nil
		}
	}
	return nil, notFound("pull request")
}

func (p *pullRecord) review(id int64) (*reviewRecord, error) {
	for _, r := range p.reviews {
		if r.review.ID == id {
			return r, nil
		}
	}
	return nil, notFound("pull review")
}

func user(name string) *forgejo_sdk.User { return &forgejo_sdk.User{UserName: name} }

// AddRepo makes owner/repo exist. Everything else under an unknown repository
// is 404, as on Forgejo.
func (f *Forgejo) AddRepo(owner, repo string) {
	f.files.addRepo(owner, repo)
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	if f.pulls.repos == nil {
		f.pulls.repos = map[string]*repoState{}
	}
	if _, ok := f.pulls.repos[owner+"/"+repo]; !ok {
		f.pulls.repos[owner+"/"+repo] = &repoState{}
	}
}

// SetPullRequestDiff sets what GetPullRequestDiff returns for a pull request.
// The port cannot write a diff, as Forgejo derives it from git.
func (f *Forgejo) SetPullRequestDiff(owner, repo string, index int64, diff string) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	p.diff = diff
	return nil
}

// SetMergeDecline makes every merge of a pull request declined with statusCode
// and message, as a Forgejo that refuses the merge for a reason of its own
// would. An empty message stands for an answer whose body is not JSON.
func (f *Forgejo) SetMergeDecline(owner, repo string, index int64, statusCode int, message string) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	p.decline = &mergeDecline{statusCode: statusCode, message: message}
	return nil
}

// SetPullRequestFiles sets the changed files of a pull request.
func (f *Forgejo) SetPullRequestFiles(owner, repo string, index int64, files ...*forgejo_sdk.ChangedFile) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	p.files = files
	return nil
}

// AddPullRequestComment appends a comment to a pull request's discussion
// thread, which the port reads but cannot write.
func (f *Forgejo) AddPullRequestComment(owner, repo string, index int64, author, body string) (*forgejo_sdk.Comment, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	p.nextID++
	now := f.Now()
	c := &forgejo_sdk.Comment{ID: p.nextID, Poster: user(author), Body: body, Created: now, Updated: now}
	p.comments = append(p.comments, c)
	p.pr.Comments = len(p.comments)
	cp := *c
	return &cp, nil
}

func (f *Forgejo) GetPullRequest(_ context.Context, owner, repo string, index int64) (*forgejo_sdk.PullRequest, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	cp := *p.pr
	return &cp, nil
}

func (f *Forgejo) ListPullRequests(_ context.Context, owner, repo string, opt forgejo_sdk.ListPullRequestsOptions) ([]*forgejo_sdk.PullRequest, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	r, err := f.pulls.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	state := opt.State
	if state == "" {
		state = forgejo_sdk.StateOpen
	}
	var out []*forgejo_sdk.PullRequest
	for _, p := range r.pulls {
		if state != forgejo_sdk.StateAll && p.pr.State != state {
			continue
		}
		if opt.Milestone > 0 && (p.pr.Milestone == nil || p.pr.Milestone.ID != opt.Milestone) {
			continue
		}
		cp := *p.pr
		out = append(out, &cp)
	}
	// Newest first, as Forgejo's default; "oldest" reverses it. The other sort
	// keys are not modelled.
	sort.SliceStable(out, func(i, j int) bool {
		if opt.Sort == "oldest" {
			return out[i].Index < out[j].Index
		}
		return out[i].Index > out[j].Index
	})
	lo, hi := page(opt.ListOptions, len(out))
	return out[lo:hi], nil
}

func (f *Forgejo) CreatePullRequest(_ context.Context, owner, repo string, opt forgejo_sdk.CreatePullRequestOption) (*forgejo_sdk.PullRequest, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	r, err := f.pulls.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	if opt.Head == "" || opt.Base == "" || strings.TrimSpace(opt.Title) == "" {
		return nil, statusErr(http.StatusUnprocessableEntity, "head, base and title are required")
	}
	r.next++
	now := f.Now()
	pr := &forgejo_sdk.PullRequest{
		ID:        r.next,
		Index:     r.next,
		Poster:    user(Poster),
		Title:     opt.Title,
		Body:      opt.Body,
		State:     forgejo_sdk.StateOpen,
		Mergeable: true,
		Head:      &forgejo_sdk.PRBranchInfo{Name: opt.Head, Ref: opt.Head, Sha: "head-sha"},
		Base:      &forgejo_sdk.PRBranchInfo{Name: opt.Base, Ref: opt.Base, Sha: "base-sha"},
		Created:   &now,
		Updated:   &now,
	}
	if opt.Milestone > 0 {
		pr.Milestone = &forgejo_sdk.Milestone{ID: opt.Milestone}
	}
	r.pulls = append(r.pulls, &pullRecord{pr: pr})
	cp := *pr
	return &cp, nil
}

func (f *Forgejo) EditPullRequest(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.EditPullRequestOption) (*forgejo_sdk.PullRequest, error) {
	if opt.Title != "" && strings.TrimSpace(opt.Title) == "" {
		return nil, errors.New("title is empty") // the SDK rejects this before any request
	}
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	if opt.Title != "" {
		p.pr.Title = opt.Title
	}
	if opt.Body != nil {
		p.pr.Body = *opt.Body
	}
	if opt.Base != "" {
		p.pr.Base = &forgejo_sdk.PRBranchInfo{Name: opt.Base, Ref: opt.Base, Sha: p.pr.Base.Sha}
	}
	if opt.Assignee != "" {
		p.pr.Assignee = user(opt.Assignee)
		p.pr.Assignees = []*forgejo_sdk.User{p.pr.Assignee}
	}
	if opt.Milestone > 0 {
		p.pr.Milestone = &forgejo_sdk.Milestone{ID: opt.Milestone}
	}
	if opt.State != nil {
		p.pr.State = *opt.State
	}
	now := f.Now()
	p.pr.Updated = &now
	cp := *p.pr
	return &cp, nil
}

func (f *Forgejo) MergePullRequest(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.MergePullRequestOption) (forge.MergeResult, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		// Forgejo answers a merge of an unknown pull request with 404, which
		// the SDK reports as a status, not an error.
		return forge.MergeResult{StatusCode: http.StatusNotFound, Message: "pull request does not exist"}, nil
	}
	if p.decline != nil {
		return forge.MergeResult{StatusCode: p.decline.statusCode, Message: p.decline.message}, nil
	}
	if p.pr.HasMerged {
		return forge.MergeResult{StatusCode: http.StatusMethodNotAllowed, Message: "Pull request has already been merged"}, nil
	}
	if p.pr.State != forgejo_sdk.StateOpen {
		return forge.MergeResult{StatusCode: http.StatusMethodNotAllowed, Message: "Pull request is closed"}, nil
	}
	if !p.pr.Mergeable {
		return forge.MergeResult{StatusCode: http.StatusMethodNotAllowed, Message: "Pull request is not mergeable"}, nil
	}
	if opt.MergeWhenChecksSucceed {
		return forge.MergeResult{StatusCode: http.StatusCreated}, nil
	}
	now := f.Now()
	p.pr.HasMerged = true
	p.pr.Merged = &now
	p.pr.Closed = &now
	p.pr.Updated = &now
	p.pr.State = forgejo_sdk.StateClosed
	p.pr.MergedBy = user(Poster)
	return forge.MergeResult{Merged: true, StatusCode: http.StatusOK}, nil
}

func (f *Forgejo) GetPullRequestDiff(_ context.Context, owner, repo string, index int64) ([]byte, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	return []byte(p.diff), nil
}

func (f *Forgejo) ListPullRequestFiles(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullRequestFilesOptions) ([]*forgejo_sdk.ChangedFile, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(p.files))
	return append([]*forgejo_sdk.ChangedFile(nil), p.files[lo:hi]...), nil
}

func (f *Forgejo) ListPullRequestComments(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Comment, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt, len(p.comments))
	return append([]*forgejo_sdk.Comment(nil), p.comments[lo:hi]...), nil
}

func (f *Forgejo) ListPullReviews(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullReviewsOptions) ([]*forgejo_sdk.PullReview, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(p.reviews))
	out := make([]*forgejo_sdk.PullReview, 0, hi-lo)
	for _, r := range p.reviews[lo:hi] {
		cp := *r.review
		out = append(out, &cp)
	}
	return out, nil
}

func (f *Forgejo) GetPullReview(_ context.Context, owner, repo string, index, id int64) (*forgejo_sdk.PullReview, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	r, err := p.review(id)
	if err != nil {
		return nil, err
	}
	cp := *r.review
	return &cp, nil
}

func (f *Forgejo) ListPullReviewComments(_ context.Context, owner, repo string, index, id int64) ([]*forgejo_sdk.PullReviewComment, error) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	r, err := p.review(id)
	if err != nil {
		return nil, err
	}
	return append([]*forgejo_sdk.PullReviewComment(nil), r.comments...), nil
}

func (f *Forgejo) CreatePullReview(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.CreatePullReviewOptions) (*forgejo_sdk.PullReview, error) {
	if err := opt.Validate(); err != nil { // the SDK validates before any request
		return nil, err
	}
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	p.nextID++
	now := f.Now()
	rec := &reviewRecord{review: &forgejo_sdk.PullReview{
		ID:                p.nextID,
		Reviewer:          user(Poster),
		State:             opt.State,
		Body:              opt.Body,
		CommitID:          opt.CommitID,
		CodeCommentsCount: len(opt.Comments),
	}}
	if opt.State != forgejo_sdk.ReviewStatePending {
		rec.review.Submitted = now
	}
	for _, c := range opt.Comments {
		p.nextID++
		rec.comments = append(rec.comments, &forgejo_sdk.PullReviewComment{
			ID: p.nextID, Body: c.Body, Path: c.Path, Reviewer: user(Poster),
			ReviewID: rec.review.ID, Created: now, Updated: now,
			LineNum: uint64(c.NewLineNum), OldLineNum: uint64(c.OldLineNum),
		})
	}
	p.reviews = append(p.reviews, rec)
	cp := *rec.review
	return &cp, nil
}

func (f *Forgejo) SubmitPullReview(_ context.Context, owner, repo string, index, id int64, opt forgejo_sdk.SubmitPullReviewOptions) (*forgejo_sdk.PullReview, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return nil, err
	}
	r, err := p.review(id)
	if err != nil {
		return nil, err
	}
	if r.review.State != forgejo_sdk.ReviewStatePending {
		return nil, statusErr(http.StatusUnprocessableEntity, "review is not pending")
	}
	r.review.State = opt.State
	r.review.Body = opt.Body
	r.review.Submitted = f.Now()
	cp := *r.review
	return &cp, nil
}

func (f *Forgejo) DismissPullReview(_ context.Context, owner, repo string, index, id int64, _ forgejo_sdk.DismissPullReviewOptions) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	r, err := p.review(id)
	if err != nil {
		return err
	}
	r.review.Dismissed = true
	return nil
}

func (f *Forgejo) DeletePullReview(_ context.Context, owner, repo string, index, id int64) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	for i, r := range p.reviews {
		if r.review.ID == id {
			p.reviews = append(p.reviews[:i], p.reviews[i+1:]...)
			return nil
		}
	}
	return notFound("pull review")
}

func (f *Forgejo) CreateReviewRequests(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	for _, name := range opt.Reviewers {
		p.nextID++
		p.reviews = append(p.reviews, &reviewRecord{review: &forgejo_sdk.PullReview{
			ID: p.nextID, Reviewer: user(name), State: forgejo_sdk.ReviewStateRequestReview,
		}})
	}
	for _, name := range opt.TeamReviewers {
		p.nextID++
		p.reviews = append(p.reviews, &reviewRecord{review: &forgejo_sdk.PullReview{
			ID: p.nextID, ReviewerTeam: &forgejo_sdk.Team{Name: name}, State: forgejo_sdk.ReviewStateRequestReview,
		}})
	}
	return nil
}

func (f *Forgejo) DeleteReviewRequests(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	p, err := f.pulls.pull(owner, repo, index)
	if err != nil {
		return err
	}
	kept := p.reviews[:0]
	for _, r := range p.reviews {
		if r.review.State == forgejo_sdk.ReviewStateRequestReview && requested(r.review, opt) {
			continue
		}
		kept = append(kept, r)
	}
	p.reviews = kept
	return nil
}

func requested(r *forgejo_sdk.PullReview, opt forgejo_sdk.PullReviewRequestOptions) bool {
	if r.Reviewer != nil {
		for _, n := range opt.Reviewers {
			if n == r.Reviewer.UserName {
				return true
			}
		}
	}
	if r.ReviewerTeam != nil {
		for _, n := range opt.TeamReviewers {
			if n == r.ReviewerTeam.Name {
				return true
			}
		}
	}
	return false
}
