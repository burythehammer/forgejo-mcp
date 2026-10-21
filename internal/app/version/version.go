// SPDX-License-Identifier: GPL-3.0-or-later

// Package version is the use case behind the version tool. Packages under
// internal/app/ are MCP-free: they must not import mcp-go or the Forgejo SDK
// (the depguard rules in .golangci.yml), so they take plain inputs and return
// repo-owned result types for an MCP adapter in internal/operation/ to render.
package version

// Result is what the version use case reports.
type Result struct {
	Version string
}

// Run reports the server version, or "dev" when none was configured.
func Run(version string) Result {
	if version == "" {
		version = "dev"
	}
	return Result{Version: version}
}
