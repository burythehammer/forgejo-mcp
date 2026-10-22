// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"strings"
	"testing"
)

// TestGetIssueByIndex_TypeFilter runs get_issue_by_index with an index of each
// kind and each type value. An accepted read returns the issue; a refused one
// is an error that names the tool for the other kind.
func TestGetIssueByIndex_TypeFilter(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	issue := openIssue(t, f, "plain issue")
	pull := openIssue(t, f, "pull request")
	f.MarkPullRequest(testOwner, testRepo, pull)

	accepted := []struct {
		name  string
		index int64
		typ   any
	}{
		{"no type, issue", issue, nil},
		{"no type, pull request", pull, nil},
		{"empty type, issue", issue, ""},
		{"issues, issue", issue, "issues"},
		{"pulls, pull request", pull, "pulls"},
	}
	for _, c := range accepted {
		t.Run(c.name, func(t *testing.T) {
			args := map[string]any{"index": float64(c.index)}
			if c.typ != nil {
				args["type"] = c.typ
			}
			got := issueJSON(t, call(t, h.GetIssueByIndexFn, args))
			if got["number"] != float64(c.index) {
				t.Fatalf("number = %v, want %d", got["number"], c.index)
			}
		})
	}

	refused := []struct {
		name     string
		index    int64
		typ      string
		contains []string
	}{
		{"issues refuses a pull request", pull, "issues", []string{"is a pull request", "get_pull_request_by_index"}},
		{"pulls refuses an issue", issue, "pulls", []string{"is an issue", "get_issue_by_index"}},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			msg := callErr(t, h.GetIssueByIndexFn, map[string]any{"index": float64(c.index), "type": c.typ})
			for _, want := range c.contains {
				if !strings.Contains(msg, want) {
					t.Fatalf("error = %q, want it to contain %q", msg, want)
				}
			}
		})
	}
}

func TestGetIssueByIndex_UnknownTypeIsRejected(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "an issue")
	for _, typ := range []string{"issue", "PULLS", "pr"} {
		msg := callErr(t, h.GetIssueByIndexFn, map[string]any{"index": float64(idx), "type": typ})
		if !strings.Contains(msg, "invalid type") {
			t.Fatalf("type %q: error = %q, want an invalid type error", typ, msg)
		}
	}
}
