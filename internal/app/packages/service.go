// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
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
func (s *Service) List(ctx context.Context, r ListRequest) (list.Bounded[model.PackageView], error) {
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	p := list.Normalize(list.Paging{Page: r.Page, Limit: r.Limit}, list.DefaultLimit(defaultLimit, c))
	return list.Run(ctx, p, c, func(ctx context.Context, q list.Paging) (list.Batch[model.PackageView], error) {
		got, err := s.port.ListPackages(ctx, r.Owner, ListQuery{Type: r.Type, Query: r.Query, Page: q.Page, Limit: q.Limit})
		if err != nil {
			return list.Batch[model.PackageView]{}, fmt.Errorf("list packages: %w", err)
		}
		return list.Batch[model.PackageView]{Items: views(got.Packages), Total: got.TotalCount}, nil
	})
}

// views is each package version in its view form.
func views(rows []model.PackageVersion) []model.PackageView {
	out := make([]model.PackageView, 0, len(rows))
	for _, row := range rows {
		out = append(out, view(row))
	}
	return out
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
func (s *Service) ListFiles(ctx context.Context, r ListFilesRequest) (list.Bounded[model.PackageFileView], error) {
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	p := list.Normalize(list.Paging{Page: r.Page, Limit: r.Limit}, list.DefaultLimit(defaultLimit, c))
	return list.Run(ctx, p, c, func(ctx context.Context, q list.Paging) (list.Batch[model.PackageFileView], error) {
		listed, err := s.port.ListPackageFiles(ctx, r.Owner, r.Type, r.Name, r.Version)
		if err != nil {
			return list.Batch[model.PackageFileView]{}, fmt.Errorf("list package files: %w", err)
		}
		all := make([]model.PackageFileView, 0, len(listed))
		for _, file := range listed {
			all = append(all, model.PackageFileView(file))
		}
		// Forgejo does not page the listing, so the window is cut here. The whole
		// listing is the grand total, so has_next needs no probe.
		rows := list.Cut(all, q).Items
		return list.Batch[model.PackageFileView]{Items: rows, Total: ptr.To(len(all))}, nil
	})
}

func view(src model.PackageVersion) model.PackageView {
	return model.PackageView(src)
}
