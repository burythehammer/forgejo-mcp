// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Tests of the label tools, which still run against the legacy client. The
// labels ticket migrates them.

func TestListOrgLabels_Success(t *testing.T) {
	_, records := newLabelsBackend(t,
		"", 0,
		`[{"id":1,"name":"bug","color":"ff0000","description":"a bug","url":""},{"id":2,"name":"enh","color":"00ff00","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "codeberg"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListOrgLabelsFn err: %v res=%+v", err, res)
	}
	if len(*records) == 0 {
		t.Fatal("expected request to backend")
	}
	last := (*records)[len(*records)-1]
	if last.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", last.method)
	}
	if !strings.HasPrefix(last.path, "/api/v1/orgs/codeberg/labels") {
		t.Fatalf("unexpected path: %s", last.path)
	}
	// Result body should serialize each entry with scope=org.
	if !strings.Contains(textOf(res), `"scope":"org"`) {
		t.Fatalf("expected scope=org in result, got %q", textOf(res))
	}
}

func TestListOrgLabels_404IsEmpty(t *testing.T) {
	_, _ = newLabelsBackend(t, "", 0, ``, http.StatusNotFound)
	res, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "ghost"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected success on 404, got err=%v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), "[]") && !strings.Contains(textOf(res), "null") {
		t.Fatalf("expected empty result for 404, got %q", textOf(res))
	}
}

func TestListOrgLabels_UnauthorizedSurfaces(t *testing.T) {
	_, _ = newLabelsBackend(t, "", 0, ``, http.StatusUnauthorized)
	_, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "codeberg"}))
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

func TestListRepoLabels_MergeWithOrgLabels(t *testing.T) {
	_, _ = newLabelsBackend(t,
		`[{"id":10,"name":"good-first-issue","color":"7057ff","description":"","url":""}]`, http.StatusOK,
		`[{"id":20,"name":"security","color":"d73a4a","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "codeberg-org", "repo": "project",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoLabelsFn err: %v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"scope":"repo"`) || !strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected both scopes in result, got %q", out)
	}
	if !strings.Contains(out, `"good-first-issue"`) || !strings.Contains(out, `"security"`) {
		t.Fatalf("expected both label names in merged result, got %q", out)
	}
}

func TestListRepoLabels_IncludeOrgFalseSkipsOrgCall(t *testing.T) {
	_, records := newLabelsBackend(t,
		`[{"id":10,"name":"good-first-issue","color":"7057ff","description":"","url":""}]`, http.StatusOK,
		`[{"id":20,"name":"security","color":"d73a4a","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner":              "codeberg-org",
		"repo":               "project",
		"include_org_labels": false,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoLabelsFn err: %v res=%+v", err, res)
	}
	for _, r := range *records {
		if strings.HasPrefix(r.path, "/api/v1/orgs/") {
			t.Fatalf("did not expect org-labels request, got %s %s", r.method, r.path)
		}
	}
	out := textOf(res)
	if strings.Contains(out, `"scope":"org"`) || strings.Contains(out, `"security"`) {
		t.Fatalf("expected no org-scoped labels in result, got %q", out)
	}
}

func TestListRepoLabels_UserOwnerOrgEndpoint404(t *testing.T) {
	_, _ = newLabelsBackend(t,
		`[{"id":10,"name":"bug","color":"ff0000","description":"","url":""}]`, http.StatusOK,
		``, http.StatusNotFound,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "alice", "repo": "project",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected success when org endpoint 404s, got err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"scope":"repo"`) {
		t.Fatalf("expected repo-scoped label in result, got %q", out)
	}
	if strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected no org-scoped labels when org endpoint 404s, got %q", out)
	}
}

func TestAddIssueLabelsFn_ByName(t *testing.T) {
	records, _ := addLabelsBackend(t)

	res, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "bug,triage",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("AddIssueLabelsFn err: %v res=%+v", err, res)
	}
	// Two calls under test: AddIssueLabels then GetIssue. The catalogue reads
	// the names needed are counted separately, not recorded here.
	if len(*records) != 2 {
		t.Fatalf("expected two requests, got %d: %+v", len(*records), *records)
	}

	post := (*records)[0]
	if post.method != http.MethodPost || !strings.HasSuffix(post.path, "/issues/1/labels") {
		t.Fatalf("expected POST to the issue labels endpoint, got %s %s", post.method, post.path)
	}
	// The names must reach Forgejo as IDs: it accepts names, but drops the
	// ones it does not know and still answers 200.
	if got := labelsFromBody(t, post.rawBody); !equalInt64s(got, []int64{5, 6}) {
		t.Fatalf("expected labels [5 6] in the POST body, got %v (body: %s)", got, post.rawBody)
	}
}

func TestAddIssueLabelsFn_ByID(t *testing.T) {
	records, _ := addLabelsBackend(t)

	res, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "5,6",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("AddIssueLabelsFn err: %v res=%+v", err, res)
	}
	if got := labelsFromBody(t, (*records)[0].rawBody); !equalInt64s(got, []int64{5, 6}) {
		t.Fatalf("expected numeric IDs to keep working, got %v", got)
	}
}

func TestAddIssueLabelsFn_UnknownLabel(t *testing.T) {
	records, _ := addLabelsBackend(t)

	res, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "not-a-label",
	}))
	if res != nil {
		t.Fatalf("expected nil result for an unknown label, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected an error for an unknown label")
	}
	if len(*records) != 0 {
		t.Fatalf("expected no write when a label cannot be resolved, got %+v", *records)
	}
}

func TestAddIssueLabelsFn_EmptyLabels(t *testing.T) {
	records, _ := addLabelsBackend(t)

	if _, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "",
	})); err == nil {
		t.Fatal("expected an error when labels names nothing")
	}
	if len(*records) != 0 {
		t.Fatalf("expected no write for an empty labels argument, got %+v", *records)
	}
}

func TestRemoveIssueLabelsFn_ByName(t *testing.T) {
	records, _ := newResolveBackend(t, resolveBackendOptions{
		repoLabels: []catalogLabel{{ID: 5, Name: "bug"}},
		orgStatus:  http.StatusNotFound,
		next: func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
			_, _ = w.Write([]byte(`{"id":1,"number":1,"labels":[]}`))
		},
	})

	res, err := RemoveIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "bug",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("RemoveIssueLabelsFn err: %v res=%+v", err, res)
	}
	// Two calls under test: DeleteIssueLabel then GetIssue.
	if len(*records) != 2 {
		t.Fatalf("expected two requests, got %d: %+v", len(*records), *records)
	}
	del := (*records)[0]
	if del.method != http.MethodDelete || !strings.HasSuffix(del.path, "/issues/1/labels/5") {
		t.Fatalf("expected DELETE of label 5, got %s %s", del.method, del.path)
	}
}
