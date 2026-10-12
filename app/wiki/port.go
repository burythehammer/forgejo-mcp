// SPDX-License-Identifier: GPL-3.0-or-later

// Package wiki holds the wiki use cases behind the wiki tools. Like all of app/
// it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh): use cases take
// typed inputs, reach Forgejo only through Port, and return repo-owned types
// from app/model for an MCP adapter in operation/wiki to render.
package wiki

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Port is the Forgejo wiki domain as the use cases need it. Page content is
// base64 in both directions, as Forgejo carries it; the use cases encode and
// decode it. pkg/forge/wikiadapter implements Port over forge.Wiki.
//
// A repository with no wiki is an empty page list, not an error: the adapter
// owns that mapping. Failures from Forgejo come back as ordinary errors for the
// use case to wrap.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (model.WikiPageList, error)
	GetWikiPage(ctx context.Context, owner, repo, pageName string) (*model.WikiPage, error)
	GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*model.WikiRevisions, error)
	CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*model.WikiPage, error)
	EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*model.WikiPage, error)
	DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error
}
