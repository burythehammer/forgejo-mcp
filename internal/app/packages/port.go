// SPDX-License-Identifier: GPL-3.0-or-later

// Package packages holds the package-registry use cases behind the package
// tools. Like all of internal/app/ it is MCP-free and SDK-free
// (the depguard rules in .golangci.yml): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from internal/app/model for an
// MCP adapter in internal/operation/packages to render. The adapter validates the
// arguments before a use case runs, so a use case takes values it can trust.
package packages

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// Port is the Forgejo package domain as the use cases need it, in repo-owned
// types. internal/forge/packagesadapter implements it over forge.Packages. Failures
// from Forgejo come back as ordinary errors for the use case to wrap.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListPackages(ctx context.Context, owner string, q ListQuery) (model.PackageList, error)
	GetPackage(ctx context.Context, owner, packageType, name, version string) (*model.PackageVersion, error)
	DeletePackage(ctx context.Context, owner, packageType, name, version string) error
	ListPackageFiles(ctx context.Context, owner, packageType, name, version string) ([]model.PackageFile, error)
}

// ListQuery selects one page of an owner's package versions. Type and Query
// are filters, ignored when empty. Page is 1-based.
type ListQuery struct {
	Type  string
	Query string
	Page  int
	Limit int
}
