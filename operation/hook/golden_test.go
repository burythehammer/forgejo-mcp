// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every webhook tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port that
// returns fixed data, so the bytes are deterministic. The stub's hooks carry a
// secret, so a golden that contained one would show it.

var allTools = []mcp.Tool{
	ListRepoHooksTool, GetRepoHookTool, CreateRepoHookTool, EditRepoHookTool, DeleteRepoHookTool, TestRepoHookTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	hkT0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	hkT1 = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
)

// fixedHook returns a fresh hook each call, so a tool that edits a returned
// config map does not affect the next call.
func fixedHook(id int64) *forgejo_sdk.Hook {
	return &forgejo_sdk.Hook{
		ID: id, Type: "forgejo", Active: true, Events: []string{"push"},
		Config: map[string]string{
			"url": "https://hooks.example/a", "content_type": "json", "secret": "s3cr3t",
			"http_method": "POST", "branch_filter": "main",
		},
		Created: hkT0, Updated: hkT1,
	}
}

// stubHook is a forge.Forgejo whose webhook domain returns fixed data and
// records the calls it receives. Any other domain would panic on the nil embed.
type stubHook struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	// fail is returned by every call; editErr replaces the edit's result;
	// refetchFail is returned by the second and later gets.
	fail        error
	editErr     error
	refetchFail error
	gets        int
	noTotal     bool
}

func (s *stubHook) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubHook) ListRepoHooks(_ context.Context, o, r string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	list := forge.HookList{Hooks: []*forgejo_sdk.Hook{fixedHook(1), fixedHook(2)}}
	if !s.noTotal {
		n := 7
		list.TotalCount = &n
	}
	return list, s.rec("ListRepoHooks", o, r, opt)
}

func (s *stubHook) GetRepoHook(_ context.Context, o, r string, id int64) (*forgejo_sdk.Hook, error) {
	s.mu.Lock()
	s.gets++
	n := s.gets
	s.mu.Unlock()
	if n > 1 && s.refetchFail != nil {
		_ = s.rec("GetRepoHook", o, r, id)
		return nil, s.refetchFail
	}
	return fixedHook(id), s.rec("GetRepoHook", o, r, id)
}

func (s *stubHook) CreateRepoHook(_ context.Context, o, r string, opt forgejo_sdk.CreateHookOption) (*forgejo_sdk.Hook, error) {
	return fixedHook(5), s.rec("CreateRepoHook", o, r, opt)
}

func (s *stubHook) EditRepoHook(_ context.Context, o, r string, id int64, opt forge.HookEdit) error {
	if err := s.rec("EditRepoHook", o, r, id, editPresence(opt)); err != nil {
		return err
	}
	return s.editErr
}

// editPresence renders an edit the way it reached the port: only the fields
// the use case set, so an unset field is absent and an empty one is shown.
// The port's HookEdit JSON would hide an empty events list (omitempty).
func editPresence(opt forge.HookEdit) map[string]any {
	m := map[string]any{}
	if opt.Config != nil {
		m["config"] = opt.Config
	}
	if opt.Events != nil {
		m["events"] = opt.Events
	}
	if opt.BranchFilter != nil {
		m["branch_filter"] = *opt.BranchFilter
	}
	if opt.Active != nil {
		m["active"] = *opt.Active
	}
	if opt.AuthorizationHeader != nil {
		m["authorization_header"] = *opt.AuthorizationHeader
	}
	return m
}

func (s *stubHook) DeleteRepoHook(_ context.Context, o, r string, id int64) error {
	return s.rec("DeleteRepoHook", o, r, id)
}

func (s *stubHook) TestRepoHook(_ context.Context, o, r string, id int64) error {
	return s.rec("TestRepoHook", o, r, id)
}

// invokeHook calls a registered tool the way the server does and renders what
// the client would see: the text, or the error, then the port calls made.
func invokeHook(t *testing.T, stub *stubHook, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTools(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type hookGoldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubHook)
}

var hookGoldenCases = []hookGoldenCase{
	{name: "list_defaults", tool: ListRepoHooksToolName},
	{name: "list_paged", tool: ListRepoHooksToolName, args: map[string]any{"page": float64(2), "limit": float64(5)}},
	{name: "list_fractional_paging", tool: ListRepoHooksToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "list_no_total", tool: ListRepoHooksToolName, stub: func(s *stubHook) { s.noTotal = true }},
	{name: "list_missing_owner", tool: ListRepoHooksToolName, args: map[string]any{"owner": nil}},
	{name: "list_port_error", tool: ListRepoHooksToolName, stub: func(s *stubHook) { s.fail = errors.New("boom") }},

	{name: "get_ok", tool: GetRepoHookToolName, args: map[string]any{"id": float64(7)}},
	{name: "get_string_id", tool: GetRepoHookToolName, args: map[string]any{"id": "7"}},
	{name: "get_fractional_id", tool: GetRepoHookToolName, args: map[string]any{"id": 7.5}},
	{name: "get_zero_id", tool: GetRepoHookToolName, args: map[string]any{"id": float64(0)}},
	{name: "get_missing_repo", tool: GetRepoHookToolName, args: map[string]any{"id": float64(7), "repo": nil}},
	{name: "get_port_error", tool: GetRepoHookToolName, args: map[string]any{"id": float64(7)}, stub: func(s *stubHook) { s.fail = errors.New("boom") }},

	{name: "create_minimal", tool: CreateRepoHookToolName, args: map[string]any{"url": "https://hooks.example/a"}},
	{name: "create_full", tool: CreateRepoHookToolName, args: map[string]any{
		"url": "https://hooks.example/b", "type": "forgejo", "content_type": "form", "secret": "x",
		"http_method": "GET", "branch_filter": "release/*", "events": "push, pull_request,", "active": false,
	}},
	{name: "create_wrong_type_active", tool: CreateRepoHookToolName, args: map[string]any{"url": "https://hooks.example/a", "active": "yes"}},
	{name: "create_missing_url", tool: CreateRepoHookToolName},
	{name: "create_port_error", tool: CreateRepoHookToolName, args: map[string]any{"url": "https://hooks.example/a"}, stub: func(s *stubHook) { s.fail = errors.New("boom") }},

	{name: "edit_merge", tool: EditRepoHookToolName, args: map[string]any{
		"id": float64(7), "url": "https://hooks.example/new", "secret": "rotated", "events": "push,create", "branch_filter": "dev", "active": false,
	}},
	{name: "edit_none", tool: EditRepoHookToolName, args: map[string]any{"id": float64(7)}},
	{name: "edit_get_error", tool: EditRepoHookToolName, args: map[string]any{"id": float64(7)}, stub: func(s *stubHook) { s.fail = errors.New("boom") }},
	{
		name: "edit_status_error", tool: EditRepoHookToolName, args: map[string]any{"id": float64(7), "url": "https://hooks.example/new"},
		stub: func(s *stubHook) {
			s.editErr = &forge.StatusError{StatusCode: 404, Err: errors.New("hook not found")}
		},
	},
	{
		name: "edit_plain_error", tool: EditRepoHookToolName, args: map[string]any{"id": float64(7)},
		stub: func(s *stubHook) { s.editErr = errors.New("write failed") },
	},
	{
		name: "edit_refetch_error", tool: EditRepoHookToolName, args: map[string]any{"id": float64(7)},
		stub: func(s *stubHook) { s.refetchFail = errors.New("gone") },
	},
	{name: "edit_missing_id", tool: EditRepoHookToolName, args: map[string]any{"id": nil}},

	{name: "delete_ok", tool: DeleteRepoHookToolName, args: map[string]any{"id": float64(7)}},
	{name: "delete_zero_id", tool: DeleteRepoHookToolName, args: map[string]any{"id": float64(0)}},
	{name: "delete_port_error", tool: DeleteRepoHookToolName, args: map[string]any{"id": float64(7)}, stub: func(s *stubHook) { s.fail = errors.New("boom") }},

	{name: "test_ok", tool: TestRepoHookToolName, args: map[string]any{"id": float64(7)}},
	{name: "test_zero_id", tool: TestRepoHookToolName, args: map[string]any{"id": float64(0)}},
	{name: "test_port_error", tool: TestRepoHookToolName, args: map[string]any{"id": float64(7)}, stub: func(s *stubHook) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range hookGoldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubHook{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invokeHook(t, stub, c.tool, c.args)))
		})
	}
}
