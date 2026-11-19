// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"strings"
	"testing"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/pull"
)

func TestDiffWithMarkerNamesTheNextPage(t *testing.T) {
	t.Parallel()
	got := diffWithMarker(apppull.DiffPage{
		Text: "diff --git a/a b/a\n+a", Page: 1, Limit: 1, Count: 1, Files: 3, HasNext: true,
	})
	for _, want := range []string{"+a", "files 1-1 of 3", "page=2", "limit=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q lacks %q", got, want)
		}
	}
}

func TestDiffWithMarkerLeavesAWholeDiffAlone(t *testing.T) {
	t.Parallel()
	const whole = "diff --git a/a b/a\n+a\n"
	if got := diffWithMarker(apppull.DiffPage{Text: whole, Page: 1, Limit: 50, Count: 1, Files: 1}); got != whole {
		t.Fatalf("got %q, want the diff unchanged", got)
	}
	if got := diffWithMarker(apppull.DiffPage{Text: whole}); got != whole {
		t.Fatalf("file_path result changed: %q", got)
	}
}

func TestDiffWithMarkerSaysWhenThePageIsPastTheEnd(t *testing.T) {
	t.Parallel()
	got := diffWithMarker(apppull.DiffPage{Page: 9, Limit: 2, Files: 3})
	if !strings.Contains(got, "no files on page 9") || !strings.Contains(got, "3 files") {
		t.Fatalf("got %q", got)
	}
}
