// SPDX-License-Identifier: GPL-3.0-or-later

package model

// BranchPage is the response of list_branches: one page of branches with the
// continuation signal of docs/design/output-bounding.md.
type BranchPage struct {
	Branches []*Branch `json:"branches"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
	Count    int       `json:"count"`
	HasNext  bool      `json:"has_next"`
}

// CommitPage is the response of list_repo_commits: one page of commits.
type CommitPage struct {
	Commits []*Commit `json:"commits"`
	Page    int       `json:"page"`
	Limit   int       `json:"limit"`
	Count   int       `json:"count"`
	HasNext bool      `json:"has_next"`
}
