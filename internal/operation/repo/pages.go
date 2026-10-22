// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// The helpers below render a bounded list from its use case as the response
// its tool has always returned. The rows and the continuation metadata both
// come from the list.Bounded.

func branchPage(b list.Bounded[*model.Branch], err error) (model.BranchPage, error) {
	if err != nil {
		return model.BranchPage{}, err
	}
	return model.BranchPage{Branches: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func commitListPage(b list.Bounded[*model.Commit], err error) (model.CommitPage, error) {
	if err != nil {
		return model.CommitPage{}, err
	}
	return model.CommitPage{Commits: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

// contentsPage renders one page of a directory level. Forgejo does not report
// the level's count, so the total is not shown; an empty page is an empty list.
func contentsPage(b list.Bounded[*model.ContentsResponse], err error) (model.ContentsPage, error) {
	if err != nil {
		return model.ContentsPage{}, err
	}
	rows := b.Items
	if rows == nil {
		rows = []*model.ContentsResponse{}
	}
	meta := model.PageMetaOf(b)
	meta.TotalCount = nil
	return model.ContentsPage{Contents: rows, PageMeta: meta}, nil
}

// treePage renders one page of a Git tree's entries; an empty page is an empty
// list.
func treePage(b list.Bounded[model.GitEntry], err error) (model.TreePage, error) {
	if err != nil {
		return model.TreePage{}, err
	}
	tree := b.Items
	if tree == nil {
		tree = []model.GitEntry{}
	}
	return model.TreePage{Tree: tree, PageMeta: model.PageMetaOf(b)}, nil
}

// topicPage renders one page of a repository's topics; an empty page is an
// empty list.
func topicPage(b list.Bounded[string], err error) (model.TopicPage, error) {
	if err != nil {
		return model.TopicPage{}, err
	}
	topics := b.Items
	if topics == nil {
		topics = []string{}
	}
	return model.TopicPage{Topics: topics, PageMeta: model.PageMetaOf(b)}, nil
}

// repoRows is the bare array list_my_repos has always returned.
func repoRows(b list.Bounded[*model.Repository], err error) ([]*model.Repository, error) {
	if err != nil {
		return nil, err
	}
	return b.Items, nil
}
