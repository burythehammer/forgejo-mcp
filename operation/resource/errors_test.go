// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"errors"
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

// The kind comes from the port's error, not from words in its message: a plain
// error that says "404" is an internal error, not a not-found.
func TestMapForgejoError_KindsByStatus(t *testing.T) {
	const uri = "forgejo://repo/a/b"
	tests := []struct {
		name    string
		err     error
		code    int
		message string
	}{
		{"401 is access denied", &forge.StatusError{StatusCode: 401, Err: errors.New("login required")}, -32002, "access denied: 401 login required"},
		{"403 is access denied", &forge.StatusError{StatusCode: 403, Err: errors.New("Forbidden")}, -32002, "access denied: 403 Forbidden"},
		{"404 is not found", &forge.StatusError{StatusCode: 404, Err: errors.New("Not Found")}, -32003, "not found: 404 Not Found"},
		{"422 is invalid params", &forge.StatusError{StatusCode: 422, Err: errors.New("title is empty")}, -32602, "invalid params: 422 title is empty"},
		{"502 is an internal error", &forge.StatusError{StatusCode: 502, Err: errors.New("bad gateway")}, -32603, "internal error: 502 bad gateway"},
		{"a plain 404 in the text is internal", errors.New("404 Not Found"), -32603, "internal error: 404 Not Found"},
		{"a plain 403 in the text is internal", errors.New("403 Forbidden"), -32603, "internal error: 403 Forbidden"},
		{"a wrapped kind still maps", fmt.Errorf("get: %w", forge.ErrNotFound), -32003, "not found: get: not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapForgejoError(uri, tt.err)
			if got == nil {
				t.Fatal("expected non-nil ResourceError")
			}
			if got.Code != tt.code {
				t.Errorf("code = %d, want %d", got.Code, tt.code)
			}
			if got.Message != tt.message {
				t.Errorf("message = %q, want %q", got.Message, tt.message)
			}
		})
	}
}

func TestMapForgejoError_Nil(t *testing.T) {
	err := MapForgejoError("forgejo://repo/a/b", nil)
	if err != nil {
		t.Errorf("expected nil for nil input, got %v", err)
	}
}

func TestMapForgejoError_InvalidKind(t *testing.T) {
	// The real parser now wraps with ErrInvalidParams; simulate that here.
	wrapped := fmt.Errorf("%w: invalid URI \"forgejo://repo/a/b/wiki/1/comment/5\": kind must be 'issue' or 'pr', got \"wiki\"", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/a/b/wiki/1/comment/5", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 for invalid kind, got %d", err.Code)
	}
}

func TestMapForgejoError_ErrInvalidParams_Sentinel(t *testing.T) {
	// Errors wrapping ErrInvalidParams must map to -32602 regardless of message content.
	wrapped := fmt.Errorf("%w: invalid URI \"forgejo://repo/x/y/commit/short\": sha must be exactly 40 hex characters, got 5", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/x/y/commit/short", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 for ErrInvalidParams sentinel, got %d", err.Code)
	}
}

func TestMapForgejoError_ErrInvalidParams_404InMessage(t *testing.T) {
	// A URI containing "404" in its body must still map to -32602 when wrapped with ErrInvalidParams,
	// not to -32003 (the substring-match path must not fire before the sentinel check).
	wrapped := fmt.Errorf("%w: invalid URI containing 404 in path", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/a/b/issue/404abc", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 (sentinel beats 404 substring), got %d", err.Code)
	}
}
