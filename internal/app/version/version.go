// SPDX-License-Identifier: GPL-3.0-or-later

// Package version is the use case behind the version tool. Packages under
// internal/app/ are MCP-free: they must not import mcp-go or the Forgejo SDK
// (the depguard rules in .golangci.yml), so they take plain inputs and return
// repo-owned result types for an MCP adapter in internal/operation/ to render.
package version

import "context"

// Result is what the version use case reports.
type Result struct {
	Version string
}

// Prober is the part of the Forgejo port the version probe needs.
type Prober interface {
	ProbeServerVersion(ctx context.Context) (string, error)
}

// Run reports the MCP server version, or "dev" when none was configured.
func Run(version string) Result {
	if version == "" {
		version = "dev"
	}
	return Result{Version: version}
}

// ProbeForgejo reports the Forgejo version the instance announces. The version
// is returned as the instance sent it, unparsed (17.0.0-rc1 stays 17.0.0-rc1).
func ProbeForgejo(ctx context.Context, p Prober) (Result, error) {
	v, err := p.ProbeServerVersion(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{Version: v}, nil
}
