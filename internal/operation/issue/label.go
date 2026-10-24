// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

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

// The exclusive flag: a scope is the part of a label name before its last "/".
// A write may not name two labels from one exclusive scope; a label the issue
// already carries is not compared.
const (
	exclusiveCreateDescription = "Exclusive: marks this label's scope (the name before the last /, e.g. status in status/ready) as exclusive. A write that names two labels from one exclusive scope is refused; a label the issue already carries is not counted, so an issue can still end up with two. Default false. No effect on a name without a /."
	exclusiveEditDescription   = "Exclusive flag to set or clear (see create). Omit to leave the stored value unchanged."
)

var (
	CreateRepoLabelTool = mcp.NewTool(
		CreateRepoLabelToolName,
		annotate.Write(),
		mcp.WithDescription("Create a repository label. Returns the created label including its numeric id."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)"), lean.Keep("6-digit hex color, with or without #")),
		mcp.WithString("description", mcp.Description("Label description")),
		mcp.WithBoolean("exclusive", mcp.Description(exclusiveCreateDescription), lean.Keep("true makes the scope before the last / exclusive")),
	)

	EditRepoLabelTool = mcp.NewTool(
		EditRepoLabelToolName,
		annotate.Write(),
		mcp.WithDescription("Edit a repository label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)"), lean.Keep("6-digit hex color, with or without #")),
		mcp.WithString("description", mcp.Description("New label description")),
		mcp.WithBoolean("exclusive", mcp.Description(exclusiveEditDescription), lean.Keep("true or false, omitted keeps the stored value")),
	)

	DeleteRepoLabelTool = mcp.NewTool(
		DeleteRepoLabelToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete a repository label. By default refuses if the label is in use; set delete_mode=force to override."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetRepoLabelTool = mcp.NewTool(
		GetRepoLabelToolName,
		annotate.Read(),
		mcp.WithDescription("Get a single repository label by ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
	)

	CreateOrgLabelTool = mcp.NewTool(
		CreateOrgLabelToolName,
		annotate.Write(),
		mcp.WithDescription("Create an organization-level label. Returns the created label including its numeric id."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Label name")),
		mcp.WithString("color", mcp.Required(), mcp.Description("Label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("Label description")),
		mcp.WithBoolean("exclusive", mcp.Description(exclusiveCreateDescription)),
	)

	EditOrgLabelTool = mcp.NewTool(
		EditOrgLabelToolName,
		annotate.Write(),
		mcp.WithDescription("Edit an organization-level label (PATCH — only supplied fields change). Providing no fields is an error."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("name", mcp.Description("New label name")),
		mcp.WithString("color", mcp.Description("New label color as 6-digit hex (e.g. #0088ff or 0088ff)")),
		mcp.WithString("description", mcp.Description("New label description")),
		mcp.WithBoolean("exclusive", mcp.Description(exclusiveEditDescription)),
	)

	DeleteOrgLabelTool = mcp.NewTool(
		DeleteOrgLabelToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete an organization-level label. By default refuses if the label is in use; set delete_mode=force to override. Note: in-use count is best-effort over repos visible to the token and may under-count."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Label ID")),
		mcp.WithString("delete_mode", mcp.Description("safe (default): refuse if in use and report count. force: delete unconditionally.")),
	)

	GetOrgLabelTool = mcp.NewTool(
		GetOrgLabelToolName,
		annotate.Read(),
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
	Exclusive   bool   `json:"exclusive"`
}

type editRepoLabelArgs struct {
	Owner       string  `json:"owner" required:"true"`
	Repo        string  `json:"repo" required:"true"`
	ID          float64 `json:"id" required:"true"`
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Exclusive   *bool   `json:"exclusive"`
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
	Exclusive   bool   `json:"exclusive"`
}

type editOrgLabelArgs struct {
	Org         string  `json:"org" required:"true"`
	ID          float64 `json:"id" required:"true"`
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Exclusive   *bool   `json:"exclusive"`
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
func labelNew(name, color, description string, exclusive bool) apppissue.LabelNew {
	return apppissue.LabelNew{Name: name, Color: color, Description: description, Exclusive: exclusive}
}

// labelEdit is the edit a call names. Tool is the tool's name, for the error
// when no field is supplied.
func labelEdit(tool string, name, color, description *string, exclusive *bool) apppissue.LabelEdit {
	return apppissue.LabelEdit{Tool: tool, Name: name, Color: color, Description: description, Exclusive: exclusive}
}

func (h handlers) CreateRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a createRepoLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.CreateRepoLabel(ctx, a.Owner, a.Repo, labelNew(a.Name, a.Color, a.Description, a.Exclusive)))
	})
}

func (h handlers) EditRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditRepoLabelFn", func(ctx context.Context, s *apppissue.Service, a editRepoLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.EditRepoLabel(ctx, a.Owner, a.Repo, int64(a.ID), labelEdit("edit_repo_label", a.Name, a.Color, a.Description, a.Exclusive)))
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
		return mcpresult.Render(s.CreateOrgLabel(ctx, a.Org, labelNew(a.Name, a.Color, a.Description, a.Exclusive)))
	})
}

func (h handlers) EditOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditOrgLabelFn", func(ctx context.Context, s *apppissue.Service, a editOrgLabelArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.EditOrgLabel(ctx, a.Org, int64(a.ID), labelEdit("edit_org_label", a.Name, a.Color, a.Description, a.Exclusive)))
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
