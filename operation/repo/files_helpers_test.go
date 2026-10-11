// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"encoding/json"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests for the file, commit and status tools run against the in-memory
// fake, build their own, and so run in parallel with no shared state.

const (
	filesOwner = "octo"
	filesRepo  = "hello"
)

// newFilesFixture returns handlers served by a fresh fake that already has the
// repository filesOwner/filesRepo.
func newFilesFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(filesOwner, filesRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

// filesReq builds a tool request for filesOwner/filesRepo plus args.
func filesReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": filesOwner, "repo": filesRepo}
	for k, v := range args {
		base[k] = v
	}
	return newCallToolRequest(base)
}

// resultText unwraps the {"Result":"..."} wrapper to.TextResult puts around a
// string payload.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	var wrapper struct{ Result string }
	if err := json.Unmarshal([]byte(extractText(t, result)), &wrapper); err != nil {
		t.Fatalf("response is not a {Result} wrapper: %v\n  got: %q", err, extractText(t, result))
	}
	return wrapper.Result
}

// seedFile puts a file on main.
func seedFile(t *testing.T, f *fake.Forgejo, filePath, content string) {
	t.Helper()
	if err := f.SetFile(filesOwner, filesRepo, "main", filePath, content); err != nil {
		t.Fatalf("seeding %s: %v", filePath, err)
	}
}
