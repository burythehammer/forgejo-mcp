// SPDX-License-Identifier: GPL-3.0-or-later

package hook_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apphook "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/hook"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/hookadapter"
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
	got, err := s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(float64(p.ID)), URL: "https://hooks.example/b"})
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
	_, err := s.Edit(context.Background(), apphook.EditRequest{HookRef: ref(99), URL: "https://x"})
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
	_, err := apphook.New(port).Edit(context.Background(), apphook.EditRequest{HookRef: ref(7), URL: "https://x"})
	if err == nil || err.Error() != "edit repo hook 404 not found" {
		t.Fatalf("got %v", err)
	}
}
