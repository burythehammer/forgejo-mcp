// SPDX-License-Identifier: GPL-3.0-or-later

package model

// BranchPage is the response of list_branches: one page of branches with the
// continuation signal of docs/design/output-bounding.md.
type BranchPage struct {
	Branches []*Branch `json:"branches"`
	PageMeta
}

// ContentsPage is the response of list_repo_contents: one page of the entries
// of one directory level.
type ContentsPage struct {
	Contents []*ContentsResponse `json:"contents"`
	PageMeta
}

// CommitPage is the response of list_repo_commits: one page of commits.
type CommitPage struct {
	Commits []*Commit `json:"commits"`
	PageMeta
}
