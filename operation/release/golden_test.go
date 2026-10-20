// SPDX-License-Identifier: GPL-3.0-or-later

package release

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

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every release tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port that
// returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	ListReleasesTool, GetReleaseByIDTool, GetReleaseByTagTool, GetLatestReleaseTool,
	CreateReleaseTool, EditReleaseTool, DeleteReleaseTool, DeleteReleaseByTagTool,
	ListReleaseAttachmentsTool, GetReleaseAttachmentTool, DownloadReleaseAttachmentTool,
	CreateReleaseAttachmentTool, EditReleaseAttachmentTool, DeleteReleaseAttachmentTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0  = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1  = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	usr = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedAttachment(size int64) *forgejo_sdk.Attachment {
	return &forgejo_sdk.Attachment{ID: 7, Name: "bin.tgz", Size: size, DownloadCount: 2, Created: t0, UUID: "u-1", DownloadURL: "https://f.example/dl/7"}
}

func fixedRelease(tag string, draft, pre bool) *forgejo_sdk.Release {
	return &forgejo_sdk.Release{
		ID: 3, TagName: tag, Target: "main", Title: "One", Note: "notes",
		URL: "https://f.example/api/v1/releases/3", HTMLURL: "https://f.example/octo/hello/releases/tag/" + tag,
		TarURL: "https://f.example/tar", ZipURL: "https://f.example/zip",
		IsDraft: draft, IsPrerelease: pre, CreatedAt: t0, PublishedAt: t1,
		Publisher: usr, Attachments: []*forgejo_sdk.Attachment{fixedAttachment(12)},
	}
}

// stubRelease is a forge.Forgejo whose release and download methods return
// fixed data and record the calls they receive. Any other domain would panic on
// the nil embed.
type stubRelease struct {
	forge.Forgejo
	mu      sync.Mutex
	calls   []string
	fail    error
	attSize int64
	// dlErr is what DownloadAttachment returns; dlBody is its body.
	dlErr  error
	dlBody string
}

func (s *stubRelease) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubRelease) ListReleases(_ context.Context, o, r string, opt forgejo_sdk.ListReleasesOptions) ([]*forgejo_sdk.Release, error) {
	return []*forgejo_sdk.Release{fixedRelease("v-draft", true, false), fixedRelease("v-pre", false, true), fixedRelease("v-pub", false, false), nil},
		s.rec("ListReleases", o, r, opt)
}

func (s *stubRelease) GetRelease(_ context.Context, o, r string, id int64) (*forgejo_sdk.Release, error) {
	return fixedRelease("v1", false, false), s.rec("GetRelease", o, r, id)
}

func (s *stubRelease) GetReleaseByTag(_ context.Context, o, r, tag string) (*forgejo_sdk.Release, error) {
	return fixedRelease(tag, false, false), s.rec("GetReleaseByTag", o, r, tag)
}

func (s *stubRelease) GetLatestRelease(_ context.Context, o, r string) (*forgejo_sdk.Release, error) {
	return fixedRelease("v9", false, false), s.rec("GetLatestRelease", o, r)
}

func (s *stubRelease) CreateRelease(_ context.Context, o, r string, opt forgejo_sdk.CreateReleaseOption) (*forgejo_sdk.Release, error) {
	return fixedRelease(opt.TagName, opt.IsDraft, opt.IsPrerelease), s.rec("CreateRelease", o, r, opt)
}

func (s *stubRelease) EditRelease(_ context.Context, o, r string, id int64, opt forgejo_sdk.EditReleaseOption) (*forgejo_sdk.Release, error) {
	return fixedRelease("edited", false, false), s.rec("EditRelease", o, r, id, opt)
}

func (s *stubRelease) DeleteRelease(_ context.Context, o, r string, id int64) error {
	return s.rec("DeleteRelease", o, r, id)
}

func (s *stubRelease) DeleteReleaseByTag(_ context.Context, o, r, tag string) error {
	return s.rec("DeleteReleaseByTag", o, r, tag)
}

func (s *stubRelease) ListReleaseAttachments(_ context.Context, o, r string, id int64) ([]*forgejo_sdk.Attachment, error) {
	var out []*forgejo_sdk.Attachment
	for i := 0; i < 3; i++ {
		a := *fixedAttachment(5)
		a.ID = int64(10 + i)
		out = append(out, &a)
	}
	return out, s.rec("ListReleaseAttachments", o, r, id)
}

func (s *stubRelease) GetReleaseAttachment(_ context.Context, o, r string, rid, id int64) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(s.size()), s.rec("GetReleaseAttachment", o, r, rid, id)
}

func (s *stubRelease) size() int64 {
	if s.attSize != 0 {
		return s.attSize
	}
	return 12
}

func (s *stubRelease) CreateReleaseAttachment(_ context.Context, o, r string, rid int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	body, _ := io.ReadAll(up.Content)
	return fixedAttachment(int64(len(body))), s.rec("CreateReleaseAttachment", o, r, rid, up.Filename, up.MIMEType, string(body))
}

func (s *stubRelease) EditReleaseAttachment(_ context.Context, o, r string, rid, id int64, name string) (*forgejo_sdk.Attachment, error) {
	return fixedAttachment(12), s.rec("EditReleaseAttachment", o, r, rid, id, name)
}

func (s *stubRelease) DeleteReleaseAttachment(_ context.Context, o, r string, rid, id int64) error {
	return s.rec("DeleteReleaseAttachment", o, r, rid, id)
}

func (s *stubRelease) DownloadAttachment(_ context.Context, url string) (forge.Download, error) {
	if err := s.rec("DownloadAttachment", url); err != nil {
		return forge.Download{}, err
	}
	if s.dlErr != nil {
		return forge.Download{}, s.dlErr
	}
	return forge.Download{Body: []byte(s.dlBody), ContentType: "text/plain"}, nil
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, any embedded resource, or the error, then the
// port calls made.
func invoke(t *testing.T, stub *stubRelease, tool string, args map[string]any) string {
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
	stub func(*stubRelease)
}

var goldenCases = []goldenCase{
	{name: "list_defaults", tool: ListReleasesToolName},
	{name: "list_draft_filtered", tool: ListReleasesToolName, args: map[string]any{"state": "draft", "page": float64(2), "limit": float64(5)}},
	{name: "list_prerelease", tool: ListReleasesToolName, args: map[string]any{"state": "prerelease"}},
	{name: "list_published", tool: ListReleasesToolName, args: map[string]any{"state": "published"}},
	{name: "list_bad_state", tool: ListReleasesToolName, args: map[string]any{"state": "archived"}},
	{name: "list_fractional_paging", tool: ListReleasesToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "list_port_error", tool: ListReleasesToolName, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "get_by_id", tool: GetReleaseByIDToolName, args: map[string]any{"release_id": float64(3)}},
	{name: "get_by_id_missing", tool: GetReleaseByIDToolName, args: map[string]any{"release_id": nil}},
	{name: "get_by_id_port_error", tool: GetReleaseByIDToolName, args: map[string]any{"release_id": float64(3)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "get_by_tag", tool: GetReleaseByTagToolName, args: map[string]any{"tag": "v1.2.3"}},
	{name: "get_by_tag_missing", tool: GetReleaseByTagToolName, args: map[string]any{"tag": nil}},
	{name: "get_by_tag_port_error", tool: GetReleaseByTagToolName, args: map[string]any{"tag": "v1"}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "latest", tool: GetLatestReleaseToolName},
	{name: "latest_port_error", tool: GetLatestReleaseToolName, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "create_defaults_name", tool: CreateReleaseToolName, args: map[string]any{"tag_name": "v2", "target_commitish": "main", "body": "b", "draft": true, "prerelease": false}},
	{name: "create_named", tool: CreateReleaseToolName, args: map[string]any{"tag_name": "v2", "name": "Two", "prerelease": true}},
	{name: "create_missing_tag", tool: CreateReleaseToolName, args: map[string]any{"tag_name": nil}},
	{name: "create_port_error", tool: CreateReleaseToolName, args: map[string]any{"tag_name": "v2"}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "edit_partial", tool: EditReleaseToolName, args: map[string]any{"release_id": float64(3), "tag_name": "v3"}},
	{name: "edit_flags", tool: EditReleaseToolName, args: map[string]any{"release_id": float64(3), "draft": false, "prerelease": true, "name": "N", "body": "B", "target_commitish": "dev"}},
	{name: "edit_missing_id", tool: EditReleaseToolName, args: map[string]any{"release_id": nil}},
	{name: "edit_port_error", tool: EditReleaseToolName, args: map[string]any{"release_id": float64(3)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "delete_ok", tool: DeleteReleaseToolName, args: map[string]any{"release_id": float64(3)}},
	{name: "delete_missing_id", tool: DeleteReleaseToolName, args: map[string]any{"release_id": nil}},
	{name: "delete_port_error", tool: DeleteReleaseToolName, args: map[string]any{"release_id": float64(3)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},
	{name: "delete_by_tag_ok", tool: DeleteReleaseByTagToolName, args: map[string]any{"tag": "v1"}},
	{name: "delete_by_tag_missing", tool: DeleteReleaseByTagToolName, args: map[string]any{"tag": nil}},
	{name: "delete_by_tag_port_error", tool: DeleteReleaseByTagToolName, args: map[string]any{"tag": "v1"}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "assets_defaults", tool: ListReleaseAttachmentsToolName, args: map[string]any{"release_id": float64(3)}},
	{name: "assets_paged", tool: ListReleaseAttachmentsToolName, args: map[string]any{"release_id": float64(3), "page": float64(2), "limit": float64(2)}},
	{name: "assets_past_end", tool: ListReleaseAttachmentsToolName, args: map[string]any{"release_id": float64(3), "page": float64(9)}},
	{name: "assets_missing_id", tool: ListReleaseAttachmentsToolName, args: map[string]any{"release_id": nil}},
	{name: "assets_port_error", tool: ListReleaseAttachmentsToolName, args: map[string]any{"release_id": float64(3)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "asset_get", tool: GetReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)}},
	{name: "asset_get_missing_attachment", tool: GetReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": nil}},
	{name: "asset_get_port_error", tool: GetReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "download_inline", tool: DownloadReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)},
		stub: func(s *stubRelease) { s.dlBody = "hello" }},
	{name: "download_over_cap_metadata_only", tool: DownloadReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)},
		stub: func(s *stubRelease) { s.attSize = 2 << 20 }},
	{name: "download_body_too_large", tool: DownloadReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)},
		stub: func(s *stubRelease) { s.dlErr = forge.ErrPayloadTooLarge }},
	{name: "download_fetch_error", tool: DownloadReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)},
		stub: func(s *stubRelease) { s.dlErr = errors.New("boom") }},
	{name: "download_metadata_error", tool: DownloadReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)},
		stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "asset_create_content", tool: CreateReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "content": "aGk=", "filename": "a.txt", "mime_type": "text/plain"}},
	{name: "asset_create_no_source", tool: CreateReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3)}},
	{name: "asset_create_bad_base64", tool: CreateReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "content": "!!", "filename": "a.txt"}},
	{name: "asset_create_port_error", tool: CreateReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "content": "aGk=", "filename": "a.txt"}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "asset_edit", tool: EditReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7), "name": "new.tgz"}},
	{name: "asset_edit_missing_name", tool: EditReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7), "name": nil}},
	{name: "asset_edit_port_error", tool: EditReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7), "name": "x"}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},

	{name: "asset_delete", tool: DeleteReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)}},
	{name: "asset_delete_missing_id", tool: DeleteReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": nil}},
	{name: "asset_delete_port_error", tool: DeleteReleaseAttachmentToolName, args: map[string]any{"release_id": float64(3), "attachment_id": float64(7)}, stub: func(s *stubRelease) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubRelease{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the release goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubRelease) MaxResponseItems(context.Context) (int, bool) { return 0, false }
