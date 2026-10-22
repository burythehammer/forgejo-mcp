// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
)

// A request that names two labels from one exclusive scope is refused before
// anything is sent: the issue keeps the labels and fields it had. These drive
// the three write tools that take a label list.

func newExclusiveRepoLabel(t *testing.T, f *fake.Forgejo, name string) int64 {
	t.Helper()
	l, err := f.CreateRepoLabel(context.Background(), testOwner, testRepo, forge.LabelCreate{Name: name, Color: "#aabbcc", Exclusive: true})
	if err != nil {
		t.Fatalf("seeding exclusive label: %v", err)
	}
	return l.ID
}

func newPlainRepoLabel(t *testing.T, f *fake.Forgejo, name string) int64 {
	t.Helper()
	l, err := f.CreateRepoLabel(context.Background(), testOwner, testRepo, forge.LabelCreate{Name: name, Color: "#aabbcc"})
	if err != nil {
		t.Fatalf("seeding label: %v", err)
	}
	return l.ID
}

func TestExclusiveGuard_AddIssueLabelsRefusesTwoFromOneScope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newExclusiveRepoLabel(t, f, "status/ready")
	newExclusiveRepoLabel(t, f, "status/done")

	out := callErr(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "status/ready,status/done"})
	for _, want := range []string{`"status"`, `"status/ready"`, `"status/done"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("refusal must name %s, got %q", want, out)
		}
	}
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 0 {
		t.Fatalf("refused request must apply nothing, issue has %v", got)
	}
}

func TestExclusiveGuard_AddIssueLabelsRefusesIDsToo(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	ready := newExclusiveRepoLabel(t, f, "status/ready")
	done := newExclusiveRepoLabel(t, f, "status/done")

	ids := strings.Join([]string{strconv.FormatInt(ready, 10), strconv.FormatInt(done, 10)}, ",")
	callErr(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": ids})
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 0 {
		t.Fatalf("refused id request must apply nothing, issue has %v", got)
	}
}

func TestExclusiveGuard_AddIssueLabelsRefusesOrgLabels(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	if _, err := f.CreateOrgLabel(context.Background(), testOwner, forge.LabelCreate{Name: "prio/1", Color: "#aabbcc", Exclusive: true}); err != nil {
		t.Fatalf("seeding org label: %v", err)
	}
	newExclusiveRepoLabel(t, f, "prio/2")

	callErr(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "prio/1,prio/2"})
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 0 {
		t.Fatalf("refused org request must apply nothing, issue has %v", got)
	}
}

func TestExclusiveGuard_AddIssueLabelsAcceptsNonExclusiveSameScope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newPlainRepoLabel(t, f, "area/api")
	newPlainRepoLabel(t, f, "area/cli")

	call(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "area/api,area/cli"})
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 2 {
		t.Fatalf("non-exclusive same-scope labels must both apply, issue has %v", got)
	}
}

func TestExclusiveGuard_AddIssueLabelsAllowsOneExclusiveOnTopOfExisting(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newExclusiveRepoLabel(t, f, "status/ready")
	newExclusiveRepoLabel(t, f, "status/done")
	call(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "status/ready"})

	// Only the request is compared: the label already on the issue does not count.
	call(t, h.AddIssueLabelsFn, map[string]any{"index": idx, "labels": "status/done"})
}

func TestExclusiveGuard_UpdateIssueSetLabelsRefusesTwoFromOneScope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "original"))
	newExclusiveRepoLabel(t, f, "status/ready")
	newExclusiveRepoLabel(t, f, "status/done")

	out := callErr(t, h.UpdateIssueFn, map[string]any{"index": idx, "title": "changed", "set_labels": "status/ready,status/done"})
	for _, want := range []string{`"status"`, `"status/ready"`, `"status/done"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("refusal must name %s, got %q", want, out)
		}
	}
	iss, err := f.GetIssue(context.Background(), testOwner, testRepo, int64(idx))
	if err != nil {
		t.Fatalf("reading issue: %v", err)
	}
	if iss.Title != "original" {
		t.Fatalf("refused update must not edit the title, got %q", iss.Title)
	}
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 0 {
		t.Fatalf("refused update must not set labels, issue has %v", got)
	}
}

func TestExclusiveGuard_UpdateIssueSetLabelsAcceptsNonExclusive(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	newPlainRepoLabel(t, f, "area/api")
	newPlainRepoLabel(t, f, "area/cli")

	call(t, h.UpdateIssueFn, map[string]any{"index": idx, "set_labels": "area/api,area/cli"})
	if got := issueLabelNames(t, f, int64(idx)); len(got) != 2 {
		t.Fatalf("non-exclusive same-scope labels must both apply, issue has %v", got)
	}
}

func TestExclusiveGuard_CreateIssueRefusesTwoFromOneScope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newExclusiveRepoLabel(t, f, "status/ready")
	newExclusiveRepoLabel(t, f, "status/done")

	out := callErr(t, h.CreateIssueFn, map[string]any{"title": "new", "labels": "status/ready,status/done"})
	for _, want := range []string{`"status"`, `"status/ready"`, `"status/done"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("refusal must name %s, got %q", want, out)
		}
	}
	if _, err := f.GetIssue(context.Background(), testOwner, testRepo, 1); err == nil {
		t.Fatalf("refused create must not open an issue")
	}
}

func TestExclusiveGuard_CreateIssueAcceptsNonExclusive(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	newPlainRepoLabel(t, f, "area/api")
	newPlainRepoLabel(t, f, "area/cli")

	call(t, h.CreateIssueFn, map[string]any{"title": "new", "labels": "area/api,area/cli"})
	if got := issueLabelNames(t, f, 1); len(got) != 2 {
		t.Fatalf("non-exclusive same-scope labels must both apply, issue has %v", got)
	}
}
