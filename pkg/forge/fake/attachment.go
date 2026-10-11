// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Attachments = (*Forgejo)(nil)

// defaultBaseURL is where attachment download URLs point unless a test sets
// Forgejo.BaseURL (the contract stub points it at its own server).
const defaultBaseURL = "https://forgejo.fake"

// assetState holds every attachment, whatever it hangs off: issues and
// comments (which the port cannot create, see AddAttachmentIssue) and releases.
// One mutex guards it all, so ids are unique across kinds as on Forgejo.
type assetState struct {
	mu    sync.Mutex
	next  int64
	sets  map[string]*attachmentSet // key: kind|owner/repo|id
	blobs map[string]blob           // by attachment UUID
	hang  bool
}

type attachmentSet struct {
	items []*forgejo_sdk.Attachment
}

type blob struct {
	body        []byte
	contentType string
}

func setKey(kind, owner, repo string, id int64) string {
	return kind + "|" + owner + "/" + repo + "|" + strconv.FormatInt(id, 10)
}

// AddAttachmentIssue makes issue or pull request number index of owner/repo
// able to carry attachments. The port cannot create issues in this domain.
func (f *Forgejo) AddAttachmentIssue(owner, repo string, index int64) {
	f.assets.ensure(setKey("issue", owner, repo, index))
}

// AddAttachmentComment makes comment commentID of owner/repo able to carry
// attachments.
func (f *Forgejo) AddAttachmentComment(owner, repo string, commentID int64) {
	f.assets.ensure(setKey("comment", owner, repo, commentID))
}

// HangUploads makes every attachment upload block until its context ends, as a
// stuck network path would, then fail with the context's error.
func (f *Forgejo) HangUploads() {
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	f.assets.hang = true
}

func (s *assetState) ensure(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sets == nil {
		s.sets = map[string]*attachmentSet{}
	}
	if _, ok := s.sets[key]; !ok {
		s.sets[key] = &attachmentSet{}
	}
}

func (f *Forgejo) baseURL() string {
	if f.BaseURL != "" {
		return strings.TrimRight(f.BaseURL, "/")
	}
	return defaultBaseURL
}

// set returns the attachment set under key, or 404 naming what. Caller holds mu.
func (s *assetState) set(key, what string) (*attachmentSet, error) {
	set, ok := s.sets[key]
	if !ok {
		return nil, notFound(what)
	}
	return set, nil
}

func (set *attachmentSet) find(id int64) (int, error) {
	for i, a := range set.items {
		if a.ID == id {
			return i, nil
		}
	}
	return 0, notFound("attachment")
}

func copyAttachment(a *forgejo_sdk.Attachment) *forgejo_sdk.Attachment {
	c := *a
	return &c
}

func (f *Forgejo) listSet(key, what string) ([]*forgejo_sdk.Attachment, error) {
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	set, err := f.assets.set(key, what)
	if err != nil {
		return nil, err
	}
	out := make([]*forgejo_sdk.Attachment, 0, len(set.items))
	for _, a := range set.items {
		out = append(out, copyAttachment(a))
	}
	return out, nil
}

func (f *Forgejo) getInSet(key, what string, id int64) (*forgejo_sdk.Attachment, error) {
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	set, err := f.assets.set(key, what)
	if err != nil {
		return nil, err
	}
	i, err := set.find(id)
	if err != nil {
		return nil, err
	}
	return copyAttachment(set.items[i]), nil
}

func (f *Forgejo) createInSet(ctx context.Context, key, what string, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	f.assets.mu.Lock()
	hang := f.assets.hang
	f.assets.mu.Unlock()
	if hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if up.Filename == "" {
		return nil, errors.New("attachment filename is required")
	}
	var body []byte
	if up.Content != nil {
		var err error
		if body, err = io.ReadAll(up.Content); err != nil {
			return nil, err
		}
	}

	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	set, err := f.assets.set(key, what)
	if err != nil {
		return nil, err
	}
	f.assets.next++
	id := f.assets.next
	uuid := fmt.Sprintf("00000000-0000-0000-0000-%012d", id)
	ct := up.MIMEType
	if ct == "" {
		ct = "application/octet-stream"
	}
	if f.assets.blobs == nil {
		f.assets.blobs = map[string]blob{}
	}
	f.assets.blobs[uuid] = blob{body: body, contentType: ct}
	att := &forgejo_sdk.Attachment{
		ID:          id,
		Name:        up.Filename,
		Size:        int64(len(body)),
		Created:     f.Now(),
		UUID:        uuid,
		DownloadURL: f.baseURL() + "/attachments/" + uuid,
	}
	set.items = append(set.items, att)
	return copyAttachment(att), nil
}

func (f *Forgejo) renameInSet(key, what string, id int64, name string) (*forgejo_sdk.Attachment, error) {
	if name == "" {
		return nil, errors.New("attachment name is required")
	}
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	set, err := f.assets.set(key, what)
	if err != nil {
		return nil, err
	}
	i, err := set.find(id)
	if err != nil {
		return nil, err
	}
	set.items[i].Name = name
	return copyAttachment(set.items[i]), nil
}

func (f *Forgejo) deleteInSet(key, what string, id int64) error {
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	set, err := f.assets.set(key, what)
	if err != nil {
		return err
	}
	i, err := set.find(id)
	if err != nil {
		return err
	}
	delete(f.assets.blobs, set.items[i].UUID)
	set.items = append(set.items[:i], set.items[i+1:]...)
	return nil
}

func (f *Forgejo) ListIssueAttachments(_ context.Context, owner, repo string, index int64) ([]*forgejo_sdk.Attachment, error) {
	return f.listSet(setKey("issue", owner, repo, index), "issue")
}

func (f *Forgejo) GetIssueAttachment(_ context.Context, owner, repo string, index, id int64) (*forgejo_sdk.Attachment, error) {
	return f.getInSet(setKey("issue", owner, repo, index), "issue", id)
}

func (f *Forgejo) CreateIssueAttachment(ctx context.Context, owner, repo string, index int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	return f.createInSet(ctx, setKey("issue", owner, repo, index), "issue", up)
}

func (f *Forgejo) EditIssueAttachment(_ context.Context, owner, repo string, index, id int64, name string) (*forgejo_sdk.Attachment, error) {
	return f.renameInSet(setKey("issue", owner, repo, index), "issue", id, name)
}

func (f *Forgejo) DeleteIssueAttachment(_ context.Context, owner, repo string, index, id int64) error {
	return f.deleteInSet(setKey("issue", owner, repo, index), "issue", id)
}

func (f *Forgejo) ListCommentAttachments(_ context.Context, owner, repo string, commentID int64) ([]*forgejo_sdk.Attachment, error) {
	return f.listSet(setKey("comment", owner, repo, commentID), "comment")
}

func (f *Forgejo) GetCommentAttachment(_ context.Context, owner, repo string, commentID, id int64) (*forgejo_sdk.Attachment, error) {
	return f.getInSet(setKey("comment", owner, repo, commentID), "comment", id)
}

func (f *Forgejo) CreateCommentAttachment(ctx context.Context, owner, repo string, commentID int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	return f.createInSet(ctx, setKey("comment", owner, repo, commentID), "comment", up)
}

func (f *Forgejo) EditCommentAttachment(_ context.Context, owner, repo string, commentID, id int64, name string) (*forgejo_sdk.Attachment, error) {
	return f.renameInSet(setKey("comment", owner, repo, commentID), "comment", id, name)
}

func (f *Forgejo) DeleteCommentAttachment(_ context.Context, owner, repo string, commentID, id int64) error {
	return f.deleteInSet(setKey("comment", owner, repo, commentID), "comment", id)
}

// DownloadAttachment serves the bytes of an attachment this fake issued, found
// by its download URL, refusing a body over the inline cap.
func (f *Forgejo) DownloadAttachment(_ context.Context, downloadURL string) (forge.Download, error) {
	d, err := f.AttachmentContent(downloadURL)
	if err == nil && int64(len(d.Body)) > forge.MaxInlineDownloadBytes {
		return forge.Download{}, forge.ErrPayloadTooLarge
	}
	return d, err
}

// AttachmentContent is the bytes behind a download URL with no size cap. The
// contract stub serves the download endpoint from it, leaving the cap to the
// adapter.
func (f *Forgejo) AttachmentContent(downloadURL string) (forge.Download, error) {
	uuid, ok := strings.CutPrefix(downloadURL, f.baseURL()+"/attachments/")
	if !ok {
		return forge.Download{}, statusErr(http.StatusNotFound, "attachment not found")
	}
	f.assets.mu.Lock()
	defer f.assets.mu.Unlock()
	b, ok := f.assets.blobs[uuid]
	if !ok {
		return forge.Download{}, notFound("attachment")
	}
	return forge.Download{Body: append([]byte(nil), b.body...), ContentType: b.contentType}, nil
}
