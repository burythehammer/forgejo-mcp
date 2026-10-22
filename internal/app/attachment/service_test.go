// SPDX-License-Identifier: GPL-3.0-or-later

package attachment_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// stubPort is an in-memory attachment.Port. The fields say what it does.
type stubPort struct {
	err       error
	listErr   error
	att       model.Attachment
	body      model.AttachmentBody
	fetchErr  error
	fetched   int
	slowCreat bool
}

func (s *stubPort) List(context.Context, string, string, attachment.Scope, int64) ([]*model.Attachment, error) {
	return nil, s.listErr
}
func (s *stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }
func (s *stubPort) Get(context.Context, string, string, attachment.Scope, int64, int64) (model.Attachment, error) {
	return s.att, s.err
}
func (s *stubPort) Create(ctx context.Context, _, _ string, _ attachment.Scope, _ int64, _ attachment.Upload) (model.Attachment, error) {
	if s.slowCreat {
		<-ctx.Done()
		return model.Attachment{}, ctx.Err()
	}
	return s.att, s.err
}
func (s *stubPort) Edit(context.Context, string, string, attachment.Scope, int64, int64, string) (model.Attachment, error) {
	return s.att, s.err
}
func (s *stubPort) Delete(context.Context, string, string, attachment.Scope, int64, int64) error {
	return s.err
}
func (s *stubPort) Download(context.Context, string) (model.AttachmentBody, error) {
	s.fetched++
	return s.body, s.fetchErr
}

// A list under a missing parent is the shared not-found outcome, not an empty list.
func TestListNotFoundIsNotFound(t *testing.T) {
	t.Parallel()
	missing := &forge.StatusError{StatusCode: http.StatusNotFound, Err: errors.New("issue 4 not found")}
	_, err := attachment.New(&stubPort{listErr: missing}, time.Minute).List(context.Background(), "o", "r", attachment.ScopeIssue, 4, attachment.Paging{Page: 1, Limit: 20})
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}

func TestListErrorsNameTheScope(t *testing.T) {
	t.Parallel()
	_, err := attachment.New(&stubPort{listErr: errors.New("boom")}, time.Minute).List(context.Background(), "o", "r", attachment.ScopeComment, 4, attachment.Paging{Page: 1, Limit: 20})
	if err == nil || err.Error() != "list comment attachments err: boom" {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadOverTheCapIsNotFetched(t *testing.T) {
	t.Parallel()
	p := &stubPort{att: model.Attachment{Size: download.MaxInlineBytes, DownloadURL: "u"}}
	dl, err := attachment.New(p, time.Minute).Download(context.Background(), "o", "r", attachment.ScopeIssue, 4, 7)
	if err != nil {
		t.Fatal(err)
	}
	if dl.Result.Inline || p.fetched != 0 {
		t.Fatalf("got %+v after %d fetches", dl.Result, p.fetched)
	}
}

func TestDownloadBodyOverTheCapFallsBack(t *testing.T) {
	t.Parallel()
	p := &stubPort{att: model.Attachment{Size: 3}, fetchErr: download.ErrPayloadTooLarge}
	dl, err := attachment.New(p, time.Minute).Download(context.Background(), "o", "r", attachment.ScopeComment, 4, 7)
	if err != nil {
		t.Fatal(err)
	}
	if dl.Result.Inline || !strings.Contains(dl.Result.Reason, "exceeded inline cap") {
		t.Fatalf("got %+v", dl.Result)
	}
}

func TestCreateTimeoutNamesTheUploadBudget(t *testing.T) {
	t.Parallel()
	p := &stubPort{slowCreat: true}
	_, err := attachment.New(p, 5*time.Millisecond).Create(context.Background(), "o", "r", attachment.ScopeIssue, 4,
		attachment.Upload{Filename: "a", Content: strings.NewReader("x")})
	if err == nil || !strings.Contains(err.Error(), "create issue attachment err: upload timed out after 5ms") {
		t.Fatalf("got %v", err)
	}
}

func TestCreatePassesTheUploadThrough(t *testing.T) {
	t.Parallel()
	var seen attachment.Upload
	_, err := attachment.New(&uploadPort{seen: &seen}, time.Minute).Create(context.Background(), "o", "r", attachment.ScopeComment, 4,
		attachment.Upload{Filename: "a.txt", MIMEType: "text/plain", Content: strings.NewReader("x")})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(seen.Content)
	if seen.Filename != "a.txt" || seen.MIMEType != "text/plain" || string(b) != "x" {
		t.Fatalf("upload %+v body %q", seen, b)
	}
}

type uploadPort struct {
	stubPort
	seen *attachment.Upload
}

func (u *uploadPort) Create(_ context.Context, _, _ string, _ attachment.Scope, _ int64, up attachment.Upload) (model.Attachment, error) {
	*u.seen = up
	return model.Attachment{}, nil
}
