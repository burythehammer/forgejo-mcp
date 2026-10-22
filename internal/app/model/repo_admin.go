// SPDX-License-Identifier: GPL-3.0-or-later

package model

// RepositoryView is a repository with its topics, as get_repo renders it. The
// SDK's Repository has no topics field, so the topics follow the flattened
// repository fields.
type RepositoryView struct {
	*Repository
	Topics []string `json:"topics"`
}

// TopicPage is one page of a repository's topics, as list_repo_topics renders it.
type TopicPage struct {
	Topics []string `json:"topics"`
	PageMeta
}

// StatusItem is one per-context commit status. The commit status resource
// shares this shape.
type StatusItem struct {
	Context     string `json:"context"`
	State       string `json:"state"`
	TargetURL   string `json:"target_url,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// CommitStatuses is one page of a commit's statuses, as get_commit_statuses
// renders it. The continuation metadata is the standard page envelope.
type CommitStatuses struct {
	SHA      string       `json:"sha"`
	Statuses []StatusItem `json:"statuses"`
	PageMeta
}
