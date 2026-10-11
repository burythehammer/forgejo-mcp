// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

var _ forge.Packages = (*Adapter)(nil)

// packageVersionJSON is the Forgejo package JSON the adapter accepts. Extra
// fields (nested owner and creator users, the full repository) are dropped.
type packageVersionJSON struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
	Repository *struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (p packageVersionJSON) port() forge.PackageVersion {
	v := forge.PackageVersion{
		ID: p.ID, Type: p.Type, Name: p.Name, Version: p.Version,
		HTMLURL: p.HTMLURL, CreatedAt: p.CreatedAt,
	}
	if p.Repository != nil {
		v.Repository = p.Repository.FullName
	}
	return v
}

func (a *Adapter) ListPackages(ctx context.Context, owner string, opt forge.ListPackagesOptions) (forge.PackageList, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(opt.Page))
	query.Set("limit", strconv.Itoa(opt.Limit))
	if opt.Type != "" {
		query.Set("type", opt.Type)
	}
	if opt.Query != "" {
		query.Set("q", opt.Query)
	}
	path := forgejo.APIPath("packages", owner) + "?" + query.Encode()

	var rows []packageVersionJSON
	header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &rows)
	if err != nil {
		return forge.PackageList{}, err
	}
	out := forge.PackageList{
		Packages:   make([]forge.PackageVersion, 0, len(rows)),
		HasNext:    linkHasNext(header),
		TotalCount: totalCount(header),
	}
	for _, row := range rows {
		out.Packages = append(out.Packages, row.port())
	}
	return out, nil
}

func (a *Adapter) GetPackage(ctx context.Context, owner, packageType, name, version string) (*forge.PackageVersion, error) {
	var src packageVersionJSON
	if _, err := a.rawJSON(ctx, http.MethodGet, forgejo.APIPath("packages", owner, packageType, name, version), nil, &src); err != nil {
		return nil, err
	}
	v := src.port()
	return &v, nil
}

func (a *Adapter) DeletePackage(ctx context.Context, owner, packageType, name, version string) error {
	_, err := a.rawJSON(ctx, http.MethodDelete, forgejo.APIPath("packages", owner, packageType, name, version), nil, nil)
	return err
}

func (a *Adapter) ListPackageFiles(ctx context.Context, owner, packageType, name, version string) ([]forge.PackageFile, error) {
	var rows []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	}
	if _, err := a.rawJSON(ctx, http.MethodGet, forgejo.APIPath("packages", owner, packageType, name, version, "files"), nil, &rows); err != nil {
		return nil, err
	}
	files := make([]forge.PackageFile, 0, len(rows))
	for _, r := range rows {
		files = append(files, forge.PackageFile{ID: r.ID, Name: r.Name, Size: r.Size, SHA256: r.SHA256})
	}
	return files, nil
}

// linkHasNext reports whether Forgejo advertised another page via
// `Link: …; rel="next"`: the server said there is a next page, which is not an
// inference from how many rows arrived.
func linkHasNext(h http.Header) bool {
	for _, link := range h.Values("Link") {
		// A single Link value may carry several comma-separated relations,
		// so this is a substring test rather than a parse. Forgejo quotes
		// the rel; tolerate the unquoted form too.
		if strings.Contains(link, `rel="next"`) || strings.Contains(link, "rel=next") {
			return true
		}
	}
	return false
}
