// SPDX-License-Identifier: GPL-3.0-or-later

package model

// The package data types are the port's plain data, already reduced to what
// the tools expose.

// PackageVersion is one version of a package.
type PackageVersion struct {
	ID         int64
	Type       string
	Name       string
	Version    string
	HTMLURL    string
	CreatedAt  string
	Repository string
}

// PackageList is one page of an owner's package versions. TotalCount is the
// grand total when Forgejo reports one; the use case derives has_next from it.
type PackageList struct {
	Packages   []PackageVersion
	TotalCount *int
}

// PackageFile is one file of a package version.
type PackageFile struct {
	ID     int64
	Name   string
	Size   int64
	SHA256 string
}

// PackageView is a package version as the tools return it.
type PackageView struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	HTMLURL    string `json:"html_url,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// PackageFileView is a package file as the files tool returns it.
type PackageFileView struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// ListPackagesResult is one page of the list tool's output.
type ListPackagesResult struct {
	Packages []PackageView `json:"packages"`
	PageMeta
}

// ListPackageFilesResult is one page of the files tool's output.
type ListPackageFilesResult struct {
	Files []PackageFileView `json:"files"`
	PageMeta
}

// DeletePackageResult is what the delete tool returns.
type DeletePackageResult struct {
	Owner   string `json:"owner"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
}
