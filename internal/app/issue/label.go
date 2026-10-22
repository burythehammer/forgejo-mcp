// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/labelresolve"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
)

const (
	// orgUsagePageSize is the page size used while enumerating an
	// organization's repositories to count an org label's use.
	orgUsagePageSize = 50
)

// catalog presents the issue port's label reads to the label resolver, in the
// resolver's own paging type. Issues and pull requests resolve label names
// through it, so the two share one rule.
type catalog struct{ port Port }

func (c catalog) ListRepoLabels(ctx context.Context, owner, repo string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListRepoLabels(ctx, owner, repo, Paging{Page: p.Page, Limit: p.Limit})
}

func (c catalog) ListOrgLabels(ctx context.Context, org string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListOrgLabels(ctx, org, Paging{Page: p.Page, Limit: p.Limit})
}

func (c catalog) MaxResponseItems(ctx context.Context) (int, bool) {
	return c.port.MaxResponseItems(ctx)
}

// RepoLabels lists a repository's labels. With IncludeOrg, an organization's
// labels follow, each tagged with its scope.
func (s *Service) RepoLabels(ctx context.Context, in RepoLabelsInput) (list.Bounded[model.ScopedLabel], error) {
	return s.ReadRepoLabels(ctx, in)
}

// OrgLabels lists an organization's labels, each tagged with scope "org". See
// ReadOrgLabels for the paging defaults.
func (s *Service) OrgLabels(ctx context.Context, org string, p Paging) (list.Bounded[model.ScopedLabel], error) {
	return s.ReadOrgLabels(ctx, org, p)
}

// GetRepoLabel returns one repository label by its ID.
func (s *Service) GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*model.CatalogLabel, error) {
	label, err := s.port.GetRepoLabel(ctx, owner, repo, id)
	if err != nil {
		return nil, fmt.Errorf("get repo label: %w", err)
	}
	return label, nil
}

// CreateRepoLabel creates a repository label. The colour is checked and
// normalised to #rrggbb before anything is sent.
func (s *Service) CreateRepoLabel(ctx context.Context, owner, repo string, in LabelNew) (*model.CatalogLabel, error) {
	color, err := normalizeColor(in.Color)
	if err != nil {
		return nil, err
	}
	in.Color = color
	label, err := s.port.CreateRepoLabel(ctx, owner, repo, in)
	if err != nil {
		return nil, fmt.Errorf("create repo label: %w", err)
	}
	return label, nil
}

// EditRepoLabel changes the supplied fields of a repository label.
func (s *Service) EditRepoLabel(ctx context.Context, owner, repo string, id int64, in LabelEdit) (*model.CatalogLabel, error) {
	edit, err := labelEdit(in)
	if err != nil {
		return nil, err
	}
	label, err := s.port.EditRepoLabel(ctx, owner, repo, id, edit)
	if err != nil {
		return nil, fmt.Errorf("edit repo label: %w", err)
	}
	return label, nil
}

// DeleteRepoLabel deletes a repository label. Unless force is set, a label
// that issues or pull requests still carry is kept, and the count is reported.
func (s *Service) DeleteRepoLabel(ctx context.Context, owner, repo string, id int64, force bool) error {
	if !force {
		label, err := s.port.GetRepoLabel(ctx, owner, repo, id)
		if err != nil {
			return fmt.Errorf("get repo label: %w", err)
		}
		count, err := s.labelUsage(ctx, owner, repo, label.Name)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("label %q is used by %d issue(s)/PR(s); set delete_mode=force to delete anyway", label.Name, count)
		}
	}
	if err := s.port.DeleteRepoLabel(ctx, owner, repo, id); err != nil {
		if errors.Is(err, forge.ErrNotFound) {
			return fmt.Errorf("label %d not found", id)
		}
		return fmt.Errorf("delete repo label: %w", err)
	}
	return nil
}

// GetOrgLabel returns one organization label by its ID.
func (s *Service) GetOrgLabel(ctx context.Context, org string, id int64) (*model.CatalogLabel, error) {
	label, err := s.port.GetOrgLabel(ctx, org, id)
	if err != nil {
		return nil, fmt.Errorf("get org label: %w", err)
	}
	return label, nil
}

// CreateOrgLabel creates an organization label.
func (s *Service) CreateOrgLabel(ctx context.Context, org string, in LabelNew) (*model.CatalogLabel, error) {
	color, err := normalizeColor(in.Color)
	if err != nil {
		return nil, err
	}
	in.Color = color
	label, err := s.port.CreateOrgLabel(ctx, org, in)
	if err != nil {
		return nil, fmt.Errorf("create org label: %w", err)
	}
	return label, nil
}

// EditOrgLabel changes the supplied fields of an organization label.
func (s *Service) EditOrgLabel(ctx context.Context, org string, id int64, in LabelEdit) (*model.CatalogLabel, error) {
	edit, err := labelEdit(in)
	if err != nil {
		return nil, err
	}
	label, err := s.port.EditOrgLabel(ctx, org, id, edit)
	if err != nil {
		return nil, fmt.Errorf("edit org label: %w", err)
	}
	return label, nil
}

// DeleteOrgLabel deletes an organization label. Unless force is set, a label
// that is in use in the organization's visible repositories is kept. That
// count skips repositories the token cannot read; any other failed count
// refuses the delete, since an unknown usage must not read as unused.
func (s *Service) DeleteOrgLabel(ctx context.Context, org string, id int64, force bool) error {
	if !force {
		label, err := s.port.GetOrgLabel(ctx, org, id)
		if err != nil {
			return fmt.Errorf("get org label: %w", err)
		}
		count, cerr := s.orgLabelUsage(ctx, org, label.Name)
		if cerr != nil {
			return fmt.Errorf("label %q in-use count failed, so the label was not deleted; use delete_mode=force to override: %w", label.Name, cerr)
		}
		if count > 0 {
			return fmt.Errorf("label %q is used by %d issue(s)/PR(s) in visible repos (count excludes inaccessible repos); set delete_mode=force to delete anyway", label.Name, count)
		}
	}
	if err := s.port.DeleteOrgLabel(ctx, org, id); err != nil {
		return fmt.Errorf("delete org label: %w", err)
	}
	return nil
}

// AddLabels adds labels, by name or ID, to an issue's existing ones and returns
// the issue as it now is.
func (s *Service) AddLabels(ctx context.Context, t Target, labels string) (*model.Issue, error) {
	ids, err := s.resolveApplied(ctx, t.Owner, t.Repo, labels)
	if err != nil {
		return nil, err
	}
	if err := s.port.AddIssueLabels(ctx, t.Owner, t.Repo, t.Index, ids); err != nil {
		return nil, fmt.Errorf("add issue labels err: %w", err)
	}
	issue, err := s.port.GetIssue(ctx, t.Owner, t.Repo, t.Index)
	if err != nil {
		return nil, fmt.Errorf("get updated issue err: %w", err)
	}
	return issue, nil
}

// RemoveLabels removes labels, by name or ID, from an issue and returns the
// issue as it now is.
func (s *Service) RemoveLabels(ctx context.Context, t Target, labels string) (*model.Issue, error) {
	ids, err := s.labels.Resolve(ctx, t.Owner, t.Repo, labels)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := s.port.RemoveIssueLabel(ctx, t.Owner, t.Repo, t.Index, id); err != nil {
			return nil, fmt.Errorf("remove issue label err: %w", err)
		}
	}
	issue, err := s.port.GetIssue(ctx, t.Owner, t.Repo, t.Index)
	if err != nil {
		return nil, fmt.Errorf("get updated issue err: %w", err)
	}
	return issue, nil
}

// labelUsage counts the issues and pull requests of a repository that carry the
// label. An unreported total counts as none.
func (s *Service) labelUsage(ctx context.Context, owner, repo, label string) (int, error) {
	total, err := s.port.CountIssuesLabelled(ctx, owner, repo, label)
	if err != nil {
		return 0, fmt.Errorf("count label usage: %w", err)
	}
	return ptr.Deref(total, 0), nil
}

// orgLabelUsage counts, best-effort, the issues and pull requests in an
// organization's visible repositories that carry the label. A repository the
// token cannot see adds nothing; any other count failure, or a failure to list
// the repositories, is returned with the count so far.
func (s *Service) orgLabelUsage(ctx context.Context, org, label string) (int, error) {
	total := 0
	for page := 1; ; page++ {
		names, err := s.port.ListOrgRepoNames(ctx, org, Paging{Page: page, Limit: orgUsagePageSize})
		if err != nil {
			return total, fmt.Errorf("enumerate org repos for label usage count: %w", err)
		}
		if len(names) == 0 {
			break
		}
		for _, name := range names {
			n, err := s.port.CountIssuesLabelled(ctx, org, name, label)
			var inaccessible *InaccessibleError
			if errors.As(err, &inaccessible) {
				continue
			}
			if err != nil {
				return total, fmt.Errorf("repo %s/%s: label may be in use: %w", org, name, err)
			}
			if n == nil {
				return total, fmt.Errorf("repo %s/%s: label may be in use: Forgejo reported no total", org, name)
			}
			total += *n
		}
	}
	return total, nil
}

// labelEdit checks that an edit supplies something, and turns its fields into
// the edit the port takes: an empty name is left alone, a colour is normalised,
// and a description, even an empty one, is set.
func labelEdit(in LabelEdit) (LabelEdit, error) {
	if in.Name == nil && in.Color == nil && in.Description == nil && in.Exclusive == nil {
		return LabelEdit{}, fmt.Errorf("%s: at least one of name, color, description, exclusive must be provided", in.Tool)
	}
	out := LabelEdit{Tool: in.Tool, Exclusive: in.Exclusive}
	if in.Name != nil && *in.Name != "" {
		name := *in.Name
		out.Name = &name
	}
	if in.Color != nil && *in.Color != "" {
		color, err := normalizeColor(*in.Color)
		if err != nil {
			return LabelEdit{}, err
		}
		out.Color = &color
	}
	if in.Description != nil {
		desc := *in.Description
		out.Description = &desc
	}
	return out, nil
}

// normalizeColor accepts rrggbb or #rrggbb (6-digit only), lowercases and
// prepends #. 3-digit shorthand is rejected on purpose: the upstream behaviour
// for it is unverified.
func normalizeColor(color string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(color))
	c = strings.TrimPrefix(c, "#")
	if len(c) != 6 {
		return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
	}
	for _, ch := range c {
		isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
		if !isHex {
			return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
		}
	}
	return "#" + c, nil
}
