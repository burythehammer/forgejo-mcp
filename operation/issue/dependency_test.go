// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// dependsOn lists the indices of the issues testOwner/testRepo#index depends on.
func dependsOn(t *testing.T, f *fake.Forgejo, index int64) []int64 {
	t.Helper()
	issues, err := f.ListIssueDependencies(context.Background(), testOwner, testRepo, index, 1, 50)
	if err != nil {
		t.Fatalf("listing dependencies: %v", err)
	}
	out := make([]int64, len(issues))
	for i, iss := range issues {
		out[i] = iss.Index
	}
	return out
}

func wantIndices(t *testing.T, got []int64, want ...int64) {
	t.Helper()
	if !equalInt64s(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func equalInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestListIssueDependencies_EchoesDefaultPageAndLimit(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a, b := openIssue(t, f, "a"), openIssue(t, f, "b")
	if err := f.AddIssueDependency(context.Background(), testOwner, testRepo, a, forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: b}); err != nil {
		t.Fatal(err)
	}

	out := call(t, h.ListIssueDependenciesFn, map[string]any{"index": float64(a)})
	if !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("expected response to echo default page/limit, got %q", out)
	}
	if !strings.Contains(out, `"title":"b"`) {
		t.Fatalf("expected the dependency in the result, got %q", out)
	}
}

func TestListIssueDependents_ListsTheIssuesThatDependOnIt(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a, b := openIssue(t, f, "a"), openIssue(t, f, "b")
	if err := f.AddIssueDependency(context.Background(), testOwner, testRepo, a, forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: b}); err != nil {
		t.Fatal(err)
	}

	out := call(t, h.ListIssueDependentsFn, map[string]any{"index": float64(b)})
	if !strings.Contains(out, `"title":"a"`) || !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("got %q", out)
	}
}

func TestListIssueDependencies_PagesAndEchoesThem(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")
	for _, title := range []string{"b", "c", "d"} {
		dep := openIssue(t, f, title)
		if err := f.AddIssueDependency(context.Background(), testOwner, testRepo, a, forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: dep}); err != nil {
			t.Fatal(err)
		}
	}

	out := call(t, h.ListIssueDependenciesFn, map[string]any{"index": float64(a), "page": float64(2), "limit": float64(2)})
	if !strings.Contains(out, `"title":"d"`) || strings.Contains(out, `"title":"b"`) {
		t.Fatalf("expected only the second page, got %q", out)
	}
	if !strings.Contains(out, `"page":2`) || !strings.Contains(out, `"limit":2`) {
		t.Fatalf("expected response to echo requested page/limit, got %q", out)
	}
}

func TestListIssueDependencies_UnknownIssueIsEmpty(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for name, fn := range map[string]toolFn{"dependencies": h.ListIssueDependenciesFn, "dependents": h.ListIssueDependentsFn} {
		out := call(t, fn, map[string]any{"index": float64(42)})
		if !strings.Contains(out, `"issues":[]`) || !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
			t.Fatalf("%s: expected an empty result echoing page/limit, got %q", name, out)
		}
	}
}

// failingDeps is a port whose dependency listing fails the way a repository
// with dependencies disabled does.
type failingDeps struct{ *fake.Forgejo }

func (failingDeps) ListIssueDependencies(context.Context, string, string, int64, int, int) ([]*forgejo_sdk.Issue, error) {
	return nil, &forge.StatusError{StatusCode: http.StatusBadRequest, Err: errString("issue dependencies are disabled")}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestListIssueDependencies_APIErrorSurfaces(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	msg := callErr(t, withPort(failingDeps{f}).ListIssueDependenciesFn, map[string]any{"index": float64(42)})
	if !strings.Contains(msg, "issue dependencies are disabled") {
		t.Fatalf("got %q", msg)
	}
}

func TestAddIssueDependency_RecordsTheDependency(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a, b := openIssue(t, f, "a"), openIssue(t, f, "b")

	out := call(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(b)})
	if !strings.Contains(out, "depends on") {
		t.Fatalf("got %q", out)
	}
	wantIndices(t, dependsOn(t, f, a), b)
}

func TestAddIssueDependency_CrossRepo(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddRepo(testOwner, "other-repo")
	a := openIssue(t, f, "a")
	other, err := f.CreateIssue(context.Background(), testOwner, "other-repo", forgejo_sdk.CreateIssueOption{Title: "elsewhere"})
	if err != nil {
		t.Fatal(err)
	}

	out := call(t, h.AddIssueDependencyFn, map[string]any{
		"index": float64(a), "depends_on_index": float64(other.Index), "depends_on_repo": "other-repo",
	})
	if !strings.Contains(out, testOwner+"/other-repo#") {
		t.Fatalf("expected the dependency's repository in the message, got %q", out)
	}
	deps, err := f.ListIssueDependencies(context.Background(), testOwner, testRepo, a, 1, 50)
	if err != nil || len(deps) != 1 || deps[0].Title != "elsewhere" {
		t.Fatalf("got %+v err=%v", deps, err)
	}
}

func TestAddIssueDependency_CrossRepoSameIndexAllowed(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddRepo(testOwner, "other-repo")
	a := openIssue(t, f, "a")
	if _, err := f.CreateIssue(context.Background(), testOwner, "other-repo", forgejo_sdk.CreateIssueOption{Title: "same index"}); err != nil {
		t.Fatal(err)
	}

	// Same index in a DIFFERENT repo is not a self-dependency.
	call(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(a), "depends_on_repo": "other-repo"})
	if len(dependsOn(t, f, a)) != 1 {
		t.Fatal("expected the cross-repo dependency to be recorded")
	}
}

func TestAddIssueDependency_SelfDependencyRejected(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")

	msg := callErr(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(a)})
	if !strings.Contains(msg, "cannot depend on itself") {
		t.Fatalf("got %q", msg)
	}
	wantIndices(t, dependsOn(t, f, a))
}

// Forgejo treats owner and repository names case-insensitively, so a
// differently-cased spelling names the same repository and must fail early with
// the clear message.
func TestAddIssueDependency_SelfDependencyIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")

	msg := callErr(t, h.AddIssueDependencyFn, map[string]any{
		"index": float64(a), "depends_on_index": float64(a),
		"depends_on_owner": strings.ToUpper(testOwner), "depends_on_repo": strings.ToUpper(testRepo),
	})
	if !strings.Contains(msg, "cannot depend on itself") {
		t.Fatalf("got %q", msg)
	}
}

// index and depends_on_index are validated before the self-dependency check, so
// a missing required argument is reported as itself.
func TestAddIssueDependency_MissingDependsOnIndexIsNotMisreportedAsSelfDependency(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")

	msg := callErr(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a)})
	if strings.Contains(msg, "cannot depend on itself") || !strings.Contains(msg, "depends_on_index") {
		t.Fatalf("got %q", msg)
	}
}

func TestRemoveIssueDependency_RemovesIt(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a, b := openIssue(t, f, "a"), openIssue(t, f, "b")
	call(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(b)})

	out := call(t, h.RemoveIssueDependencyFn, map[string]any{"index": float64(a), "dependency_index": float64(b)})
	if !strings.Contains(out, "Removed dependency") {
		t.Fatalf("got %q", out)
	}
	wantIndices(t, dependsOn(t, f, a))
}

func TestRemoveIssueDependency_CrossRepo(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddRepo(testOwner, "other-repo")
	a := openIssue(t, f, "a")
	other, err := f.CreateIssue(context.Background(), testOwner, "other-repo", forgejo_sdk.CreateIssueOption{Title: "elsewhere"})
	if err != nil {
		t.Fatal(err)
	}
	call(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(other.Index), "depends_on_repo": "other-repo"})

	call(t, h.RemoveIssueDependencyFn, map[string]any{"index": float64(a), "dependency_index": float64(other.Index), "dependency_repo": "other-repo"})
	wantIndices(t, dependsOn(t, f, a))
}

func TestRemoveIssueDependency_APIErrorSurfaces(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")

	// Nothing to remove: Forgejo's 404 reaches the caller.
	msg := callErr(t, h.RemoveIssueDependencyFn, map[string]any{"index": float64(a), "dependency_index": float64(99)})
	if !strings.Contains(msg, "remove issue dependency err") {
		t.Fatalf("got %q", msg)
	}
}

// A non-string cross-repo argument is malformed, not absent. If it silently
// defaulted to the target's own repository a dependency would be written
// against the wrong repo and reported as success.
func TestCrossRepoArgumentsOfTheWrongTypeAreRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func(h handlers) toolFn
		args map[string]any
		arg  string
	}{
		{"add owner", func(h handlers) toolFn { return h.AddIssueDependencyFn }, map[string]any{"depends_on_index": float64(2), "depends_on_owner": float64(123)}, "depends_on_owner"},
		{"add repo", func(h handlers) toolFn { return h.AddIssueDependencyFn }, map[string]any{"depends_on_index": float64(2), "depends_on_repo": true}, "depends_on_repo"},
		{"remove owner", func(h handlers) toolFn { return h.RemoveIssueDependencyFn }, map[string]any{"dependency_index": float64(2), "dependency_owner": float64(123)}, "dependency_owner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, f := newFixture(t)
			a, b := openIssue(t, f, "a"), openIssue(t, f, "b")
			tc.args["index"] = float64(a)
			msg := callErr(t, tc.fn(h), tc.args)
			if !strings.Contains(msg, tc.arg) {
				t.Fatalf("error does not name the offending argument: %q", msg)
			}
			wantIndices(t, dependsOn(t, f, a))
			_ = b
		})
	}
}

// An explicit JSON null is absent, not malformed: it keeps the documented
// default instead of being refused.
func TestAddIssueDependency_NullCrossRepoOwnerDefaultsToTargetRepo(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a, b := openIssue(t, f, "a"), openIssue(t, f, "b")

	call(t, h.AddIssueDependencyFn, map[string]any{"index": float64(a), "depends_on_index": float64(b), "depends_on_owner": nil})
	wantIndices(t, dependsOn(t, f, a), b)
}
