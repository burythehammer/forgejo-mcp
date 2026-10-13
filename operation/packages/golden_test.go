// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every package tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port with
// fixed data: five files, so the file paging is exercised for real.

var allTools = []mcp.Tool{ListPackagesTool, GetPackageTool, DeletePackageTool, ListPackageFilesTool}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

func fixedVersion() forge.PackageVersion {
	return forge.PackageVersion{
		ID: 3, Type: "generic", Name: "tool", Version: "1.0", HTMLURL: "https://f.example/pkg/3",
		CreatedAt: "2026-01-02T03:04:05Z", Repository: "octo/hello",
	}
}

func fixedFiles() []forge.PackageFile {
	return []forge.PackageFile{
		{ID: 1, Name: "a.tgz", Size: 10, SHA256: "sha-a"},
		{ID: 2, Name: "b.tgz", Size: 20, SHA256: "sha-b"},
		{ID: 3, Name: "c.tgz", Size: 30},
		{ID: 4, Name: "d.tgz", Size: 40, SHA256: "sha-d"},
		{ID: 5, Name: "e.tgz", Size: 50, SHA256: "sha-e"},
	}
}

// stubPackages is a forge.Forgejo whose package domain returns fixed data and
// records the calls it receives. Any other domain would panic on the nil embed.
type stubPackages struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	// fail is returned by every call. noTotal drops the total; hasNext sets
	// the page's next-page flag.
	fail    error
	noTotal bool
	hasNext bool
}

func (s *stubPackages) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubPackages) ListPackages(_ context.Context, owner string, opt forge.ListPackagesOptions) (forge.PackageList, error) {
	if err := s.rec("ListPackages", owner, opt); err != nil {
		return forge.PackageList{}, err
	}
	list := forge.PackageList{Packages: []forge.PackageVersion{fixedVersion(), fixedVersion()}, HasNext: s.hasNext}
	if !s.noTotal {
		n := 2
		list.TotalCount = &n
	}
	return list, nil
}

func (s *stubPackages) GetPackage(_ context.Context, owner, packageType, name, version string) (*forge.PackageVersion, error) {
	v := fixedVersion()
	return &v, s.rec("GetPackage", owner, packageType, name, version)
}

func (s *stubPackages) DeletePackage(_ context.Context, owner, packageType, name, version string) error {
	return s.rec("DeletePackage", owner, packageType, name, version)
}

func (s *stubPackages) ListPackageFiles(_ context.Context, owner, packageType, name, version string) ([]forge.PackageFile, error) {
	return fixedFiles(), s.rec("ListPackageFiles", owner, packageType, name, version)
}

// invokePkg calls a registered tool the way the server does and renders what
// the client would see: the text, or the error, then the port calls made.
func invokePkg(t *testing.T, stub *stubPackages, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo"}
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

type pkgGoldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubPackages)
}

var coords = map[string]any{"type": "generic", "name": "tool", "version": "1.0"}

func with(extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range coords {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

var pkgGoldenCases = []pkgGoldenCase{
	{name: "list_defaults", tool: ListPackagesToolName},
	{name: "list_filtered", tool: ListPackagesToolName, args: map[string]any{"type": "npm", "q": "tool", "page": float64(2), "limit": float64(5)}},
	{name: "list_string_numbers_accepted", tool: ListPackagesToolName, args: map[string]any{"page": "2", "limit": "5"}},
	{name: "list_empty_numbers_default", tool: ListPackagesToolName, args: map[string]any{"page": "", "limit": ""}},
	{name: "list_has_next_no_total", tool: ListPackagesToolName, args: map[string]any{"limit": float64(50)}, stub: func(s *stubPackages) { s.hasNext = true; s.noTotal = true }},
	{name: "list_fractional_page", tool: ListPackagesToolName, args: map[string]any{"page": 2.5}},
	{name: "list_page_zero", tool: ListPackagesToolName, args: map[string]any{"page": float64(0)}},
	{name: "list_limit_over_max", tool: ListPackagesToolName, args: map[string]any{"limit": float64(51)}},
	{name: "list_limit_not_a_number", tool: ListPackagesToolName, args: map[string]any{"limit": "many"}},
	{name: "list_missing_owner", tool: ListPackagesToolName, args: map[string]any{"owner": nil}},
	{name: "list_port_error", tool: ListPackagesToolName, stub: func(s *stubPackages) { s.fail = errors.New("boom") }},

	{name: "get_ok", tool: GetPackageToolName, args: coords},
	{name: "get_missing_type", tool: GetPackageToolName, args: with(map[string]any{"type": nil})},
	{name: "get_missing_name", tool: GetPackageToolName, args: with(map[string]any{"name": nil})},
	{name: "get_missing_version", tool: GetPackageToolName, args: with(map[string]any{"version": nil})},
	{name: "get_missing_owner", tool: GetPackageToolName, args: with(map[string]any{"owner": nil})},
	{name: "get_port_error", tool: GetPackageToolName, args: coords, stub: func(s *stubPackages) { s.fail = errors.New("boom") }},

	{name: "delete_ok", tool: DeletePackageToolName, args: coords},
	{name: "delete_missing_version", tool: DeletePackageToolName, args: with(map[string]any{"version": nil})},
	{name: "delete_port_error", tool: DeletePackageToolName, args: coords, stub: func(s *stubPackages) { s.fail = errors.New("boom") }},

	{name: "files_defaults", tool: ListPackageFilesToolName, args: coords},
	{name: "files_paged", tool: ListPackageFilesToolName, args: with(map[string]any{"page": float64(2), "limit": float64(2)})},
	{name: "files_last_page", tool: ListPackageFilesToolName, args: with(map[string]any{"page": float64(3), "limit": float64(2)})},
	{name: "files_past_end", tool: ListPackageFilesToolName, args: with(map[string]any{"page": float64(9), "limit": float64(2)})},
	{name: "files_string_limit_accepted", tool: ListPackageFilesToolName, args: with(map[string]any{"limit": "2"})},
	{name: "files_bad_page", tool: ListPackageFilesToolName, args: with(map[string]any{"page": float64(0)})},
	{name: "files_missing_name", tool: ListPackageFilesToolName, args: with(map[string]any{"name": nil})},
	{name: "files_port_error", tool: ListPackageFilesToolName, args: coords, stub: func(s *stubPackages) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range pkgGoldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubPackages{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invokePkg(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the package goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubPackages) MaxResponseItems(context.Context) (int, bool) { return 0, false }
