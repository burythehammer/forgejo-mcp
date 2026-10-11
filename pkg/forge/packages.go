// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "context"

// Packages is the package-registry domain of the port. The SDK's package
// support cannot filter a listing or report paging headers, so these types are
// the port's own plain data, already reduced to what the tools expose.
//
// Failures from Forgejo are *StatusError; a missing owner is not-found, never
// an empty list.
type Packages interface {
	ListPackages(ctx context.Context, owner string, opt ListPackagesOptions) (PackageList, error)
	GetPackage(ctx context.Context, owner, packageType, name, version string) (*PackageVersion, error)
	// DeletePackage removes that one version only.
	DeletePackage(ctx context.Context, owner, packageType, name, version string) error
	// ListPackageFiles returns every file of the version; Forgejo does not page it.
	ListPackageFiles(ctx context.Context, owner, packageType, name, version string) ([]PackageFile, error)
}

// ListPackagesOptions selects one page of an owner's package versions. Type and
// Query are filters, ignored when empty. Page is 1-based.
type ListPackagesOptions struct {
	Type  string
	Query string
	Page  int
	Limit int
}

// PackageList is one page of ListPackages.
type PackageList struct {
	Packages []PackageVersion
	// HasNext is whether Forgejo advertised a next page (Link rel="next").
	HasNext bool
	// TotalCount is Forgejo's X-Total-Count, nil when it did not report one.
	TotalCount *int
}

// PackageVersion is one version of a package.
type PackageVersion struct {
	ID        int64
	Type      string
	Name      string
	Version   string
	HTMLURL   string
	CreatedAt string
	// Repository is the linked repository's full name, empty when unlinked.
	Repository string
}

type PackageFile struct {
	ID     int64
	Name   string
	Size   int64
	SHA256 string
}
