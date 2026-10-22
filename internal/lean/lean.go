// SPDX-License-Identifier: GPL-3.0-or-later

// Package lean rewrites registered tools into the terse form served under
// --lean. The rewrite changes text and annotations only: tool names, argument
// names and required lists are untouched.
package lean

import (
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// kept names the arguments whose property description stays under lean mode:
// an argument with a format (RFC3339 time, hex SHA, JSON, comma-separated list),
// an allowed-value list (state, sort, type), or a rule its name and type do not
// show (exactly one of two arguments, a required pairing). The set is by name,
// because a name carries the same meaning in every tool that takes it. owner,
// repo, index, page, limit and verbose are never kept.
var kept = map[string]bool{
	// RFC3339 times and durations
	"since": true, "before": true, "due_date": true, "created_at": true,
	"last_read_at": true, "seconds": true, "duration": true, "clear_due_date": true,
	// comma-separated lists
	"labels": true, "set_labels": true, "assignees": true, "reviewers": true,
	"team_reviewers": true, "events": true, "milestones": true, "topics": true,
	"status_check_contexts": true, "push_whitelist_usernames": true,
	"merge_whitelist_usernames": true, "approvals_whitelist_usernames": true,
	// allowed values
	"state": true, "sort": true, "order": true, "type": true, "event": true,
	"status": true, "style": true, "permission": true, "visibility": true,
	"content_type": true, "http_method": true, "delete_mode": true, "color": true,
	// formats and rules
	"sha": true, "secret_name": true, "inputs": true, "comments": true, "path": true,
	"offset": true, "start_line": true, "end_line": true, "max_bytes": true,
	"content": true, "content_base64": true, "file_path": true, "files": true,
	"filename": true, "old": true, "new": true, "exclusive": true,
}

// Keeps reports whether an argument of the given name keeps its description.
func Keeps(arg string) bool {
	return kept[arg]
}

// Apply rewrites every tool registered on s into its lean form, in place. Call
// it after registration and selection, before the server serves anything.
func Apply(s *server.MCPServer) {
	for _, st := range s.ListTools() {
		s.AddTool(Tool(st.Tool), st.Handler)
	}
}

// Tool returns the lean form of t: the description cut to its first sentence,
// property descriptions removed except for kept arguments, and empty
// annotations. The schema's other keys, and every name, are kept as they are.
func Tool(t mcp.Tool) mcp.Tool {
	t.Description = FirstSentence(t.Description)
	t.Annotations = mcp.ToolAnnotation{}
	props := make(map[string]any, len(t.InputSchema.Properties))
	for name, v := range t.InputSchema.Properties {
		prop, ok := v.(map[string]any)
		if !ok {
			props[name] = v
			continue
		}
		out := stripSchema(prop)
		if kept[name] {
			if d, has := prop["description"]; has {
				out["description"] = d
			}
		}
		props[name] = out
	}
	t.InputSchema.Properties = props
	return t
}

// stripSchema returns a copy of schema with its description removed, and with
// the same removal applied to its nested properties and items.
func stripSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		out[k] = v
	}
	delete(out, "description")
	if nested, ok := out["properties"].(map[string]any); ok {
		inner := make(map[string]any, len(nested))
		for name, v := range nested {
			if p, ok := v.(map[string]any); ok {
				inner[name] = stripSchema(p)
			} else {
				inner[name] = v
			}
		}
		out["properties"] = inner
	}
	if items, ok := out["items"].(map[string]any); ok {
		out["items"] = stripSchema(items)
	}
	return out
}

// FirstSentence returns s up to the end of its first sentence. A full stop ends
// a sentence only when whitespace follows it and the next word starts with an
// upper-case letter or a backtick, or when nothing follows. A full stop before a
// lower-case word, as in "(e.g. push)", does not end one.
func FirstSentence(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '.' {
			continue
		}
		if i+1 == len(s) {
			return s
		}
		if !unicode.IsSpace(rune(s[i+1])) {
			continue
		}
		rest := strings.TrimLeftFunc(s[i+1:], unicode.IsSpace)
		if rest == "" {
			return s[:i+1]
		}
		if r := []rune(rest)[0]; unicode.IsUpper(r) || r == '`' {
			return s[:i+1]
		}
	}
	return s
}
