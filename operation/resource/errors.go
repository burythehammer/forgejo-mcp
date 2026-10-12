// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"errors"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// ResourceError wraps a forgejo URI and an HTTP status code into a structured error
// for MCP resource handlers.
type ResourceError struct {
	URI     string
	Code    int
	Message string
}

func (e *ResourceError) Error() string {
	return fmt.Sprintf("resource %q: %s (code %d)", e.URI, e.Message, e.Code)
}

// MapForgejoError maps a port error to a ResourceError by its domain kind
// (forge.ErrUnauthorized and friends), never by reading its message or status.
// The message carries the HTTP status in front when Forgejo gave one. Returns a
// *ResourceError with:
//   - Code -32002 for unauthorized (401, 403)
//   - Code -32003 for not found (404)
//   - Code -32602 for invalid params (ErrInvalidParams, or a refused input from Forgejo)
//   - Code -32603 (internal error) for upstream failures and anything else
func MapForgejoError(uri string, err error) *ResourceError {
	if err == nil {
		return nil
	}
	// Check sentinel first — covers all parse-time invalid-input errors.
	if errors.Is(err, ErrInvalidParams) {
		return &ResourceError{URI: uri, Code: -32602, Message: "invalid params: " + err.Error()}
	}
	msg := forge.Detail(err)
	switch {
	case errors.Is(err, forge.ErrUnauthorized):
		return &ResourceError{URI: uri, Code: -32002, Message: "access denied: " + msg}
	case errors.Is(err, forge.ErrNotFound):
		return &ResourceError{URI: uri, Code: -32003, Message: "not found: " + msg}
	case errors.Is(err, forge.ErrValidation):
		return &ResourceError{URI: uri, Code: -32602, Message: "invalid params: " + msg}
	default:
		return &ResourceError{URI: uri, Code: -32603, Message: "internal error: " + msg}
	}
}
