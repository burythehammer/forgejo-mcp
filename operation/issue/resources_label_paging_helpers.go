// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Pagination helpers for the label resources, which still read the SDK
// response and raw headers directly. The labels ticket removes this file along
// with them.

// hasMore reports whether the server said another page exists. The SDK parses
// the response's `Link: …; rel="next"` header into Response.NextPage, so this
// is upstream's own answer rather than an inference from how many rows arrived
// — which is what lets these resources page without over-fetching.
func hasMore(resp *forgejo_sdk.Response) bool {
	return resp != nil && resp.NextPage != 0
}

// totalCount reads Forgejo's X-Total-Count header, the authoritative number of
// rows matching the query. Returns 0 when the header is absent or unparseable,
// which the sentinel renders as "total unknown" rather than guessing.
func totalCount(resp *forgejo_sdk.Response) int {
	if resp == nil || resp.Response == nil {
		return 0
	}
	return headerTotalCount(resp.Header)
}

// headerHasMore is hasMore for the raw-HTTP path, which has no SDK Response to
// parse the Link header for it. Same question, same answer: did the server
// advertise a next page.
func headerHasMore(h http.Header) bool {
	for _, link := range h.Values("Link") {
		// A single Link value may carry several comma-separated relations,
		// so this is a substring test rather than a parse. Forgejo quotes
		// the rel; tolerate the unquoted form too.
		if strings.Contains(link, `rel="next"`) || strings.Contains(link, "rel=next") {
			return true
		}
	}
	return false
}

// headerTotalCount is totalCount for a bare http.Header. Delegates to the
// shared pkg/forgejo.TotalCount parser so there is one X-Total-Count reader
// in the codebase, not several.
func headerTotalCount(h http.Header) int {
	n, _ := forgejo.TotalCount(h)
	return n
}
