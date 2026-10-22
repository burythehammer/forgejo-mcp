// SPDX-License-Identifier: GPL-3.0-or-later

// Package issueadapter implements the issue use cases' port (internal/app/issue.Port)
// over the Forgejo port (forge.Issues, forge.Labels and forge.Dependencies). It
// is the one place that translates between repo-owned types and SDK types for
// the issue domain: inputs become SDK options, SDK results become internal/app/model
// values. Results are converted through their JSON form, which is exactly what
// a tool renders, so a field the repo-owned type shares by JSON name cannot
// drift in value.
package issueadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	appissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/bridge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Source is the part of the Forgejo port the issue domain uses. *fake.Forgejo
// and the SDK adapter both satisfy it.
type Source interface {
	forge.Issues
	forge.Labels
	forge.Dependencies
}

// New returns the use cases' port over s.
func New(s Source) appissue.Port { return adapter{s: s} }

type adapter struct{ s Source }

var _ appissue.Port = adapter{}

func listOptions(p appissue.Paging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) GetIssue(ctx context.Context, owner, repo string, index int64) (*model.Issue, error) {
	issue, err := a.s.GetIssue(ctx, owner, repo, index)
	return bridge.Convert[*model.Issue](issue, err)
}

func (a adapter) ListIssues(ctx context.Context, owner, repo string, q appissue.IssueList) ([]*model.Issue, error) {
	issues, _, err := a.s.ListRepoIssues(ctx, owner, repo, forge.ListIssuesOptions{
		ListIssueOption: forgejo_sdk.ListIssueOption{
			State:       forgejo_sdk.StateType(q.State),
			Type:        forgejo_sdk.IssueType(q.Type),
			Labels:      q.Labels,
			Milestones:  q.Milestones,
			ListOptions: listOptions(q.Paging),
		},
		// The adapter carries sort to Forgejo (the SDK cannot), see forge.ListIssuesOptions.
		Sort: q.Sort,
	})
	return bridge.Convert[[]*model.Issue](issues, err)
}

func (a adapter) SearchIssues(ctx context.Context, q appissue.SearchQuery) ([]*model.Issue, *int, error) {
	issues, info, err := a.s.SearchIssues(ctx, forgejo_sdk.ListIssueOption{
		Owner:       q.Owner,
		State:       forgejo_sdk.StateType(q.State),
		Type:        forgejo_sdk.IssueType(q.Type),
		Labels:      q.Labels,
		Milestones:  q.Milestones,
		KeyWord:     q.Keyword,
		CreatedBy:   q.CreatedBy,
		AssignedBy:  q.AssignedBy,
		MentionedBy: q.MentionedBy,
		ListOptions: listOptions(q.Paging),
	})
	out, cerr := bridge.Convert[[]*model.Issue](issues, err)
	return out, list.Reported(info.TotalCount, info.TotalKnown), cerr
}

func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.s.MaxResponseItems(ctx)
}

func (a adapter) CreateIssue(ctx context.Context, owner, repo string, in appissue.NewIssue) (*model.Issue, error) {
	issue, err := a.s.CreateIssue(ctx, owner, repo, forgejo_sdk.CreateIssueOption{
		Title:     in.Title,
		Body:      in.Body,
		Ref:       in.Ref,
		Assignees: in.Assignees,
		Milestone: in.Milestone,
		Labels:    in.Labels,
	})
	return bridge.Convert[*model.Issue](issue, err)
}

func (a adapter) EditIssue(ctx context.Context, owner, repo string, index int64, in appissue.IssueEdit) (*model.Issue, error) {
	opt := forgejo_sdk.EditIssueOption{
		Title:          in.Title,
		Body:           in.Body,
		Ref:            in.Ref,
		Assignees:      in.Assignees,
		Milestone:      in.Milestone,
		Deadline:       in.Deadline,
		RemoveDeadline: in.RemoveDeadline,
	}
	if in.State != nil {
		opt.State = ptr.To(forgejo_sdk.StateType(*in.State))
	}
	issue, err := a.s.EditIssue(ctx, owner, repo, index, opt)
	return bridge.Convert[*model.Issue](issue, err)
}

func (a adapter) ReplaceIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error {
	return a.s.ReplaceIssueLabels(ctx, owner, repo, index, labelIDs)
}

func (a adapter) ClearIssueLabels(ctx context.Context, owner, repo string, index int64) error {
	return a.s.ClearIssueLabels(ctx, owner, repo, index)
}

func (a adapter) AddIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error {
	return a.s.AddIssueLabels(ctx, owner, repo, index, labelIDs)
}

func (a adapter) RemoveIssueLabel(ctx context.Context, owner, repo string, index, labelID int64) error {
	return a.s.RemoveIssueLabel(ctx, owner, repo, index, labelID)
}

func (a adapter) ListIssueComments(ctx context.Context, owner, repo string, index int64, c appissue.CommentList) ([]*model.Comment, error) {
	comments, _, err := a.s.ListIssueComments(ctx, owner, repo, index, forgejo_sdk.ListIssueCommentOptions{
		ListOptions: listOptions(c.Paging),
		Since:       c.Since,
		Before:      c.Before,
	})
	return bridge.Convert[[]*model.Comment](comments, err)
}

func (a adapter) GetIssueComment(ctx context.Context, owner, repo string, id int64) (*model.Comment, error) {
	comment, err := a.s.GetIssueComment(ctx, owner, repo, id)
	return bridge.Convert[*model.Comment](comment, err)
}

func (a adapter) CreateIssueComment(ctx context.Context, owner, repo string, index int64, body string) (*model.Comment, error) {
	comment, err := a.s.CreateIssueComment(ctx, owner, repo, index, body)
	return bridge.Convert[*model.Comment](comment, err)
}

func (a adapter) EditIssueComment(ctx context.Context, owner, repo string, id int64, body string) (*model.Comment, error) {
	comment, err := a.s.EditIssueComment(ctx, owner, repo, id, body)
	return bridge.Convert[*model.Comment](comment, err)
}

func (a adapter) DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error {
	return a.s.DeleteIssueComment(ctx, owner, repo, id)
}

func (a adapter) ListRepoMilestones(ctx context.Context, owner, repo string, m appissue.MilestoneList) ([]*model.Milestone, error) {
	milestones, err := a.s.ListRepoMilestones(ctx, owner, repo, forgejo_sdk.ListMilestoneOption{
		ListOptions: listOptions(m.Paging),
		State:       forgejo_sdk.StateType(m.State),
	})
	return bridge.Convert[[]*model.Milestone](milestones, err)
}

func (a adapter) GetMilestone(ctx context.Context, owner, repo string, id int64) (*model.Milestone, error) {
	m, err := a.s.GetMilestone(ctx, owner, repo, id)
	if forge.IsNotFound(err) {
		return nil, &forge.StatusError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("milestone %d not found", id)}
	}
	return bridge.Convert[*model.Milestone](m, err)
}

func (a adapter) CreateMilestone(ctx context.Context, owner, repo string, in appissue.MilestoneNew) (*model.Milestone, error) {
	opt := forgejo_sdk.CreateMilestoneOption{Title: in.Title, Description: in.Description, Deadline: in.Due}
	m, err := a.s.CreateMilestone(ctx, owner, repo, opt)
	return bridge.Convert[*model.Milestone](m, err)
}

func (a adapter) EditMilestone(ctx context.Context, owner, repo string, id int64, in appissue.MilestoneEdit) (*model.Milestone, error) {
	opt := forgejo_sdk.EditMilestoneOption{Deadline: in.Due}
	if in.Title != nil {
		opt.Title = *in.Title
	}
	if in.Description != nil {
		desc := *in.Description
		opt.Description = &desc
	}
	if in.State != nil {
		state := forgejo_sdk.StateType(*in.State)
		opt.State = &state
	}
	m, err := a.s.EditMilestone(ctx, owner, repo, id, opt)
	if forge.IsNotFound(err) {
		return nil, &forge.StatusError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("milestone %d not found", id)}
	}
	return bridge.Convert[*model.Milestone](m, err)
}

func (a adapter) ListRepoLabels(ctx context.Context, owner, repo string, p appissue.Paging) ([]*model.CatalogLabel, error) {
	labels, _, err := a.s.ListRepoLabels(ctx, owner, repo, p.Page, p.Limit)
	return bridge.Convert[[]*model.CatalogLabel](labels, err)
}

func (a adapter) GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*model.CatalogLabel, error) {
	label, err := a.s.GetRepoLabel(ctx, owner, repo, id)
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) CreateRepoLabel(ctx context.Context, owner, repo string, in appissue.LabelNew) (*model.CatalogLabel, error) {
	label, err := a.s.CreateRepoLabel(ctx, owner, repo, labelNew(in))
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) EditRepoLabel(ctx context.Context, owner, repo string, id int64, in appissue.LabelEdit) (*model.CatalogLabel, error) {
	label, err := a.s.EditRepoLabel(ctx, owner, repo, id, labelEdit(in))
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) DeleteRepoLabel(ctx context.Context, owner, repo string, id int64) error {
	return a.s.DeleteRepoLabel(ctx, owner, repo, id)
}

func (a adapter) ListOrgLabels(ctx context.Context, org string, p appissue.Paging) ([]*model.CatalogLabel, error) {
	labels, _, err := a.s.ListOrgLabels(ctx, org, p.Page, p.Limit)
	return bridge.Convert[[]*model.CatalogLabel](labels, orgLabelsErr(err))
}

func (a adapter) ListRepoLabelsPage(ctx context.Context, owner, repo string, p appissue.Paging) (appissue.LabelPage, error) {
	labels, info, err := a.s.ListRepoLabels(ctx, owner, repo, p.Page, p.Limit)
	return labelPage(labels, info, err)
}

func (a adapter) ListOrgLabelsPage(ctx context.Context, org string, p appissue.Paging) (appissue.LabelPage, error) {
	labels, info, err := a.s.ListOrgLabels(ctx, org, p.Page, p.Limit)
	return labelPage(labels, info, orgLabelsErr(err))
}

// labelPage converts one label page and carries what the server reported past
// it. The total is kept only when the server reported one.
// orgLabelsErr marks a forge not-found on an organization's labels as
// appissue.ErrOrgNotFound, keeping the forge kind for the renderers.
func orgLabelsErr(err error) error {
	if forge.IsNotFound(err) {
		return appissue.AsOrgNotFound(err)
	}
	return err
}

func labelPage(labels []*forge.Label, info forge.PageInfo, err error) (appissue.LabelPage, error) {
	out, err := bridge.Convert[[]*model.CatalogLabel](labels, err)
	if err != nil {
		return appissue.LabelPage{}, err
	}
	return appissue.LabelPage{
		Labels: out,
		More:   info.HasNext,
		Total:  list.Reported(info.TotalCount, info.TotalKnown),
	}, nil
}

func (a adapter) GetOrgLabel(ctx context.Context, org string, id int64) (*model.CatalogLabel, error) {
	label, err := a.s.GetOrgLabel(ctx, org, id)
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) CreateOrgLabel(ctx context.Context, org string, in appissue.LabelNew) (*model.CatalogLabel, error) {
	label, err := a.s.CreateOrgLabel(ctx, org, labelNew(in))
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) EditOrgLabel(ctx context.Context, org string, id int64, in appissue.LabelEdit) (*model.CatalogLabel, error) {
	label, err := a.s.EditOrgLabel(ctx, org, id, labelEdit(in))
	return bridge.Convert[*model.CatalogLabel](label, err)
}

func (a adapter) DeleteOrgLabel(ctx context.Context, org string, id int64) error {
	return a.s.DeleteOrgLabel(ctx, org, id)
}

func (a adapter) ListOrgRepoNames(ctx context.Context, org string, p appissue.Paging) ([]string, error) {
	return a.s.ListOrgRepoNames(ctx, org, p.Page, p.Limit)
}

// CountIssuesLabelled asks for one row of the repository's issues carrying the
// label, in any state: the list's total is the count.
func (a adapter) CountIssuesLabelled(ctx context.Context, owner, repo, label string) (*int, error) {
	_, info, err := a.s.ListRepoIssues(ctx, owner, repo, forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{
		State:       forgejo_sdk.StateType("all"),
		Labels:      []string{label},
		ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 1},
	}})
	if forge.IsNotFound(err) || errors.Is(err, forge.ErrUnauthorized) {
		return nil, &appissue.InaccessibleError{Err: err}
	}
	if err != nil {
		return nil, err
	}
	return list.Reported(info.TotalCount, info.TotalKnown), nil
}

func (a adapter) ListIssueDependencies(ctx context.Context, owner, repo string, index int64, p appissue.Paging) ([]*model.Issue, error) {
	issues, err := a.s.ListIssueDependencies(ctx, owner, repo, index, p.Page, p.Limit)
	return bridge.Convert[[]*model.Issue](issues, err)
}

func (a adapter) ListIssueDependents(ctx context.Context, owner, repo string, index int64, p appissue.Paging) ([]*model.Issue, error) {
	issues, err := a.s.ListIssueDependents(ctx, owner, repo, index, p.Page, p.Limit)
	return bridge.Convert[[]*model.Issue](issues, err)
}

func (a adapter) AddIssueDependency(ctx context.Context, owner, repo string, index int64, dep appissue.Target) error {
	return a.s.AddIssueDependency(ctx, owner, repo, index, issueRef(dep))
}

func (a adapter) RemoveIssueDependency(ctx context.Context, owner, repo string, index int64, dep appissue.Target) error {
	return a.s.RemoveIssueDependency(ctx, owner, repo, index, issueRef(dep))
}

func issueRef(t appissue.Target) forge.IssueRef {
	return forge.IssueRef{Owner: t.Owner, Repo: t.Repo, Index: t.Index}
}

func labelNew(in appissue.LabelNew) forge.LabelCreate {
	return forge.LabelCreate{Name: in.Name, Color: in.Color, Description: in.Description, Exclusive: in.Exclusive}
}

func labelEdit(in appissue.LabelEdit) forge.LabelPatch {
	return forge.LabelPatch{Name: in.Name, Color: in.Color, Description: in.Description, Exclusive: in.Exclusive}
}
