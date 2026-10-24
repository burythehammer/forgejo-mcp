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

// markerKey is the property key that Keep writes into a tool's schema. Finish
// removes it from every tool in both modes, so it is never served.
const markerKey = "x-lean-keep"

// Keep marks an argument whose description stays under --lean, and gives the
// one short clause served in its place. Mark an argument with a format (a
// comma-separated list, an RFC3339 time), an allowed-value list, or a rule that
// its name and type do not show. Never mark owner, repo, index, page, limit or
// verbose. Place it in the argument's option list next to its Description.
func Keep(clause string) mcp.PropertyOption {
	return func(schema map[string]any) {
		schema[markerKey] = clause
	}
}

// Marked returns the arguments of t that carry a keep marker, each with its
// clause.
func Marked(t mcp.Tool) map[string]string {
	out := map[string]string{}
	for name, v := range t.InputSchema.Properties {
		prop, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if clause, marked := prop[markerKey].(string); marked {
			out[name] = clause
		}
	}
	return out
}

// Finish sets the tool definitions on s for the mode: the lean form when on is
// true, the full form otherwise. Either way the keep markers are removed. Call
// it after registration and selection, before the server serves anything.
func Finish(s *server.MCPServer, on bool) {
	for _, st := range s.ListTools() {
		t := Strip(st.Tool)
		if on {
			t = Tool(st.Tool)
		}
		s.AddTool(t, st.Handler)
	}
}

// Strip returns t with its keep markers removed and every description left as
// it was.
func Strip(t mcp.Tool) mcp.Tool {
	props := make(map[string]any, len(t.InputSchema.Properties))
	for name, v := range t.InputSchema.Properties {
		prop, ok := v.(map[string]any)
		if !ok {
			props[name] = v
			continue
		}
		out := make(map[string]any, len(prop))
		for k, val := range prop {
			if k != markerKey {
				out[k] = val
			}
		}
		props[name] = out
	}
	t.InputSchema.Properties = props
	return t
}

// Tool returns the lean form of t: the description cut to its first sentence,
// a property description only for each marked argument (the marker's clause),
// and empty annotations. The schema's other keys, and every name, are kept as
// they are.
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
		if clause, marked := prop[markerKey].(string); marked {
			out["description"] = clause
		}
		props[name] = out
	}
	t.InputSchema.Properties = props
	return t
}

// stripSchema returns a copy of schema with its description and keep marker
// removed, and with the same removal applied to its nested properties and items.
func stripSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		out[k] = v
	}
	delete(out, "description")
	delete(out, markerKey)
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
