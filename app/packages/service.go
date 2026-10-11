// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service runs the package use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// Coords names one version of a package.
type Coords struct {
	Owner, Type, Name, Version string
}

// ListRequest lists an owner's package versions. Page and Limit are already
// validated and defaulted by the caller.
type ListRequest struct {
	Owner, Type, Query string
	Page, Limit        int
}

// ListFilesRequest pages through a package version's files. Page and Limit are
// already validated and defaulted by the caller.
type ListFilesRequest struct {
	Coords
	Page, Limit int
}

// List returns one page of an owner's package versions.
func (s *Service) List(ctx context.Context, r ListRequest) (model.ListPackagesResult, error) {
	list, err := s.port.ListPackages(ctx, r.Owner, ListQuery{Type: r.Type, Query: r.Query, Page: r.Page, Limit: r.Limit})
	if err != nil {
		return model.ListPackagesResult{}, fmt.Errorf("list packages: %w", err)
	}
	packages := make([]model.PackageView, 0, len(list.Packages))
	for _, row := range list.Packages {
		packages = append(packages, view(row))
	}
	return model.ListPackagesResult{
		Packages:   packages,
		Page:       r.Page,
		Limit:      r.Limit,
		Count:      len(packages),
		HasNext:    list.HasNext,
		TotalCount: list.TotalCount,
	}, nil
}

// Get returns one package version.
func (s *Service) Get(ctx context.Context, c Coords) (model.PackageView, error) {
	src, err := s.port.GetPackage(ctx, c.Owner, c.Type, c.Name, c.Version)
	if err != nil {
		return model.PackageView{}, fmt.Errorf("get package: %w", err)
	}
	return view(*src), nil
}

// Delete removes one version of a package.
func (s *Service) Delete(ctx context.Context, c Coords) (model.DeletePackageResult, error) {
	if err := s.port.DeletePackage(ctx, c.Owner, c.Type, c.Name, c.Version); err != nil {
		return model.DeletePackageResult{}, fmt.Errorf("delete package: %w", err)
	}
	return model.DeletePackageResult{Owner: c.Owner, Type: c.Type, Name: c.Name, Version: c.Version, Status: "deleted"}, nil
}

// ListFiles returns one page of a package version's files. Forgejo does not
// page the listing, so the page is cut here.
func (s *Service) ListFiles(ctx context.Context, r ListFilesRequest) (model.ListPackageFilesResult, error) {
	listed, err := s.port.ListPackageFiles(ctx, r.Owner, r.Type, r.Name, r.Version)
	if err != nil {
		return model.ListPackageFilesResult{}, fmt.Errorf("list package files: %w", err)
	}
	allFiles := make([]model.PackageFileView, 0, len(listed))
	for _, file := range listed {
		allFiles = append(allFiles, model.PackageFileView{ID: file.ID, Name: file.Name, Size: file.Size, SHA256: file.SHA256})
	}

	start64 := int64(r.Page-1) * int64(r.Limit)
	start := len(allFiles)
	if start64 < int64(len(allFiles)) {
		start = int(start64)
	}
	end := min(start+r.Limit, len(allFiles))
	files := allFiles[start:end]
	return model.ListPackageFilesResult{
		Files:      files,
		Page:       r.Page,
		Limit:      r.Limit,
		Count:      len(files),
		HasNext:    end < len(allFiles),
		TotalCount: len(allFiles),
	}, nil
}

func view(src model.PackageVersion) model.PackageView {
	return model.PackageView{
		ID:         src.ID,
		Type:       src.Type,
		Name:       src.Name,
		Version:    src.Version,
		HTMLURL:    src.HTMLURL,
		CreatedAt:  src.CreatedAt,
		Repository: src.Repository,
	}
}
