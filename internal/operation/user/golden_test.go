// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every user and notification tool: its
// tool definition (schema snapshot) and its output and error text (golden),
// together with the calls the tool made on the Forgejo port. They run against a
// stub port that returns fixed, fully populated data, so the bytes are
// deterministic.

var allTools = []mcp.Tool{
	GetMyUserInfoTool, CheckNotificationsTool, GetNotificationThreadTool, MarkNotificationReadTool,
	MarkAllNotificationsReadTool, ListRepoNotificationsTool, MarkRepoNotificationsReadTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1   = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	user = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedThread(id int64) *forgejo_sdk.NotificationThread {
	return &forgejo_sdk.NotificationThread{
		ID:         id,
		Repository: &forgejo_sdk.Repository{ID: 9, Name: "hello", FullName: "octo/hello", Owner: user, Created: t0, Updated: t1},
		Subject: &forgejo_sdk.NotificationSubject{
			Title: "fixed", URL: "https://f.example/api/v1/repos/octo/hello/issues/7", HTMLURL: "https://f.example/octo/hello/issues/7",
			Type: forgejo_sdk.NotifySubjectIssue, State: forgejo_sdk.NotifySubjectOpen,
		},
		Unread: true, UpdatedAt: t1, URL: "https://f.example/api/v1/notifications/threads/" + fmt.Sprint(id),
	}
}

// stubUsers is a forge.Forgejo whose user domain returns fixed data and records
// the calls it receives. Any other domain would panic on the nil embed.
type stubUsers struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
}

func (s *stubUsers) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubUsers) GetMyUserInfo(context.Context) (*forgejo_sdk.User, error) {
	return user, s.rec("GetMyUserInfo")
}

func (s *stubUsers) ListNotifications(_ context.Context, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	return []*forgejo_sdk.NotificationThread{fixedThread(1), fixedThread(2)}, s.rec("ListNotifications", opt)
}

func (s *stubUsers) ListRepoNotifications(_ context.Context, o, r string, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	return []*forgejo_sdk.NotificationThread{fixedThread(3)}, s.rec("ListRepoNotifications", o, r, opt)
}

func (s *stubUsers) GetNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	return fixedThread(id), s.rec("GetNotification", id)
}

func (s *stubUsers) ReadNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	t := fixedThread(id)
	t.Unread = false
	return t, s.rec("ReadNotification", id)
}

func (s *stubUsers) ReadNotifications(_ context.Context, opt forgejo_sdk.MarkNotificationOptions) error {
	return s.rec("ReadNotifications", opt)
}

func (s *stubUsers) ReadRepoNotifications(_ context.Context, o, r string, opt forgejo_sdk.MarkNotificationOptions) error {
	return s.rec("ReadRepoNotifications", o, r, opt)
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made. A
// non-nil credErr makes the credential policy refuse the request.
func invoke(t *testing.T, stub *stubUsers, credErr error, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub, Err: credErr})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{}
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

type goldenCase struct {
	name    string
	tool    string
	args    map[string]any
	stub    func(*stubUsers)
	credErr error
}

func failPort(s *stubUsers) { s.fail = errors.New("boom") }

var goldenCases = []goldenCase{
	// get_my_user_info
	{name: "my_info_ok", tool: GetMyUserInfoToolName},
	{name: "my_info_port_error", tool: GetMyUserInfoToolName, stub: failPort},
	{name: "my_info_no_credential", tool: GetMyUserInfoToolName, credErr: errors.New("no credential")},

	// check_notifications
	{name: "check_defaults", tool: CheckNotificationsToolName},
	{name: "check_all_filtered", tool: CheckNotificationsToolName, args: map[string]any{"all": true, "since": "2026-01-01T00:00:00Z", "before": "2026-02-01T00:00:00Z", "page": float64(2), "limit": float64(5)}},
	{name: "check_wrong_type_all", tool: CheckNotificationsToolName, args: map[string]any{"all": "yes"}},
	{name: "check_fractional_paging", tool: CheckNotificationsToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "check_string_paging", tool: CheckNotificationsToolName, args: map[string]any{"page": "4", "limit": "6"}},
	{name: "check_bad_since", tool: CheckNotificationsToolName, args: map[string]any{"since": "yesterday"}},
	{name: "check_bad_before", tool: CheckNotificationsToolName, args: map[string]any{"before": "2026-02-01"}},
	{name: "check_bad_since_no_credential", tool: CheckNotificationsToolName, args: map[string]any{"since": "yesterday"}, credErr: errors.New("no credential")},
	{name: "check_no_credential", tool: CheckNotificationsToolName, credErr: errors.New("no credential")},
	{name: "check_port_error", tool: CheckNotificationsToolName, stub: failPort},

	// get_notification_thread
	{name: "thread_ok", tool: GetNotificationThreadToolName, args: map[string]any{"id": float64(7)}},
	{name: "thread_string_id", tool: GetNotificationThreadToolName, args: map[string]any{"id": "7"}},
	{name: "thread_zero_id", tool: GetNotificationThreadToolName, args: map[string]any{"id": float64(0)}},
	{name: "thread_missing_id", tool: GetNotificationThreadToolName, args: map[string]any{"id": nil}},
	{name: "thread_malformed_id", tool: GetNotificationThreadToolName, args: map[string]any{"id": "seven"}},
	{name: "thread_missing_id_no_credential", tool: GetNotificationThreadToolName, args: map[string]any{"id": nil}, credErr: errors.New("no credential")},
	{name: "thread_port_error", tool: GetNotificationThreadToolName, args: map[string]any{"id": float64(7)}, stub: failPort},

	// mark_notification_read
	{name: "mark_read_ok", tool: MarkNotificationReadToolName, args: map[string]any{"id": float64(7)}},
	{name: "mark_read_missing_id", tool: MarkNotificationReadToolName, args: map[string]any{"id": nil}},
	{name: "mark_read_port_error", tool: MarkNotificationReadToolName, args: map[string]any{"id": float64(7)}, stub: failPort},

	// mark_all_notifications_read
	{name: "mark_all_defaults", tool: MarkAllNotificationsReadToolName},
	{name: "mark_all_last_read_at", tool: MarkAllNotificationsReadToolName, args: map[string]any{"last_read_at": "2026-01-15T10:00:00Z"}},
	{name: "mark_all_bad_last_read_at", tool: MarkAllNotificationsReadToolName, args: map[string]any{"last_read_at": "later"}},
	{name: "mark_all_port_error", tool: MarkAllNotificationsReadToolName, stub: failPort},

	// list_repo_notifications
	{name: "repo_check_defaults", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": "octo", "repo": "hello"}},
	{name: "repo_check_filtered", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": "octo", "repo": "hello", "all": true, "since": "2026-01-01T00:00:00Z", "page": float64(2), "limit": float64(3)}},
	{name: "repo_check_missing_owner", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": nil, "repo": "hello"}},
	{name: "repo_check_wrong_type_repo", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": "octo", "repo": float64(1)}},
	{name: "repo_check_bad_before", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": "octo", "repo": "hello", "before": "soon"}},
	{name: "repo_check_missing_owner_no_credential", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": nil}, credErr: errors.New("no credential")},
	{name: "repo_check_port_error", tool: ListRepoNotificationsToolName, args: map[string]any{"owner": "octo", "repo": "hello"}, stub: failPort},

	// mark_repo_notifications_read
	{name: "repo_mark_ok", tool: MarkRepoNotificationsReadToolName, args: map[string]any{"owner": "octo", "repo": "hello"}},
	{name: "repo_mark_last_read_at", tool: MarkRepoNotificationsReadToolName, args: map[string]any{"owner": "octo", "repo": "hello", "last_read_at": "2026-01-15T10:00:00Z"}},
	{name: "repo_mark_missing_repo", tool: MarkRepoNotificationsReadToolName, args: map[string]any{"owner": "octo", "repo": nil}},
	{name: "repo_mark_bad_last_read_at", tool: MarkRepoNotificationsReadToolName, args: map[string]any{"owner": "octo", "repo": "hello", "last_read_at": "later"}},
	{name: "repo_mark_port_error", tool: MarkRepoNotificationsReadToolName, args: map[string]any{"owner": "octo", "repo": "hello"}, stub: failPort},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubUsers{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.credErr, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the notification goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubUsers) MaxResponseItems(context.Context) (int, bool) { return 0, false }
