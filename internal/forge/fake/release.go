// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Releases = (*Forgejo)(nil)

// releaseState holds release metadata per repository. Attachments live in
// assetState (under key "release|owner/repo|id"), which keeps one lock order:
// releases first, then assets, never the reverse.
type releaseState struct {
	mu    sync.Mutex
	repos map[string]*releaseRepo // key "owner/repo"
}

type releaseRepo struct {
	releases []*forgejo_sdk.Release
	next     int64
}

// release returns the release with id, 404 otherwise. Caller holds mu.
func (r *releaseRepo) release(id int64) (*forgejo_sdk.Release, int, error) {
	for i, rel := range r.releases {
		if rel.ID == id {
			return rel, i, nil
		}
	}
	return nil, 0, notFound("release")
}

func (r *releaseRepo) byTag(tag string) (*forgejo_sdk.Release, int, error) {
	for i, rel := range r.releases {
		if rel.TagName == tag {
			return rel, i, nil
		}
	}
	return nil, 0, notFound("release")
}

// repo returns the release state of owner/repo, creating it for an existing
// repository and 404 for an unknown one. Caller holds mu.
func (s *releaseState) repo(f *Forgejo, owner, repo string) (*releaseRepo, error) {
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	if s.repos == nil {
		s.repos = map[string]*releaseRepo{}
	}
	r, ok := s.repos[owner+"/"+repo]
	if !ok {
		r = &releaseRepo{}
		s.repos[owner+"/"+repo] = r
	}
	return r, nil
}

// withAssets returns a copy of rel carrying its attachments, as Forgejo
// embeds them in a release.
func (f *Forgejo) withAssets(owner, repo string, rel *forgejo_sdk.Release) *forgejo_sdk.Release {
	c := *rel
	atts, _ := f.listSet(setKey("release", owner, repo, rel.ID), "release")
	c.Attachments = atts
	return &c
}

func (f *Forgejo) ListReleases(_ context.Context, owner, repo string, opt forgejo_sdk.ListReleasesOptions) ([]*forgejo_sdk.Release, error) {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	var all []*forgejo_sdk.Release
	for _, rel := range r.releases {
		if opt.IsDraft != nil && rel.IsDraft != *opt.IsDraft {
			continue
		}
		if opt.IsPreRelease != nil && rel.IsPrerelease != *opt.IsPreRelease {
			continue
		}
		cp := *rel
		all = append(all, &cp)
	}
	f.releases.mu.Unlock()

	// Newest first.
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	lo, hi := page(opt.ListOptions, len(all))
	out := make([]*forgejo_sdk.Release, 0, hi-lo)
	for _, rel := range all[lo:hi] {
		out = append(out, f.withAssets(owner, repo, rel))
	}
	return out, nil
}

func (f *Forgejo) GetRelease(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Release, error) {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	rel, _, err := r.release(id)
	var cp forgejo_sdk.Release
	if err == nil {
		cp = *rel
	}
	f.releases.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.withAssets(owner, repo, &cp), nil
}

func (f *Forgejo) GetReleaseByTag(_ context.Context, owner, repo, tag string) (*forgejo_sdk.Release, error) {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	rel, _, err := r.byTag(tag)
	var cp forgejo_sdk.Release
	if err == nil {
		cp = *rel
	}
	f.releases.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.withAssets(owner, repo, &cp), nil
}

func (f *Forgejo) GetLatestRelease(_ context.Context, owner, repo string) (*forgejo_sdk.Release, error) {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	var latest *forgejo_sdk.Release
	for _, rel := range r.releases {
		if rel.IsDraft || rel.IsPrerelease {
			continue
		}
		if latest == nil || rel.ID > latest.ID {
			latest = rel
		}
	}
	var cp forgejo_sdk.Release
	if latest != nil {
		cp = *latest
	}
	f.releases.mu.Unlock()
	if latest == nil {
		return nil, notFound("release")
	}
	return f.withAssets(owner, repo, &cp), nil
}

func (f *Forgejo) CreateRelease(_ context.Context, owner, repo string, opt forgejo_sdk.CreateReleaseOption) (*forgejo_sdk.Release, error) {
	if opt.TagName == "" {
		return nil, errors.New("tag_name is required")
	}
	if opt.Title == "" {
		return nil, errors.New("title is empty")
	}
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	if _, _, err := r.byTag(opt.TagName); err == nil {
		f.releases.mu.Unlock()
		return nil, statusErr(http.StatusConflict, "release tag already exists")
	}
	r.next++
	now := f.Now()
	target := opt.Target
	if target == "" {
		target = "main"
	}
	rel := &forgejo_sdk.Release{
		ID:           r.next,
		TagName:      opt.TagName,
		Target:       target,
		Title:        opt.Title,
		Note:         opt.Note,
		IsDraft:      opt.IsDraft,
		IsPrerelease: opt.IsPrerelease,
		CreatedAt:    now,
		PublishedAt:  now,
		Publisher:    user(Poster),
	}
	r.releases = append(r.releases, rel)
	cp := *rel
	f.releases.mu.Unlock()

	f.assets.ensure(setKey("release", owner, repo, cp.ID))
	return f.withAssets(owner, repo, &cp), nil
}

func (f *Forgejo) EditRelease(_ context.Context, owner, repo string, id int64, opt forgejo_sdk.EditReleaseOption) (*forgejo_sdk.Release, error) {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	rel, _, err := r.release(id)
	if err != nil {
		f.releases.mu.Unlock()
		return nil, err
	}
	if opt.TagName != "" && opt.TagName != rel.TagName {
		if _, _, err := r.byTag(opt.TagName); err == nil {
			f.releases.mu.Unlock()
			return nil, statusErr(http.StatusConflict, "release tag already exists")
		}
		rel.TagName = opt.TagName
	}
	if opt.Target != "" {
		rel.Target = opt.Target
	}
	if opt.Title != "" {
		rel.Title = opt.Title
	}
	if opt.Note != "" {
		rel.Note = opt.Note
	}
	if opt.IsDraft != nil {
		rel.IsDraft = *opt.IsDraft
	}
	if opt.IsPrerelease != nil {
		rel.IsPrerelease = *opt.IsPrerelease
	}
	cp := *rel
	f.releases.mu.Unlock()
	return f.withAssets(owner, repo, &cp), nil
}

func (f *Forgejo) DeleteRelease(_ context.Context, owner, repo string, id int64) error {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return err
	}
	_, i, err := r.release(id)
	if err != nil {
		f.releases.mu.Unlock()
		return err
	}
	r.releases = append(r.releases[:i], r.releases[i+1:]...)
	f.releases.mu.Unlock()
	f.dropReleaseAssets(owner, repo, id)
	return nil
}

func (f *Forgejo) DeleteReleaseByTag(_ context.Context, owner, repo, tag string) error {
	f.releases.mu.Lock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		f.releases.mu.Unlock()
		return err
	}
	rel, i, err := r.byTag(tag)
	if err != nil {
		f.releases.mu.Unlock()
		return err
	}
	id := rel.ID
	r.releases = append(r.releases[:i], r.releases[i+1:]...)
	f.releases.mu.Unlock()
	f.dropReleaseAssets(owner, repo, id)
	return nil
}

func (f *Forgejo) dropReleaseAssets(owner, repo string, id int64) {
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	key := setKey("release", owner, repo, id)
	if set, ok := f.assets.sets[key]; ok {
		for _, a := range set.items {
			delete(f.assets.blobs, a.UUID)
		}
		delete(f.assets.sets, key)
	}
}

// releaseKey checks that the release exists and returns its attachment key.
func (f *Forgejo) releaseKey(owner, repo string, id int64) (string, error) {
	f.releases.mu.Lock()
	defer f.releases.mu.Unlock()
	r, err := f.releases.repo(f, owner, repo)
	if err != nil {
		return "", err
	}
	if _, _, err := r.release(id); err != nil {
		return "", err
	}
	return setKey("release", owner, repo, id), nil
}

func (f *Forgejo) ListReleaseAttachments(_ context.Context, owner, repo string, releaseID int64) ([]*forgejo_sdk.Attachment, error) {
	key, err := f.releaseKey(owner, repo, releaseID)
	if err != nil {
		return nil, err
	}
	return f.listSet(key, "release")
}

func (f *Forgejo) GetReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64) (*forgejo_sdk.Attachment, error) {
	key, err := f.releaseKey(owner, repo, releaseID)
	if err != nil {
		return nil, err
	}
	return f.getInSet(key, "release", id)
}

func (f *Forgejo) CreateReleaseAttachment(ctx context.Context, owner, repo string, releaseID int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	key, err := f.releaseKey(owner, repo, releaseID)
	if err != nil {
		return nil, err
	}
	return f.createInSet(ctx, key, "release", up)
}

func (f *Forgejo) EditReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64, name string) (*forgejo_sdk.Attachment, error) {
	key, err := f.releaseKey(owner, repo, releaseID)
	if err != nil {
		return nil, err
	}
	return f.renameInSet(key, "release", id, name)
}

func (f *Forgejo) DeleteReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64) error {
	key, err := f.releaseKey(owner, repo, releaseID)
	if err != nil {
		return err
	}
	return f.deleteInSet(key, "release", id)
}
