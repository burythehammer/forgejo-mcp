// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/upload"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	owner = "octo"
	repo  = "hello"
)

type fn = func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// newFixture returns handlers served by a fresh fake with issue 3 and comment 5
// ready to take attachments.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	f.AddAttachmentIssue(owner, repo, 3)
	f.AddAttachmentComment(owner, repo, 5)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func req(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": owner, "repo": repo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no text content: %+v", res.Content)
	return ""
}

func blob(res *mcp.CallToolResult) (mcp.BlobResourceContents, bool) {
	for _, c := range res.Content {
		if er, ok := c.(mcp.EmbeddedResource); ok {
			if br, ok := er.Resource.(mcp.BlobResourceContents); ok {
				return br, true
			}
		}
	}
	return mcp.BlobResourceContents{}, false
}

// ok calls fn and fails the test unless it succeeded.
func ok(t *testing.T, f fn, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := f(context.Background(), req(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return res
}

// fails calls fn and returns the error it reported, failing if it succeeded.
func fails(t *testing.T, f fn, args map[string]any) string {
	t.Helper()
	res, err := f(context.Background(), req(args))
	switch {
	case err != nil:
		return err.Error()
	case res != nil && res.IsError:
		return text(t, res)
	}
	t.Fatalf("expected an error, got %+v", res)
	return ""
}

func upload64(t *testing.T, h handlers, content, filename string) {
	t.Helper()
	ok(t, h.CreateIssueAttachmentFn, map[string]any{"index": 3.0, "content": b64(content), "filename": filename})
}

func TestIssueAttachmentLifecycle(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	res := ok(t, h.CreateIssueAttachmentFn, map[string]any{
		"index": 3.0, "content": b64("payload bytes"), "filename": "f.bin", "mime_type": "text/plain",
	})
	if body := text(t, res); !strings.Contains(body, `"name":"f.bin"`) || !strings.Contains(body, `"size":13`) {
		t.Fatalf("create: %s", body)
	}
	if body := text(t, ok(t, h.ListIssueAttachmentsFn, map[string]any{"index": 3.0})); !strings.Contains(body, `"f.bin"`) {
		t.Fatalf("list: %s", body)
	}
	if body := text(t, ok(t, h.GetIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0})); !strings.Contains(body, `"f.bin"`) {
		t.Fatalf("get: %s", body)
	}
	if body := text(t, ok(t, h.EditIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0, "name": "g.bin"})); !strings.Contains(body, `"g.bin"`) {
		t.Fatalf("edit: %s", body)
	}
	dl := ok(t, h.DownloadIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0})
	b, found := blob(dl)
	if !found || b.Blob != b64("payload bytes") || b.MIMEType != "text/plain" {
		t.Fatalf("download blob: %+v found=%v", b, found)
	}
	if !strings.Contains(text(t, dl), `"inline":true`) {
		t.Fatalf("download text: %s", text(t, dl))
	}
	if body := text(t, ok(t, h.DeleteIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0})); !strings.Contains(body, "deleted") {
		t.Fatalf("delete: %s", body)
	}
	if msg := fails(t, h.GetIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0}); !strings.Contains(msg, "get issue attachment err") {
		t.Fatalf("get after delete: %s", msg)
	}
}

func TestCommentAttachmentLifecycle(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	ok(t, h.CreateCommentAttachmentFn, map[string]any{"comment_id": 5.0, "content": b64("c"), "filename": "c.txt"})
	if body := text(t, ok(t, h.ListCommentAttachmentsFn, map[string]any{"comment_id": 5.0})); !strings.Contains(body, `"c.txt"`) {
		t.Fatalf("list: %s", body)
	}
	ok(t, h.GetCommentAttachmentFn, map[string]any{"comment_id": 5.0, "attachment_id": 1.0})
	ok(t, h.EditCommentAttachmentFn, map[string]any{"comment_id": 5.0, "attachment_id": 1.0, "name": "d.txt"})
	if b, found := blob(ok(t, h.DownloadCommentAttachmentFn, map[string]any{"comment_id": 5.0, "attachment_id": 1.0})); !found || b.Blob != b64("c") {
		t.Fatalf("download blob: %+v", b)
	}
	ok(t, h.DeleteCommentAttachmentFn, map[string]any{"comment_id": 5.0, "attachment_id": 1.0})
	if body := text(t, ok(t, h.ListCommentAttachmentsFn, map[string]any{"comment_id": 5.0})); strings.Contains(body, "d.txt") {
		t.Fatalf("still listed: %s", body)
	}
}

func TestListUnknownParentIsEmptyArray(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if body := text(t, ok(t, h.ListIssueAttachmentsFn, map[string]any{"index": 99.0})); !strings.Contains(body, "[]") {
		t.Fatalf("issue: %s", body)
	}
	if body := text(t, ok(t, h.ListCommentAttachmentsFn, map[string]any{"comment_id": 99.0})); !strings.Contains(body, "[]") {
		t.Fatalf("comment: %s", body)
	}
}

func TestDownloadOverCapReturnsMetadataOnly(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	upload64(t, h, strings.Repeat("x", int(forge.MaxInlineDownloadBytes)), "big.bin")
	res := ok(t, h.DownloadIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0})
	if _, found := blob(res); found {
		t.Fatal("blob must not be inlined at or above the cap")
	}
	if body := text(t, res); !strings.Contains(body, `"inline":false`) || !strings.Contains(body, "inline cap") {
		t.Fatalf("body: %s", body)
	}
}

// capLiar reports a small size but serves a body over the cap, as a server whose
// metadata is stale would.
type capLiar struct{ forge.Forgejo }

func (capLiar) DownloadAttachment(context.Context, string) (forge.Download, error) {
	return forge.Download{}, forge.ErrPayloadTooLarge
}

func TestDownloadBodyExceedingCapFallsBackGracefully(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	h := handlers{forge: fake.Factory{Forgejo: capLiar{f}}}
	upload64(t, handlers{forge: fake.Factory{Forgejo: f}}, "tiny", "t.bin")
	res := ok(t, h.DownloadIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0})
	if _, found := blob(res); found {
		t.Fatal("blob must not be inlined")
	}
	if body := text(t, res); !strings.Contains(body, `"inline":false`) || !strings.Contains(body, "exceeded inline cap") {
		t.Fatalf("body: %s", body)
	}
}

func TestCreateUsesFilePathAndBasename(t *testing.T) {
	t.Setenv("FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD", "1")
	h, _ := newFixture(t)
	p := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(p, []byte("from disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if body := text(t, ok(t, h.CreateIssueAttachmentFn, map[string]any{"index": 3.0, "file_path": p})); !strings.Contains(body, `"note.txt"`) {
		t.Fatalf("body: %s", body)
	}
}

func TestCreateRejectsBadInputBeforeUploading(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	cases := map[string]map[string]any{
		"both sources":  {"index": 3.0, "content": b64("x"), "file_path": "/etc/hostname", "filename": "f"},
		"not base64":    {"index": 3.0, "content": "not base64!!!", "filename": "f"},
		"no filename":   {"index": 3.0, "content": b64("x")},
		"missing index": {"content": b64("x"), "filename": "f"},
	}
	for name, args := range cases {
		fails(t, h.CreateIssueAttachmentFn, args)
		_ = name
	}
	if atts, _ := f.ListIssueAttachments(context.Background(), owner, repo, 3); len(atts) != 0 {
		t.Fatalf("nothing should have been uploaded: %+v", atts)
	}
}

// TestCreateRejectsOversizedContent lowers the upload cap, a package variable in
// pkg/upload, so it does not run in parallel. The oversized argument is small
// because the cap is lowered, instead of a 90 MiB string.
func TestCreateRejectsOversizedContent(t *testing.T) {
	restore := upload.SetContentLimitForTest(1024)
	t.Cleanup(restore)
	h, f := newFixture(t)
	oversized := strings.Repeat("A", 1025)
	if msg := fails(t, h.CreateIssueAttachmentFn, map[string]any{"index": 3.0, "content": oversized, "filename": "f"}); !strings.Contains(msg, "too large") {
		t.Fatalf("issue: want a size-limit error, got %s", msg)
	}
	if msg := fails(t, h.CreateCommentAttachmentFn, map[string]any{"comment_id": 5.0, "content": oversized, "filename": "f"}); !strings.Contains(msg, "too large") {
		t.Fatalf("comment: want a size-limit error, got %s", msg)
	}
	if atts, _ := f.ListIssueAttachments(context.Background(), owner, repo, 3); len(atts) != 0 {
		t.Fatalf("nothing should have been uploaded: %+v", atts)
	}
}

func TestEditRequiresName(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if msg := fails(t, h.EditIssueAttachmentFn, map[string]any{"index": 3.0, "attachment_id": 1.0}); !strings.Contains(msg, "name is required") {
		t.Fatal(msg)
	}
	if msg := fails(t, h.EditCommentAttachmentFn, map[string]any{"comment_id": 5.0, "attachment_id": 1.0}); !strings.Contains(msg, "name is required") {
		t.Fatal(msg)
	}
}

func TestHandlersRejectMissingNumericArgs(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	fails(t, h.GetIssueAttachmentFn, map[string]any{})
	fails(t, h.GetCommentAttachmentFn, map[string]any{})
}

func TestCredentialPolicyErrorIsReported(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: errors.New("no credential")}}
	if msg := fails(t, h.ListIssueAttachmentsFn, map[string]any{"index": 3.0}); !strings.Contains(msg, "no credential") {
		t.Fatal(msg)
	}
}

func withShortUploadTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := attachmentUploadTimeout
	attachmentUploadTimeout = d
	t.Cleanup(func() { attachmentUploadTimeout = orig })
}

// These tests change a package variable, so they do not run in parallel.

func TestUploadTimeout(t *testing.T) {
	withShortUploadTimeout(t, 100*time.Millisecond)
	h, f := newFixture(t)
	f.HangUploads()

	for name, call := range map[string]func() string{
		"issue": func() string {
			return fails(t, h.CreateIssueAttachmentFn, map[string]any{"index": 3.0, "content": b64("hang"), "filename": "f.bin"})
		},
		"comment": func() string {
			return fails(t, h.CreateCommentAttachmentFn, map[string]any{"comment_id": 5.0, "content": b64("hang"), "filename": "f.bin"})
		},
	} {
		start := time.Now()
		msg := call()
		if !strings.Contains(msg, "upload timed out after") {
			t.Fatalf("%s: want a clear timeout error, got %s", name, msg)
		}
		if time.Since(start) > time.Second {
			t.Fatalf("%s: took %s, want bounded", name, time.Since(start))
		}
	}
}

func TestParentDeadlineNotMisattributed(t *testing.T) {
	withShortUploadTimeout(t, 5*time.Second)
	h, f := newFixture(t)
	f.HangUploads()
	parent, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := h.CreateIssueAttachmentFn(parent, req(map[string]any{"index": 3.0, "content": b64("hang"), "filename": "f.bin"}))
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if res != nil && res.IsError {
		msg = text(t, res)
	} else {
		t.Fatal("expected an error")
	}
	if strings.Contains(msg, "upload timed out after") {
		t.Fatalf("caller-deadline expiry misattributed: %s", msg)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %s, want the parent's 20ms bound", time.Since(start))
	}
}
