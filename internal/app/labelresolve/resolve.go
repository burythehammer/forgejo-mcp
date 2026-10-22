// SPDX-License-Identifier: GPL-3.0-or-later

// Package labelresolve turns the label names and IDs a caller gives a tool into
// the label IDs Forgejo's issue-label endpoints take. Issues and pull requests
// share it: Forgejo labels both through the same endpoints, from the same
// catalogue of a repository's labels and its organization's.
//
// It is MCP-free and SDK-free like all of internal/app/. It reaches Forgejo only through
// Catalog, which the owning use case adapts from its own port.
package labelresolve

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/split"
)

const (
	// catalogPageSize is the page size used while enumerating labels for
	// name resolution. It is independent of the caller-facing page and limit of
	// list_repo_labels: that tool browses, this enumeration must be complete,
	// or a label on page two would resolve as "unknown".
	catalogPageSize = 100

	// maxCatalogLabels caps the labels enumerated per scope. A server that
	// ignores paging would otherwise be read forever; the cap turns that into a
	// message, and numeric IDs still work without a catalogue. It counts labels,
	// not pages: a low instance ceiling needs many pages for a catalogue that is
	// well within this cap.
	maxCatalogLabels = 2000
)

// ErrOrgNotFound is what a Catalog reports for an organization Forgejo does not
// know. A repository owner that is a user yields it, and the resolver then reads
// the owner as having no organization labels.
var ErrOrgNotFound = errors.New("organization not found")

// Paging selects one page of a label list.
type Paging struct{ Page, Limit int }

// Catalog lists the labels a repository and its organization carry. The
// use case that owns the Forgejo port adapts its reads to this shape.
type Catalog interface {
	ListRepoLabels(ctx context.Context, owner, repo string, p Paging) ([]*model.CatalogLabel, error)
	ListOrgLabels(ctx context.Context, org string, p Paging) ([]*model.CatalogLabel, error)
	// MaxResponseItems is the instance's pagination ceiling; ok is false when
	// it could not be determined.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
}

// Resolver resolves label names and IDs against a Catalog.
type Resolver struct{ catalog Catalog }

// New returns a Resolver that reads its catalogue from c.
func New(c Catalog) *Resolver { return &Resolver{catalog: c} }

// Resolve turns a comma-separated list of label names or numeric IDs into the
// label IDs every Forgejo issue-label endpoint takes. Duplicates collapse.
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
func (r *Resolver) Resolve(ctx context.Context, owner, repo, csv string) ([]int64, error) {
	labels, err := r.ResolveLabels(ctx, owner, repo, csv)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(labels))
	for i, l := range labels {
		ids[i] = l.ID
	}
	return ids, nil
}

// ResolveLabels is Resolve for callers that need each label's scope and
// exclusive flag, not only its ID. Duplicates collapse, in request order.
func (r *Resolver) ResolveLabels(ctx context.Context, owner, repo, csv string) ([]model.ScopedLabel, error) {
	tokens := split.CSV(csv)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("labels names nothing: pass at least one label name or numeric ID")
	}
	catalog, err := r.assignable(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	byName := make(map[string][]model.ScopedLabel, len(catalog))
	byID := make(map[int64]model.ScopedLabel, len(catalog))
	for _, l := range catalog {
		byName[l.Name] = append(byName[l.Name], l)
		byID[l.ID] = l
	}

	labels := make([]model.ScopedLabel, 0, len(tokens))
	seen := make(map[int64]bool, len(tokens))
	for _, token := range tokens {
		l, err := resolveOne(token, byName, byID)
		if err != nil {
			return nil, err
		}
		if seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		labels = append(labels, l)
	}
	return labels, nil
}

// assignable returns every label that can be applied to an issue or pull
// request in the repository: the repository's own labels plus, for an
// organization-owned repository, the organization's. Each scope is enumerated
// on its own.
func (r *Resolver) assignable(ctx context.Context, owner, repo string) ([]model.ScopedLabel, error) {
	fullPage := r.fullPage(ctx)
	repoLabels, err := enumerate(fmt.Sprintf("%s/%s", owner, repo), "repo", "repo", fullPage,
		func(p Paging) ([]*model.CatalogLabel, error) {
			return r.catalog.ListRepoLabels(ctx, owner, repo, p)
		})
	if err != nil {
		return nil, err
	}
	orgLabels, err := enumerate("organization "+owner, "org", "org", fullPage,
		func(p Paging) ([]*model.CatalogLabel, error) {
			return r.catalog.ListOrgLabels(ctx, owner, p)
		})
	// The owner may be a user, which has no organization labels. The repository's
	// own labels were read first and proved the repository exists.
	if errors.Is(err, ErrOrgNotFound) {
		orgLabels, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append(repoLabels, orgLabels...), nil
}

// fullPage is how many labels a full page holds. The request asks for
// catalogPageSize, but the server clamps a larger limit to the instance ceiling
// (max_response_items), so a page that is full at the ceiling is not short.
// An unknown ceiling leaves the page at catalogPageSize.
func (r *Resolver) fullPage(ctx context.Context) int {
	ceiling, ok := r.catalog.MaxResponseItems(ctx)
	if ok && ceiling >= 1 && ceiling < catalogPageSize {
		return ceiling
	}
	return catalogPageSize
}

// enumerate reads every page of one scope's labels and tags each with scope.
// Paging stops at the first page shorter than fullPage, so it needs no probe
// request past the end. what names the scope in the cap's error, and kind
// names it in the fetch error.
func enumerate(what, scope, kind string, fullPage int, fetch func(Paging) ([]*model.CatalogLabel, error)) ([]model.ScopedLabel, error) {
	var out []model.ScopedLabel
	for page := 1; ; page++ {
		labels, err := fetch(Paging{Page: page, Limit: catalogPageSize})
		if err != nil {
			return nil, fmt.Errorf("list %s labels err: %w", kind, err)
		}
		for _, l := range labels {
			out = append(out, model.ScopedLabel{CatalogLabel: *l, Scope: scope})
		}
		if len(out) > maxCatalogLabels {
			return nil, fmt.Errorf(
				"%s has more than %d labels; pass numeric label IDs instead of names",
				what, maxCatalogLabels,
			)
		}
		if len(labels) < fullPage {
			return out, nil
		}
	}
}

func resolveOne(token string, byName map[string][]model.ScopedLabel, byID map[int64]model.ScopedLabel) (model.ScopedLabel, error) {
	switch matches := byName[token]; len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		// Fall through to the numeric form below.
	default:
		return model.ScopedLabel{}, fmt.Errorf(
			"label %q is ambiguous: it matches %s; pass the numeric label ID instead",
			token, describeScopes(matches),
		)
	}

	if id, err := strconv.ParseInt(token, 10, 64); err == nil {
		if l, ok := byID[id]; ok {
			return l, nil
		}
		return model.ScopedLabel{}, fmt.Errorf(
			"label %q is neither a label name nor the ID of a label on this issue's repository or organization; list_repo_labels shows what exists",
			token,
		)
	}

	return model.ScopedLabel{}, fmt.Errorf(
		"unknown label %q: no repository or organization label has that name; list_repo_labels shows what exists, create_repo_label adds one",
		token,
	)
}

// describeScopes renders the colliding labels as "repository label 7 and
// organization label 12", so the caller can pick an ID without a second call.
func describeScopes(matches []model.ScopedLabel) string {
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
