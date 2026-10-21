// SPDX-License-Identifier: GPL-3.0-or-later

package hook_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	apphook "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/hookadapter"
)

// The use cases run against the in-memory fake through the hook adapter: no
// MCP, no HTTP. The fake is built per test, so tests run in parallel.

const (
	owner  = "octo"
	repo   = "hello"
	secret = "decoy-hmac-secret"
)

func newService(t *testing.T) *apphook.Service {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return apphook.New(hookadapter.New(f))
}

func ref(id float64) apphook.HookRef { return apphook.HookRef{Owner: owner, Repo: repo, ID: id} }

func boolp(b bool) *bool { return &b }

func create(t *testing.T, s *apphook.Service, req apphook.CreateRequest) model.HookPayload {
	t.Helper()
	req.Owner, req.Repo = owner, repo
	p, err := s.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSecretIsNeverReturned(t *testing.T) {
	t.Parallel()
	s := newService(t)
	p := create(t, s, apphook.CreateRequest{URL: "https://hooks.example/a", Secret: secret})
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatalf("secret leaked into payload: %s", b)
	}
	list, err := s.List(context.Background(), apphook.ListRequest{Owner: owner, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(list)
	if strings.Contains(string(b), secret) {
		t.Fatalf("secret leaked into list: %s", b)
	}
}

func TestCreateDefaultsTypeAndActive(t *testing.T) {
	t.Parallel()
	p := create(t, newService(t), apphook.CreateRequest{URL: "https://hooks.example/a"})
	if p.Type != apphook.DefaultType || !p.Active || p.URL != "https://hooks.example/a" || p.ContentType != "json" {
		t.Fatalf("created: %+v", p)
	}
}

func TestCreateHonoursActiveFalse(t *testing.T) {
	t.Parallel()
	p := create(t, newService(t), apphook.CreateRequest{URL: "https://hooks.example/a", Active: boolp(false)})
	if p.Active {
		t.Fatal("active not cleared")
	}
}

func TestCreateNeedsURL(t *testing.T) {
	t.Parallel()
	_, err := newService(t).Create(context.Background(), apphook.CreateRequest{Owner: owner, Repo: repo})
	if err == nil || err.Error() != "owner, repo and url are required" {
		t.Fatalf("got %v", err)
	}
}

func TestEditMergesConfigAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	s := newService(t)
	p := create(t, s, apphook.CreateRequest{URL: "https://hooks.example/a", ContentType: "form", Secret: secret})
	got, err := s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(float64(p.ID)), URL: strp("https://hooks.example/b")})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://hooks.example/b" || got.ContentType != "form" || !got.Active {
		t.Fatalf("edited: %+v", got)
	}
}

func TestEditUnknownHookIsWrapped(t *testing.T) {
	t.Parallel()
	s := newService(t)
	_, err := s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(99), URL: strp("https://x")})
	if err == nil || !strings.HasPrefix(err.Error(), "get repo hook for edit: ") {
		t.Fatalf("got %v", err)
	}
}

func TestGetAndDeleteNeedAnID(t *testing.T) {
	t.Parallel()
	s := newService(t)
	if _, err := s.Get(context.Background(), ref(0)); err == nil || err.Error() != "owner, repo and id are required" {
		t.Fatalf("get: got %v", err)
	}
	if err := s.Delete(context.Background(), ref(0)); err == nil || err.Error() != "owner, repo and id are required" {
		t.Fatalf("delete: got %v", err)
	}
}

func TestDeleteRemovesTheHook(t *testing.T) {
	t.Parallel()
	s := newService(t)
	p := create(t, s, apphook.CreateRequest{URL: "https://hooks.example/a"})
	if err := s.Delete(context.Background(), ref(float64(p.ID))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(context.Background(), ref(float64(p.ID))); err == nil {
		t.Fatal("hook still readable after delete")
	}
}

// stubPort is a hand-written Port for the cases the fake cannot produce.
type stubPort struct {
	total   *int
	editErr error
}

func (s stubPort) ListRepoHooks(context.Context, string, string, apphook.Paging) (model.HookList, error) {
	return model.HookList{TotalCount: s.total}, nil
}

func (stubPort) GetRepoHook(context.Context, string, string, int64) (*model.Hook, error) {
	return &model.Hook{Config: map[string]string{"url": "https://x"}}, nil
}

func (stubPort) CreateRepoHook(context.Context, string, string, apphook.CreateInput) (*model.Hook, error) {
	return &model.Hook{}, nil
}

func (s stubPort) EditRepoHook(context.Context, string, string, int64, apphook.EditInput) error {
	return s.editErr
}
func (stubPort) DeleteRepoHook(context.Context, string, string, int64) error { return nil }
func (stubPort) TestRepoHook(context.Context, string, string, int64) error   { return nil }

func TestListReportsAZeroTotal(t *testing.T) {
	t.Parallel()
	zero := 0
	got, err := apphook.New(stubPort{total: &zero}).List(context.Background(), apphook.ListRequest{Owner: owner, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalCount == nil || *got.TotalCount != 0 {
		t.Fatalf("total: got %v, want a pointer to 0", got.TotalCount)
	}
}

func TestEditStatusErrorIsReported(t *testing.T) {
	t.Parallel()
	port := stubPort{editErr: &apphook.StatusError{Code: 404, Err: errors.New("not found")}}
	_, err := apphook.New(port).Edit(context.Background(), apphook.EditRequest{HookRef: ref(7), URL: strp("https://x")})
	if err == nil || err.Error() != "edit repo hook 404 not found" {
		t.Fatalf("got %v", err)
	}
}

// editPort records the EditInput the use case sends, and serves a stored hook
// with a branch filter and two events, so a test can see what an edit leaves
// out.
type editPort struct {
	stubPort
	in apphook.EditInput
}

func (p *editPort) GetRepoHook(context.Context, string, string, int64) (*model.Hook, error) {
	return &model.Hook{
		Config: map[string]string{"url": "https://x", "content_type": "json", "branch_filter": "main"},
		Events: []string{"push", "create"},
		Active: true,
	}, nil
}

func (p *editPort) EditRepoHook(_ context.Context, _, _ string, _ int64, in apphook.EditInput) error {
	p.in = in
	return nil
}

func editInput(t *testing.T, req apphook.EditRequest) apphook.EditInput {
	t.Helper()
	p := &editPort{}
	req.HookRef = ref(7)
	if _, err := apphook.New(p).Edit(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	return p.in
}

func strp(s string) *string { return &s }

// A field the request does not set is not sent, so Forgejo keeps it.
func TestEditSendsOnlyTheFieldsSet(t *testing.T) {
	t.Parallel()
	in := editInput(t, apphook.EditRequest{URL: strp("https://y")})
	if in.BranchFilter != nil || in.Events != nil || in.Active != nil {
		t.Fatalf("url-only edit sent unset fields: %+v", in)
	}
	if in.Config["url"] != "https://y" || in.Config["branch_filter"] != "main" {
		t.Fatalf("config = %v, want url changed and the stored keys kept", in.Config)
	}
}

func TestEditSendsNothingWhenNothingIsSet(t *testing.T) {
	t.Parallel()
	in := editInput(t, apphook.EditRequest{})
	if in.Config != nil || in.Events != nil || in.BranchFilter != nil || in.Active != nil {
		t.Fatalf("empty edit sent fields: %+v", in)
	}
}

// An explicit empty branch filter is a value: it is sent, so it clears.
func TestEditSendsAnExplicitEmptyBranchFilter(t *testing.T) {
	t.Parallel()
	in := editInput(t, apphook.EditRequest{BranchFilter: strp("")})
	if in.BranchFilter == nil || *in.BranchFilter != "" {
		t.Fatalf("branch filter = %v, want a pointer to empty", in.BranchFilter)
	}
}

// Explicit empty events clear the list: sent as a non-nil empty slice.
func TestEditSendsExplicitEmptyEvents(t *testing.T) {
	t.Parallel()
	in := editInput(t, apphook.EditRequest{Events: strp("")})
	if in.Events == nil || len(in.Events) != 0 {
		t.Fatalf("events = %#v, want a non-nil empty list", in.Events)
	}
}

// An explicit empty config value is sent in the merged config.
func TestEditSendsAnExplicitEmptyConfigValue(t *testing.T) {
	t.Parallel()
	in := editInput(t, apphook.EditRequest{ContentType: strp("")})
	if v, ok := in.Config["content_type"]; !ok || v != "" {
		t.Fatalf("config = %v, want content_type present and empty", in.Config)
	}
}

// Against the fake: a plain edit keeps the branch filter, an explicit empty
// one clears it.
func TestEditBranchFilterThroughTheFake(t *testing.T) {
	t.Parallel()
	s := newService(t)
	p := create(t, s, apphook.CreateRequest{URL: "https://hooks.example/a", BranchFilter: "main"})
	got, err := s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(float64(p.ID)), URL: strp("https://hooks.example/b")})
	if err != nil {
		t.Fatal(err)
	}
	if got.BranchFilter != "main" {
		t.Fatalf("url-only edit changed branch filter to %q", got.BranchFilter)
	}
	got, err = s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(float64(p.ID)), BranchFilter: strp("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.BranchFilter != "" {
		t.Fatalf("explicit empty branch filter left %q", got.BranchFilter)
	}
}

// MaxResponseItems reports an unknown ceiling for the test doubles.
func (stubPort) MaxResponseItems(context.Context) (int, bool)  { return 0, false }
func (*editPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func TestListHasNextUsesTheReportedTotal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		n       int
		page    float64
		count   int
		hasNext bool
	}{
		{"full page with more hooks", 5, 1, 2, true},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"short last page", 5, 3, 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newService(t)
			for i := range c.n {
				create(t, s, apphook.CreateRequest{URL: fmt.Sprintf("https://hooks.example/%d", i+1)})
			}
			got, err := s.List(context.Background(), apphook.ListRequest{Owner: owner, Repo: repo, Page: c.page, Limit: 2})
			if err != nil || got.Count != c.count || got.HasNext != c.hasNext {
				t.Fatalf("got %+v, %v", got, err)
			}
			if got.TotalCount == nil || *got.TotalCount != c.n {
				t.Fatalf("total: got %v, want %d", got.TotalCount, c.n)
			}
		})
	}
}
