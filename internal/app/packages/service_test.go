// SPDX-License-Identifier: GPL-3.0-or-later

package packages_test

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/packages"
)

// The use cases run against a hand-written port: the behaviour under test is
// the use case's own (wrapping, projection, paging), not the registry's.

type stubPort struct {
	list    model.PackageList
	files   []model.PackageFile
	version *model.PackageVersion
	err     error
}

func (s stubPort) ListPackages(context.Context, string, apppkg.ListQuery) (model.PackageList, error) {
	return s.list, s.err
}

func (s stubPort) GetPackage(context.Context, string, string, string, string) (*model.PackageVersion, error) {
	return s.version, s.err
}

func (s stubPort) DeletePackage(context.Context, string, string, string, string) error {
	return s.err
}

func (s stubPort) ListPackageFiles(context.Context, string, string, string, string) ([]model.PackageFile, error) {
	return s.files, s.err
}

var coords = apppkg.Coords{Owner: "octo", Type: "generic", Name: "tool", Version: "1.0"}

func TestListEmptyIsAnArrayNotNull(t *testing.T) {
	t.Parallel()
	got, err := apppkg.New(stubPort{}).List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 1, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	if got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("empty list should be a non-nil empty page, got %+v", got)
	}
}

func TestListCarriesPageAndTotal(t *testing.T) {
	t.Parallel()
	zero := 0
	port := stubPort{list: model.PackageList{Packages: []model.PackageVersion{{ID: 1, Name: "a"}}, TotalCount: &zero}}
	got, err := apppkg.New(port).List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 2, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.HasNext || got.Page != 2 || got.Limit != 5 || got.Total == nil || *got.Total != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestListFailureIsWrapped(t *testing.T) {
	t.Parallel()
	_, err := apppkg.New(stubPort{err: errors.New("boom")}).List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 1, Limit: 30})
	if err == nil || err.Error() != "list packages: boom" {
		t.Fatalf("got %v", err)
	}
}

func TestGetProjectsTheVersion(t *testing.T) {
	t.Parallel()
	port := stubPort{version: &model.PackageVersion{ID: 3, Type: "generic", Name: "tool", Version: "1.0", Repository: "octo/hello"}}
	got, err := apppkg.New(port).Get(context.Background(), coords)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 3 || got.Repository != "octo/hello" {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteReportsTheVersion(t *testing.T) {
	t.Parallel()
	got, err := apppkg.New(stubPort{}).Delete(context.Background(), coords)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "deleted" || got.Version != "1.0" || got.Owner != "octo" {
		t.Fatalf("got %+v", got)
	}
}

func TestListFilesPagesAndReportsHasNext(t *testing.T) {
	t.Parallel()
	files := []model.PackageFile{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}
	s := apppkg.New(stubPort{files: files})

	second, err := s.ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 2 || second.Items[0].ID != 3 || !second.HasNext || (second.Total == nil || *second.Total != 5) {
		t.Fatalf("page 2: got %+v", second)
	}
	last, err := s.ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.HasNext {
		t.Fatalf("page 3: got %+v", last)
	}
}

func TestListFilesPastTheEndIsAnEmptyArray(t *testing.T) {
	t.Parallel()
	got, err := apppkg.New(stubPort{}).ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 9, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Items == nil || len(got.Items) != 0 || got.Total == nil || *got.Total != 0 {
		t.Fatalf("got %+v", got)
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }

// pagedPackagesPort serves all page by page, as Forgejo does. When total is
// set it reports the grand total, as X-Total-Count does; otherwise it reports
// none.
type pagedPackagesPort struct {
	stubPort
	all   []model.PackageVersion
	total bool
	calls int
}

func (p *pagedPackagesPort) ListPackages(_ context.Context, _ string, q apppkg.ListQuery) (model.PackageList, error) {
	p.calls++
	var rows []model.PackageVersion
	if off := (q.Page - 1) * q.Limit; off < len(p.all) {
		rows = p.all[off:min(off+q.Limit, len(p.all))]
	}
	out := model.PackageList{Packages: rows}
	if p.total {
		n := len(p.all)
		out.TotalCount = &n
	}
	return out, nil
}

func versions(n int) []model.PackageVersion {
	out := make([]model.PackageVersion, n)
	for i := range out {
		out[i] = model.PackageVersion{ID: int64(i + 1), Name: "pkg"}
	}
	return out
}

func TestListHasNextFromTheReportedTotalWithoutProbe(t *testing.T) {
	t.Parallel()
	p := &pagedPackagesPort{all: versions(3), total: true}
	s := apppkg.New(p)
	first, err := s.List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 2 || !first.HasNext || first.Page != 1 || first.Limit != 2 {
		t.Fatalf("page 1: got %+v", first)
	}
	if p.calls != 1 {
		t.Fatalf("a reported total makes no probe: %d calls", p.calls)
	}
	last, err := s.List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if last.Count != 1 || last.HasNext {
		t.Fatalf("page 2: got %+v", last)
	}
}

func TestListProbesForTheNextPageWhenNoTotalIsReported(t *testing.T) {
	t.Parallel()
	p := &pagedPackagesPort{all: versions(4)}
	s := apppkg.New(p)
	exact, err := s.List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if exact.Count != 2 || exact.HasNext {
		t.Fatalf("exact multiple, last page: got %+v", exact)
	}
	full, err := s.List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !full.HasNext {
		t.Fatalf("full page with rows after it: got %+v", full)
	}
}

func TestListFilesEnvelopeOnAnExactMultiple(t *testing.T) {
	t.Parallel()
	s := apppkg.New(stubPort{files: []model.PackageFile{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}})
	got, err := s.ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 2 || got.Limit != 2 || got.Count != 2 || got.HasNext || got.Total == nil || *got.Total != 4 {
		t.Fatalf("got %+v", got)
	}
}
