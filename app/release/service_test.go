// SPDX-License-Identifier: GPL-3.0-or-later

package release_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/download"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apprelease "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/release"
)

// stubPort is an in-memory apprelease.Port. The fields say what it returns.
type stubPort struct {
	err      error
	rels     []*model.Release
	att      model.Attachment
	atts     []*model.Attachment
	body     model.AttachmentBody
	fetchErr error
	fetched  int
	gotTitle string
}

func (s *stubPort) ListReleases(context.Context, string, string, apprelease.Paging) ([]*model.Release, error) {
	return s.rels, s.err
}
func (s *stubPort) GetRelease(context.Context, string, string, int64) (model.Release, error) {
	return model.Release{}, s.err
}
func (s *stubPort) GetReleaseByTag(context.Context, string, string, string) (model.Release, error) {
	return model.Release{}, s.err
}
func (s *stubPort) GetLatestRelease(context.Context, string, string) (model.Release, error) {
	return model.Release{}, s.err
}
func (s *stubPort) CreateRelease(_ context.Context, _, _ string, in apprelease.CreateInput) (model.Release, error) {
	s.gotTitle = in.Title
	return model.Release{}, s.err
}
func (s *stubPort) EditRelease(context.Context, string, string, int64, apprelease.EditInput) (model.Release, error) {
	return model.Release{}, s.err
}
func (s *stubPort) DeleteRelease(context.Context, string, string, int64) error       { return s.err }
func (s *stubPort) DeleteReleaseByTag(context.Context, string, string, string) error { return s.err }
func (s *stubPort) ListAttachments(context.Context, string, string, int64) ([]*model.Attachment, error) {
	return s.atts, s.err
}
func (s *stubPort) GetAttachment(context.Context, string, string, int64, int64) (model.Attachment, error) {
	return s.att, s.err
}
func (s *stubPort) CreateAttachment(context.Context, string, string, int64, apprelease.Upload) (model.Attachment, error) {
	return s.att, s.err
}
func (s *stubPort) EditAttachment(context.Context, string, string, int64, int64, string) (model.Attachment, error) {
	return s.att, s.err
}
func (s *stubPort) DeleteAttachment(context.Context, string, string, int64, int64) error {
	return s.err
}
func (s *stubPort) DownloadAttachment(_ context.Context, url string) (model.AttachmentBody, error) {
	s.fetched++
	return s.body, s.fetchErr
}

func TestListFiltersByStateAndNeverReturnsNil(t *testing.T) {
	t.Parallel()
	p := &stubPort{rels: []*model.Release{
		{TagName: "d", IsDraft: true},
		{TagName: "p", IsPrerelease: true},
		{TagName: "u"},
		nil,
	}}
	svc := apprelease.New(p)
	for state, want := range map[string]string{"": "d p u", "all": "d p u", "draft": "d", "prerelease": "p", "published": "u"} {
		got, err := svc.ListReleases(context.Background(), "o", "r", apprelease.ListQuery{State: state})
		if err != nil {
			t.Fatal(err)
		}
		var tags []string
		for _, r := range got {
			if r != nil {
				tags = append(tags, r.TagName)
			}
		}
		if strings.Join(tags, " ") != want {
			t.Errorf("state %q: got %v, want %s", state, tags, want)
		}
	}
	empty, _ := apprelease.New(&stubPort{}).ListReleases(context.Background(), "o", "r", apprelease.ListQuery{State: "draft"})
	if empty == nil {
		t.Fatal("an empty list must be [] not nil")
	}
}

func TestCreateDefaultsTitleToTheTag(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	if _, err := apprelease.New(p).CreateRelease(context.Background(), "o", "r", apprelease.CreateInput{TagName: "v1"}); err != nil {
		t.Fatal(err)
	}
	if p.gotTitle != "v1" {
		t.Fatalf("title %q", p.gotTitle)
	}
}

func TestListAttachmentsPagesAndPastTheEnd(t *testing.T) {
	t.Parallel()
	p := &stubPort{atts: []*model.Attachment{{ID: 1}, {ID: 2}, {ID: 3}}}
	svc := apprelease.New(p)
	page, _ := svc.ListAttachments(context.Background(), "o", "r", 1, apprelease.Paging{Page: 2, Limit: 2})
	if len(page) != 1 || page[0].ID != 3 {
		t.Fatalf("page 2: %+v", page)
	}
	beyond, _ := svc.ListAttachments(context.Background(), "o", "r", 1, apprelease.Paging{Page: 9, Limit: 2})
	if beyond == nil || len(beyond) != 0 {
		t.Fatalf("past the end: %+v", beyond)
	}
}

func TestDownloadInlineUnderTheCap(t *testing.T) {
	t.Parallel()
	p := &stubPort{att: model.Attachment{ID: 7, Size: 5, DownloadURL: "u"}, body: model.AttachmentBody{Body: []byte("hello"), ContentType: "text/plain"}}
	dl, err := apprelease.New(p).DownloadAttachment(context.Background(), "o", "r", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !dl.Result.Inline || dl.Result.BytesIncluded != 5 || string(dl.Body.Body) != "hello" {
		t.Fatalf("got %+v", dl)
	}
}

func TestDownloadOverTheCapIsMetadataOnly(t *testing.T) {
	t.Parallel()
	p := &stubPort{att: model.Attachment{ID: 7, Size: download.MaxInlineBytes}}
	dl, err := apprelease.New(p).DownloadAttachment(context.Background(), "o", "r", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if dl.Result.Inline || p.fetched != 0 || dl.Result.Reason == "" {
		t.Fatalf("an oversized asset must not be fetched: %+v, fetches %d", dl.Result, p.fetched)
	}
}

func TestDownloadBodyGrowingPastTheCapIsMetadataOnly(t *testing.T) {
	t.Parallel()
	p := &stubPort{att: model.Attachment{ID: 7, Size: 10}, fetchErr: download.ErrPayloadTooLarge}
	dl, err := apprelease.New(p).DownloadAttachment(context.Background(), "o", "r", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if dl.Result.Inline || !strings.Contains(dl.Result.Reason, "exceeded inline cap") {
		t.Fatalf("got %+v", dl.Result)
	}
}

func TestDownloadErrorsAreWrapped(t *testing.T) {
	t.Parallel()
	_, err := apprelease.New(&stubPort{err: errors.New("boom")}).DownloadAttachment(context.Background(), "o", "r", 1, 7)
	if err == nil || err.Error() != "download release attachment (metadata) err: boom" {
		t.Fatalf("metadata error: %v", err)
	}
	p := &stubPort{att: model.Attachment{Size: 1}, fetchErr: errors.New("net")}
	_, err = apprelease.New(p).DownloadAttachment(context.Background(), "o", "r", 1, 7)
	if err == nil || err.Error() != "download body err: net" {
		t.Fatalf("body error: %v", err)
	}
}

func TestCreateAttachmentPassesTheUpload(t *testing.T) {
	t.Parallel()
	var got apprelease.Upload
	p := &uploadPort{stubPort: &stubPort{}, seen: &got}
	if _, err := apprelease.New(p).CreateAttachment(context.Background(), "o", "r", 1, apprelease.Upload{Filename: "a.txt", Content: strings.NewReader("x")}); err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(got.Content)
	if got.Filename != "a.txt" || string(b) != "x" {
		t.Fatalf("upload %+v body %q", got, b)
	}
}

// uploadPort records the upload it was given.
type uploadPort struct {
	*stubPort
	seen *apprelease.Upload
}

func (u *uploadPort) CreateAttachment(_ context.Context, _, _ string, _ int64, up apprelease.Upload) (model.Attachment, error) {
	*u.seen = up
	return model.Attachment{}, nil
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (*stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }
