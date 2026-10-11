// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// The types below are the branch, commit, contents and file results of the
// repo tools. Like the rest of this package they mirror the Forgejo wire shapes
// field for field, with the same JSON names and order, so a tool renders the
// bytes it rendered when it returned SDK types.

// PayloadUser is the author or committer of a commit as a push payload names
// them.
type PayloadUser struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserName string `json:"username"`
}

// PayloadCommitVerification is the GPG verification of a commit.
type PayloadCommitVerification struct {
	Verified  bool   `json:"verified"`
	Reason    string `json:"reason"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
}

// PayloadCommit is a commit as a branch carries it.
type PayloadCommit struct {
	ID           string                     `json:"id"`
	Message      string                     `json:"message"`
	URL          string                     `json:"url"`
	Author       *PayloadUser               `json:"author"`
	Committer    *PayloadUser               `json:"committer"`
	Verification *PayloadCommitVerification `json:"verification"`
	Timestamp    time.Time                  `json:"timestamp"`
	Added        []string                   `json:"added"`
	Removed      []string                   `json:"removed"`
	Modified     []string                   `json:"modified"`
}

// Branch is a repository branch.
type Branch struct {
	Name                          string         `json:"name"`
	Commit                        *PayloadCommit `json:"commit"`
	Protected                     bool           `json:"protected"`
	RequiredApprovals             int64          `json:"required_approvals"`
	EnableStatusCheck             bool           `json:"enable_status_check"`
	StatusCheckContexts           []string       `json:"status_check_contexts"`
	UserCanPush                   bool           `json:"user_can_push"`
	UserCanMerge                  bool           `json:"user_can_merge"`
	EffectiveBranchProtectionName string         `json:"effective_branch_protection_name"`
}

// CommitMeta names a commit by URL and SHA, with when it was created.
type CommitMeta struct {
	URL     string    `json:"url"`
	SHA     string    `json:"sha"`
	Created time.Time `json:"created"`
}

// Identity is a name and email, as Git records an author or committer.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CommitUser is a commit's author or committer.
type CommitUser struct {
	Identity
	Date string `json:"date"`
}

// RepoCommit is a commit in the context of a repository.
type RepoCommit struct {
	URL          string                     `json:"url"`
	Author       *CommitUser                `json:"author"`
	Committer    *CommitUser                `json:"committer"`
	Message      string                     `json:"message"`
	Tree         *CommitMeta                `json:"tree"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// CommitStats is the size of a commit's change.
type CommitStats struct {
	Total     int `json:"total"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// CommitAffectedFiles is one file a commit touches.
type CommitAffectedFiles struct {
	Filename string `json:"filename"`
}

// Commit is a commit as the commit listing returns it.
type Commit struct {
	*CommitMeta
	HTMLURL    string                 `json:"html_url"`
	RepoCommit *RepoCommit            `json:"commit"`
	Author     *User                  `json:"author"`
	Committer  *User                  `json:"committer"`
	Parents    []*CommitMeta          `json:"parents"`
	Files      []*CommitAffectedFiles `json:"files"`
	Stats      *CommitStats           `json:"stats"`
}

// FileLinksResponse holds the API, Git and HTML links of a repository entry.
type FileLinksResponse struct {
	Self    *string `json:"self"`
	GitURL  *string `json:"git"`
	HTMLURL *string `json:"html"`
}

// ContentsResponse is the metadata and content of a repository entry: a file,
// directory, symlink or submodule.
type ContentsResponse struct {
	Name            string             `json:"name"`
	Path            string             `json:"path"`
	SHA             string             `json:"sha"`
	Type            string             `json:"type"`
	Size            int64              `json:"size"`
	Encoding        *string            `json:"encoding"`
	Content         *string            `json:"content"`
	Target          *string            `json:"target"`
	URL             *string            `json:"url"`
	HTMLURL         *string            `json:"html_url"`
	GitURL          *string            `json:"git_url"`
	DownloadURL     *string            `json:"download_url"`
	SubmoduleGitURL *string            `json:"submodule_git_url"`
	Links           *FileLinksResponse `json:"_links"`
}

// FileCommitResponse is the commit a file change produced.
type FileCommitResponse struct {
	CommitMeta
	HTMLURL   string        `json:"html_url"`
	Author    *CommitUser   `json:"author"`
	Committer *CommitUser   `json:"committer"`
	Parents   []*CommitMeta `json:"parents"`
	Message   string        `json:"message"`
	Tree      *CommitMeta   `json:"tree"`
}

// FileResponse is the result of creating or updating a file: the entry, the
// commit that recorded it and its verification.
type FileResponse struct {
	Content      *ContentsResponse          `json:"content"`
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// GitEntry is one entry of a Git tree.
type GitEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitTreeResponse is a Git tree and the page of its entries that was asked for.
type GitTreeResponse struct {
	SHA        string     `json:"sha"`
	URL        string     `json:"url"`
	Entries    []GitEntry `json:"tree"`
	Truncated  bool       `json:"truncated"`
	Page       int        `json:"page"`
	TotalCount int        `json:"total_count"`
}
