// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// These tests pin the wire contract of every attachment tool: its tool
// definition (schema snapshot) and its output and error text (golden), with the
// calls the tool made on the Forgejo port. They run against a stub port with
// fixed data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	ListIssueAttachmentsTool, GetIssueAttachmentTool, DownloadIssueAttachmentTool,
	CreateIssueAttachmentTool, EditIssueAttachmentTool, DeleteIssueAttachmentTool,
	ListCommentAttachmentsTool, GetCommentAttachmentTool, DownloadCommentAttachmentTool,
	CreateCommentAttachmentTool, EditCommentAttachmentTool, DeleteCommentAttachmentTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func fixedAttachment(size int64) *forgejo_sdk.Attachment {
	return &forgejo_sdk.Attachment{ID: 7, Name: "shot.png", Size: size, DownloadCount: 1, Created: t0, UUID: "u-7", DownloadURL: "https://f.example/dl/7"}
}

// stubAttach is a forge.Forgejo whose attachment methods return fixed data and
// record their calls. Any other domain would panic on the nil embed.
type stubAttach struct {
	forge.Forgejo
	mu      sync.Mutex
	calls   []string
	fail    error
	attSize int64
	dlErr   error
	dlBody  string
	// notFound makes the list calls answer 404.
	notFound bool
}

// MaxResponseItems reports no ceiling, so the list default applies.
func (s *stubAttach) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func (s *stubAttach) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubAttach) size() int64 {
	if s.attSize != 0 {
		return s.attSize
	}
	return 12
}

func (s *stubAttach) listErr(err error) ([]*forgejo_sdk.Attachment, error) {
	if s.notFound {
		return nil, &forge.StatusError{StatusCode: 404, Err: errors.New("not found")}
	}
	return []*forgejo_sdk.Attachment{fixedAttachment(5)}, err
}

func (s *stubAttach) ListIssueAttachments(_ context.Context, o, r string, i int64) ([]*forgejo_sdk.Attachment, error) {
	return s.listErr(s.rec("ListIssueAttachments", o, r, i))
}

func (s *stubAttach) GetIssueAttachment(_ context.Context, o, r string, i, id int64) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(s.size()), s.rec("GetIssueAttachment", o, r, i, id)
}

func (s *stubAttach) CreateIssueAttachment(_ context.Context, o, r string, i int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	body, _ := io.ReadAll(up.Content)
	return fixedAttachment(int64(len(body))), s.rec("CreateIssueAttachment", o, r, i, up.Filename, up.MIMEType, string(body))
}

func (s *stubAttach) EditIssueAttachment(_ context.Context, o, r string, i, id int64, name string) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(12), s.rec("EditIssueAttachment", o, r, i, id, name)
}

func (s *stubAttach) DeleteIssueAttachment(_ context.Context, o, r string, i, id int64) error {
	return s.rec("DeleteIssueAttachment", o, r, i, id)
}

func (s *stubAttach) ListCommentAttachments(_ context.Context, o, r string, c int64) ([]*forgejo_sdk.Attachment, error) {
	return s.listErr(s.rec("ListCommentAttachments", o, r, c))
}

func (s *stubAttach) GetCommentAttachment(_ context.Context, o, r string, c, id int64) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(s.size()), s.rec("GetCommentAttachment", o, r, c, id)
}

func (s *stubAttach) CreateCommentAttachment(_ context.Context, o, r string, c int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	body, _ := io.ReadAll(up.Content)
	return fixedAttachment(int64(len(body))), s.rec("CreateCommentAttachment", o, r, c, up.Filename, up.MIMEType, string(body))
}

func (s *stubAttach) EditCommentAttachment(_ context.Context, o, r string, c, id int64, name string) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(12), s.rec("EditCommentAttachment", o, r, c, id, name)
}

func (s *stubAttach) DeleteCommentAttachment(_ context.Context, o, r string, c, id int64) error {
	return s.rec("DeleteCommentAttachment", o, r, c, id)
}

func (s *stubAttach) DownloadAttachment(_ context.Context, url string) (forge.Download, error) {
	if err := s.rec("DownloadAttachment", url); err != nil {
		return forge.Download{}, err
	}
	if s.dlErr != nil {
		return forge.Download{}, s.dlErr
	}
	return forge.Download{Body: []byte(s.dlBody), ContentType: "text/plain"}, nil
}

// invoke calls a registered tool and renders what the client would see: the
// text and any embedded resource, or the error, then the port calls made.
func invoke(t *testing.T, stub *stubAttach, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k)
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
		var parts []string
		for _, c := range res.Content {
			switch v := c.(type) {
			case mcp.TextContent:
				parts = append(parts, v.Text)
			case mcp.EmbeddedResource:
				if b, ok := v.Resource.(mcp.BlobResourceContents); ok {
					parts = append(parts, fmt.Sprintf("RESOURCE %s %s blob=%s", b.URI, b.MIMEType, b.Blob))
				}
			}
		}
		out = strings.Join(parts, "\n")
	}
	var buf bytes.Buffer
	buf.WriteString(out)
	fmt.Fprintf(&buf, "\n-- port calls --\n%s\n", strings.Join(stub.calls, "\n"))
	return buf.String()
}

type goldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubAttach)
}

var goldenCases = []goldenCase{
	{name: "issue_list", tool: ListIssueAttachmentsToolName, args: map[string]any{"index": float64(4)}},
	{name: "issue_list_not_found_is_empty", tool: ListIssueAttachmentsToolName, args: map[string]any{"index": float64(4)}, stub: func(s *stubAttach) { s.notFound = true }},
	{name: "issue_list_missing_index", tool: ListIssueAttachmentsToolName, args: map[string]any{"index": nil}},
	{name: "issue_list_port_error", tool: ListIssueAttachmentsToolName, args: map[string]any{"index": float64(4)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "issue_get", tool: GetIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}},
	{name: "issue_get_missing_id", tool: GetIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": nil}},
	{name: "issue_get_zero_attachment_id", tool: GetIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(0)}},
	{name: "issue_get_fractional_attachment_id", tool: GetIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": 7.5}},
	{name: "issue_list_fractional_index", tool: ListIssueAttachmentsToolName, args: map[string]any{"index": 4.5}},
	{name: "comment_get_negative_attachment_id", tool: GetCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(-7)}},
	{name: "issue_get_port_error", tool: GetIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "issue_download_inline", tool: DownloadIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.dlBody = "png" }},
	{name: "issue_download_over_cap", tool: DownloadIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.attSize = 2 << 20 }},
	{name: "issue_download_body_too_large", tool: DownloadIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.dlErr = forge.ErrPayloadTooLarge }},
	{name: "issue_download_fetch_error", tool: DownloadIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.dlErr = errors.New("boom") }},
	{name: "issue_download_metadata_error", tool: DownloadIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "issue_create_content", tool: CreateIssueAttachmentToolName, args: map[string]any{"index": float64(4), "content": "aGk=", "filename": "a.txt", "mime_type": "text/plain"}},
	{name: "issue_create_no_source", tool: CreateIssueAttachmentToolName, args: map[string]any{"index": float64(4)}},
	{name: "issue_create_port_error", tool: CreateIssueAttachmentToolName, args: map[string]any{"index": float64(4), "content": "aGk=", "filename": "a.txt"}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "issue_edit", tool: EditIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7), "name": "new.png"}},
	{name: "issue_edit_missing_name", tool: EditIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7), "name": nil}},
	{name: "issue_edit_port_error", tool: EditIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7), "name": "x"}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "issue_delete", tool: DeleteIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}},
	{name: "issue_delete_port_error", tool: DeleteIssueAttachmentToolName, args: map[string]any{"index": float64(4), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},

	{name: "comment_list", tool: ListCommentAttachmentsToolName, args: map[string]any{"comment_id": float64(9)}},
	{name: "comment_list_not_found_is_empty", tool: ListCommentAttachmentsToolName, args: map[string]any{"comment_id": float64(9)}, stub: func(s *stubAttach) { s.notFound = true }},
	{name: "comment_list_missing_id", tool: ListCommentAttachmentsToolName, args: map[string]any{"comment_id": nil}},
	{name: "comment_list_port_error", tool: ListCommentAttachmentsToolName, args: map[string]any{"comment_id": float64(9)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "comment_get", tool: GetCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}},
	{name: "comment_get_missing_id", tool: GetCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": nil}},
	{name: "comment_get_port_error", tool: GetCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "comment_download_inline", tool: DownloadCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.dlBody = "png" }},
	{name: "comment_download_over_cap", tool: DownloadCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.attSize = 2 << 20 }},
	{name: "comment_download_fetch_error", tool: DownloadCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.dlErr = errors.New("boom") }},
	{name: "comment_download_metadata_error", tool: DownloadCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "comment_create_content", tool: CreateCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "content": "aGk=", "filename": "a.txt"}},
	{name: "comment_create_bad_comment_id", tool: CreateCommentAttachmentToolName, args: map[string]any{"comment_id": nil, "content": "aGk=", "filename": "a.txt"}},
	{name: "comment_create_port_error", tool: CreateCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "content": "aGk=", "filename": "a.txt"}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "comment_edit", tool: EditCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7), "name": "new.png"}},
	{name: "comment_edit_missing_name", tool: EditCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7), "name": nil}},
	{name: "comment_edit_port_error", tool: EditCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7), "name": "x"}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
	{name: "comment_delete", tool: DeleteCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}},
	{name: "comment_delete_port_error", tool: DeleteCommentAttachmentToolName, args: map[string]any{"comment_id": float64(9), "attachment_id": float64(7)}, stub: func(s *stubAttach) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubAttach{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.tool, c.args)))
		})
	}
}
