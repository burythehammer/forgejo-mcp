// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	apppissue "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateRepoLabelToolName = "create_repo_label"
	EditRepoLabelToolName   = "edit_repo_label"
	DeleteRepoLabelToolName = "delete_repo_label"
	GetRepoLabelToolName    = "get_repo_label"

	CreateOrgLabelToolName = "create_org_label"
	EditOrgLabelToolName   = "edit_org_label"
	DeleteOrgLabelToolName = "delete_org_label"
	GetOrgLabelToolName    = "get_org_label"
)

var (
	CreateRepoLabelTool = mcp.NewTool(
		CreateRepoLabelToolName,
		mcp.WithDescription("Create a repository label. Returns the created label including its numeric id."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
	)

	EditRepoLabelTool = mcp.NewTool(
		EditRepoLabelToolName,
		mcp.WithDescription("Edit a repository label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
	)

	DeleteRepoLabelTool = mcp.NewTool(
		DeleteRepoLabelToolName,
		mcp.WithDescription("Delete a repository label. By default refuses if the label is in use; set delete_mode=force to override."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetRepoLabelTool = mcp.NewTool(
		GetRepoLabelToolName,
		mcp.WithDescription("Get a single repository label by ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)

	CreateOrgLabelTool = mcp.NewTool(
		CreateOrgLabelToolName,
		mcp.WithDescription("Create an organization-level label. Returns the created label including its numeric id."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
	)

	EditOrgLabelTool = mcp.NewTool(
		EditOrgLabelToolName,
		mcp.WithDescription("Edit an organization-level label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
	)

	DeleteOrgLabelTool = mcp.NewTool(
		DeleteOrgLabelToolName,
		mcp.WithDescription("Delete an organization-level label. By default refuses if the label is in use; set delete_mode=force to override. Note: in-use count is best-effort over repos visible to the token and may under-count."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetOrgLabelTool = mcp.NewTool(
		GetOrgLabelToolName,
		mcp.WithDescription("Get a single organization-level label by ID."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)
)

func (h handlers) registerLabelTools(s *server.MCPServer) {
	s.AddTool(CreateRepoLabelTool, h.CreateRepoLabelFn)
	s.AddTool(EditRepoLabelTool, h.EditRepoLabelFn)
	s.AddTool(DeleteRepoLabelTool, h.DeleteRepoLabelFn)
	s.AddTool(GetRepoLabelTool, h.GetRepoLabelFn)
	s.AddTool(CreateOrgLabelTool, h.CreateOrgLabelFn)
	s.AddTool(EditOrgLabelTool, h.EditOrgLabelFn)
	s.AddTool(DeleteOrgLabelTool, h.DeleteOrgLabelFn)
	s.AddTool(GetOrgLabelTool, h.GetOrgLabelFn)
}

type createRepoLabelArgs struct {
	Owner       string `json:"owner" required:"true"`
	Repo        string `json:"repo" required:"true"`
	Name        string `json:"name" required:"true"`
	Color       string `json:"color" required:"true"`
	Description string `json:"description"`
}

type editRepoLabelArgs struct {
	Owner       string  `json:"owner" required:"true"`
	Repo        string  `json:"repo" required:"true"`
	ID          float64 `json:"id" required:"true"`
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

type deleteRepoLabelArgs struct {
	Owner      string  `json:"owner" required:"true"`
	Repo       string  `json:"repo" required:"true"`
	ID         float64 `json:"id" required:"true"`
	DeleteMode string  `json:"delete_mode"`
}

type getRepoLabelArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	ID    float64 `json:"id" required:"true"`
}

type createOrgLabelArgs struct {
	Org         string `json:"org" required:"true"`
	Name        string `json:"name" required:"true"`
	Color       string `json:"color" required:"true"`
	Description string `json:"description"`
}

type editOrgLabelArgs struct {
	Org         string  `json:"org" required:"true"`
	ID          float64 `json:"id" required:"true"`
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

type deleteOrgLabelArgs struct {
	Org        string  `json:"org" required:"true"`
	ID         float64 `json:"id" required:"true"`
	DeleteMode string  `json:"delete_mode"`
}

type getOrgLabelArgs struct {
	Org string  `json:"org" required:"true"`
	ID  float64 `json:"id" required:"true"`
}

// labelNew is the label a create call names.
func labelNew(name, color, description string) apppissue.LabelNew {
	return apppissue.LabelNew{Name: name, Color: color, Description: description}
}

// labelEdit is the edit a call names. Tool is the tool's name, for the error
// when no field is supplied.
func labelEdit(tool string, name, color, description *string) apppissue.LabelEdit {
	return apppissue.LabelEdit{Tool: tool, Name: name, Color: color, Description: description}
}

func (h handlers) CreateRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a createRepoLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.CreateRepoLabel(ctx, a.Owner, a.Repo, labelNew(a.Name, a.Color, a.Description)))
	})
}

func (h handlers) EditRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a editRepoLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.EditRepoLabel(ctx, a.Owner, a.Repo, int64(a.ID), labelEdit("edit_repo_label", a.Name, a.Color, a.Description)))
	})
}

func (h handlers) DeleteRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a deleteRepoLabelArgs) (*mcp.CallToolResult, error) {
		id := int64(a.ID)
		if err := s.DeleteRepoLabel(ctx, a.Owner, a.Repo, id, a.DeleteMode == "force"); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(map[string]any{"deleted": true, "id": id})
	})
}

// repoLabelURI, repoLabelsURI and orgLabelsURI name the resource that serves
// the same read as a label tool. The label tools use them as the subject of
// the resource error mapping that both paths share until #11 lands.
func repoLabelURI(owner, repo string, id int64) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/label/%d", owner, repo, id)
}

func repoLabelsURI(owner, repo string) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/labels", owner, repo)
}

func orgLabelsURI(org string) string {
	return fmt.Sprintf("forgejo://org/%s/labels", org)
}

func (h handlers) GetRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a getRepoLabelArgs) (*mcp.CallToolResult, error) {
		label, err := s.GetRepoLabel(ctx, a.Owner, a.Repo, int64(a.ID))
		if err != nil {
			return nil, mapErr(repoLabelURI(a.Owner, a.Repo, int64(a.ID)), err)
		}
		return mcpresult.Render(label, nil)
	})
}

func (h handlers) CreateOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateOrgLabelFn", func(ctx context.Context, s *apppissue.Service, a createOrgLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.CreateOrgLabel(ctx, a.Org, labelNew(a.Name, a.Color, a.Description)))
	})
}

func (h handlers) EditOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditOrgLabelFn", func(ctx context.Context, s *apppissue.Service, a editOrgLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.EditOrgLabel(ctx, a.Org, int64(a.ID), labelEdit("edit_org_label", a.Name, a.Color, a.Description)))
	})
}

func (h handlers) DeleteOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteOrgLabelFn", func(ctx context.Context, s *apppissue.Service, a deleteOrgLabelArgs) (*mcp.CallToolResult, error) {
		id := int64(a.ID)
		if err := s.DeleteOrgLabel(ctx, a.Org, id, a.DeleteMode == "force"); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(map[string]any{"deleted": true, "id": id})
	})
}

func (h handlers) GetOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetOrgLabelFn", func(ctx context.Context, s *apppissue.Service, a getOrgLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.GetOrgLabel(ctx, a.Org, int64(a.ID)))
	})
}
