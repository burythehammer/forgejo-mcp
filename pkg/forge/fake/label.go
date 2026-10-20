// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"sort"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Labels = (*Forgejo)(nil)

// Label state lives in issueState (it shares the lock and the label ID
// sequence with issue label assignment): repository labels in issueRepo,
// organization labels in issueState.orgLabels.

// assignableLabel finds a label an issue in r may carry: one of the
// repository's own, or one of its owner's organization. The caller holds
// f.issues.mu.
func (f *Forgejo) assignableLabel(r *issueRepo, id int64) (*forgejo_sdk.Label, bool) {
	if l, ok := r.labels[id]; ok {
		return l, true
	}
	l, ok := f.issues.orgLabels[r.owner][id]
	return l, ok
}

func sortedLabels(m map[int64]*forgejo_sdk.Label) []*forgejo_sdk.Label {
	out := make([]*forgejo_sdk.Label, 0, len(m))
	for _, l := range m {
		cp := *l
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// labelPage pages a label list. Like Forgejo, it clamps the page size to the
// instance ceiling (max_response_items) when one is set, so a page can come back
// shorter than the size asked for without being the last one.
func labelPage(all []*forgejo_sdk.Label, opt forgejo_sdk.ListOptions, ceiling int) ([]*forgejo_sdk.Label, forge.PageInfo) {
	if opt.PageSize <= 0 {
		opt.PageSize = defaultPageSize
	}
	if ceiling > 0 && opt.PageSize > ceiling {
		opt.PageSize = ceiling
	}
	lo, hi, info := pageInfoFor(opt, len(all))
	return all[lo:hi], info
}

func validColor(c string) error {
	if len(strings.TrimPrefix(c, "#")) != 6 {
		return errors.New("invalid color")
	}
	return nil
}

func applyEdit(l *forgejo_sdk.Label, opt forgejo_sdk.EditLabelOption) {
	if opt.Name != nil {
		l.Name = *opt.Name
	}
	if opt.Color != nil {
		l.Color = strings.TrimPrefix(*opt.Color, "#")
	}
	if opt.Description != nil {
		l.Description = *opt.Description
	}
}

func (f *Forgejo) ListRepoLabels(_ context.Context, owner, repo string, opt forgejo_sdk.ListLabelsOptions) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	out, info := labelPage(sortedLabels(r.labels), opt.ListOptions, f.issues.maxItems)
	return out, info, nil
}

func (f *Forgejo) GetRepoLabel(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	l, ok := r.labels[id]
	if !ok {
		return nil, notFound("label")
	}
	cp := *l
	return &cp, nil
}

func (f *Forgejo) CreateRepoLabel(_ context.Context, owner, repo string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	if opt.Name == "" {
		return nil, errors.New("label name is required")
	}
	if err := validColor(opt.Color); err != nil {
		return nil, err
	}
	f.issues.nextLabel++
	l := &forgejo_sdk.Label{ID: f.issues.nextLabel, Name: opt.Name, Color: strings.TrimPrefix(opt.Color, "#"), Description: opt.Description}
	r.labels[l.ID] = l
	cp := *l
	return &cp, nil
}

func (f *Forgejo) EditRepoLabel(_ context.Context, owner, repo string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	l, ok := r.labels[id]
	if !ok {
		return nil, notFound("label")
	}
	if opt.Color != nil {
		if err := validColor(*opt.Color); err != nil {
			return nil, err
		}
	}
	applyEdit(l, opt)
	cp := *l
	return &cp, nil
}

// dropFromIssues removes label id from every issue of r (deleting a label
// unlabels what carried it).
func dropFromIssues(r *issueRepo, id int64) {
	for _, rec := range r.issues {
		kept := rec.issue.Labels[:0:0]
		for _, l := range rec.issue.Labels {
			if l.ID != id {
				kept = append(kept, l)
			}
		}
		rec.issue.Labels = kept
	}
}

func (f *Forgejo) DeleteRepoLabel(_ context.Context, owner, repo string, id int64) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	r, err := f.issueRepo(owner, repo)
	if err != nil {
		return err
	}
	if _, ok := r.labels[id]; !ok {
		return notFound("label")
	}
	delete(r.labels, id)
	dropFromIssues(r, id)
	return nil
}

func (f *Forgejo) ListOrgLabels(_ context.Context, org string, pg, limit int) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	out, info := labelPage(sortedLabels(f.issues.orgLabels[org]), forgejo_sdk.ListOptions{Page: pg, PageSize: limit}, f.issues.maxItems)
	return out, info, nil
}

func (f *Forgejo) GetOrgLabel(_ context.Context, org string, id int64) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	l, ok := f.issues.orgLabels[org][id]
	if !ok {
		return nil, notFound("label")
	}
	cp := *l
	return &cp, nil
}

func (f *Forgejo) CreateOrgLabel(_ context.Context, org string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	if opt.Name == "" {
		return nil, errors.New("label name is required")
	}
	if err := validColor(opt.Color); err != nil {
		return nil, err
	}
	if f.issues.orgLabels == nil {
		f.issues.orgLabels = map[string]map[int64]*forgejo_sdk.Label{}
	}
	if f.issues.orgLabels[org] == nil {
		f.issues.orgLabels[org] = map[int64]*forgejo_sdk.Label{}
	}
	f.issues.nextLabel++
	l := &forgejo_sdk.Label{ID: f.issues.nextLabel, Name: opt.Name, Color: strings.TrimPrefix(opt.Color, "#"), Description: opt.Description}
	f.issues.orgLabels[org][l.ID] = l
	cp := *l
	return &cp, nil
}

func (f *Forgejo) EditOrgLabel(_ context.Context, org string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	l, ok := f.issues.orgLabels[org][id]
	if !ok {
		return nil, notFound("label")
	}
	if opt.Color != nil {
		if err := validColor(*opt.Color); err != nil {
			return nil, err
		}
	}
	applyEdit(l, opt)
	cp := *l
	return &cp, nil
}

func (f *Forgejo) DeleteOrgLabel(_ context.Context, org string, id int64) error {
	f.issues.mu.Lock()
	defer f.issues.mu.Unlock()
	if _, ok := f.issues.orgLabels[org][id]; !ok {
		return notFound("label")
	}
	delete(f.issues.orgLabels[org], id)
	for _, r := range f.issues.repos {
		if r.owner == org {
			dropFromIssues(r, id)
		}
	}
	return nil
}

func (f *Forgejo) ListOrgRepoNames(_ context.Context, org string, pg, limit int) ([]string, error) {
	f.pulls.mu.Lock()
	var names []string
	for key := range f.pulls.repos {
		if owner, name, ok := strings.Cut(key, "/"); ok && owner == org {
			names = append(names, name)
		}
	}
	f.pulls.mu.Unlock()
	sort.Strings(names)
	lo, hi := page(forgejo_sdk.ListOptions{Page: pg, PageSize: limit}, len(names))
	return names[lo:hi], nil
}

func (f *Forgejo) AddIssueLabels(_ context.Context, owner, repo string, index int64, labelIDs []int64) error {
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
	for _, id := range labelIDs {
		l, ok := f.assignableLabel(r, id)
		if !ok {
			return notFound("label")
		}
		have := false
		for _, h := range rec.issue.Labels {
			have = have || h.ID == id
		}
		if !have {
			cp := *l
			rec.issue.Labels = append(rec.issue.Labels, &cp)
		}
	}
	rec.issue.Updated = f.Now()
	return nil
}

func (f *Forgejo) RemoveIssueLabel(_ context.Context, owner, repo string, index, labelID int64) error {
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
	if _, ok := f.assignableLabel(r, labelID); !ok {
		return notFound("label")
	}
	kept := rec.issue.Labels[:0:0]
	for _, l := range rec.issue.Labels {
		if l.ID != labelID {
			kept = append(kept, l)
		}
	}
	rec.issue.Labels = kept
	rec.issue.Updated = f.Now()
	return nil
}
