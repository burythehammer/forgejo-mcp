// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
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

// ListRequest lists an owner's package versions. A page or limit left at zero
// takes the default; the use case applies it.
type ListRequest struct {
	Owner, Type, Query string
	Page, Limit        int
}

// ListFilesRequest pages through a package version's files. A page or limit left
// at zero takes the default; the use case applies it.
type ListFilesRequest struct {
	Coords
	Page, Limit int
}

// defaultLimit is the page size a package list uses when the caller gives none.
const defaultLimit = 30

// List returns one page of an owner's package versions. The page goes through
// the list module, so the instance ceiling applies and a default limit is
// lowered to it.
func (s *Service) List(ctx context.Context, r ListRequest) (model.ListPackagesResult, error) {
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	p := list.Normalize(list.Paging{Page: r.Page, Limit: r.Limit}, list.DefaultLimit(defaultLimit, c))
	var hasNext bool
	var total *int
	rows, err := list.Items(ctx, p, c, func(ctx context.Context, q list.Paging) (list.Batch[model.PackageVersion], error) {
		got, err := s.port.ListPackages(ctx, r.Owner, ListQuery{Type: r.Type, Query: r.Query, Page: q.Page, Limit: q.Limit})
		if err != nil {
			return list.Batch[model.PackageVersion]{}, fmt.Errorf("list packages: %w", err)
		}
		hasNext, total = got.HasNext, got.TotalCount
		return list.Batch[model.PackageVersion]{Items: got.Packages}, nil
	})
	if err != nil {
		return model.ListPackagesResult{}, err
	}
	packages := make([]model.PackageView, 0, len(rows))
	for _, row := range rows {
		packages = append(packages, view(row))
	}
	return model.ListPackagesResult{
		Packages:   packages,
		Page:       p.Page,
		Limit:      p.Limit,
		Count:      len(packages),
		HasNext:    hasNext,
		TotalCount: total,
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
	var hasNext bool
	var total int
	// A page below 1 is page 1; the limit default is lowered to the ceiling by
	// list.Window.
	files, p, err := list.Window(ctx, s.port, list.Paging{Page: max(r.Page, 1), Limit: r.Limit}, defaultLimit,
		func(ctx context.Context, q list.Paging) ([]model.PackageFileView, error) {
			listed, err := s.port.ListPackageFiles(ctx, r.Owner, r.Type, r.Name, r.Version)
			if err != nil {
				return nil, fmt.Errorf("list package files: %w", err)
			}
			all := make([]model.PackageFileView, 0, len(listed))
			for _, file := range listed {
				all = append(all, model.PackageFileView(file))
			}
			// Forgejo does not page the listing, so the window is cut here.
			rows, more := list.Cut(all, q)
			hasNext, total = more, len(all)
			return rows, nil
		})
	if err != nil {
		return model.ListPackageFilesResult{}, err
	}
	return model.ListPackageFilesResult{
		Files:      files,
		Page:       p.Page,
		Limit:      p.Limit,
		Count:      len(files),
		HasNext:    hasNext,
		TotalCount: total,
	}, nil
}

func view(src model.PackageVersion) model.PackageView {
	return model.PackageView(src)
}
