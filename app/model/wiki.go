// SPDX-License-Identifier: GPL-3.0-or-later

package model

// The wiki data types are the port's plain data: page content travels
// base64-encoded, as Forgejo carries it, and decoding is the use case's job.

// WikiPerson is a wiki commit's author.
type WikiPerson struct {
	Name  string
	Email string
	Date  string
}

// WikiCommit is one commit of a wiki page's history.
type WikiCommit struct {
	SHA     string
	Author  WikiPerson
	Message string
}

// WikiPageMeta is a wiki page without its content.
type WikiPageMeta struct {
	Title      string
	HTMLURL    string
	SubURL     string
	LastCommit WikiCommit
}

// WikiPage is a wiki page with its content.
type WikiPage struct {
	WikiPageMeta
	ContentBase64 string
	CommitCount   int
	Sidebar       string
	Footer        string
}

// WikiPageList is one page of a wiki's pages. TotalCount is nil when Forgejo
// did not report one.
type WikiPageList struct {
	Pages      []WikiPageMeta
	TotalCount *int
}

// WikiRevisions is one page of a wiki page's history. Count is the page's
// total number of revisions.
type WikiRevisions struct {
	Commits []WikiCommit
	Count   int
}

// WikiPageSummary is a wiki page as the list tool shows it.
type WikiPageSummary struct {
	Title    string `json:"title"`
	PageName string `json:"page_name"`
	SubURL   string `json:"sub_url"`
}

// WikiPageListResult is one page of the list tool's output.
type WikiPageListResult struct {
	Pages []WikiPageSummary `json:"pages"`
	PageMeta
}

// WikiPageView is the get tool's output: a page's content, optionally cut to a
// line range.
type WikiPageView struct {
	Title      string `json:"title"`
	PageName   string `json:"page_name"`
	Content    string `json:"content"`
	CommitSHA  string `json:"commit_sha"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
}

// WikiRevisionSummary is one revision as the revisions tool shows it.
type WikiRevisionSummary struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Message string `json:"message"`
}

// WikiRevisionsResult is one page of the revisions tool's output.
type WikiRevisionsResult struct {
	Revisions  []WikiRevisionSummary `json:"revisions"`
	Page       int                   `json:"page"`
	HasNext    bool                  `json:"has_next"`
	TotalCount int                   `json:"total_count"`
}

// WikiWriteResult is what the create and update tools return.
type WikiWriteResult struct {
	Title     string `json:"title"`
	PageName  string `json:"page_name"`
	CommitSHA string `json:"commit_sha"`
}

// WikiDeleteResult is what the delete tool returns.
type WikiDeleteResult struct {
	Deleted  bool   `json:"deleted"`
	PageName string `json:"page_name"`
}
