// SPDX-License-Identifier: GPL-3.0-or-later

package release_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprelease "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/release"
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
		for _, r := range got.Items {
			if r != nil {
				tags = append(tags, r.TagName)
			}
		}
		if strings.Join(tags, " ") != want {
			t.Errorf("state %q: got %v, want %s", state, tags, want)
		}
	}
	empty, _ := apprelease.New(&stubPort{}).ListReleases(context.Background(), "o", "r", apprelease.ListQuery{State: "draft"})
	if empty.Items == nil {
		t.Fatal("an empty list must be [] not nil")
	}
}

// pagedReleasesPort serves all as Forgejo does: one page per call, cut by the
// paging the use case asks for. The ceiling is the one the test sets.
type pagedReleasesPort struct {
	stubPort
	all   []*model.Release
	max   int
	known bool
	calls int
}

func (p *pagedReleasesPort) MaxResponseItems(context.Context) (int, bool) { return p.max, p.known }

func (p *pagedReleasesPort) ListReleases(_ context.Context, _, _ string, pg apprelease.Paging) ([]*model.Release, error) {
	p.calls++
	off := (pg.Page - 1) * pg.Limit
	if off >= len(p.all) {
		return nil, nil
	}
	return p.all[off:min(off+pg.Limit, len(p.all))], nil
}

// published returns n published releases tagged v1..vn.
func published(n int) []*model.Release {
	out := make([]*model.Release, n)
	for i := range out {
		out[i] = &model.Release{TagName: fmt.Sprintf("v%d", i+1)}
	}
	return out
}

func TestListReleasesEnvelopeHasNextFromTheProbe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		page, limit, count int
		hasNext            bool
	}{
		{1, 2, 2, true},  // full page, rows follow
		{2, 2, 2, false}, // exact multiple: the last page is full
		{1, 3, 3, true},  // full page of a different size
		{2, 3, 1, false}, // short last page
	}
	for _, c := range cases {
		p := &pagedReleasesPort{all: published(4)}
		got, err := apprelease.New(p).ListReleases(context.Background(), "o", "r", apprelease.ListQuery{Paging: apprelease.Paging{Page: c.page, Limit: c.limit}})
		if err != nil {
			t.Fatal(err)
		}
		if got.Page != c.page || got.Limit != c.limit || got.Count != c.count || got.HasNext != c.hasNext {
			t.Errorf("page %d limit %d: got page=%d limit=%d count=%d has_next=%v, want count=%d has_next=%v",
				c.page, c.limit, got.Page, got.Limit, got.Count, got.HasNext, c.count, c.hasNext)
		}
	}
}

func TestListReleasesHasNextReadsTheRawPageNotTheFilteredOne(t *testing.T) {
	t.Parallel()
	// Draft, published, published, draft: a draft page of two raw rows keeps one
	// draft, but a further page still follows, so has_next must be true.
	all := []*model.Release{{TagName: "d1", IsDraft: true}, {TagName: "p1"}, {TagName: "p2"}, {TagName: "d2", IsDraft: true}}
	p := &pagedReleasesPort{all: all}
	got, err := apprelease.New(p).ListReleases(context.Background(), "o", "r", apprelease.ListQuery{State: "draft", Paging: apprelease.Paging{Page: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || !got.HasNext {
		t.Fatalf("page 1 of drafts: count=%d has_next=%v, want 1 and true", got.Count, got.HasNext)
	}
}

func TestListReleasesShortPageUnderKnownCeilingIsTheLast(t *testing.T) {
	t.Parallel()
	p := &pagedReleasesPort{all: published(3), max: 10, known: true}
	got, err := apprelease.New(p).ListReleases(context.Background(), "o", "r", apprelease.ListQuery{Paging: apprelease.Paging{Page: 1, Limit: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasNext || got.Count != 3 {
		t.Fatalf("got count=%d has_next=%v", got.Count, got.HasNext)
	}
	if p.calls != 1 {
		t.Fatalf("a short page under a known ceiling makes no probe: %d calls", p.calls)
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
	if len(page.Items) != 1 || page.Items[0].ID != 3 {
		t.Fatalf("page 2: %+v", page)
	}
	beyond, _ := svc.ListAttachments(context.Background(), "o", "r", 1, apprelease.Paging{Page: 9, Limit: 2})
	if beyond.Items == nil || len(beyond.Items) != 0 {
		t.Fatalf("past the end: %+v", beyond)
	}
}

func TestListAttachmentsEnvelopeHasNextFromTheReportedTotal(t *testing.T) {
	t.Parallel()
	// Forgejo does not page attachments, so the whole list is the grand total:
	// has_next comes from it, and no probe is made even under an unknown ceiling.
	var all []*model.Attachment
	for i := range 4 {
		all = append(all, &model.Attachment{ID: int64(i + 1)})
	}
	svc := apprelease.New(&stubPort{atts: all})
	cases := []struct {
		page, count int
		hasNext     bool
	}{
		{1, 2, true},
		{2, 2, false}, // exact multiple
	}
	for _, c := range cases {
		got, err := svc.ListAttachments(context.Background(), "o", "r", 1, apprelease.Paging{Page: c.page, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if got.Page != c.page || got.Limit != 2 || got.Count != c.count || got.HasNext != c.hasNext {
			t.Errorf("page %d: got page=%d limit=%d count=%d has_next=%v", c.page, got.Page, got.Limit, got.Count, got.HasNext)
		}
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
