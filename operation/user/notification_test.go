// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func newFixture() (handlers, *fake.Forgejo) {
	f := fake.New()
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func req(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// call runs a tool handler and fails the test unless it succeeded, returning
// the result text.
func call(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), req(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func callErr(t *testing.T, fn toolFn, args map[string]any) error {
	t.Helper()
	res, err := fn(context.Background(), req(args))
	if err == nil {
		t.Fatalf("expected an error, got result %+v", res)
	}
	return err
}

// decodeResult unmarshals the {"Result": ...} envelope of a tool result.
func decodeResult(t *testing.T, text string, into any) {
	t.Helper()
	var env struct{ Result json.RawMessage }
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	if err := json.Unmarshal(env.Result, into); err != nil {
		t.Fatalf("decoding result %s: %v", env.Result, err)
	}
}

func ids(threads []forgejo_sdk.NotificationThread) []int64 {
	out := make([]int64, len(threads))
	for i, n := range threads {
		out[i] = n.ID
	}
	return out
}

func TestGetUserInfoFn(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	var got forgejo_sdk.User
	decodeResult(t, call(t, h.GetUserInfoFn, map[string]any{}), &got)
	if got.UserName != fake.Poster {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckNotificationsFn_UnreadByDefault(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	unread := f.AddNotification("o", "r", "new", true)
	f.AddNotification("o", "r", "seen", false)

	var got []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.CheckNotificationsFn, map[string]any{}), &got)
	if want := []int64{unread.ID}; len(got) != 1 || ids(got)[0] != want[0] {
		t.Fatalf("got %v, want %v", ids(got), want)
	}
}

func TestCheckNotificationsFn_Paginates(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	for i := 0; i < 3; i++ {
		f.AddNotification("o", "r", "n", true)
	}
	var got []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.CheckNotificationsFn, map[string]any{"page": 2.0, "limit": 2.0}), &got)
	if len(got) != 1 {
		t.Fatalf("page 2 = %v", ids(got))
	}
}

func TestCheckNotificationsFn_BadTimes(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	if err := callErr(t, h.CheckNotificationsFn, map[string]any{"since": "yesterday"}); !strings.Contains(err.Error(), "invalid since time format") {
		t.Fatalf("got %v", err)
	}
	if err := callErr(t, h.CheckNotificationsFn, map[string]any{"before": "tomorrow"}); !strings.Contains(err.Error(), "invalid before time format") {
		t.Fatalf("got %v", err)
	}
}

func TestGetNotificationThreadFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	n := f.AddNotification("o", "r", "hello", true)
	var got forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.GetNotificationThreadFn, map[string]any{"id": float64(n.ID)}), &got)
	if got.ID != n.ID || got.Subject.Title != "hello" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetNotificationThreadFn_Errors(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.GetNotificationThreadFn, map[string]any{})
	if err := callErr(t, h.GetNotificationThreadFn, map[string]any{"id": 99.0}); !strings.HasPrefix(err.Error(), "get notification thread err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestMarkNotificationReadFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	n := f.AddNotification("o", "r", "hello", true)
	other := f.AddNotification("o", "r", "other", true)
	call(t, h.MarkNotificationReadFn, map[string]any{"id": float64(n.ID)})

	var left []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.CheckNotificationsFn, map[string]any{}), &left)
	if len(left) != 1 || left[0].ID != other.ID {
		t.Fatalf("left %v", ids(left))
	}
}

func TestMarkNotificationReadFn_MissingID(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.MarkNotificationReadFn, map[string]any{})
}

func TestMarkAllNotificationsReadFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.AddNotification("o", "r", "a", true)
	f.AddNotification("o", "r2", "b", true)
	if got := call(t, h.MarkAllNotificationsReadFn, map[string]any{"last_read_at": time.Now().Add(time.Hour).Format(time.RFC3339)}); !strings.Contains(got, "All notifications marked as read") {
		t.Fatalf("got %s", got)
	}
	var left []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.CheckNotificationsFn, map[string]any{}), &left)
	if len(left) != 0 {
		t.Fatalf("left %v", ids(left))
	}
}

func TestMarkAllNotificationsReadFn_BadTime(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	if err := callErr(t, h.MarkAllNotificationsReadFn, map[string]any{"last_read_at": "soon"}); !strings.Contains(err.Error(), "invalid last_read_at time format") {
		t.Fatalf("got %v", err)
	}
}

func TestListRepoNotificationsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	here := f.AddNotification("synapse", "neural-net", "here", true)
	f.AddNotification("synapse", "other", "there", true)
	var got []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.ListRepoNotificationsFn, map[string]any{"owner": "synapse", "repo": "neural-net", "limit": 10.0, "page": 1.0}), &got)
	if len(got) != 1 || got[0].ID != here.ID {
		t.Fatalf("got %v", ids(got))
	}
}

func TestListRepoNotificationsFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.ListRepoNotificationsFn, map[string]any{"owner": "o"})
	callErr(t, h.ListRepoNotificationsFn, map[string]any{"repo": "r"})
}

func TestMarkRepoNotificationsReadFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.AddNotification("synapse", "neural-net", "here", true)
	there := f.AddNotification("synapse", "other", "there", true)
	if got := call(t, h.MarkRepoNotificationsReadFn, map[string]any{"owner": "synapse", "repo": "neural-net"}); !strings.Contains(got, "Repo notifications marked as read") {
		t.Fatalf("got %s", got)
	}
	var left []forgejo_sdk.NotificationThread
	decodeResult(t, call(t, h.CheckNotificationsFn, map[string]any{}), &left)
	if len(left) != 1 || left[0].ID != there.ID {
		t.Fatalf("left %v", ids(left))
	}
}

func TestMarkRepoNotificationsReadFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.MarkRepoNotificationsReadFn, map[string]any{"owner": "o"})
}
