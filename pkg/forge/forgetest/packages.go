// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

func addPkg(h Harness, typ, name, version string) forge.PackageVersion {
	return h.Seed.AddPackage(Owner, forge.PackageVersion{
		Type: typ, Name: name, Version: version,
		HTMLURL:    fmt.Sprintf("https://forge.test/%s/-/packages/%s/%s/%s", Owner, typ, name, version),
		CreatedAt:  "2026-08-01T00:00:00Z",
		Repository: Owner + "/" + Repo,
	})
}

// RunPackages runs the package-registry contract against the implementation
// that newHarness builds. newHarness is called once per case.
func RunPackages(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"an unknown owner is not found, not empty", func(t *testing.T, h Harness) {
			_, err := h.Port.ListPackages(ctx, "nobody", forge.ListPackagesOptions{Page: 1, Limit: 30})
			wantNotFound(t, err)
			_, err = h.Port.GetPackage(ctx, "nobody", "generic", "dist", "1")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeletePackage(ctx, "nobody", "generic", "dist", "1"))
			_, err = h.Port.ListPackageFiles(ctx, "nobody", "generic", "dist", "1")
			wantNotFound(t, err)
		}},
		{"an owner without packages lists none", func(t *testing.T, h Harness) {
			h.Seed.AddPackageOwner(Owner)
			got, err := h.Port.ListPackages(ctx, Owner, forge.ListPackagesOptions{Page: 1, Limit: 30})
			must(t, err)
			if got.Packages == nil || len(got.Packages) != 0 || got.HasNext {
				t.Fatalf("got %+v", got)
			}
		}},
		{"a package reads back with its repository", func(t *testing.T, h Harness) {
			want := addPkg(h, "container", "core", "1.0.0")
			got, err := h.Port.GetPackage(ctx, Owner, "container", "core", "1.0.0")
			must(t, err)
			if *got != want {
				t.Fatalf("got %+v, want %+v", *got, want)
			}
			_, err = h.Port.GetPackage(ctx, Owner, "container", "core", "9.9.9")
			wantNotFound(t, err)
		}},
		{"a slash in a name is part of one path segment", func(t *testing.T, h Harness) {
			addPkg(h, "container", "youscore/core", "1.0.0")
			got, err := h.Port.GetPackage(ctx, Owner, "container", "youscore/core", "1.0.0")
			must(t, err)
			if got.Name != "youscore/core" {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetPackage(ctx, Owner, "container", "youscore", "1.0.0")
			wantNotFound(t, err)
		}},
		{"list filters by type and query", func(t *testing.T, h Harness) {
			addPkg(h, "container", "core", "1")
			addPkg(h, "generic", "dist", "1")
			addPkg(h, "generic", "core-tools", "1")
			names := func(opt forge.ListPackagesOptions) []string {
				opt.Page, opt.Limit = 1, 30
				got, err := h.Port.ListPackages(ctx, Owner, opt)
				must(t, err)
				var out []string
				for _, p := range got.Packages {
					out = append(out, p.Name)
				}
				return out
			}
			if got := names(forge.ListPackagesOptions{}); len(got) != 3 {
				t.Fatalf("unfiltered: %v", got)
			}
			if got := names(forge.ListPackagesOptions{Type: "generic"}); len(got) != 2 {
				t.Fatalf("type filter: %v", got)
			}
			if got := names(forge.ListPackagesOptions{Query: "core"}); len(got) != 2 {
				t.Fatalf("query filter: %v", got)
			}
			if got := names(forge.ListPackagesOptions{Type: "generic", Query: "core"}); len(got) != 1 || got[0] != "core-tools" {
				t.Fatalf("both filters: %v", got)
			}
		}},
		{"list paginates, flags a next page and reports the total", func(t *testing.T, h Harness) {
			for i := 0; i < 5; i++ {
				addPkg(h, "generic", fmt.Sprint("pkg", i), "1")
			}
			page := func(p int) forge.PackageList {
				got, err := h.Port.ListPackages(ctx, Owner, forge.ListPackagesOptions{Page: p, Limit: 2})
				must(t, err)
				return got
			}
			p1, p2, p3 := page(1), page(2), page(3)
			if len(p1.Packages) != 2 || len(p2.Packages) != 2 || len(p3.Packages) != 1 {
				t.Fatalf("page sizes %d %d %d", len(p1.Packages), len(p2.Packages), len(p3.Packages))
			}
			if !p1.HasNext || !p2.HasNext || p3.HasNext {
				t.Fatalf("has_next: %v %v %v", p1.HasNext, p2.HasNext, p3.HasNext)
			}
			if p1.TotalCount == nil || *p1.TotalCount != 5 {
				t.Fatalf("total count %v, want 5", p1.TotalCount)
			}
			if p1.Packages[0].ID == p2.Packages[0].ID {
				t.Fatal("pages must not overlap")
			}
		}},
		{"files list in full", func(t *testing.T, h Harness) {
			h.Seed.AddPackage(Owner, forge.PackageVersion{Type: "generic", Name: "dist", Version: "1"},
				forge.PackageFile{ID: 1, Name: "a.bin", Size: 10, SHA256: "aa"},
				forge.PackageFile{ID: 2, Name: "b.bin", Size: 20, SHA256: "bb"})
			got, err := h.Port.ListPackageFiles(ctx, Owner, "generic", "dist", "1")
			must(t, err)
			if len(got) != 2 || got[0] != (forge.PackageFile{ID: 1, Name: "a.bin", Size: 10, SHA256: "aa"}) || got[1].Name != "b.bin" {
				t.Fatalf("got %+v", got)
			}
			addPkg(h, "generic", "bare", "1")
			none, err := h.Port.ListPackageFiles(ctx, Owner, "generic", "bare", "1")
			must(t, err)
			if none == nil || len(none) != 0 {
				t.Fatalf("want an empty non-nil list, got %#v", none)
			}
			_, err = h.Port.ListPackageFiles(ctx, Owner, "generic", "dist", "9")
			wantNotFound(t, err)
		}},
		{"delete removes that version only", func(t *testing.T, h Harness) {
			addPkg(h, "generic", "dist", "1")
			addPkg(h, "generic", "dist", "2")
			must(t, h.Port.DeletePackage(ctx, Owner, "generic", "dist", "1"))
			_, err := h.Port.GetPackage(ctx, Owner, "generic", "dist", "1")
			wantNotFound(t, err)
			_, err = h.Port.GetPackage(ctx, Owner, "generic", "dist", "2")
			must(t, err)
			wantNotFound(t, h.Port.DeletePackage(ctx, Owner, "generic", "dist", "1"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}
