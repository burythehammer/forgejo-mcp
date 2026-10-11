// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"sort"
	"strings"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

var _ forge.Packages = (*Forgejo)(nil)

type packageState struct {
	mu     sync.Mutex
	owners map[string]*packageOwner
}

type packageOwner struct {
	versions []*packageRecord
	nextID   int64
}

type packageRecord struct {
	version forge.PackageVersion
	files   []forge.PackageFile
}

func (s *packageState) owner(name string) (*packageOwner, error) {
	o, ok := s.owners[name]
	if !ok {
		return nil, notFound("owner")
	}
	return o, nil
}

func (o *packageOwner) find(packageType, name, version string) (int, *packageRecord) {
	for i, r := range o.versions {
		if r.version.Type == packageType && r.version.Name == name && r.version.Version == version {
			return i, r
		}
	}
	return -1, nil
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// AddPackageOwner makes a user or organisation exist for package listing. An
// unknown owner is 404, as on Forgejo, which is different from an owner with
// no packages.
func (f *Forgejo) AddPackageOwner(owner string) {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	f.packageOwnerLocked(owner)
}

func (f *Forgejo) packageOwnerLocked(owner string) *packageOwner {
	if f.packages.owners == nil {
		f.packages.owners = map[string]*packageOwner{}
	}
	o, ok := f.packages.owners[owner]
	if !ok {
		o = &packageOwner{}
		f.packages.owners[owner] = o
	}
	return o
}

// AddPackage publishes a package version (the port cannot upload one) with its
// files, creating the owner if needed. The fake assigns the ID and returns the
// stored version. Publishing an existing type/name/version replaces it.
func (f *Forgejo) AddPackage(owner string, v forge.PackageVersion, files ...forge.PackageFile) forge.PackageVersion {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	o := f.packageOwnerLocked(owner)
	if i, _ := o.find(v.Type, v.Name, v.Version); i >= 0 {
		o.versions = append(o.versions[:i], o.versions[i+1:]...)
	}
	o.nextID++
	v.ID = o.nextID
	o.versions = append(o.versions, &packageRecord{version: v, files: append([]forge.PackageFile{}, files...)})
	return v
}

// ListPackages lists newest first, as Forgejo's package search does.
func (f *Forgejo) ListPackages(_ context.Context, owner string, opt forge.ListPackagesOptions) (forge.PackageList, error) {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	o, err := f.packages.owner(owner)
	if err != nil {
		return forge.PackageList{}, err
	}
	var match []forge.PackageVersion
	for _, r := range o.versions {
		v := r.version
		if opt.Type != "" && v.Type != opt.Type {
			continue
		}
		if opt.Query != "" && !containsFold(v.Name, opt.Query) {
			continue
		}
		match = append(match, v)
	}
	sort.Slice(match, func(i, j int) bool { return match[i].ID > match[j].ID })
	lo, hi := listPage(opt.Page, opt.Limit, len(match))
	total := len(match)
	out := forge.PackageList{
		Packages:   append([]forge.PackageVersion{}, match[lo:hi]...),
		HasNext:    hi < total,
		TotalCount: &total,
	}
	return out, nil
}

func (f *Forgejo) GetPackage(_ context.Context, owner, packageType, name, version string) (*forge.PackageVersion, error) {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	o, err := f.packages.owner(owner)
	if err != nil {
		return nil, err
	}
	_, r := o.find(packageType, name, version)
	if r == nil {
		return nil, notFound("package")
	}
	v := r.version
	return &v, nil
}

func (f *Forgejo) DeletePackage(_ context.Context, owner, packageType, name, version string) error {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	o, err := f.packages.owner(owner)
	if err != nil {
		return err
	}
	i, _ := o.find(packageType, name, version)
	if i < 0 {
		return notFound("package")
	}
	o.versions = append(o.versions[:i], o.versions[i+1:]...)
	return nil
}

func (f *Forgejo) ListPackageFiles(_ context.Context, owner, packageType, name, version string) ([]forge.PackageFile, error) {
	f.packages.mu.Lock()
	defer f.packages.mu.Unlock()
	o, err := f.packages.owner(owner)
	if err != nil {
		return nil, err
	}
	_, r := o.find(packageType, name, version)
	if r == nil {
		return nil, notFound("package")
	}
	return append([]forge.PackageFile{}, r.files...), nil
}
