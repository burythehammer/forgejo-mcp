// SPDX-License-Identifier: GPL-3.0-or-later

package attachment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// listedPort is a stubPort whose List answers a fixed attachment list.
type listedPort struct {
	*stubPort
	atts []*model.Attachment
}

func (p listedPort) List(context.Context, string, string, attachment.Scope, int64) ([]*model.Attachment, error) {
	return p.atts, p.listErr
}

func attachments(n int) []*model.Attachment {
	out := make([]*model.Attachment, n)
	for i := range out {
		out[i] = &model.Attachment{ID: int64(i + 1), Name: fmt.Sprintf("a%d.txt", i+1)}
	}
	return out
}

func TestListReturnsOnePageWithItsNextMarker(t *testing.T) {
	t.Parallel()
	svc := attachment.New(listedPort{stubPort: &stubPort{}, atts: attachments(5)}, time.Minute)
	page, err := svc.List(context.Background(), "o", "r", attachment.ScopeIssue, 4, attachment.Paging{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Attachments) != 2 || page.Attachments[0].Name != "a1.txt" || page.Count != 2 || !page.HasNext || page.TotalCount != nil {
		t.Fatalf("page 1: %+v", page)
	}
	last, err := svc.List(context.Background(), "o", "r", attachment.ScopeIssue, 4, attachment.Paging{Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Attachments) != 1 || last.Attachments[0].Name != "a5.txt" || last.HasNext {
		t.Fatalf("last page: %+v", last)
	}
}

func TestListDefaultLimitIsTwentyAndPastTheEndIsEmpty(t *testing.T) {
	t.Parallel()
	svc := attachment.New(listedPort{stubPort: &stubPort{}, atts: attachments(21)}, time.Minute)
	first, err := svc.List(context.Background(), "o", "r", attachment.ScopeComment, 4, attachment.Paging{})
	if err != nil || first.Limit != 20 || len(first.Attachments) != 20 || !first.HasNext {
		t.Fatalf("default page: %+v, %v", first, err)
	}
	past, err := svc.List(context.Background(), "o", "r", attachment.ScopeComment, 4, attachment.Paging{Page: 9, Limit: 20})
	if err != nil || past.Attachments == nil || len(past.Attachments) != 0 || past.HasNext {
		t.Fatalf("past the end: %+v, %v", past, err)
	}
}

func TestListNotFoundPagesAsAnEmptyList(t *testing.T) {
	t.Parallel()
	svc := attachment.New(listedPort{stubPort: &stubPort{listErr: attachment.ErrNotFound}}, time.Minute)
	page, err := svc.List(context.Background(), "o", "r", attachment.ScopeIssue, 4, attachment.Paging{Page: 1, Limit: 5})
	if err != nil || page.Attachments == nil || len(page.Attachments) != 0 || page.HasNext {
		t.Fatalf("got %+v, %v", page, err)
	}
}
