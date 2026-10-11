// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/internal/split"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

const (
	// labelCatalogPageSize is the page size used while enumerating labels for
	// name resolution. It is independent of the caller-facing page and limit of
	// list_repo_labels: that tool browses, this enumeration must be complete,
	// or a label on page two would resolve as "unknown".
	labelCatalogPageSize = 100

	// maxLabelCatalogPages caps the enumeration per scope. A server that
	// ignores paging would otherwise spin forever; the cap turns that into a
	// message, and numeric IDs still work without a catalogue.
	maxLabelCatalogPages = 20

	// orgUsagePageSize is the page size used while enumerating an
	// organization's repositories to count an org label's use.
	orgUsagePageSize = 50
)

// RepoLabels lists a repository's labels. With IncludeOrg, an organization's
// labels follow, each tagged with its scope.
func (s *Service) RepoLabels(ctx context.Context, in RepoLabelsInput) ([]model.ScopedLabel, error) {
	repoLabels, err := s.port.ListRepoLabels(ctx, in.Owner, in.Repo, in.Paging)
	if err != nil {
		return nil, fmt.Errorf("list repo labels err: %w", err)
	}
	merged := make([]model.ScopedLabel, 0, len(repoLabels))
	for _, l := range repoLabels {
		merged = append(merged, model.ScopedLabel{Label: *l, Scope: "repo"})
	}
	if in.IncludeOrg {
		orgLabels, err := s.OrgLabels(ctx, in.Owner, in.Paging)
		if err != nil {
			return nil, err
		}
		merged = append(merged, orgLabels...)
	}
	return merged, nil
}

// OrgLabels lists an organization's labels, each tagged with scope "org". A
// page below 1 reads as page 1 and a limit below 1 as the default of 100.
func (s *Service) OrgLabels(ctx context.Context, org string, p Paging) ([]model.ScopedLabel, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 100
	}
	raw, err := s.port.ListOrgLabels(ctx, org, p)
	if err != nil {
		return nil, fmt.Errorf("list org labels err: %w", err)
	}
	out := make([]model.ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, model.ScopedLabel{Label: *l, Scope: "org"})
	}
	return out, nil
}

// GetRepoLabel returns one repository label by its ID.
func (s *Service) GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*model.Label, error) {
	label, err := s.port.GetRepoLabel(ctx, owner, repo, id)
	if err != nil {
		return nil, fmt.Errorf("get repo label: %w", err)
	}
	return label, nil
}

// CreateRepoLabel creates a repository label. The colour is checked and
// normalised to #rrggbb before anything is sent.
func (s *Service) CreateRepoLabel(ctx context.Context, owner, repo string, in LabelNew) (*model.Label, error) {
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
func (s *Service) EditRepoLabel(ctx context.Context, owner, repo string, id int64, in LabelEdit) (*model.Label, error) {
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
		var nf *NotFoundError
		if errors.As(err, &nf) {
			return fmt.Errorf("label %d not found", id)
		}
		return fmt.Errorf("delete repo label: %w", err)
	}
	return nil
}

// GetOrgLabel returns one organization label by its ID.
func (s *Service) GetOrgLabel(ctx context.Context, org string, id int64) (*model.Label, error) {
	label, err := s.port.GetOrgLabel(ctx, org, id)
	if err != nil {
		return nil, fmt.Errorf("get org label: %w", err)
	}
	return label, nil
}

// CreateOrgLabel creates an organization label.
func (s *Service) CreateOrgLabel(ctx context.Context, org string, in LabelNew) (*model.Label, error) {
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
func (s *Service) EditOrgLabel(ctx context.Context, org string, id int64, in LabelEdit) (*model.Label, error) {
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
// count is best-effort and may under-count repositories the token cannot read.
func (s *Service) DeleteOrgLabel(ctx context.Context, org string, id int64, force bool) error {
	if !force {
		label, err := s.port.GetOrgLabel(ctx, org, id)
		if err != nil {
			return fmt.Errorf("get org label: %w", err)
		}
		count, cerr := s.orgLabelUsage(ctx, org, label.Name)
		if cerr != nil {
			return fmt.Errorf("label %q in-use count failed (token may lack org-repo access); use delete_mode=force to override: %w", label.Name, cerr)
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
	ids, err := s.resolveLabels(ctx, t.Owner, t.Repo, labels)
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
	ids, err := s.resolveLabels(ctx, t.Owner, t.Repo, labels)
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

// resolveLabels turns a comma-separated list of label names or numeric IDs
// into the label IDs every Forgejo issue-label endpoint takes.
//
// Forgejo does accept names on the assignment endpoints, but it drops the ones
// it does not recognise and still answers 200. Half an applied request
// reported as success is worse for an agent than a failure, so names are
// resolved here and an unresolved token aborts the call before any write.
//
// A token is matched as a name first and only then as an ID, so a label
// literally named "123" stays reachable through an argument documented to take
// names. An ID is honoured only when the catalogue has it, which turns a stale
// ID into the same clear error as a stale name.
func (s *Service) resolveLabels(ctx context.Context, owner, repo, csv string) ([]int64, error) {
	tokens := split.CSV(csv)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("labels names nothing: pass at least one label name or numeric ID")
	}
	catalog, err := s.assignableLabels(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	byName := make(map[string][]model.ScopedLabel, len(catalog))
	byID := make(map[int64]model.ScopedLabel, len(catalog))
	for _, l := range catalog {
		byName[l.Name] = append(byName[l.Name], l)
		byID[l.ID] = l
	}

	ids := make([]int64, 0, len(tokens))
	seen := make(map[int64]bool, len(tokens))
	for _, token := range tokens {
		id, err := resolveOneLabel(token, byName, byID)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

// assignableLabels returns every label that can be applied to an issue in the
// repository: the repository's own labels plus, for an organization-owned
// repository, the organization's. Each scope is paged on its own.
func (s *Service) assignableLabels(ctx context.Context, owner, repo string) ([]model.ScopedLabel, error) {
	catalog := []model.ScopedLabel{}
	for page := 1; ; page++ {
		if page > maxLabelCatalogPages {
			return nil, fmt.Errorf(
				"%s/%s has more than %d labels; pass numeric label IDs instead of names",
				owner, repo, maxLabelCatalogPages*labelCatalogPageSize,
			)
		}
		labels, err := s.port.ListRepoLabels(ctx, owner, repo, Paging{Page: page, Limit: labelCatalogPageSize})
		if err != nil {
			return nil, fmt.Errorf("list repo labels err: %w", err)
		}
		for _, l := range labels {
			catalog = append(catalog, model.ScopedLabel{Label: *l, Scope: "repo"})
		}
		if len(labels) < labelCatalogPageSize {
			break
		}
	}
	for page := 1; ; page++ {
		if page > maxLabelCatalogPages {
			return nil, fmt.Errorf(
				"organization %s has more than %d labels; pass numeric label IDs instead of names",
				owner, maxLabelCatalogPages*labelCatalogPageSize,
			)
		}
		orgLabels, err := s.OrgLabels(ctx, owner, Paging{Page: page, Limit: labelCatalogPageSize})
		if err != nil {
			return nil, err
		}
		catalog = append(catalog, orgLabels...)
		if len(orgLabels) < labelCatalogPageSize {
			break
		}
	}
	return catalog, nil
}

func resolveOneLabel(token string, byName map[string][]model.ScopedLabel, byID map[int64]model.ScopedLabel) (int64, error) {
	switch matches := byName[token]; len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		// Fall through to the numeric form below.
	default:
		return 0, fmt.Errorf(
			"label %q is ambiguous: it matches %s; pass the numeric label ID instead",
			token, describeLabelScopes(matches),
		)
	}

	if id, err := strconv.ParseInt(token, 10, 64); err == nil {
		if _, ok := byID[id]; ok {
			return id, nil
		}
		return 0, fmt.Errorf(
			"label %q is neither a label name nor the ID of a label on this issue's repository or organization; list_repo_labels shows what exists",
			token,
		)
	}

	return 0, fmt.Errorf(
		"unknown label %q: no repository or organization label has that name; list_repo_labels shows what exists, create_repo_label adds one",
		token,
	)
}

// describeLabelScopes renders the colliding labels as "repository label 7 and
// organization label 12", so the caller can pick an ID without a second call.
func describeLabelScopes(matches []model.ScopedLabel) string {
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		scope := m.Scope
		switch scope {
		case "repo":
			scope = "repository"
		case "org":
			scope = "organization"
		}
		parts = append(parts, fmt.Sprintf("%s label %d", scope, m.ID))
	}
	sort.Strings(parts)
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts, ", ")
}

// labelUsage counts the issues and pull requests of a repository that carry the
// label. An unreported total counts as none.
func (s *Service) labelUsage(ctx context.Context, owner, repo, label string) (int, error) {
	total, err := s.port.CountIssuesLabelled(ctx, owner, repo, label)
	if err != nil {
		return 0, fmt.Errorf("count label usage: %w", err)
	}
	if total.Known {
		return total.Count, nil
	}
	return 0, nil
}

// orgLabelUsage counts, best-effort, the issues and pull requests in an
// organization's visible repositories that carry the label. A repository whose
// count fails adds nothing; a failure to list the repositories is returned with
// the count so far.
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
			n, _ := s.labelUsage(ctx, org, name, label)
			total += n
		}
	}
	return total, nil
}

// labelEdit checks that an edit supplies something, and turns its fields into
// the edit the port takes: an empty name is left alone, a colour is normalised,
// and a description, even an empty one, is set.
func labelEdit(in LabelEdit) (LabelEdit, error) {
	if in.Name == nil && in.Color == nil && in.Description == nil {
		return LabelEdit{}, fmt.Errorf("%s: at least one of name, color, description must be provided", in.Tool)
	}
	out := LabelEdit{Tool: in.Tool}
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
