// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// listGit is a fakeGit whose ListContents answers a fixed directory.
type listGit struct {
	*fakeGit
	entries []*model.ContentsResponse
}

func (g listGit) ListContents(context.Context, string, string, string, string) ([]*model.ContentsResponse, error) {
	return g.entries, nil
}

func directory(n int) []*model.ContentsResponse {
	out := make([]*model.ContentsResponse, n)
	for i := range out {
		out[i] = &model.ContentsResponse{Name: fmt.Sprintf("f%d", i), Path: fmt.Sprintf("d/f%d", i), Type: "file"}
	}
	return out
}

func TestListContentsReturnsOnePageWithItsNextMarker(t *testing.T) {
	t.Parallel()
	svc := NewGitService(listGit{fakeGit: &fakeGit{}, entries: directory(5)})
	page, err := svc.ListContents(context.Background(), "o", "r", "main", "d", GitPage{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Contents) != 2 || page.Contents[0].Name != "f0" || page.Count != 2 || !page.HasNext || page.TotalCount != nil {
		t.Fatalf("page 1: %+v", page)
	}
	last, err := svc.ListContents(context.Background(), "o", "r", "main", "d", GitPage{Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Contents) != 1 || last.Contents[0].Name != "f4" || last.HasNext {
		t.Fatalf("last page: %+v", last)
	}
}

func TestListContentsPastTheEndIsEmptyNotNull(t *testing.T) {
	t.Parallel()
	svc := NewGitService(listGit{fakeGit: &fakeGit{}, entries: directory(3)})
	got, err := svc.ListContents(context.Background(), "o", "r", "main", "d", GitPage{Page: 9, Limit: 2})
	if err != nil || got.Contents == nil || len(got.Contents) != 0 || got.HasNext {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestListContentsUsesTheDefaultLimitWhenNoneIsGiven(t *testing.T) {
	t.Parallel()
	svc := NewGitService(listGit{fakeGit: &fakeGit{}, entries: directory(defaultListLimit + 1)})
	got, err := svc.ListContents(context.Background(), "o", "r", "main", "d", GitPage{})
	if err != nil || got.Limit != defaultListLimit || len(got.Contents) != defaultListLimit || !got.HasNext {
		t.Fatalf("got limit %d, %d rows, has_next %v, %v", got.Limit, len(got.Contents), got.HasNext, err)
	}
}
