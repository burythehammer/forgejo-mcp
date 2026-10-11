// SPDX-License-Identifier: GPL-3.0-or-later

package releaseadapter

import (
	"encoding/json"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The repo-owned release and attachment types must keep the SDK's JSON: the
// bridge converts through JSON, so a field the copy lost would be dropped from
// every release tool's output. This fails when an SDK upgrade drifts from them.
func TestModelTypesMirrorTheSDKWireShape(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	user := &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo", Created: ts, LastLogin: ts, IsActive: true}
	att := &forgejo_sdk.Attachment{ID: 2, Name: "a", Size: 3, DownloadCount: 4, Created: ts, UUID: "u", DownloadURL: "d"}
	rel := &forgejo_sdk.Release{ID: 5, TagName: "v1", Target: "main", Title: "t", Note: "n", URL: "x", HTMLURL: "h",
		TarURL: "tar", ZipURL: "zip", IsDraft: true, IsPrerelease: true, CreatedAt: ts, PublishedAt: ts,
		Publisher: user, Attachments: []*forgejo_sdk.Attachment{att}}

	var gotAtt model.Attachment
	if !sameJSON(t, att, &gotAtt) {
		t.Errorf("model.Attachment drops or renames fields of forgejo_sdk.Attachment")
	}
	var gotRel model.Release
	if !sameJSON(t, rel, &gotRel) {
		t.Errorf("model.Release drops or renames fields of forgejo_sdk.Release")
	}
}

// sameJSON round-trips from through to and reports whether nothing was lost.
func sameJSON(t *testing.T, from, to any) bool {
	t.Helper()
	a, err := json.Marshal(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(a, to); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(to)
	if err != nil {
		t.Fatal(err)
	}
	return string(a) == string(b)
}
