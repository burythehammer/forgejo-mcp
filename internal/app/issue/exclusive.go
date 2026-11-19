// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// resolveApplied resolves the labels a write applies to an issue and refuses a
// request that names two exclusive labels from one scope. Create, update and
// add all go through it, so the rule is one function.
func (s *Service) resolveApplied(ctx context.Context, owner, repo, csv string) ([]int64, error) {
	labels, err := s.labels.ResolveLabels(ctx, owner, repo, csv)
	if err != nil {
		return nil, err
	}
	if err := refuseExclusiveClash(labels); err != nil {
		return nil, err
	}
	ids := make([]int64, len(labels))
	for i, l := range labels {
		ids[i] = l.ID
	}
	return ids, nil
}

// refuseExclusiveClash returns an ExclusiveConflictError when two of labels are
// exclusive and share a scope. Only the labels given are compared, so one
// exclusive label may still be added on top of one the issue already has.
func refuseExclusiveClash(labels []model.ScopedLabel) error {
	first := make(map[string]string)
	for _, l := range labels {
		scope := scopeOf(l.Name)
		if !l.Exclusive || scope == "" {
			continue
		}
		if prev, ok := first[scope]; ok {
			return &ExclusiveConflictError{Scope: scope, First: prev, Next: l.Name}
		}
		first[scope] = l.Name
	}
	return nil
}

// scopeOf is the part of a label name before its last "/", or "" for a name
// with no scope.
func scopeOf(name string) string {
	i := strings.LastIndex(name, "/")
	if i <= 0 {
		return ""
	}
	return name[:i]
}
