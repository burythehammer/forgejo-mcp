// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service runs the webhook use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// HookRef names one webhook of a repository. ID is the number as the caller
// gave it: zero is a missing ID, and a fraction truncates after that check.
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

// EditRequest changes a webhook. An empty string field leaves that config key
// as it was; Events is comma-separated; Active nil leaves the flag alone.
type EditRequest struct {
	HookRef
	URL, ContentType, Secret, HTTPMethod, BranchFilter, Events string
	Active                                                     *bool
}

// List returns one page of a repository's webhooks, each in its safe form.
func (s *Service) List(ctx context.Context, r ListRequest) (model.HookListResult, error) {
	if r.Owner == "" || r.Repo == "" {
		return model.HookListResult{}, fmt.Errorf("owner and repo are required")
	}
	page := r.Page
	if page == 0 {
		page = 1
	}
	limit := r.Limit
	if limit == 0 {
		limit = 30
	}
	p := Paging{Page: int(page), Limit: int(limit)}
	list, err := s.port.ListRepoHooks(ctx, r.Owner, r.Repo, p)
	if err != nil {
		return model.HookListResult{}, fmt.Errorf("list repo hooks: %w", err)
	}
	payloads := make([]model.HookPayload, len(list.Hooks))
	for i, hk := range list.Hooks {
		payloads[i] = model.SafeHook(hk)
	}
	return model.HookListResult{
		Page:       p.Page,
		Limit:      p.Limit,
		Count:      len(payloads),
		TotalCount: list.TotalCount,
		Hooks:      payloads,
	}, nil
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
		in.Events = splitEvents(r.Events)
	}
	hk, err := s.port.CreateRepoHook(ctx, r.Owner, r.Repo, in)
	if err != nil {
		return model.HookPayload{}, fmt.Errorf("create repo hook: %w", err)
	}
	return model.SafeHook(hk), nil
}

// Edit changes the fields of a webhook that the request sets, and returns the
// webhook in its safe form. The stored config is merged, so keys the request
// does not name keep their values.
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
	if r.URL != "" {
		config["url"] = r.URL
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
	in := EditInput{Config: config, BranchFilter: r.BranchFilter, Active: r.Active}
	if r.Events != "" {
		in.Events = splitEvents(r.Events)
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

func (r HookRef) check() error {
	if r.Owner == "" || r.Repo == "" || r.ID == 0 {
		return fmt.Errorf("owner, repo and id are required")
	}
	return nil
}

// splitEvents parses a comma-separated event list into a trimmed, empty-free
// slice. It returns an empty (non-nil) slice when nothing is left.
func splitEvents(csv string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
