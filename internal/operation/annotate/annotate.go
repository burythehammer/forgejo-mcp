// SPDX-License-Identifier: GPL-3.0-or-later

// Package annotate sets the MCP hint annotations on a tool definition. Each
// tool's mcp.NewTool call passes exactly one of these; the registry test
// internal/operation/annotations_test.go pins the class of every tool.
package annotate

import "github.com/mark3labs/mcp-go/mcp"

// Read marks a tool that does not modify its environment: readOnlyHint true,
// destructiveHint false.
func Read() mcp.ToolOption {
	return func(t *mcp.Tool) {
		mcp.WithReadOnlyHintAnnotation(true)(t)
		mcp.WithDestructiveHintAnnotation(false)(t)
	}
}

// Write marks a tool that modifies its environment without destroying data:
// readOnlyHint false, destructiveHint false.
func Write() mcp.ToolOption {
	return func(t *mcp.Tool) {
		mcp.WithReadOnlyHintAnnotation(false)(t)
		mcp.WithDestructiveHintAnnotation(false)(t)
	}
}

// Destructive marks a tool that may destroy data (delete, remove, cancel):
// readOnlyHint false, destructiveHint true.
func Destructive() mcp.ToolOption {
	return func(t *mcp.Tool) {
		mcp.WithReadOnlyHintAnnotation(false)(t)
		mcp.WithDestructiveHintAnnotation(true)(t)
	}
}
