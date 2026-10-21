// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"encoding/json"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// answered is the result of a call whose SDK method discards the response
// body. A status in ok is success and returns nil. Any other status is a
// failure that keeps the status and, when the body is JSON with a message,
// Forgejo's own message; the SDK's bare "unexpected Status" text is not used.
func answered(status int, body []byte, ok ...int) error {
	for _, want := range ok {
		if status == want {
			return nil
		}
	}
	if msg := jsonMessage(body); msg != "" {
		return &forge.StatusError{StatusCode: status, Err: fmt.Errorf("HTTP %d: %s", status, msg)}
	}
	return &forge.StatusError{StatusCode: status, Err: fmt.Errorf("HTTP %d", status)}
}

// jsonMessage is the "message" of a JSON answer, or "" when the body is not
// JSON with one.
func jsonMessage(body []byte) string {
	var answer struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return ""
	}
	return strings.TrimSpace(answer.Message)
}

// contentsPath is the API path of a file in a repository. Each segment of the
// file path is escaped on its own, so the slashes that separate directories
// stay path separators while a segment cannot retarget the request.
func contentsPath(owner, repo, filePath string) string {
	segments := []any{"repos", owner, repo, "contents"}
	for _, p := range strings.Split(filePath, "/") {
		segments = append(segments, p)
	}
	return forge.APIPath(segments...)
}
