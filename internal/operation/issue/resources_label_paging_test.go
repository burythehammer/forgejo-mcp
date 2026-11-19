// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The label resources page. These tests guard the defect where a handler
// requests limit+1 rows as a truncation probe while showing only limit of them,
// so that the (page-1)*PageSize offset makes page N+1 begin one row past the
// last row page N showed. That row is returned by no page a client can ask for.

const (
	repoLabelsURIFmt = "forgejo://repo/octo/hello/labels?page=%d&limit=%d"
	orgLabelsURIFmt  = "forgejo://org/o/labels?page=%d&limit=%d"
)

// labelsFixture is a fake with n labels at each scope: repo labels on
// octo/hello, org labels on "o".
func labelsFixture(t *testing.T, n int) (handlers, *fake.Forgejo) {
	t.Helper()
	h, f := newFixture(t)
	seedRepoLabels(t, f, n)
	seedOrgLabels(t, f, "o", n)
	return h, f
}

type labelPage struct {
	Labels    []labelResourcePayload `json:"labels"`
	Truncated bool                   `json:"truncated"`
	Sentinel  string                 `json:"sentinel"`
	ListTool  string                 `json:"list_tool"`
	Org       string                 `json:"org"`
}

func readLabelPage(t *testing.T, read readFn, uri string) labelPage {
	t.Helper()
	var p labelPage
	readJSON(t, read, uri, &p)
	return p
}

// walkLabelPages reads pages 1..pages and returns the label names the client
// saw, in order. A row skipped at a page boundary simply never appears.
func walkLabelPages(t *testing.T, read readFn, uriFmt string, pages, limit int) []string {
	t.Helper()
	var seen []string
	for page := 1; page <= pages; page++ {
		for _, l := range readLabelPage(t, read, fmt.Sprintf(uriFmt, page, limit)).Labels {
			seen = append(seen, l.Name)
		}
	}
	return seen
}

// assertPagesCoverCorpus fails if the union of what the client saw is not
// exactly the contiguous run label-1..label-N.
func assertPagesCoverCorpus(t *testing.T, seen []string, pages, limit int) {
	t.Helper()
	want := pages * limit
	if len(seen) != want {
		t.Fatalf("expected %d rows across %d pages of %d, got %d: %v", want, pages, limit, len(seen), seen)
	}
	for i, got := range seen {
		if got != fmt.Sprintf("label-%d", i+1) {
			t.Fatalf("page boundary lost or repeated a row at position %d: got %s\nfull sequence: %v", i, got, seen)
		}
	}
}

// ---- the property that matters: successive pages cover every row exactly once ----

func TestRepoLabelsResource_PagesCoverEveryRow(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 100)
	assertPagesCoverCorpus(t, walkLabelPages(t, h.repoLabelsResourceHandler, repoLabelsURIFmt, 3, 30), 3, 30)
}

func TestOrgLabelsResource_PagesCoverEveryRow(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 100)
	assertPagesCoverCorpus(t, walkLabelPages(t, h.orgLabelsResourceHandler, orgLabelsURIFmt, 3, 30), 3, 30)
}

// A non-default limit exercises the boundary arithmetic at a different stride.
func TestRepoLabelsResource_PagesCoverEveryRowAtSmallLimit(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 100)
	assertPagesCoverCorpus(t, walkLabelPages(t, h.repoLabelsResourceHandler, repoLabelsURIFmt, 4, 7), 4, 7)
}

// ---- truncation is the server's answer, not an item count ----

func TestLabelsResources_TruncationComesFromTheServer(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 100)
	cases := []struct {
		name string
		read readFn
		uri  string
		tool string
	}{
		{"repo", h.repoLabelsResourceHandler, "forgejo://repo/octo/hello/labels?page=1&limit=30", ListRepoLabelsToolName},
		{"org", h.orgLabelsResourceHandler, "forgejo://org/o/labels?page=1&limit=30", ListOrgLabelsToolName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := readLabelPage(t, tc.read, tc.uri)
			if len(p.Labels) != 30 {
				t.Fatalf("expected a full page of 30 rows, got %d", len(p.Labels))
			}
			if !p.Truncated {
				t.Fatalf("a full page with a next page must report truncated (sentinel=%q)", p.Sentinel)
			}
			if !strings.Contains(p.Sentinel, "30 of 100 items shown") {
				t.Fatalf("sentinel should carry the server's total, got %q", p.Sentinel)
			}
			if p.ListTool != tc.tool {
				t.Fatalf("truncated payload must name the enumeration tool, got %q", p.ListTool)
			}
		})
	}
}

// The other half: no next page, so no sentinel. Without this, "always
// truncated" would pass the test above.
func TestLabelsResources_LastPageIsNotTruncated(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 30)
	cases := map[string]struct {
		read readFn
		uri  string
	}{
		"repo": {h.repoLabelsResourceHandler, "forgejo://repo/octo/hello/labels?page=1&limit=30"},
		"org":  {h.orgLabelsResourceHandler, "forgejo://org/o/labels?page=1&limit=30"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := readLabelPage(t, tc.read, tc.uri)
			if len(p.Labels) != 30 {
				t.Fatalf("expected all 30 rows, got %d", len(p.Labels))
			}
			if p.Truncated || p.Sentinel != "" {
				t.Fatalf("a corpus that ends exactly at the page boundary is not truncated (sentinel=%q)", p.Sentinel)
			}
		})
	}
}

// noTotal is a port whose label lists advertise a next page but no total, like
// an instance that sends a Link header without X-Total-Count.
type noTotal struct{ *fake.Forgejo }

func (n noTotal) ListRepoLabels(ctx context.Context, owner, repo string, page, limit int) ([]*forge.Label, forge.PageInfo, error) {
	out, info, err := n.Forgejo.ListRepoLabels(ctx, owner, repo, page, limit)
	info.TotalKnown, info.TotalCount = false, 0
	return out, info, err
}

// An instance that advertises a next page but sends no total must not have one
// invented for it.
func TestRepoLabelsResource_TruncationWithoutTotal(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	seedRepoLabels(t, f, 100)
	h := withPort(noTotal{f})

	p := readLabelPage(t, h.repoLabelsResourceHandler, "forgejo://repo/octo/hello/labels?page=1&limit=30")
	if !p.Truncated {
		t.Fatalf("a next page alone must still report truncated (sentinel=%q)", p.Sentinel)
	}
	if !strings.Contains(p.Sentinel, "30 items shown, more remain") {
		t.Fatalf("without a total the sentinel must not claim one, got %q", p.Sentinel)
	}
}

func TestRepoLabelsResource_ClampsLimitToCap(t *testing.T) {
	t.Parallel()
	h, _ := labelsFixture(t, 100)

	p := readLabelPage(t, h.repoLabelsResourceHandler, "forgejo://repo/octo/hello/labels?limit=999")
	if len(p.Labels) != resource.EmbeddedListCap {
		t.Fatalf("a limit above the ceiling must clamp to %d, got %d rows", resource.EmbeddedListCap, len(p.Labels))
	}
}

// An owner with no organization labels (Forgejo answers 404) reads as an empty
// list, not a failed read.
func TestOrgLabelsResource_NoLabelsMeansEmptyList(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	// The org exists and has no labels; a missing org is not-found (see the
	// unknown-org test in label_tools_test.go).
	if _, err := f.CreateOrg(context.Background(), forgejo_sdk.CreateOrgOption{Name: "o"}); err != nil {
		t.Fatal(err)
	}

	p := readLabelPage(t, h.orgLabelsResourceHandler, "forgejo://org/o/labels?limit=5")
	if len(p.Labels) != 0 || p.Truncated {
		t.Fatalf("expected an empty, untruncated list, got %+v", p)
	}
	if p.Org != "o" {
		t.Fatalf("payload lost the org it was asked about: %q", p.Org)
	}
}
