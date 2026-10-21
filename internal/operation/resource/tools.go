// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import "sync/atomic"

// registeredTools is the set of tool names the server registered, recorded by
// operation.RegisterTool once every tool is in place. nil means no server has
// recorded one, which is the state of a single domain's unit tests.
var registeredTools atomic.Pointer[map[string]struct{}]

// SetRegisteredTools records the names of the tools a server has registered.
// Truncation notes may name only these. Calling it with nil clears the record.
func SetRegisteredTools(names []string) {
	if names == nil {
		registeredTools.Store(nil)
		return
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	registeredTools.Store(&set)
}

// ToolRef returns name when a truncation note may point at it: it is a
// registered tool, or no registry has been recorded. It returns "" for a name
// that is not registered, so a note built from it omits the pointer rather
// than sending the caller to a tool that does not exist.
func ToolRef(name string) string {
	set := registeredTools.Load()
	if set == nil {
		return name
	}
	if _, ok := (*set)[name]; ok {
		return name
	}
	return ""
}
