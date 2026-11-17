// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/labelresolve"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/split"
)

// Service runs the issue, label and dependency use cases over a Port. Build
// one per request from the port the request's credential selects; it holds no
// other state.
type Service struct {
	port   Port
	labels *labelresolve.Resolver
}

// New returns the use cases bound to port.
func New(port Port) *Service {
	return &Service{port: port, labels: labelresolve.New(catalog{port})}
}

// The inputs below are what an MCP adapter has decoded from a tool call. The
// adapter applies the tool's defaults for an argument that was left out or
// was zero, because that defaulting happens before a number is truncated to an
// integer; the use cases validate and shape what they are given.

// ListInput lists a repository's issues. State and Sort are passed on as given.
type ListInput struct {
	Owner, Repo, State, Type, Sort string
	// Milestones and Labels are comma-separated, unsplit.
	Milestones, Labels string
	Paging
}

// SearchInput searches an owner's issues. Labels and Milestones are
// comma-separated, unsplit.
type SearchInput struct {
	Owner, State, Type, Labels, Milestones      string
	Keyword, CreatedBy, AssignedBy, MentionedBy string
	Paging
}

// CreateInput opens an issue. Labels and Assignees are comma-separated and nil
// when the argument was not given; Milestone is a milestone ID as text.
type CreateInput struct {
	Owner, Repo, Title, Body, Ref string
	Labels, Assignees             *string
	Milestone                     string
}

// UpdateInput changes an issue. Title, Body, Assignee, Milestone and DueDate
// are empty when not given. Assignees, Ref and SetLabels are nil when not
// given; a non-nil empty value is meaningful (clear the assignees, clear the
// branch link, remove every label).
type UpdateInput struct {
	Target
	Title, Body, Assignee string
	Assignees, Ref        *string
	Milestone, DueDate    string
	ClearDueDate          bool
	SetLabels             *string
}

// CommentsInput lists an issue's comments. Since and Before are RFC 3339 text.
type CommentsInput struct {
	Target
	Since, Before string
	Paging
}

// RepoLabelsInput lists a repository's labels, merged with its organization's
// when IncludeOrg is set.
type RepoLabelsInput struct {
	Owner, Repo string
	IncludeOrg  bool
	Paging
}

// List returns a repository's issues. The owner and repository are required,
// and sort, when given, must be one of Forgejo's sort orders.
func (s *Service) List(ctx context.Context, in ListInput) (list.Bounded[*model.Issue], error) {
	if err := checkList(in); err != nil {
		return list.Bounded[*model.Issue]{}, err
	}
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	page, err := list.Run(ctx, list.NewRequest(list.Paging(in.Paging), defaultLimit), c, s.fetchIssues(in))
	if err != nil {
		return list.Bounded[*model.Issue]{}, err
	}
	return page, nil
}

// ListAll returns every issue of a repository's list, read page by page at the
// default limit (lowered to a known instance ceiling). It is for a caller that
// writes the whole list to a file, so it holds the list to list.FileCeiling
// issues: a listing over that fails with an error naming the ceiling, and the
// caller never receives a truncated list.
func (s *Service) ListAll(ctx context.Context, in ListInput) ([]*model.Issue, error) {
	if err := checkList(in); err != nil {
		return nil, err
	}
	ceiling, known := s.port.MaxResponseItems(ctx)
	limit := list.DefaultLimit(defaultLimit, list.Ceiling{Max: ceiling, Known: known})
	issues, err := list.ReadWhole(ctx, limit, list.FileCeiling, s.fetchIssues(in))
	var over *list.OverCeiling
	if errors.As(err, &over) {
		return nil, fmt.Errorf("issue listing is over the %d-issue file-output ceiling; nothing was written", list.FileCeiling)
	}
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// checkList refuses a list request the port must not see: no owner, no repo
// (owner-wide listing is search_issues), or a sort order Forgejo does not know.
func checkList(in ListInput) error {
	if in.Owner == "" {
		return fmt.Errorf("owner is required")
	}
	if in.Repo == "" {
		return fmt.Errorf("repo is required; for owner-wide listing across repositories use search_issues instead")
	}
	if in.Sort != "" && !isValidSort(in.Sort) {
		return fmt.Errorf("invalid sort %q: must be one of %s", in.Sort, strings.Join(sortOrders, ", "))
	}
	return nil
}

// fetchIssues returns the list.Fetch that reads one page of in from the port.
func (s *Service) fetchIssues(in ListInput) list.Fetch[*model.Issue] {
	return func(ctx context.Context, p list.Paging) (list.Batch[*model.Issue], error) {
		issues, err := s.port.ListIssues(ctx, in.Owner, in.Repo, IssueList{
			State: in.State, Type: in.Type, Sort: in.Sort,
			Milestones: splitRaw(in.Milestones), Labels: splitRaw(in.Labels),
			Paging: Paging(p),
		})
		if err != nil {
			return list.Batch[*model.Issue]{}, fmt.Errorf("get issues list err: %w", err)
		}
		return list.Batch[*model.Issue]{Items: issues}, nil
	}
}

// defaultLimit is the page size a list or search uses when the caller gives none.
const defaultLimit = 20

// runPage reads one page of a list whose endpoint reports no grand total,
// through the list module: a default limit is lowered to a known ceiling, a
// limit above it is refused, and has_next comes from at most one probe.
func runPage[T any](ctx context.Context, src list.CeilingSource, p list.Paging, def int,
	fetch func(context.Context, list.Paging) ([]T, error),
) (list.Bounded[T], error) {
	ceiling, known := src.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	return list.Run(ctx, list.NewRequest(p, def), c, func(ctx context.Context, q list.Paging) (list.Batch[T], error) {
		got, err := fetch(ctx, q)
		return list.Batch[T]{Items: got}, err
	})
}

// sortOrders are the Forgejo API's valid sort values for listing issues. The
// SDK has no field for them, so they are listed here.
var sortOrders = []string{
	"relevance", "latest", "oldest", "recentupdate", "leastupdate",
	"mostcomment", "leastcomment", "nearduedate", "farduedate",
}

func isValidSort(s string) bool {
	for _, v := range sortOrders {
		if s == v {
			return true
		}
	}
	return false
}

// splitRaw splits a comma-separated filter as written: no trimming and no
// dropping of empty items. An empty argument is no filter at all.
func splitRaw(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// Search returns one page of issues across an owner's repositories. The limit
// is checked against the instance's ceiling before the call, so the page never
// claims a limit larger than the one applied. has_next is true when a full
// page came back; when the ceiling is unknown it asks Forgejo for the next page
// at the same size.
func (s *Service) Search(ctx context.Context, in SearchInput) (list.Bounded[*model.Issue], error) {
	if in.Owner == "" {
		return list.Bounded[*model.Issue]{}, fmt.Errorf("owner is required")
	}
	state := in.State
	if state == "" {
		state = "open"
	}
	ceiling, known := s.port.MaxResponseItems(ctx)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultLimit),
		list.Ceiling{Max: ceiling, Known: known},
		func(ctx context.Context, p list.Paging) (list.Batch[*model.Issue], error) {
			issues, total, err := s.port.SearchIssues(ctx, SearchQuery{
				Owner: in.Owner, State: state, Type: in.Type,
				Labels: splitRaw(in.Labels), Milestones: splitRaw(in.Milestones),
				Keyword: in.Keyword, CreatedBy: in.CreatedBy, AssignedBy: in.AssignedBy, MentionedBy: in.MentionedBy,
				Paging: Paging(p),
			})
			if err != nil {
				return list.Batch[*model.Issue]{}, fmt.Errorf("search issues err: %w", err)
			}
			return list.Batch[*model.Issue]{Items: issues, Total: total}, nil
		})
	if err != nil {
		return list.Bounded[*model.Issue]{}, err
	}
	return page, nil
}

// Create opens an issue. Labels resolve before the POST, not after it: a typo
// must not leave a real unlabelled issue behind for someone to clean up.
func (s *Service) Create(ctx context.Context, in CreateInput) (*model.Issue, error) {
	opt := NewIssue{Title: in.Title, Body: in.Body, Ref: in.Ref}
	if in.Labels != nil {
		ids, err := s.resolveApplied(ctx, in.Owner, in.Repo, *in.Labels)
		if err != nil {
			return nil, err
		}
		opt.Labels = ids
	}
	if in.Assignees != nil {
		opt.Assignees = split.CSV(*in.Assignees)
	}
	if in.Milestone != "" {
		id, err := parseMilestone(in.Milestone)
		if err != nil {
			return nil, err
		}
		opt.Milestone = id
	}
	issue, err := s.port.CreateIssue(ctx, in.Owner, in.Repo, opt)
	if err != nil {
		return nil, fmt.Errorf("create issue err: %w", err)
	}
	return issue, nil
}

// Update changes an issue in up to three requests, in this order: the edit
// (PATCH), the label set (replaced, or cleared when SetLabels names nothing),
// and a re-read. The re-read is needed because the PATCH response predates the
// label change. Every argument is validated, and every label name resolved,
// before the first request, so a bad argument leaves the issue untouched.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*model.Issue, error) {
	edit := IssueEdit{Title: in.Title}
	if in.Body != "" {
		body := in.Body
		edit.Body = &body
	}
	if in.Ref != nil {
		ref := *in.Ref
		edit.Ref = &ref
	}
	// Assignees: the list wins if given; otherwise the single assignee does.
	// An explicitly given empty list clears them, sent as [].
	switch {
	case in.Assignees != nil:
		edit.Assignees = split.CSV(*in.Assignees)
	case in.Assignee != "":
		edit.Assignees = []string{in.Assignee}
	}
	if in.Milestone != "" {
		id, err := parseMilestone(in.Milestone)
		if err != nil {
			return nil, err
		}
		edit.Milestone = &id
	}
	// Due date and its clearing are mutually exclusive: ambiguous otherwise.
	switch {
	case in.ClearDueDate && in.DueDate != "":
		return nil, fmt.Errorf("cannot set 'due_date' and 'clear_due_date' at the same time")
	case in.ClearDueDate:
		remove := true
		edit.RemoveDeadline = &remove
	case in.DueDate != "":
		parsed, err := time.Parse(time.RFC3339, in.DueDate)
		if err != nil {
			return nil, fmt.Errorf("invalid due_date format (expected RFC3339): %w", err)
		}
		edit.Deadline = &parsed
	}

	// Resolve before either request, so an unknown label name leaves the
	// issue untouched instead of half-updated.
	var labelIDs []int64
	clearLabels := false
	if in.SetLabels != nil {
		if len(split.CSV(*in.SetLabels)) == 0 {
			clearLabels = true
		} else {
			ids, err := s.resolveApplied(ctx, in.Owner, in.Repo, *in.SetLabels)
			if err != nil {
				return nil, err
			}
			labelIDs = ids
		}
	}

	// A labels-only update sends no PATCH, which would move updated_at for
	// nothing. With no set_labels at all the edit is always sent.
	patch := in.Title != "" || in.Body != "" || in.Assignees != nil || in.Assignee != "" ||
		in.Milestone != "" || in.DueDate != "" || in.ClearDueDate || in.Ref != nil

	var issue *model.Issue
	if patch || in.SetLabels == nil {
		var err error
		issue, err = s.port.EditIssue(ctx, in.Owner, in.Repo, in.Index, edit)
		if err != nil {
			return nil, fmt.Errorf("update issue err: %w", err)
		}
	}
	if in.SetLabels != nil {
		if clearLabels {
			if err := s.port.ClearIssueLabels(ctx, in.Owner, in.Repo, in.Index); err != nil {
				return nil, fmt.Errorf("clear issue labels err: %w", err)
			}
		} else if err := s.port.ReplaceIssueLabels(ctx, in.Owner, in.Repo, in.Index, labelIDs); err != nil {
			return nil, fmt.Errorf("replace issue labels err: %w", err)
		}
		var err error
		issue, err = s.port.GetIssue(ctx, in.Owner, in.Repo, in.Index)
		if err != nil {
			return nil, fmt.Errorf("get updated issue err: %w", err)
		}
	}
	return issue, nil
}

// ChangeState opens or closes an issue.
func (s *Service) ChangeState(ctx context.Context, t Target, state string) (*model.Issue, error) {
	if state != "open" && state != "closed" {
		return nil, fmt.Errorf("invalid state: %s, must be 'open' or 'closed'", state)
	}
	issue, err := s.port.EditIssue(ctx, t.Owner, t.Repo, t.Index, IssueEdit{State: &state})
	if err != nil {
		return nil, fmt.Errorf("change issue state err: %w", err)
	}
	return issue, nil
}

// Milestones lists a repository's milestones. State defaults to open.
func (s *Service) Milestones(ctx context.Context, owner, repo string, m MilestoneList) (list.Bounded[*model.Milestone], error) {
	if m.State == "" {
		m.State = "open"
	}
	page, err := runPage(ctx, s.port, list.Paging(m.Paging), milestoneDefaultLimit,
		func(ctx context.Context, p list.Paging) ([]*model.Milestone, error) {
			q := m
			q.Paging = Paging(p)
			milestones, err := s.port.ListRepoMilestones(ctx, owner, repo, q)
			if err != nil {
				return nil, fmt.Errorf("list repo milestones err: %w", err)
			}
			return milestones, nil
		})
	if err != nil {
		return list.Bounded[*model.Milestone]{}, err
	}
	return page, nil
}

// milestoneDefaultLimit is the page size a milestone list uses when the caller
// gives none.
const milestoneDefaultLimit = 100

func parseMilestone(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid milestone ID: %w", err)
	}
	return id, nil
}
