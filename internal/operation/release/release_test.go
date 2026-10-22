// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/upload"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	owner = "octo"
	repo  = "hello"
)

type fn = func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
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

func ok(t *testing.T, f fn, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := f(context.Background(), req(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return res
}

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

func seed(t *testing.T, f *fake.Forgejo, tag string, draft, pre bool) *forgejo_sdk.Release {
	t.Helper()
	rel, err := f.CreateRelease(context.Background(), owner, repo, forgejo_sdk.CreateReleaseOption{
		TagName: tag, Title: "title " + tag, IsDraft: draft, IsPrerelease: pre,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func TestListReleases_PaginationAndStateFilter(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seed(t, f, "v1", false, false)
	seed(t, f, "v2-draft", true, false)
	seed(t, f, "v3-rc", false, true)
	seed(t, f, "v4", false, false)

	all := text(t, ok(t, h.ListReleasesFn, nil))
	for _, tag := range []string{"v1", "v2-draft", "v3-rc", "v4"} {
		if !strings.Contains(all, `"`+tag+`"`) {
			t.Fatalf("default list is missing %s: %s", tag, all)
		}
	}
	published := text(t, ok(t, h.ListReleasesFn, map[string]any{"state": "published"}))
	if !strings.Contains(published, `"v1"`) || !strings.Contains(published, `"v4"`) || strings.Contains(published, "v2-draft") || strings.Contains(published, "v3-rc") {
		t.Fatalf("published: %s", published)
	}
	if d := text(t, ok(t, h.ListReleasesFn, map[string]any{"state": "draft"})); !strings.Contains(d, "v2-draft") || strings.Contains(d, `"v1"`) {
		t.Fatalf("draft: %s", d)
	}
	if p := text(t, ok(t, h.ListReleasesFn, map[string]any{"state": "prerelease"})); !strings.Contains(p, "v3-rc") || strings.Contains(p, `"v1"`) {
		t.Fatalf("prerelease: %s", p)
	}
	page := text(t, ok(t, h.ListReleasesFn, map[string]any{"page": 2.0, "limit": 3.0}))
	if !strings.Contains(page, `"v1"`) || strings.Contains(page, `"v4"`) {
		t.Fatalf("page 2: %s", page)
	}
}

func TestListReleases_InvalidStateRejected(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if msg := fails(t, h.ListReleasesFn, map[string]any{"state": "bogus"}); !strings.Contains(msg, "invalid state") {
		t.Fatal(msg)
	}
}

func TestGetReleases(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rel := seed(t, f, "v1", false, false)
	seed(t, f, "v2-draft", true, false)

	if b := text(t, ok(t, h.GetReleaseByIDFn, map[string]any{"release_id": float64(rel.ID)})); !strings.Contains(b, `"v1"`) {
		t.Fatal(b)
	}
	if b := text(t, ok(t, h.GetReleaseByTagFn, map[string]any{"tag": "v1"})); !strings.Contains(b, `"v1"`) {
		t.Fatal(b)
	}
	if b := text(t, ok(t, h.GetLatestReleaseFn, nil)); !strings.Contains(b, `"v1"`) {
		t.Fatal(b)
	}
	if msg := fails(t, h.GetReleaseByIDFn, map[string]any{"release_id": 99.0}); !strings.Contains(msg, "get release err") {
		t.Fatal(msg)
	}
	if msg := fails(t, h.GetReleaseByTagFn, map[string]any{}); !strings.Contains(msg, "tag is required") {
		t.Fatal(msg)
	}
}

func TestCreateRelease(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	body := text(t, ok(t, h.CreateReleaseFn, map[string]any{"tag_name": "v1.0.0", "target_commitish": "abc123", "body": "notes", "prerelease": true}))
	// The name defaults to the tag.
	for _, want := range []string{`"v1.0.0"`, `"abc123"`, `"notes"`, `"prerelease":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s: %s", want, body)
		}
	}
	rel, err := f.GetReleaseByTag(context.Background(), owner, repo, "v1.0.0")
	if err != nil || rel.Title != "v1.0.0" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
	if msg := fails(t, h.CreateReleaseFn, map[string]any{}); !strings.Contains(msg, "tag_name is required") {
		t.Fatal(msg)
	}
}

func TestEditRelease_PartialUpdate(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rel := seed(t, f, "v1", false, false)
	ok(t, h.EditReleaseFn, map[string]any{"release_id": float64(rel.ID), "body": "updated"})
	got, _ := f.GetRelease(context.Background(), owner, repo, rel.ID)
	if got.Note != "updated" || got.Title != "title v1" || got.IsDraft {
		t.Fatalf("got %+v", got)
	}
	ok(t, h.EditReleaseFn, map[string]any{"release_id": float64(rel.ID), "draft": true})
	got, _ = f.GetRelease(context.Background(), owner, repo, rel.ID)
	if !got.IsDraft || got.Note != "updated" {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteReleases(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := seed(t, f, "v1", false, false)
	seed(t, f, "v2", false, false)
	if b := text(t, ok(t, h.DeleteReleaseFn, map[string]any{"release_id": float64(a.ID)})); !strings.Contains(b, "deleted") {
		t.Fatal(b)
	}
	if b := text(t, ok(t, h.DeleteReleaseByTagFn, map[string]any{"tag": "v2"})); !strings.Contains(b, "deleted") {
		t.Fatal(b)
	}
	if rels, _ := f.ListReleases(context.Background(), owner, repo, forgejo_sdk.ListReleasesOptions{}); len(rels) != 0 {
		t.Fatalf("left: %+v", rels)
	}
	if msg := fails(t, h.DeleteReleaseByTagFn, map[string]any{}); !strings.Contains(msg, "tag is required") {
		t.Fatal(msg)
	}
}

func TestReleaseAttachmentLifecycle(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)

	created := text(t, ok(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "content": b64("archive"), "filename": "a.zip", "mime_type": "application/zip"}))
	if !strings.Contains(created, `"a.zip"`) {
		t.Fatal(created)
	}
	if b := text(t, ok(t, h.GetReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0})); !strings.Contains(b, `"a.zip"`) {
		t.Fatal(b)
	}
	if b := text(t, ok(t, h.EditReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0, "name": "b.zip"})); !strings.Contains(b, `"b.zip"`) {
		t.Fatal(b)
	}
	dl := ok(t, h.DownloadReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0})
	var blobFound bool
	for _, c := range dl.Content {
		if er, isRes := c.(mcp.EmbeddedResource); isRes {
			if br, isBlob := er.Resource.(mcp.BlobResourceContents); isBlob && br.Blob == b64("archive") && br.MIMEType == "application/zip" {
				blobFound = true
			}
		}
	}
	if !blobFound {
		t.Fatalf("no blob in %+v", dl.Content)
	}
	ok(t, h.DeleteReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0})
	if msg := fails(t, h.DownloadReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0}); !strings.Contains(msg, "download release attachment (metadata) err") {
		t.Fatal(msg)
	}
}

func TestListReleaseAttachments_ClientSideSlicing(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)
	for i := 1; i <= 5; i++ {
		ok(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "content": b64("x"), "filename": fmt.Sprintf("f%d", i)})
	}
	got := text(t, ok(t, h.ListReleaseAttachmentsFn, map[string]any{"release_id": rid, "page": 2.0, "limit": 2.0}))
	if !strings.Contains(got, `"f3"`) || !strings.Contains(got, `"f4"`) || strings.Contains(got, `"f2"`) || strings.Contains(got, `"f5"`) {
		t.Fatalf("page 2 limit 2: %s", got)
	}
	if past := text(t, ok(t, h.ListReleaseAttachmentsFn, map[string]any{"release_id": rid, "page": 9.0})); strings.Contains(past, `"f1"`) {
		t.Fatalf("past the end: %s", past)
	}
}

func TestDownloadReleaseAttachment_AtCapNoBlob(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)
	ok(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "content": b64(strings.Repeat("x", int(forge.MaxInlineDownloadBytes))), "filename": "big"})
	res := ok(t, h.DownloadReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0})
	if len(res.Content) != 1 || !strings.Contains(text(t, res), `"inline":false`) {
		t.Fatalf("got %+v", res.Content)
	}
}

// TestCreateReleaseAttachment_RejectsBadInput lowers the upload cap, a package
// variable in internal/upload, so it does not run in parallel. The oversized
// argument is small because the cap is lowered, instead of a 90 MiB string.
func TestCreateReleaseAttachment_RejectsBadInput(t *testing.T) {
	t.Cleanup(upload.SetContentLimitForTest(1024))
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)
	fails(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "content": "not base64!!!", "filename": "f"})
	if msg := fails(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "content": strings.Repeat("A", 1025), "filename": "f"}); !strings.Contains(msg, "too large") {
		t.Fatal(msg)
	}
	if atts, _ := f.ListReleaseAttachments(context.Background(), owner, repo, int64(rid)); len(atts) != 0 {
		t.Fatalf("nothing should be uploaded: %+v", atts)
	}
}

func TestEditReleaseAttachment_RequiresName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)
	if msg := fails(t, h.EditReleaseAttachmentFn, map[string]any{"release_id": rid, "attachment_id": 1.0}); !strings.Contains(msg, "name is required") {
		t.Fatal(msg)
	}
}

func TestCredentialPolicyErrorIsReported(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: errors.New("no credential")}}
	if msg := fails(t, h.ListReleasesFn, nil); !strings.Contains(msg, "no credential") {
		t.Fatal(msg)
	}
}

// TestCreateReleaseAttachment_RejectsNonStringSource: a present non-string
// content or file_path fails with "must be a string" and uploads nothing.
func TestCreateReleaseAttachment_RejectsNonStringSource(t *testing.T) {
	h, f := newFixture(t)
	rid := float64(seed(t, f, "v1", false, false).ID)
	for _, key := range []string{"content", "file_path"} {
		for _, value := range []any{123.0, true, nil} {
			msg := fails(t, h.CreateReleaseAttachmentFn, map[string]any{"release_id": rid, "filename": "a.png", key: value})
			if !strings.Contains(msg, key+" must be a string") {
				t.Fatalf("%s=%v: want must-be-a-string error, got %s", key, value, msg)
			}
		}
	}
	if atts, _ := f.ListReleaseAttachments(context.Background(), owner, repo, int64(rid)); len(atts) != 0 {
		t.Fatalf("nothing should be uploaded: %+v", atts)
	}
}
