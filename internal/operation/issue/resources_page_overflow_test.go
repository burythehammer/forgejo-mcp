// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
)

// A huge page on a thread where the server ignores paging must return an
// empty page and must not panic. (page-1)*limit wraps negative for these
// values, and slicing with that offset used to panic.
func TestIssueCommentsResource_HugePageDoesNotPanic(t *testing.T) {
	t.Parallel()
	for _, q := range []string{
		"?limit=2&page=2305843009213693953",
		"?limit=4&page=2305843009213693953",
		"?limit=8&page=1152921504606846977",
		"?limit=2&page=9223372036854775807",
	} {
		t.Run(q, func(t *testing.T) {
			t.Parallel()
			f := fake.New()
			f.AddRepo(testOwner, testRepo)
			idx := openIssue(t, f, "t")
			for i := 0; i < 7; i++ {
				comment(t, f, idx, fmt.Sprint("c", i))
			}
			h := withPort(boundsIgnoring{f})
			var p commentsListPayload
			readJSON(t, h.issueCommentsResourceHandler, commentsURI("issue", idx, q), &p)
			if len(p.Comments) != 0 || p.Truncated {
				t.Fatalf("want an empty page, got %+v", p)
			}
		})
	}
}
