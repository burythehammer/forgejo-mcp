// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// Hook is a repository webhook as Forgejo returns it, with its secret in
// Config. Its JSON names are those of the SDK's Hook. Use cases never hand a
// Hook to a tool: they project it through SafeHook first.
type Hook struct {
	ID      int64             `json:"id"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config"`
	Events  []string          `json:"events"`
	Active  bool              `json:"active"`
	Updated time.Time         `json:"updated_at"`
	Created time.Time         `json:"created_at"`
}

// HookList is one page of hooks plus the total Forgejo reports.
type HookList struct {
	Hooks []*Hook
	// TotalCount is Forgejo's X-Total-Count, or nil when it did not say.
	TotalCount *int
}

// HookPayload is the safe form of a webhook that the tools return. It is an
// explicit allowlist of fields: Config.secret is never copied, so no tool
// output can carry it. Add a field here only if it is safe to return.
type HookPayload struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	Active       bool      `json:"active"`
	Events       []string  `json:"events"`
	URL          string    `json:"url"`
	ContentType  string    `json:"content_type,omitempty"`
	HTTPMethod   string    `json:"http_method,omitempty"`
	BranchFilter string    `json:"branch_filter,omitempty"`
	Created      time.Time `json:"created_at"`
}

// SafeHook projects a webhook onto its allowlisted payload.
func SafeHook(h *Hook) HookPayload {
	return HookPayload{
		ID:           h.ID,
		Type:         h.Type,
		Active:       h.Active,
		Events:       h.Events,
		URL:          h.Config["url"],
		ContentType:  h.Config["content_type"],
		HTTPMethod:   h.Config["http_method"],
		BranchFilter: h.Config["branch_filter"],
		Created:      h.Created,
	}
}

// HookListResult is one page of webhooks as the list tool returns it, echoing
// the page so a caller can resume pagination.
type HookListResult struct {
	Hooks []HookPayload `json:"hooks"`
	PageWindow
	PageTotal
	PageNext
}
