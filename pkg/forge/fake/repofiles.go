// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"crypto/sha1" //nolint:gosec // git object ids are SHA-1
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.RepoFiles = (*Forgejo)(nil)

// DefaultBranch is the branch every repository starts with.
const DefaultBranch = "main"

type filesState struct {
	mu    sync.Mutex
	repos map[string]*fileRepo // key "owner/repo"
}

type fileRepo struct {
	branches map[string]map[string][]byte // branch -> path -> content
	commits  []*forgejo_sdk.Commit        // oldest first
	statuses map[string][]*forgejo_sdk.Status
	nextID   int
}

func (s *filesState) repo(owner, repo string) (*fileRepo, error) {
	r, ok := s.repos[owner+"/"+repo]
	if !ok {
		return nil, notFound("repository")
	}
	return r, nil
}

func (r *fileRepo) tree(ref string) (map[string][]byte, error) {
	if ref == "" {
		ref = DefaultBranch
	}
	t, ok := r.branches[ref]
	if !ok {
		return nil, notFound("ref " + ref)
	}
	return t, nil
}

// addRepo makes owner/repo exist with an empty default branch. AddRepo calls it.
func (s *filesState) addRepo(owner, repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repos == nil {
		s.repos = map[string]*fileRepo{}
	}
	if _, ok := s.repos[owner+"/"+repo]; !ok {
		s.repos[owner+"/"+repo] = &fileRepo{
			branches: map[string]map[string][]byte{DefaultBranch: {}},
			statuses: map[string][]*forgejo_sdk.Status{},
		}
	}
}

func blobSHA(b []byte) string {
	h := sha1.New() //nolint:gosec // git object ids are SHA-1
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (r *fileRepo) newSHA(seed string) string {
	r.nextID++
	sum := sha1.Sum([]byte(fmt.Sprintf("%d:%s", r.nextID, seed))) //nolint:gosec // git object ids are SHA-1
	return hex.EncodeToString(sum[:])
}

// commit records a commit touching paths and returns it.
func (f *Forgejo) commit(r *fileRepo, message string, paths ...string) *forgejo_sdk.Commit {
	now := f.Now()
	sha := r.newSHA(message)
	c := &forgejo_sdk.Commit{
		CommitMeta: &forgejo_sdk.CommitMeta{SHA: sha, Created: now},
		RepoCommit: &forgejo_sdk.RepoCommit{Message: message},
		Author:     user(Poster),
		Committer:  user(Poster),
	}
	for _, p := range paths {
		c.Files = append(c.Files, &forgejo_sdk.CommitAffectedFiles{Filename: p})
	}
	r.commits = append(r.commits, c)
	return c
}

// SetFile seeds a file on a branch, creating the branch if needed, without
// making a commit.
func (f *Forgejo) SetFile(owner, repo, branch, filePath, content string) error {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return err
	}
	if branch == "" {
		branch = DefaultBranch
	}
	if r.branches[branch] == nil {
		r.branches[branch] = map[string][]byte{}
	}
	r.branches[branch][strings.TrimPrefix(filePath, "/")] = []byte(content)
	return nil
}

// AddCommit seeds a commit with the given sha, message and touched paths.
func (f *Forgejo) AddCommit(owner, repo, sha, message string, paths ...string) error {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return err
	}
	c := f.commit(r, message, paths...)
	c.SHA = sha
	return nil
}

// AddCommitStatus seeds a status on a commit sha. The sha becomes known even if
// no commit was seeded for it.
func (f *Forgejo) AddCommitStatus(owner, repo, sha, statusContext string, state forgejo_sdk.StatusState, description string) error {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return err
	}
	now := f.Now()
	id := int64(len(r.statuses[sha]) + 1)
	r.statuses[sha] = append(r.statuses[sha], &forgejo_sdk.Status{
		ID: id, State: state, Context: statusContext, Description: description,
		TargetURL: "https://ci.example/" + statusContext, Created: now, Updated: now,
	})
	return nil
}

func (r *fileRepo) knowsSHA(sha string) bool {
	if _, ok := r.statuses[sha]; ok {
		return true
	}
	for _, c := range r.commits {
		if c.SHA == sha {
			return true
		}
	}
	return false
}

func contentsOf(p string, b []byte) *forgejo_sdk.ContentsResponse {
	enc := "base64"
	body := base64.StdEncoding.EncodeToString(b)
	return &forgejo_sdk.ContentsResponse{
		Name: path.Base(p), Path: p, SHA: blobSHA(b), Type: "file", Size: int64(len(b)),
		Encoding: &enc, Content: &body,
	}
}

func (f *Forgejo) GetFile(_ context.Context, owner, repo, ref, filePath string) ([]byte, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	t, err := r.tree(ref)
	if err != nil {
		return nil, err
	}
	b, ok := t[strings.TrimPrefix(filePath, "/")]
	if !ok {
		return nil, notFound("file")
	}
	return append([]byte(nil), b...), nil
}

// children returns the immediate entries under dir ("" is the root): files by
// path, subdirectory names as dirs.
func children(t map[string][]byte, dir string) (files map[string][]byte, dirs map[string]bool) {
	files, dirs = map[string][]byte{}, map[string]bool{}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	for p, b := range t {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := p[len(prefix):]
		if i := strings.Index(rest, "/"); i >= 0 {
			dirs[prefix+rest[:i]] = true
		} else {
			files[p] = b
		}
	}
	return files, dirs
}

func (f *Forgejo) GetContents(_ context.Context, owner, repo, ref, filePath string) (*forgejo_sdk.ContentsResponse, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	t, err := r.tree(ref)
	if err != nil {
		return nil, err
	}
	p := strings.Trim(filePath, "/")
	if b, ok := t[p]; ok {
		return contentsOf(p, b), nil
	}
	if fs, ds := children(t, p); p != "" && len(fs)+len(ds) > 0 {
		return nil, errors.New("expect file, got directory")
	}
	return nil, notFound("file")
}

func (f *Forgejo) ListContents(_ context.Context, owner, repo, ref, filePath string) ([]*forgejo_sdk.ContentsResponse, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	t, err := r.tree(ref)
	if err != nil {
		return nil, err
	}
	p := strings.Trim(filePath, "/")
	if _, ok := t[p]; ok {
		return nil, errors.New("expect directory, got file")
	}
	fs, ds := children(t, p)
	if p != "" && len(fs)+len(ds) == 0 {
		return nil, notFound("directory")
	}
	out := make([]*forgejo_sdk.ContentsResponse, 0, len(fs)+len(ds))
	for fp, b := range fs {
		c := contentsOf(fp, b)
		c.Encoding, c.Content = nil, nil
		out = append(out, c)
	}
	for d := range ds {
		out = append(out, &forgejo_sdk.ContentsResponse{Name: path.Base(d), Path: d, Type: "dir"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (f *Forgejo) GetTree(_ context.Context, owner, repo, ref string, opt forgejo_sdk.GetTreesOptions) (*forgejo_sdk.GitTreeResponse, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	t, err := r.tree(ref)
	if err != nil {
		return nil, err
	}
	var entries []forgejo_sdk.GitEntry
	if opt.Recursive {
		for p, b := range t {
			entries = append(entries, forgejo_sdk.GitEntry{Path: p, Mode: "100644", Type: "blob", Size: int64(len(b)), SHA: blobSHA(b)})
		}
	} else {
		fs, ds := children(t, "")
		for p, b := range fs {
			entries = append(entries, forgejo_sdk.GitEntry{Path: p, Mode: "100644", Type: "blob", Size: int64(len(b)), SHA: blobSHA(b)})
		}
		for d := range ds {
			entries = append(entries, forgejo_sdk.GitEntry{Path: d, Mode: "040000", Type: "tree"})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	lo, hi := page(opt.ListOptions, len(entries))
	pg := opt.Page
	if pg == 0 {
		pg = 1
	}
	return &forgejo_sdk.GitTreeResponse{
		SHA: blobSHA([]byte(ref)), Entries: entries[lo:hi], Page: pg, TotalCount: len(entries),
		Truncated: hi < len(entries),
	}, nil
}

// target resolves the branch a write lands on, creating NewBranchName from it.
func (r *fileRepo) target(opt forgejo_sdk.FileOptions) (string, error) {
	branch := opt.BranchName
	if branch == "" {
		branch = DefaultBranch
	}
	src, err := r.tree(branch)
	if err != nil {
		return "", err
	}
	if opt.NewBranchName == "" {
		return branch, nil
	}
	if _, exists := r.branches[opt.NewBranchName]; exists {
		return "", statusErr(http.StatusConflict, "branch "+opt.NewBranchName+" already exists")
	}
	cp := make(map[string][]byte, len(src))
	for k, v := range src {
		cp[k] = v
	}
	r.branches[opt.NewBranchName] = cp
	return opt.NewBranchName, nil
}

func decodeContent(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, statusErr(http.StatusUnprocessableEntity, "invalid base64 content: "+err.Error())
	}
	return b, nil
}

func fileResponse(c *forgejo_sdk.Commit, p string, b []byte) *forgejo_sdk.FileResponse {
	resp := &forgejo_sdk.FileResponse{Commit: &forgejo_sdk.FileCommitResponse{
		CommitMeta: *c.CommitMeta, Message: c.RepoCommit.Message,
	}}
	if b != nil {
		resp.Content = contentsOf(p, b)
		resp.Content.Encoding, resp.Content.Content = nil, nil
	}
	return resp
}

func (f *Forgejo) CreateFile(_ context.Context, owner, repo, filePath string, opt forgejo_sdk.CreateFileOptions) (*forgejo_sdk.FileResponse, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	b, err := decodeContent(opt.Content)
	if err != nil {
		return nil, err
	}
	branch, err := r.target(opt.FileOptions)
	if err != nil {
		return nil, err
	}
	p := strings.TrimPrefix(filePath, "/")
	if _, exists := r.branches[branch][p]; exists {
		return nil, statusErr(http.StatusUnprocessableEntity, "repository file already exists ["+p+"]")
	}
	r.branches[branch][p] = b
	return fileResponse(f.commit(r, opt.Message, p), p, b), nil
}

func (f *Forgejo) UpdateFile(_ context.Context, owner, repo, filePath string, opt forgejo_sdk.UpdateFileOptions) (*forgejo_sdk.FileResponse, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	b, err := decodeContent(opt.Content)
	if err != nil {
		return nil, err
	}
	p := strings.TrimPrefix(filePath, "/")
	t, err := r.tree(opt.BranchName)
	if err != nil {
		return nil, err
	}
	old, ok := t[p]
	if !ok {
		return nil, notFound("file")
	}
	if opt.SHA != blobSHA(old) {
		return nil, statusErr(http.StatusUnprocessableEntity, "sha does not match ["+opt.SHA+" != "+blobSHA(old)+"]")
	}
	branch, err := r.target(opt.FileOptions)
	if err != nil {
		return nil, err
	}
	r.branches[branch][p] = b
	return fileResponse(f.commit(r, opt.Message, p), p, b), nil
}

func (f *Forgejo) DeleteFile(_ context.Context, owner, repo, filePath string, opt forgejo_sdk.DeleteFileOptions) error {
	if err := f.refused("DeleteFile"); err != nil {
		return err
	}
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return err
	}
	p := strings.TrimPrefix(filePath, "/")
	t, err := r.tree(opt.BranchName)
	if err != nil {
		return err
	}
	old, ok := t[p]
	if !ok {
		return notFound("file")
	}
	if opt.SHA != blobSHA(old) {
		return statusErr(http.StatusUnprocessableEntity, "sha does not match ["+opt.SHA+" != "+blobSHA(old)+"]")
	}
	branch, err := r.target(opt.FileOptions)
	if err != nil {
		return err
	}
	delete(r.branches[branch], p)
	f.commit(r, opt.Message, p)
	return nil
}

func (f *Forgejo) ChangeFiles(_ context.Context, owner, repo string, opt forge.ChangeFilesOptions) (map[string]any, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	src, err := r.tree(opt.BranchName)
	if err != nil {
		return nil, err
	}
	// Work on a copy so a failing operation leaves nothing behind.
	work := make(map[string][]byte, len(src))
	for k, v := range src {
		work[k] = v
	}
	bad := func(format string, a ...any) error {
		return statusErr(http.StatusUnprocessableEntity, fmt.Sprintf(format, a...))
	}
	var paths []string
	for _, op := range opt.Files {
		old, exists := work[op.Path]
		switch op.Operation {
		case "create":
			if exists {
				return nil, bad("repository file already exists [%s]", op.Path)
			}
		case "update", "delete":
			if !exists {
				return nil, notFound("file " + op.Path)
			}
			if op.SHA != blobSHA(old) {
				return nil, bad("sha does not match for %s", op.Path)
			}
		default:
			return nil, bad("unsupported operation %q", op.Operation)
		}
		if op.Operation == "delete" {
			delete(work, op.Path)
		} else {
			if op.Content == nil {
				return nil, bad("content is required for %s", op.Path)
			}
			b, err := decodeContent(*op.Content)
			if err != nil {
				return nil, err
			}
			work[op.Path] = b
		}
		paths = append(paths, op.Path)
	}
	branch := opt.BranchName
	if branch == "" {
		branch = DefaultBranch
	}
	if opt.NewBranchName != "" {
		if _, exists := r.branches[opt.NewBranchName]; exists {
			return nil, statusErr(http.StatusConflict, "branch "+opt.NewBranchName+" already exists")
		}
		branch = opt.NewBranchName
	}
	r.branches[branch] = work
	c := f.commit(r, opt.Message, paths...)
	files := make([]any, 0, len(opt.Files))
	for _, op := range opt.Files {
		files = append(files, map[string]any{"path": op.Path, "operation": op.Operation})
	}
	return map[string]any{
		"commit": map[string]any{"sha": c.SHA, "message": opt.Message},
		"files":  files,
	}, nil
}

func (f *Forgejo) GetCommit(_ context.Context, owner, repo, sha string) (*forgejo_sdk.Commit, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	for _, c := range r.commits {
		if c.SHA == sha {
			cp := *c
			return &cp, nil
		}
	}
	return nil, notFound("commit")
}

func (f *Forgejo) ListCommits(_ context.Context, owner, repo string, opt forgejo_sdk.ListCommitOptions) ([]*forgejo_sdk.Commit, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return nil, err
	}
	var all []*forgejo_sdk.Commit
	for i := len(r.commits) - 1; i >= 0; i-- {
		c := r.commits[i]
		if opt.Path != "" && !touches(c, opt.Path) {
			continue
		}
		cp := *c
		all = append(all, &cp)
	}
	lo, hi := page(opt.ListOptions, len(all))
	return all[lo:hi], nil
}

func touches(c *forgejo_sdk.Commit, p string) bool {
	p = strings.Trim(p, "/")
	for _, f := range c.Files {
		if f.Filename == p || strings.HasPrefix(f.Filename, p+"/") {
			return true
		}
	}
	return false
}

func (f *Forgejo) ListCommitStatuses(_ context.Context, owner, repo, sha string, opt forgejo_sdk.ListStatusesOption) (forge.CommitStatuses, error) {
	f.files.mu.Lock()
	defer f.files.mu.Unlock()
	r, err := f.files.repo(owner, repo)
	if err != nil {
		return forge.CommitStatuses{}, err
	}
	if !r.knowsSHA(sha) {
		return forge.CommitStatuses{}, notFound("commit")
	}
	all := r.statuses[sha]
	lo, hi := page(opt.ListOptions, len(all))
	total := len(all)
	return forge.CommitStatuses{Statuses: append([]*forgejo_sdk.Status(nil), all[lo:hi]...), TotalCount: &total}, nil
}

// AddCommitStatusState is AddCommitStatus with the state given as a plain
// string, for callers (use-case tests under app/) that may not import the SDK.
func (f *Forgejo) AddCommitStatusState(owner, repo, sha, statusContext, state, description string) error {
	return f.AddCommitStatus(owner, repo, sha, statusContext, forgejo_sdk.StatusState(state), description)
}
