// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
)

// GitService runs the branch, commit, contents and file use cases over a
// GitPort. Errors from the port are wrapped with the wording the tools have
// always reported.
type GitService struct{ port GitPort }

// NewGitService returns the use cases over p.
func NewGitService(p GitPort) *GitService { return &GitService{port: p} }

// ListBranches returns one page of a repository's branches with its next marker.
func (s *GitService) ListBranches(ctx context.Context, owner, repo string, p GitPage) (model.BranchPage, error) {
	c := ceiling(ctx, s.port)
	page, err := list.Run(ctx, list.Given(list.Paging(p), defaultListLimit, c), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.Branch], error) {
			out, err := s.port.ListBranches(ctx, owner, repo, GitPage(q))
			if err != nil {
				return list.Batch[*model.Branch]{}, fmt.Errorf("list branches err: %w", err)
			}
			return list.Batch[*model.Branch]{Items: out}, nil
		})
	if err != nil {
		return model.BranchPage{}, err
	}
	return model.BranchPage{Branches: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// defaultListLimit is the page size a repository list uses when the caller gives
// none; the ceiling lowers it when the instance's ceiling is smaller.
const defaultListLimit = 100

// ceiling reads the instance's pagination ceiling through a port, per call.
func ceiling(ctx context.Context, src list.CeilingSource) list.Ceiling {
	max, known := src.MaxResponseItems(ctx)
	return list.Ceiling{Max: max, Known: known}
}

// CreateBranch creates a branch from another one.
func (s *GitService) CreateBranch(ctx context.Context, owner, repo string, in CreateBranchInput) error {
	if err := s.port.CreateBranch(ctx, owner, repo, in); err != nil {
		return fmt.Errorf("create branch error: %w", err)
	}
	return nil
}

// DeleteBranch deletes a branch. Forgejo declining to answer 204 is an error.
func (s *GitService) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	deleted, err := s.port.DeleteBranch(ctx, owner, repo, branch)
	if err != nil {
		return fmt.Errorf("delete branch err: %w", err)
	}
	if !deleted {
		return fmt.Errorf("failed to delete branch (status not 204)")
	}
	return nil
}

// ListCommits returns one page of a repository's commits, newest first, with
// its next marker.
func (s *GitService) ListCommits(ctx context.Context, owner, repo string, q ListCommitsInput) (model.CommitPage, error) {
	c := ceiling(ctx, s.port)
	page, err := list.Run(ctx, list.Given(list.Paging(q.Page), defaultListLimit, c), c,
		func(ctx context.Context, p list.Paging) (list.Batch[*model.Commit], error) {
			in := q
			in.Page = GitPage(p)
			out, err := s.port.ListCommits(ctx, owner, repo, in)
			if err != nil {
				return list.Batch[*model.Commit]{}, fmt.Errorf("list repo commits error: %w", err)
			}
			return list.Batch[*model.Commit]{Items: out}, nil
		})
	if err != nil {
		return model.CommitPage{}, err
	}
	return model.CommitPage{Commits: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// ListContents returns one page of the entries of one directory level of a
// repository at a ref. Forgejo does not page this listing, so the whole level
// is read and cut here; the whole count is not reported as TotalCount, since
// Forgejo does not report it.
func (s *GitService) ListContents(ctx context.Context, owner, repo, ref, path string, p GitPage) (model.ContentsPage, error) {
	c := ceiling(ctx, s.port)
	page, err := list.Run(ctx, list.Given(list.Paging(p), defaultListLimit, c), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.ContentsResponse], error) {
			all, err := s.port.ListContents(ctx, owner, repo, ref, path)
			if err != nil {
				return list.Batch[*model.ContentsResponse]{}, fmt.Errorf("list repo contents err: %w", err)
			}
			rows, _ := list.Cut(all, q)
			return list.Batch[*model.ContentsResponse]{Items: rows, Total: ptr.To(len(all))}, nil
		})
	if err != nil {
		return model.ContentsPage{}, err
	}
	rows := page.Items
	if rows == nil {
		rows = []*model.ContentsResponse{}
	}
	meta := model.PageMetaOf(page)
	meta.TotalCount = nil
	return model.ContentsPage{Contents: rows, PageMeta: meta}, nil
}

// GetTree returns the Git tree of a repository at a ref.
func (s *GitService) GetTree(ctx context.Context, owner, repo, ref string, q TreeInput) (*model.GitTreeResponse, error) {
	out, err := s.port.GetTree(ctx, owner, repo, ref, q)
	if err != nil {
		return nil, fmt.Errorf("get repo tree err: %w", err)
	}
	return out, nil
}

// InlineFileBytes is the most text one file read returns. A read whose text
// would be larger is refused, with a hint to ask for a narrower line range, so
// no single file read floods the caller's context.
const InlineFileBytes = 1 << 20

// tooLarge is the refusal for a file read whose text is over InlineFileBytes.
func tooLarge(size int, ranged bool) error {
	if ranged {
		return fmt.Errorf("the requested lines are %d bytes, over the %d-byte inline limit; narrow the range with start_line and end_line", size, InlineFileBytes)
	}
	return fmt.Errorf("file is %d bytes, over the %d-byte inline limit; read a line range with start_line and end_line", size, InlineFileBytes)
}

// GetFileMetadata returns a file's entry with its content in Base64. A line
// range replaces the content with the Base64 of that slice and keeps the
// metadata, so a large file's section comes back with its entry.
func (s *GitService) GetFileMetadata(ctx context.Context, owner, repo, ref, path string, lines LineRange) (*model.ContentsResponse, error) {
	out, err := s.port.GetContents(ctx, owner, repo, ref, path)
	if err != nil {
		return nil, fmt.Errorf("get file err: %w", err)
	}
	if out == nil || out.Content == nil {
		return out, nil
	}
	if lines.Start == 0 && lines.End == 0 {
		if out.Size > InlineFileBytes {
			return nil, tooLarge(int(out.Size), false)
		}
		return out, nil
	}
	raw, err := base64.StdEncoding.DecodeString(*out.Content)
	if err != nil {
		return nil, fmt.Errorf("decode file content err: %w", err)
	}
	part, err := SliceLines(string(raw), lines.Start, lines.End)
	if err != nil {
		return nil, err
	}
	if len(part) > InlineFileBytes {
		return nil, tooLarge(len(part), true)
	}
	enc := base64.StdEncoding.EncodeToString([]byte(part))
	out.Content = &enc
	return out, nil
}

// LineRange is a 1-indexed inclusive slice of a file. Both zero means the
// whole file.
type LineRange struct{ Start, End int }

// GetFileText returns a file as plain text, sliced to lines when a range is
// given. A whole file, or a range, whose text is over InlineFileBytes is refused.
func (s *GitService) GetFileText(ctx context.Context, owner, repo, ref, path string, lines LineRange) (string, error) {
	raw, err := s.port.GetFile(ctx, owner, repo, ref, path)
	if err != nil {
		return "", fmt.Errorf("get file err: %w", err)
	}
	if lines.Start == 0 && lines.End == 0 {
		if len(raw) > InlineFileBytes {
			return "", tooLarge(len(raw), false)
		}
		return string(raw), nil
	}
	part, err := SliceLines(string(raw), lines.Start, lines.End)
	if err != nil {
		return "", err
	}
	if len(part) > InlineFileBytes {
		return "", tooLarge(len(part), true)
	}
	return part, nil
}

// SliceLines returns the 1-indexed inclusive [start, end] line range
// of content. start=0 means "from line 1"; end=0 means "to the last
// line". Out-of-range values clamp; an inverted range after clamping
// is reported as an error.
func SliceLines(content string, start, end int) (string, error) {
	lines := strings.Split(content, "\n")
	count := len(lines)

	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = count
	}
	if end > count {
		end = count
	}
	if start > end {
		return "", fmt.Errorf("start_line (%d) is after end_line (%d) for a file with %d lines", start, end, count)
	}

	return strings.Join(lines[start-1:end], "\n"), nil
}

// CreateFile creates a file in a branch.
func (s *GitService) CreateFile(ctx context.Context, owner, repo, path string, in FileWriteInput) (*model.FileResponse, error) {
	out, err := s.port.CreateFile(ctx, owner, repo, path, in)
	if err != nil {
		return nil, fmt.Errorf("create file error: %w", err)
	}
	return out, nil
}

// UpdateFile replaces a file in a branch, given its current SHA.
func (s *GitService) UpdateFile(ctx context.Context, owner, repo, path string, in FileWriteInput) (*model.FileResponse, error) {
	out, err := s.port.UpdateFile(ctx, owner, repo, path, in)
	if err != nil {
		return nil, fmt.Errorf("update file error: %w", err)
	}
	return out, nil
}

// DeleteFile removes a file from a branch, given its current SHA.
func (s *GitService) DeleteFile(ctx context.Context, owner, repo, path string, in FileDeleteInput) error {
	if err := s.port.DeleteFile(ctx, owner, repo, path, in); err != nil {
		return fmt.Errorf("delete file err: %w", err)
	}
	return nil
}

// ChangeFiles applies a batch of file operations in one commit.
func (s *GitService) ChangeFiles(ctx context.Context, owner, repo string, in ChangeFilesInput) (map[string]any, error) {
	out, err := s.port.ChangeFiles(ctx, owner, repo, in)
	if err != nil {
		return nil, fmt.Errorf("change files error: %w", err)
	}
	return out, nil
}
