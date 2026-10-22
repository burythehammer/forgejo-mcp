// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"errors"
	"fmt"
	"math"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/split"
)

// Service runs the webhook use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// HookRef names one webhook of a repository. ID is the number as the caller
// gave it. It must be a positive whole number: zero, a negative number and a
// fraction are all refused with ErrRequired before any request is made.
type HookRef struct {
	Owner, Repo string
	ID          float64
}

// ListRequest lists a repository's webhooks. Page and Limit are the numbers as
// the caller gave them: zero means page 1 and 30 hooks.
type ListRequest struct {
	Owner, Repo string
	Page, Limit float64
}

// CreateRequest creates a webhook. Events is comma-separated. An empty string
// field is the same as an absent one, as the tool always treated it. Active nil
// means true.
type CreateRequest struct {
	Owner, Repo, URL, Type, ContentType, Secret, HTTPMethod, BranchFilter, Events string
	Active                                                                        *bool
}

// EditRequest changes a webhook. A nil field is one the caller did not set and
// is left as it is; a non-nil field is applied even when it is empty, so an
// empty BranchFilter clears it and an empty Events clears the list. Events is
// comma-separated.
type EditRequest struct {
	HookRef
	URL, ContentType, Secret, HTTPMethod, BranchFilter, Events *string
	Active                                                     *bool
}

// List returns one page of a repository's webhooks, each in its safe form.
func (s *Service) List(ctx context.Context, r ListRequest) (list.Bounded[model.HookPayload], error) {
	if r.Owner == "" || r.Repo == "" {
		return list.Bounded[model.HookPayload]{}, fmt.Errorf("owner and repo are required")
	}
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	page := r.Page
	if page == 0 {
		page = 1
	}
	limit := r.Limit
	if limit == 0 {
		limit = float64(list.DefaultLimit(30, c))
	}
	p := list.Paging{Page: int(page), Limit: int(limit)}
	return list.Run(ctx, p, c, func(ctx context.Context, q list.Paging) (list.Batch[model.HookPayload], error) {
		hooks, err := s.port.ListRepoHooks(ctx, r.Owner, r.Repo, Paging{Page: q.Page, Limit: q.Limit})
		if err != nil {
			return list.Batch[model.HookPayload]{}, fmt.Errorf("list repo hooks: %w", err)
		}
		return list.Batch[model.HookPayload]{Items: safeHooks(hooks.Hooks), Total: hooks.TotalCount}, nil
	})
}

// Get returns one webhook in its safe form.
func (s *Service) Get(ctx context.Context, r HookRef) (model.HookPayload, error) {
	if err := r.check(); err != nil {
		return model.HookPayload{}, err
	}
	hk, err := s.port.GetRepoHook(ctx, r.Owner, r.Repo, int64(r.ID))
	if err != nil {
		return model.HookPayload{}, fmt.Errorf("get repo hook: %w", err)
	}
	return model.SafeHook(hk), nil
}

// Create adds a webhook and returns it in its safe form.
func (s *Service) Create(ctx context.Context, r CreateRequest) (model.HookPayload, error) {
	if r.Owner == "" || r.Repo == "" || r.URL == "" {
		return model.HookPayload{}, fmt.Errorf("owner, repo and url are required")
	}
	config := map[string]string{
		"url":          r.URL,
		"content_type": "json",
	}
	if r.ContentType != "" {
		config["content_type"] = r.ContentType
	}
	if r.Secret != "" {
		config["secret"] = r.Secret
	}
	if r.HTTPMethod != "" {
		config["http_method"] = r.HTTPMethod
	}
	typ := DefaultType
	if r.Type != "" {
		typ = r.Type
	}
	active := true
	if r.Active != nil {
		active = *r.Active
	}
	in := CreateInput{Type: typ, Config: config, BranchFilter: r.BranchFilter, Active: active}
	if r.Events != "" {
		in.Events = split.CSV(r.Events)
	}
	hk, err := s.port.CreateRepoHook(ctx, r.Owner, r.Repo, in)
	if err != nil {
		return model.HookPayload{}, fmt.Errorf("create repo hook: %w", err)
	}
	return model.SafeHook(hk), nil
}

// Edit changes the fields of a webhook that the request sets, and returns the
// webhook in its safe form. Only the set fields are sent: Forgejo keeps every
// other field. Config is the one whole-map field, so when a config key is set
// the stored config is merged with it and sent in full.
func (s *Service) Edit(ctx context.Context, r EditRequest) (model.HookPayload, error) {
	if err := r.check(); err != nil {
		return model.HookPayload{}, err
	}
	id := int64(r.ID)
	existing, err := s.port.GetRepoHook(ctx, r.Owner, r.Repo, id)
	if err != nil {
		return model.HookPayload{}, fmt.Errorf("get repo hook for edit: %w", err)
	}
	config := existing.Config
	if config == nil {
		config = map[string]string{}
	}
	configTouched := false
	for key, v := range map[string]*string{
		"url": r.URL, "content_type": r.ContentType, "secret": r.Secret, "http_method": r.HTTPMethod,
	} {
		if v != nil {
			config[key] = *v
			configTouched = true
		}
	}
	in := EditInput{BranchFilter: r.BranchFilter, Active: r.Active}
	if configTouched {
		in.Config = config
	}
	if r.Events != nil {
		in.Events = split.CSV(*r.Events)
	}
	if err := s.port.EditRepoHook(ctx, r.Owner, r.Repo, id, in); err != nil {
		var se *StatusError
		if errors.As(err, &se) {
			return model.HookPayload{}, fmt.Errorf("edit repo hook %d %s", se.Code, se.Err.Error())
		}
		return model.HookPayload{}, fmt.Errorf("edit repo hook: %w", err)
	}
	hk, err := s.port.GetRepoHook(ctx, r.Owner, r.Repo, id)
	if err != nil {
		return model.HookPayload{}, fmt.Errorf("get repo hook after edit: %w", err)
	}
	return model.SafeHook(hk), nil
}

// Delete removes a webhook.
func (s *Service) Delete(ctx context.Context, r HookRef) error {
	if err := r.check(); err != nil {
		return err
	}
	if err := s.port.DeleteRepoHook(ctx, r.Owner, r.Repo, int64(r.ID)); err != nil {
		return fmt.Errorf("delete repo hook: %w", err)
	}
	return nil
}

// Test triggers a test delivery for a webhook. Each call is a live HTTP
// delivery to the webhook's URL.
func (s *Service) Test(ctx context.Context, r HookRef) error {
	if err := r.check(); err != nil {
		return err
	}
	if err := s.port.TestRepoHook(ctx, r.Owner, r.Repo, int64(r.ID)); err != nil {
		return fmt.Errorf("test repo hook: %w", err)
	}
	return nil
}

// ErrRequired is returned, unwrapped, when a hook reference lacks its owner,
// repo or id. An adapter can tell it apart from a Forgejo failure.
var ErrRequired = errors.New("owner, repo and id are required")

// check accepts only a positive whole number as the hook ID. A fraction is
// refused rather than truncated, so the call never acts on a hook other than
// the one named. The comparisons are written so NaN and values past int64 fail.
func (r HookRef) check() error {
	if r.Owner == "" || r.Repo == "" || !(r.ID >= 1 && r.ID == math.Trunc(r.ID) && r.ID <= math.MaxInt64) {
		return ErrRequired
	}
	return nil
}
