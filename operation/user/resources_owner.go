package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
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

	client, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	u, userErr := client.GetUserInfo(ctx, params.Owner)

	var payload ownerResourcePayload

	if userErr == nil && u != nil {
		payload = ownerResourcePayload{
			Login:          u.UserName,
			FullName:       u.FullName,
			HTMLURL:        u.HTMLURL,
			Kind:           "user",
			Description:    u.Description,
			Location:       u.Location,
			Website:        u.Website,
			CreatedAt:      u.Created.Format("2006-01-02T15:04:05Z07:00"),
			FollowersCount: u.FollowerCount,
			FollowingCount: u.FollowingCount,
		}
	} else {
		// if user fetch failed for a reason other than 404, surface it immediately
		if userErr != nil && !is404(userErr) {
			return nil, resource.MapForgejoError(uri, withStatus(userErr))
		}

		// fall back to org
		org, orgErr := client.GetOrg(ctx, params.Owner)
		if orgErr != nil {
			// Both user and org are "not found" → explicit -32003 with a clear message.
			// This also guards the SDK edge where userErr==nil (u==nil, err==nil).
			if is404(orgErr) && (userErr == nil || is404(userErr)) {
				return nil, &resource.ResourceError{URI: uri, Code: -32003, Message: "owner not found: " + uri}
			}
			// A non-404 HTTP error surfaces with its status; a transport or
			// context error surfaces as it is.
			return nil, resource.MapForgejoError(uri, withStatus(orgErr))
		}

		payload = ownerResourcePayload{
			Login:       org.UserName,
			FullName:    org.FullName,
			Kind:        "org",
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

// withStatus puts the HTTP status Forgejo answered with in front of the error's
// message, when there was one.
func withStatus(err error) error {
	if code := forge.StatusCode(err); code != 0 {
		return fmt.Errorf("%d %s", code, err.Error())
	}
	return err
}

// is404 returns true if the error carries HTTP 404, or, for an error with no
// status, if its message says so.
func is404(err error) bool {
	if err == nil {
		return false
	}
	if forge.IsNotFound(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found")
}
