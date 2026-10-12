// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appowner "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/owner"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/owneradapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const ownerResourceURITemplate = "forgejo://owner/{owner}"

// ownerResourcePayload is the JSON body returned for user or org.
type ownerResourcePayload struct {
	Login            string `json:"login"`
	FullName         string `json:"full_name,omitempty"`
	HTMLURL          string `json:"html_url,omitempty"`
	Kind             string `json:"kind"` // "user" or "org"
	Description      string `json:"description,omitempty"`
	Location         string `json:"location,omitempty"`
	Website          string `json:"website,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	FollowersCount   int    `json:"followers_count,omitempty"`
	FollowingCount   int    `json:"following_count,omitempty"`
	PublicReposCount int    `json:"public_repos_count,omitempty"`
}

// RegisterOwnerResource registers the forgejo://owner/{owner} resource template.
func RegisterOwnerResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		ownerResourceURITemplate,
		"Forgejo Owner",
		h.ownerResourceHandler,
		mcp.WithTemplateDescription(
			"User or organization profile addressed by login. "+
				"Tries user first; falls back to org if user returns 404. "+
				"URI: forgejo://owner/{owner}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered owner resource template")
}

func (h handlers) ownerResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseOwner(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	o, err := appowner.New(owneradapter.New(fg, fg)).Read(ctx, params.Owner)
	if err != nil {
		if errors.Is(err, appowner.ErrOwnerNotFound) {
			return nil, &resource.ResourceError{URI: uri, Code: -32003, Message: "owner not found: " + uri}
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	var payload ownerResourcePayload
	if o.Kind == appowner.KindUser {
		u := o.User
		payload = ownerResourcePayload{
			Login:          u.UserName,
			FullName:       u.FullName,
			HTMLURL:        u.HTMLURL,
			Kind:           string(appowner.KindUser),
			Description:    u.Description,
			Location:       u.Location,
			Website:        u.Website,
			CreatedAt:      u.Created.Format("2006-01-02T15:04:05Z07:00"),
			FollowersCount: u.FollowerCount,
			FollowingCount: u.FollowingCount,
		}
	} else {
		org := o.Org
		payload = ownerResourcePayload{
			Login:       org.UserName,
			FullName:    org.FullName,
			Kind:        string(appowner.KindOrg),
			Description: org.Description,
			Location:    org.Location,
			Website:     org.Website,
		}
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal owner payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}, nil
}

