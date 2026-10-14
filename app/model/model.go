// SPDX-License-Identifier: GPL-3.0-or-later

// Package model holds repo-owned domain types that use cases return. They
// mirror the Forgejo API wire shapes field for field (same JSON names, same
// order) so a tool renders the same bytes it did when it returned SDK types;
// named SDK enums (state, access mode, ...) are plain strings here. Adapters
// map SDK values onto them (see pkg/forge/pulladapter). Like all of app/,
// this package must not import mcp-go or the Forgejo SDK.
package model

import "time"

type ChangedFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	HTMLURL          string `json:"html_url"`
	ContentsURL      string `json:"contents_url"`
	RawURL           string `json:"raw_url"`
}

type ExternalTracker struct {
	ExternalTrackerURL          string `json:"external_tracker_url"`
	ExternalTrackerFormat       string `json:"external_tracker_format"`
	ExternalTrackerStyle        string `json:"external_tracker_style"`
	ExternalTrackerRegexPattern string `json:"external_tracker_regexp_pattern"`
}

type ExternalWiki struct {
	ExternalWikiURL string `json:"external_wiki_url"`
}

type InternalTracker struct {
	EnableTimeTracker                bool `json:"enable_time_tracker"`
	AllowOnlyContributorsToTrackTime bool `json:"allow_only_contributors_to_track_time"`
	EnableIssueDependencies          bool `json:"enable_issue_dependencies"`
}

type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type Milestone struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	State        string     `json:"state"`
	OpenIssues   int        `json:"open_issues"`
	ClosedIssues int        `json:"closed_issues"`
	Created      time.Time  `json:"created_at"`
	Updated      *time.Time `json:"updated_at"`
	Closed       *time.Time `json:"closed_at"`
	Deadline     *time.Time `json:"due_on"`
}

type Organization struct {
	ID          int64  `json:"id"`
	UserName    string `json:"username"`
	FullName    string `json:"full_name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	Visibility  string `json:"visibility"`
}

type PRBranchInfo struct {
	Name       string      `json:"label"`
	Ref        string      `json:"ref"`
	Sha        string      `json:"sha"`
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `json:"repo"`
}

type Permission struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

type PullRequest struct {
	ID                  int64         `json:"id"`
	URL                 string        `json:"url"`
	Index               int64         `json:"number"`
	Poster              *User         `json:"user"`
	Title               string        `json:"title"`
	Body                string        `json:"body"`
	Labels              []*Label      `json:"labels"`
	Milestone           *Milestone    `json:"milestone"`
	Assignee            *User         `json:"assignee"`
	Assignees           []*User       `json:"assignees"`
	State               string        `json:"state"`
	IsLocked            bool          `json:"is_locked"`
	Comments            int           `json:"comments"`
	HTMLURL             string        `json:"html_url"`
	DiffURL             string        `json:"diff_url"`
	PatchURL            string        `json:"patch_url"`
	Mergeable           bool          `json:"mergeable"`
	HasMerged           bool          `json:"merged"`
	Merged              *time.Time    `json:"merged_at"`
	MergedCommitID      *string       `json:"merge_commit_sha"`
	MergedBy            *User         `json:"merged_by"`
	AllowMaintainerEdit bool          `json:"allow_maintainer_edit"`
	Base                *PRBranchInfo `json:"base"`
	Head                *PRBranchInfo `json:"head"`
	MergeBase           string        `json:"merge_base"`
	Deadline            *time.Time    `json:"due_date"`
	Created             *time.Time    `json:"created_at"`
	Updated             *time.Time    `json:"updated_at"`
	Closed              *time.Time    `json:"closed_at"`
}

type PullReview struct {
	ID                int64     `json:"id"`
	Reviewer          *User     `json:"user"`
	ReviewerTeam      *Team     `json:"team"`
	State             string    `json:"state"`
	Body              string    `json:"body"`
	CommitID          string    `json:"commit_id"`
	Stale             bool      `json:"stale"`
	Official          bool      `json:"official"`
	Dismissed         bool      `json:"dismissed"`
	CodeCommentsCount int       `json:"comments_count"`
	Submitted         time.Time `json:"submitted_at"`
	HTMLURL           string    `json:"html_url"`
	HTMLPullURL       string    `json:"pull_request_url"`
}

type PullReviewComment struct {
	ID           int64     `json:"id"`
	Body         string    `json:"body"`
	Reviewer     *User     `json:"user"`
	ReviewID     int64     `json:"pull_request_review_id"`
	Resolver     *User     `json:"resolver"`
	Created      time.Time `json:"created_at"`
	Updated      time.Time `json:"updated_at"`
	Path         string    `json:"path"`
	CommitID     string    `json:"commit_id"`
	OrigCommitID string    `json:"original_commit_id"`
	DiffHunk     string    `json:"diff_hunk"`
	LineNum      uint64    `json:"position"`
	OldLineNum   uint64    `json:"original_position"`
	HTMLURL      string    `json:"html_url"`
	HTMLPullURL  string    `json:"pull_request_url"`
}

type Repository struct {
	ID                        int64            `json:"id"`
	Owner                     *User            `json:"owner"`
	Name                      string           `json:"name"`
	FullName                  string           `json:"full_name"`
	Description               string           `json:"description"`
	Empty                     bool             `json:"empty"`
	Private                   bool             `json:"private"`
	Fork                      bool             `json:"fork"`
	Template                  bool             `json:"template"`
	Parent                    *Repository      `json:"parent"`
	Mirror                    bool             `json:"mirror"`
	Size                      int              `json:"size"`
	HTMLURL                   string           `json:"html_url"`
	SSHURL                    string           `json:"ssh_url"`
	CloneURL                  string           `json:"clone_url"`
	OriginalURL               string           `json:"original_url"`
	Website                   string           `json:"website"`
	Stars                     int              `json:"stars_count"`
	Forks                     int              `json:"forks_count"`
	Watchers                  int              `json:"watchers_count"`
	OpenIssues                int              `json:"open_issues_count"`
	OpenPulls                 int              `json:"open_pr_counter"`
	Releases                  int              `json:"release_counter"`
	DefaultBranch             string           `json:"default_branch"`
	Archived                  bool             `json:"archived"`
	Created                   time.Time        `json:"created_at"`
	Updated                   time.Time        `json:"updated_at"`
	Permissions               *Permission      `json:"permissions,omitempty"`
	HasIssues                 bool             `json:"has_issues"`
	InternalTracker           *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker           *ExternalTracker `json:"external_tracker,omitempty"`
	HasWiki                   bool             `json:"has_wiki"`
	ExternalWiki              *ExternalWiki    `json:"external_wiki,omitempty"`
	HasPullRequests           bool             `json:"has_pull_requests"`
	HasProjects               bool             `json:"has_projects"`
	HasReleases               bool             `json:"has_releases,omitempty"`
	HasPackages               bool             `json:"has_packages,omitempty"`
	HasActions                bool             `json:"has_actions,omitempty"`
	IgnoreWhitespaceConflicts bool             `json:"ignore_whitespace_conflicts"`
	AllowMerge                bool             `json:"allow_merge_commits"`
	AllowRebase               bool             `json:"allow_rebase"`
	AllowRebaseMerge          bool             `json:"allow_rebase_explicit"`
	AllowSquash               bool             `json:"allow_squash_merge"`
	AvatarURL                 string           `json:"avatar_url"`
	Internal                  bool             `json:"internal"`
	MirrorInterval            string           `json:"mirror_interval"`
	MirrorUpdated             time.Time        `json:"mirror_updated,omitempty"`
	DefaultMergeStyle         string           `json:"default_merge_style"`
}

type Team struct {
	ID                      int64             `json:"id"`
	Name                    string            `json:"name"`
	Description             string            `json:"description"`
	Organization            *Organization     `json:"organization"`
	Permission              string            `json:"permission"`
	CanCreateOrgRepo        bool              `json:"can_create_org_repo"`
	IncludesAllRepositories bool              `json:"includes_all_repositories"`
	UnitsMap                map[string]string `json:"units_map"`
	Units                   []string          `json:"units"`
}

type User struct {
	ID               int64     `json:"id"`
	UserName         string    `json:"login"`
	LoginName        string    `json:"login_name"`
	SourceID         int64     `json:"source_id"`
	FullName         string    `json:"full_name"`
	Email            string    `json:"email"`
	HTMLURL          string    `json:"html_url"`
	AvatarURL        string    `json:"avatar_url"`
	Language         string    `json:"language"`
	IsAdmin          bool      `json:"is_admin"`
	LastLogin        time.Time `json:"last_login"`
	Created          time.Time `json:"created"`
	Restricted       bool      `json:"restricted"`
	IsActive         bool      `json:"active"`
	ProhibitLogin    bool      `json:"prohibit_login"`
	Location         string    `json:"location"`
	Website          string    `json:"website"`
	Description      string    `json:"description"`
	Visibility       string    `json:"visibility"`
	FollowerCount    int       `json:"followers_count"`
	FollowingCount   int       `json:"following_count"`
	StarredRepoCount int       `json:"starred_repos_count"`
}

// ScopedLabel is a label listed for an issue, tagged with where it lives:
// "repo" for the repository's own labels, "org" for its organization's.
type ScopedLabel struct {
	Label
	Scope string `json:"scope"`
}

// PullRequestMeta is the pull-request link an issue carries when the issue is
// a pull request.
type PullRequestMeta struct {
	HasMerged bool       `json:"merged"`
	Merged    *time.Time `json:"merged_at"`
}

// RepositoryMeta is the repository an issue belongs to, as an issue carries it.
type RepositoryMeta struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

type Issue struct {
	ID               int64            `json:"id"`
	URL              string           `json:"url"`
	HTMLURL          string           `json:"html_url"`
	Index            int64            `json:"number"`
	Poster           *User            `json:"user"`
	OriginalAuthor   string           `json:"original_author"`
	OriginalAuthorID int64            `json:"original_author_id"`
	Title            string           `json:"title"`
	Body             string           `json:"body"`
	Ref              string           `json:"ref"`
	Labels           []*Label         `json:"labels"`
	Milestone        *Milestone       `json:"milestone"`
	Assignees        []*User          `json:"assignees"`
	State            string           `json:"state"`
	IsLocked         bool             `json:"is_locked"`
	Comments         int              `json:"comments"`
	Created          time.Time        `json:"created_at"`
	Updated          time.Time        `json:"updated_at"`
	Closed           *time.Time       `json:"closed_at"`
	Deadline         *time.Time       `json:"due_date"`
	PullRequest      *PullRequestMeta `json:"pull_request"`
	Repository       *RepositoryMeta  `json:"repository"`
}

type Comment struct {
	ID               int64     `json:"id"`
	HTMLURL          string    `json:"html_url"`
	PRURL            string    `json:"pull_request_url"`
	IssueURL         string    `json:"issue_url"`
	Poster           *User     `json:"user"`
	OriginalAuthor   string    `json:"original_author"`
	OriginalAuthorID int64     `json:"original_author_id"`
	Body             string    `json:"body"`
	Created          time.Time `json:"created_at"`
	Updated          time.Time `json:"updated_at"`
}

// IssueSearchPage is the response of search_issues: the issues with the
// continuation signal of docs/design/output-bounding.md.
type IssueSearchPage struct {
	Issues []*Issue `json:"issues"`
	PageMeta
}
