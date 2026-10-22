// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"strings"
	"testing"
)

// An index is a positive whole number. Anything else is refused with the one
// message the numeric-argument module owns, naming the argument and its range,
// instead of being truncated into a different issue (#246, under #228).
func TestGetIssueByIndex_RefusesInvalidNumbers(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "hello")

	const want = "index must be an integer between 1 and 9223372036854775807"
	for _, raw := range []any{1.5, 0.0, -3.0, 1e300, "abc", "1.5"} {
		if msg := callErr(t, h.GetIssueByIndexFn, map[string]any{"index": raw}); !strings.Contains(msg, want) {
			t.Errorf("index %v: error = %q, want it to contain %q", raw, msg, want)
		}
	}

	// A numeric string is a number on every tool, so it still resolves.
	iss := decodeIssue(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": "1"}))
	if iss.Index != idx {
		t.Fatalf("index \"1\": got issue %d, want %d", iss.Index, idx)
	}
}
