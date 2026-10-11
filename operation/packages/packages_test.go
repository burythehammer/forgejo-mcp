// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const testOwner = "o"

// newFixture returns handlers served by a fresh fake that already has the
// package owner testOwner.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddPackageOwner(testOwner)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func addCore(f *fake.Forgejo) {
	f.AddPackage(testOwner, forge.PackageVersion{
		Type: "container", Name: "core", Version: "1.0.0",
		HTMLURL: "https://example/packages/1", CreatedAt: "2026-08-01T00:00:00Z", Repository: "alice/core",
	})
}

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func decodePackageResult[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	var envelope struct {
		Result T `json:"Result"`
	}
	if err := json.Unmarshal([]byte(resultText(t, result)), &envelope); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, resultText(t, result))
	}
	return envelope.Result
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return text.Text
}

// noTotal is a port whose listing carries no X-Total-Count.
type noTotal struct{ *fake.Forgejo }

func (n noTotal) ListPackages(ctx context.Context, owner string, opt forge.ListPackagesOptions) (forge.PackageList, error) {
	l, err := n.Forgejo.ListPackages(ctx, owner, opt)
	l.TotalCount = nil
	return l, err
}

func TestListPackagesFn_QueryAndEnvelope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	addCore(f)
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1"})

	result, err := h.ListPackagesFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "container", "q": "core", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if decoded.Page != 1 || decoded.Limit != 30 || decoded.Count != 1 {
		t.Fatalf("envelope: %+v", decoded)
	}
	if decoded.TotalCount == nil || *decoded.TotalCount != 1 {
		t.Fatalf("total_count: %+v", decoded.TotalCount)
	}
	if decoded.HasNext {
		t.Fatalf("has_next on the last page: %+v", decoded)
	}
	if len(decoded.Packages) != 1 || decoded.Packages[0].Name != "core" || decoded.Packages[0].HTMLURL == "" {
		t.Fatalf("packages: %+v", decoded.Packages)
	}
	if decoded.Packages[0].Repository != "alice/core" {
		t.Fatalf("repository: %+v", decoded.Packages[0])
	}
}

func TestListPackagesFn_OmitsTotalCountWhenNotReported(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	addCore(f)
	h := handlers{forge: fake.Factory{Forgejo: noTotal{f}}}

	result, err := h.ListPackagesFn(context.Background(), newCallToolRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text := resultText(t, result); strings.Contains(text, "total_count") {
		t.Fatalf("total_count must be omitted when not reported: %s", text)
	}
}

func TestListPackagesFn_EmptyOwnerListsEmptyArray(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	result, err := h.ListPackagesFn(context.Background(), newCallToolRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if strings.Contains(text, `"packages":null`) {
		t.Fatalf("packages must be [] not null: %s", text)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if decoded.Count != 0 || decoded.Packages == nil || decoded.HasNext {
		t.Fatalf("empty page: %+v", decoded)
	}
}

func TestListPackagesFn_MissingOwnerIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	result, err := h.ListPackagesFn(context.Background(), newCallToolRequest(map[string]any{"owner": "missing"}))
	if !forge.IsNotFound(err) {
		t.Fatalf("404 owner must be a not-found error, not an empty list; got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestListPackagesFn_HasNextFromServer(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	addCore(f)
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1"})

	result, err := h.ListPackagesFn(context.Background(), newCallToolRequest(map[string]any{"limit": float64(1)}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[listPackagesResult](t, result)
	if !decoded.HasNext || decoded.Count != 1 || decoded.TotalCount == nil || *decoded.TotalCount != 2 {
		t.Fatalf("envelope: %+v", decoded)
	}
}

func TestListPackagesFn_PageZeroIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	if _, err := h.ListPackagesFn(context.Background(), newCallToolRequest(map[string]any{"page": float64(0)})); err == nil {
		t.Fatal("page=0 must be an error")
	}
}

func TestGetPackageFn_Projection(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddPackage(testOwner, forge.PackageVersion{
		Type: "container", Name: "youscore/core", Version: "1.0.0",
		HTMLURL: "https://example/packages/1", Repository: "alice/core",
	})

	result, err := h.GetPackageFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "container", "name": "youscore/core", "version": "1.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[packageVersion](t, result)
	if decoded.Name != "youscore/core" || decoded.HTMLURL == "" || decoded.Repository != "alice/core" {
		t.Fatalf("projection: %+v", decoded)
	}
	if strings.Contains(resultText(t, result), `"login"`) {
		t.Fatalf("nested user leaked: %s", resultText(t, result))
	}
}

func TestGetPackageFn_UnknownIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.GetPackageFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "container", "name": "nope", "version": "1",
	}))
	if !forge.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetPackageFn_MissingArgsAreErrors(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	for name, args := range map[string]map[string]any{
		"owner":   {"owner": "", "type": "container", "name": "core", "version": "1.0.0"},
		"type":    {"type": "", "name": "core", "version": "1.0.0"},
		"name":    {"type": "container", "version": "1.0.0"},
		"version": {"type": "container", "name": "core"},
	} {
		if _, err := h.GetPackageFn(context.Background(), newCallToolRequest(args)); err == nil {
			t.Fatalf("expected error for missing %s", name)
		}
	}
}

func TestDeletePackageFn_Success(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1.2.3"})
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1.2.4"})

	result, err := h.DeletePackageFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "generic", "name": "dist", "version": "1.2.3",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[deletePackageResult](t, result)
	if decoded.Status != "deleted" || decoded.Owner != testOwner || decoded.Name != "dist" || decoded.Version != "1.2.3" {
		t.Fatalf("result: %+v", decoded)
	}
	if _, err := f.GetPackage(context.Background(), testOwner, "generic", "dist", "1.2.3"); !forge.IsNotFound(err) {
		t.Fatalf("version should be gone, got %v", err)
	}
	if _, err := f.GetPackage(context.Background(), testOwner, "generic", "dist", "1.2.4"); err != nil {
		t.Fatalf("other version must stay: %v", err)
	}
}

func TestDeletePackageFn_NotFoundIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	result, err := h.DeletePackageFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "generic", "name": "dist", "version": "9.9.9",
	}))
	if !forge.IsNotFound(err) {
		t.Fatalf("404 delete must be an MCP error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func seedFiles(f *fake.Forgejo) {
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1.0.0"},
		forge.PackageFile{ID: 1, Name: "a.bin", Size: 10, SHA256: "aa"},
		forge.PackageFile{ID: 2, Name: "b.bin", Size: 20, SHA256: "bb"},
	)
}

func TestListPackageFilesFn_ClientSliceHasNext(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedFiles(f)
	coords := map[string]any{"type": "generic", "name": "dist", "version": "1.0.0", "limit": float64(1)}

	coords["page"] = float64(1)
	page1, err := h.ListPackageFilesFn(context.Background(), newCallToolRequest(coords))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodePackageResult[listPackageFilesResult](t, page1)
	if decoded.Count != 1 || !decoded.HasNext || len(decoded.Files) != 1 || decoded.Files[0].Name != "a.bin" || decoded.TotalCount != 2 {
		t.Fatalf("page 1: %+v", decoded)
	}

	coords["page"] = float64(2)
	page2, err := h.ListPackageFilesFn(context.Background(), newCallToolRequest(coords))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded2 := decodePackageResult[listPackageFilesResult](t, page2)
	if decoded2.Count != 1 || decoded2.HasNext || decoded2.Files[0].Name != "b.bin" || decoded2.TotalCount != 2 {
		t.Fatalf("page 2: %+v", decoded2)
	}
}

func TestListPackageFilesFn_EmptyListTotalCountZero(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddPackage(testOwner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1.0.0"})

	result, err := h.ListPackageFilesFn(context.Background(), newCallToolRequest(map[string]any{
		"type": "generic", "name": "dist", "version": "1.0.0",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text := resultText(t, result); !strings.Contains(text, `"total_count":0`) {
		t.Fatalf("empty list must emit total_count 0: %s", text)
	}
	decoded := decodePackageResult[listPackageFilesResult](t, result)
	if decoded.TotalCount != 0 || decoded.HasNext || decoded.Count != 0 || decoded.Files == nil {
		t.Fatalf("empty list: %+v", decoded)
	}
}

func TestToolsRefuseWhenTheFactoryRefuses(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: errors.New("no credential")}}
	_, err := h.ListPackagesFn(context.Background(), newCallToolRequest(nil))
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v, want the factory's error", err)
	}
}
