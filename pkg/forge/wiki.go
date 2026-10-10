// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "context"

// Wiki is the wiki domain of the port. The Forgejo SDK has no wiki support, so
// these types are the port's own plain data, shaped like Forgejo's wiki JSON.
//
// Page content travels base64-encoded, as the API carries it; decoding is the
// tools' concern. Page names are the server-normalized SubURL; a name may be
// passed percent-encoded (Guides%2FSetup) or plain, and both address the same
// page. Failures from Forgejo are *StatusError.
type Wiki interface {
	// ListWikiPages returns one page of the wiki's pages. A repository with no
	// wiki, or no such repository, is an empty list rather than an error:
	// Forgejo answers 404 and the tools treat that as "no pages".
	ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (WikiPageList, error)
	GetWikiPage(ctx context.Context, owner, repo, pageName string) (*WikiPage, error)
	// GetWikiPageRevisions returns one page of a wiki page's history. Unlike
	// ListWikiPages, a page that does not exist is a not-found error.
	GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*WikiRevisions, error)
	CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*WikiPage, error)
	EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*WikiPage, error)
	DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error
}

// WikiPageList is one page of ListWikiPages.
type WikiPageList struct {
	Pages []WikiPageMeta
	// TotalCount is Forgejo's X-Total-Count, nil when it did not report one.
	// It counts raw wiki tree entries, so it can exceed len of all pages.
	TotalCount *int
}

type WikiPerson struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Date  string `json:"date,omitempty"`
}

type WikiCommit struct {
	SHA     string     `json:"sha"`
	Author  WikiPerson `json:"author"`
	Message string     `json:"message"`
}

type WikiPageMeta struct {
	Title      string     `json:"title"`
	HTMLURL    string     `json:"html_url,omitempty"`
	SubURL     string     `json:"sub_url"`
	LastCommit WikiCommit `json:"last_commit"`
}

type WikiPage struct {
	WikiPageMeta
	ContentBase64 string `json:"content_base64"`
	CommitCount   int    `json:"commit_count"`
	Sidebar       string `json:"sidebar,omitempty"`
	Footer        string `json:"footer,omitempty"`
}

// WikiRevisions is one page of a wiki page's history. Count is the page's
// total number of revisions, not the length of Commits.
type WikiRevisions struct {
	Commits []WikiCommit `json:"commits"`
	Count   int          `json:"count"`
}
