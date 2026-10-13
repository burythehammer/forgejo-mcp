// SPDX-License-Identifier: GPL-3.0-or-later

package model

// CommentPage is the response of list_issue_comments: one page of an issue's
// or pull request's comments with the continuation signal of
// docs/design/output-bounding.md.
type CommentPage struct {
	Comments []*Comment `json:"comments"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
	Count    int        `json:"count"`
	HasNext  bool       `json:"has_next"`
}

// DependencyPage is the response of list_issue_dependencies and
// list_issue_dependents: one page of issues with the continuation signal.
type DependencyPage struct {
	Issues  []*Issue `json:"issues"`
	Page    int      `json:"page"`
	Limit   int      `json:"limit"`
	Count   int      `json:"count"`
	HasNext bool     `json:"has_next"`
}

// MilestonePage is the response of list_repo_milestones: one page of
// milestones with the continuation signal.
type MilestonePage struct {
	Milestones []*Milestone `json:"milestones"`
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	Count      int          `json:"count"`
	HasNext    bool         `json:"has_next"`
}

// ScopedLabelPage is the response of list_repo_labels and list_org_labels: one
// page of labels, each tagged with its scope. TotalCount is the server's grand
// total of the paged catalogue and is omitted when the server did not report
// one. For a repository list merged with its organization's labels, Count
// includes the organization's labels, while HasNext and TotalCount describe
// only the repository's own rows.
type ScopedLabelPage struct {
	Labels     []ScopedLabel `json:"labels"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	Count      int           `json:"count"`
	HasNext    bool          `json:"has_next"`
	TotalCount *int          `json:"total_count,omitempty"`
}
