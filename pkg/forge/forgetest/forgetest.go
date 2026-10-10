// SPDX-License-Identifier: GPL-3.0-or-later

// Package forgetest is the contract suite for the Forgejo port, plus the one
// stub HTTP server the real adapter is tested against.
//
// The suite states what the port promises, once, and runs against every
// implementation: the fake (pkg/forge/fake) and the real adapter
// (pkg/forge/sdk, pointed at a Stub). An implementation that passes is
// interchangeable, which is what lets handler tests use the fake and still
// mean something.
//
// One Run function per domain (RunPulls, ...), each in its own file. A case is
// behaviour seen through the port: what you write you can read back, unknown
// things are not found, lists paginate. Cases never inspect HTTP traffic.
package forgetest

import (
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Harness is one implementation under test, freshly built for one case.
type Harness struct {
	// Port is the implementation exercised.
	Port forge.Forgejo
	// Seed writes what the port itself cannot: repositories, diffs, comments.
	// *fake.Forgejo implements it directly, and Stub.Fake is what the real
	// adapter's stub reads.
	Seed Seeder
}

// Seeder is the out-of-band way to put data in front of the port. Each domain
// contributes the methods it needs; the fake provides them all.
type Seeder interface {
	AddRepo(owner, repo string)
	SetPullRequestDiff(owner, repo string, index int64, diff string) error
	SetPullRequestFiles(owner, repo string, index int64, files ...*forgejo_sdk.ChangedFile) error
	AddPullRequestComment(owner, repo string, index int64, author, body string) (*forgejo_sdk.Comment, error)
}

// Repo is the repository every case works in.
const (
	Owner = "octo"
	Repo  = "hello"
)
