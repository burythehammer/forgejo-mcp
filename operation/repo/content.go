// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"encoding/base64"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
)

// selectRepositoryWriteContent resolves the public repository-write content
// selector into the base64 representation expected by Forgejo's native
// contents APIs. Selection is based on whether each argument was sent, so
// explicit empty text and explicit zero-byte content remain distinguishable
// from omission.
func selectRepositoryWriteContent(content, contentBase64 decode.Arg) (string, error) {
	text, hasContent, err := content.String("content")
	if err != nil {
		return "", err
	}
	encoded, hasContentBase64, err := contentBase64.String("content_base64")
	if err != nil {
		return "", err
	}

	if hasContent == hasContentBase64 {
		return "", fmt.Errorf("exactly one of content or content_base64 must be supplied")
	}
	if hasContent {
		return base64.StdEncoding.EncodeToString([]byte(text)), nil
	}

	if _, err := base64.StdEncoding.Strict().DecodeString(encoded); err != nil {
		return "", fmt.Errorf("content_base64 must be valid standard Base64: %w", err)
	}
	return encoded, nil
}

// rejectRepositoryWriteContent rejects either public content representation by
// presence. Delete operations do not carry repository content, including
// explicit empty values.
func rejectRepositoryWriteContent(content, contentBase64 decode.Arg) error {
	if content.Present() {
		return fmt.Errorf("content must be omitted for delete")
	}
	if contentBase64.Present() {
		return fmt.Errorf("content_base64 must be omitted for delete")
	}
	return nil
}
