// SPDX-License-Identifier: GPL-3.0-or-later

package attachmentadapter

import (
	"encoding/json"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The repo-owned attachment type must keep the SDK's JSON, since the bridge
// converts through JSON and a lost field would vanish from every attachment
// tool's output.
func TestModelAttachmentMirrorsTheSDKWireShape(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	att := &forgejo_sdk.Attachment{ID: 2, Name: "a", Size: 3, DownloadCount: 4, Created: ts, UUID: "u", DownloadURL: "d"}
	a, err := json.Marshal(att)
	if err != nil {
		t.Fatal(err)
	}
	var m model.Attachment
	if err := json.Unmarshal(a, &m); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("model.Attachment drops or renames fields of forgejo_sdk.Attachment\nsdk:   %s\nmodel: %s", a, b)
	}
}

// The use cases' inline cap is a copy of the forge's; they must agree.
func TestInlineCapAgreesWithTheForge(t *testing.T) {
	t.Parallel()
	if download.MaxInlineBytes != forge.MaxInlineDownloadBytes {
		t.Fatalf("app/download cap %d != forge cap %d", download.MaxInlineBytes, forge.MaxInlineDownloadBytes)
	}
}
