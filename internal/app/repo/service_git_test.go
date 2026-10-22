// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// fakeGit is a GitPort whose answers the tests set. Methods a test does not use
// return zero values.
type fakeGit struct {
	deleted bool
	file    []byte
	err     error
}

func (f *fakeGit) ListBranches(context.Context, string, string, GitPage) ([]*model.Branch, error) {
	return nil, f.err
}
func (f *fakeGit) CreateBranch(context.Context, string, string, CreateBranchInput) error {
	return f.err
}
func (f *fakeGit) DeleteBranch(context.Context, string, string, string) (bool, error) {
	return f.deleted, f.err
}
func (f *fakeGit) ListCommits(context.Context, string, string, ListCommitsInput) ([]*model.Commit, error) {
	return nil, f.err
}
func (f *fakeGit) GetCommit(context.Context, string, string, string) (*model.Commit, error) {
	return nil, f.err
}
func (f *fakeGit) ListContents(context.Context, string, string, string, string) ([]*model.ContentsResponse, error) {
	return nil, f.err
}
func (f *fakeGit) GetTree(context.Context, string, string, string, TreeInput) (*model.GitTreeResponse, error) {
	return nil, f.err
}
func (f *fakeGit) GetFile(context.Context, string, string, string, string) ([]byte, error) {
	return f.file, f.err
}
func (f *fakeGit) GetContents(context.Context, string, string, string, string) (*model.ContentsResponse, error) {
	return nil, f.err
}
func (f *fakeGit) CreateFile(context.Context, string, string, string, FileWriteInput) (*model.FileResponse, error) {
	return nil, f.err
}
func (f *fakeGit) UpdateFile(context.Context, string, string, string, FileWriteInput) (*model.FileResponse, error) {
	return nil, f.err
}
func (f *fakeGit) DeleteFile(context.Context, string, string, string, FileDeleteInput) error {
	return f.err
}
func (f *fakeGit) ChangeFiles(context.Context, string, string, ChangeFilesInput) (map[string]any, error) {
	return nil, f.err
}

func TestDeleteBranchNotDeletedIsAnError(t *testing.T) {
	t.Parallel()
	err := NewGitService(&fakeGit{deleted: false}).DeleteBranch(context.Background(), "o", "r", "b")
	if err == nil || err.Error() != "failed to delete branch (status not 204)" {
		t.Fatalf("got %v, want the status error", err)
	}
}

func TestPortErrorsKeepTheToolWording(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	svc := NewGitService(&fakeGit{err: boom})
	ctx := context.Background()
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"create branch", svc.CreateBranch(ctx, "o", "r", CreateBranchInput{}), "create branch error: boom"},
		{"delete branch", svc.DeleteBranch(ctx, "o", "r", "b"), "delete branch err: boom"},
		{"create file", func() error { _, err := svc.CreateFile(ctx, "o", "r", "p", FileWriteInput{}); return err }(), "create file error: boom"},
		{"delete file", svc.DeleteFile(ctx, "o", "r", "p", FileDeleteInput{}), "delete file err: boom"},
		{"get file", func() error { _, err := svc.GetFileText(ctx, "o", "r", "main", "p", LineRange{}); return err }(), "get file err: boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.err == nil || c.err.Error() != c.want {
				t.Fatalf("got %v, want %q", c.err, c.want)
			}
		})
	}
}

func TestGetFileTextSlicesOnlyWhenARangeIsGiven(t *testing.T) {
	t.Parallel()
	svc := NewGitService(&fakeGit{file: []byte("a\nb\nc\n")})
	whole, err := svc.GetFileText(context.Background(), "o", "r", "main", "p", LineRange{})
	if err != nil || whole != "a\nb\nc\n" {
		t.Fatalf("whole file: got %q, %v", whole, err)
	}
	part, err := svc.GetFileText(context.Background(), "o", "r", "main", "p", LineRange{Start: 2, End: 3})
	if err != nil || part != "b\nc" {
		t.Fatalf("range: got %q, %v", part, err)
	}
}

func TestSliceLines(t *testing.T) {
	t.Parallel()
	const content = "one\ntwo\nthree"
	cases := []struct {
		name       string
		start, end int
		want       string
		wantErr    bool
	}{
		{name: "whole file", start: 0, end: 0, want: content},
		{name: "from start to end", start: 2, end: 3, want: "two\nthree"},
		{name: "only start", start: 3, end: 0, want: "three"},
		{name: "only end", start: 0, end: 1, want: "one"},
		{name: "end clamps", start: 2, end: 99, want: "two\nthree"},
		{name: "start clamps below one", start: -4, end: 1, want: "one"},
		{name: "inverted after clamping", start: 3, end: 1, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := SliceLines(content, c.start, c.end)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (*fakeGit) MaxResponseItems(context.Context) (int, bool) { return 0, false }

// pagedGit is a GitPort that serves n numbered branches and commits, sliced by
// the page and limit the use case passes, so a test sees the window it asked for.
type pagedGit struct {
	fakeGit
	n int
}

func (p *pagedGit) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func (p *pagedGit) ListBranches(_ context.Context, _, _ string, pg GitPage) ([]*model.Branch, error) {
	var out []*model.Branch
	for _, i := range window(p.n, pg) {
		out = append(out, &model.Branch{Name: fmt.Sprintf("b%d", i)})
	}
	return out, nil
}

func (p *pagedGit) ListCommits(_ context.Context, _, _ string, q ListCommitsInput) ([]*model.Commit, error) {
	var out []*model.Commit
	for _, i := range window(p.n, q.Page) {
		out = append(out, &model.Commit{CommitMeta: &model.CommitMeta{SHA: fmt.Sprintf("c%d", i)}})
	}
	return out, nil
}

// window numbers the rows of one page of n rows from 1; a page past the end is
// empty.
func window(n int, pg GitPage) []int {
	var rows []int
	for i := (pg.Page - 1) * pg.Limit; i < pg.Page*pg.Limit && i < n; i++ {
		rows = append(rows, i+1)
	}
	return rows
}

func TestBranchAndCommitPagesReportTheNextMarker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		n, page, want int
		hasNext       bool
	}{
		{"full page with more rows", 5, 1, 2, true},
		{"middle page", 5, 2, 2, true},
		{"short last page", 5, 3, 1, false},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"past the end", 4, 3, 0, false},
	}
	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := NewGitService(&pagedGit{n: c.n})
			pg := GitPage{Page: c.page, Limit: 2}
			b, err := s.ListBranches(ctx, "o", "r", pg)
			if err != nil || len(b.Items) != c.want || b.Count != c.want || b.HasNext != c.hasNext || b.Page != c.page || b.Limit != 2 {
				t.Fatalf("branches: %+v, %v", b, err)
			}
			m, err := s.ListCommits(ctx, "o", "r", ListCommitsInput{Page: pg})
			if err != nil || len(m.Items) != c.want || m.Count != c.want || m.HasNext != c.hasNext || m.Page != c.page || m.Limit != 2 {
				t.Fatalf("commits: %+v, %v", m, err)
			}
		})
	}
}

func TestBranchAndCommitPageZeroIsTheFirstPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewGitService(&pagedGit{n: 5})
	pg := GitPage{Page: 0, Limit: 2}
	b, err := s.ListBranches(ctx, "o", "r", pg)
	if err != nil || b.Page != 1 || b.Count != 2 || !b.HasNext || b.Items[0].Name != "b1" {
		t.Fatalf("branches page 0: %+v, %v", b, err)
	}
	m, err := s.ListCommits(ctx, "o", "r", ListCommitsInput{Page: pg})
	if err != nil || m.Page != 1 || m.Count != 2 || !m.HasNext || m.Items[0].SHA != "c1" {
		t.Fatalf("commits page 0: %+v, %v", m, err)
	}
}
