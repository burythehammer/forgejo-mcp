// SPDX-License-Identifier: GPL-3.0-or-later

package packages_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apppkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/packages"
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
	b, _ := json.Marshal(got)
	if !strings.Contains(string(b), `"packages":[]`) {
		t.Fatalf("empty list should render as [], got %s", b)
	}
}

func TestListCarriesPageAndTotal(t *testing.T) {
	t.Parallel()
	zero := 0
	port := stubPort{list: model.PackageList{Packages: []model.PackageVersion{{ID: 1, Name: "a"}}, HasNext: true, TotalCount: &zero}}
	got, err := apppkg.New(port).List(context.Background(), apppkg.ListRequest{Owner: "octo", Page: 2, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || !got.HasNext || got.Page != 2 || got.Limit != 5 || got.TotalCount == nil || *got.TotalCount != 0 {
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
	if len(second.Files) != 2 || second.Files[0].ID != 3 || !second.HasNext || second.TotalCount != 5 {
		t.Fatalf("page 2: got %+v", second)
	}
	last, err := s.ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Files) != 1 || last.HasNext {
		t.Fatalf("page 3: got %+v", last)
	}
}

func TestListFilesPastTheEndIsAnEmptyArray(t *testing.T) {
	t.Parallel()
	got, err := apppkg.New(stubPort{}).ListFiles(context.Background(), apppkg.ListFilesRequest{Coords: coords, Page: 9, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	if !strings.Contains(string(b), `"files":[]`) || got.TotalCount != 0 {
		t.Fatalf("got %s", b)
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }
