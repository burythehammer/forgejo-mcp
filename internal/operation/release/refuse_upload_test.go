// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// An upload with a bad source is refused as that source error before the
// credential lookup, so a refusing credential never shows.
func TestUploadRefusesBadSourceBeforeCredential(t *testing.T) {
	t.Parallel()
	refuse := errors.New("no credential for this request")
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Err: refuse})

	cases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"non-string content", CreateReleaseAttachmentTool.Name, map[string]any{"owner": "o", "repo": "r", "release_id": 1, "content": 5, "filename": "f"}, "content must be a string"},
		{"non-string file_path", CreateReleaseAttachmentTool.Name, map[string]any{"owner": "o", "repo": "r", "release_id": 1, "file_path": 5}, "file_path must be a string"},
		{"null content", CreateReleaseAttachmentTool.Name, map[string]any{"owner": "o", "repo": "r", "release_id": 1, "content": nil, "filename": "f"}, "content must be a string"},
		{"neither content nor file_path", CreateReleaseAttachmentTool.Name, map[string]any{"owner": "o", "repo": "r", "release_id": 1}, "exactly one of content or file_path is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: c.tool, Arguments: c.args}}
			res, err := s.GetTool(c.tool).Handler(context.Background(), req)
			msg := ""
			switch {
			case err != nil:
				msg = err.Error()
			case res != nil && res.IsError && len(res.Content) > 0:
				if text, ok := res.Content[0].(mcp.TextContent); ok {
					msg = text.Text
				}
			}
			if !strings.Contains(msg, c.want) {
				t.Errorf("got %q, want the source error %q", msg, c.want)
			}
			if strings.Contains(msg, refuse.Error()) {
				t.Errorf("reached the credential lookup: %q", msg)
			}
		})
	}
}
