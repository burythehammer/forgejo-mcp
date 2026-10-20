// SPDX-License-Identifier: GPL-3.0-or-later

// Package packagesadapter implements the package use cases' port
// (app/packages.Port) over the Forgejo port (forge.Packages). The forge and
// repo-owned package types are plain data of the same shape, so each is mapped
// field by field here: the one place the two are joined.
package packagesadapter

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

// New returns the use cases' port over p.
// Source is the part of the Forgejo port the package domain uses: its package
// methods and the instance's pagination ceiling.
type Source interface {
	forge.Packages
	forge.Ceilings
}

func New(p Source) apppkg.Port { return adapter{p: p} }

type adapter struct{ p Source }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

var _ apppkg.Port = adapter{}

func version(v forge.PackageVersion) model.PackageVersion {
	return model.PackageVersion{
		ID: v.ID, Type: v.Type, Name: v.Name, Version: v.Version, HTMLURL: v.HTMLURL,
		CreatedAt: v.CreatedAt, Repository: v.Repository,
	}
}

func (a adapter) ListPackages(ctx context.Context, owner string, q apppkg.ListQuery) (model.PackageList, error) {
	list, err := a.p.ListPackages(ctx, owner, forge.ListPackagesOptions{Type: q.Type, Query: q.Query, Page: q.Page, Limit: q.Limit})
	if err != nil {
		return model.PackageList{}, err
	}
	rows := make([]model.PackageVersion, len(list.Packages))
	for i, v := range list.Packages {
		rows[i] = version(v)
	}
	return model.PackageList{Packages: rows, TotalCount: list.TotalCount}, nil
}

func (a adapter) GetPackage(ctx context.Context, owner, packageType, name, ver string) (*model.PackageVersion, error) {
	v, err := a.p.GetPackage(ctx, owner, packageType, name, ver)
	if err != nil {
		return nil, err
	}
	out := version(*v)
	return &out, nil
}

func (a adapter) DeletePackage(ctx context.Context, owner, packageType, name, ver string) error {
	return a.p.DeletePackage(ctx, owner, packageType, name, ver)
}

func (a adapter) ListPackageFiles(ctx context.Context, owner, packageType, name, ver string) ([]model.PackageFile, error) {
	listed, err := a.p.ListPackageFiles(ctx, owner, packageType, name, ver)
	if err != nil {
		return nil, err
	}
	files := make([]model.PackageFile, len(listed))
	for i, f := range listed {
		files[i] = model.PackageFile{ID: f.ID, Name: f.Name, Size: f.Size, SHA256: f.SHA256}
	}
	return files, nil
}
