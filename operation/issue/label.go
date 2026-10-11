// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/ptr"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
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

// normalizeColor accepts rrggbb or #rrggbb (6-digit only), lowercases,
// prepends # if absent. Returns an error for invalid input.
// 3-digit shorthand is intentionally rejected (upstream behavior unverified).
func normalizeColor(color string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(color))
	c = strings.TrimPrefix(c, "#")
	if len(c) != 6 {
		return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
	}
	for _, ch := range c {
		isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
		if !isHex {
			return "", fmt.Errorf("color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got %q", color)
		}
	}
	return "#" + c, nil
}

// repoLabelInUseCount returns the number of issues/PRs in the repo that carry
// the label (identified by name), from the list's total count.
func repoLabelInUseCount(ctx context.Context, client forge.Forgejo, owner, repo, labelName string) (int, error) {
	_, info, err := client.ListRepoIssues(ctx, owner, repo, forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{
		State:       forgejo_sdk.StateType("all"),
		Labels:      []string{labelName},
		ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 1},
	}})
	if err != nil {
		return 0, fmt.Errorf("count label usage: %w", err)
	}
	if info.TotalKnown {
		return info.TotalCount, nil
	}
	return 0, nil
}

// orgLabelInUseCount returns the best-effort count of issues/PRs in the org's
// visible repos that carry the label (identified by name). May under-count for
// repos the token cannot read.
func orgLabelInUseCount(ctx context.Context, client forge.Forgejo, org, labelName string) (int, error) {
	total := 0
	page := 1
	for {
		names, err := client.ListOrgRepoNames(ctx, org, page, 50)
		if err != nil {
			return total, fmt.Errorf("enumerate org repos for label usage count: %w", err)
		}
		if len(names) == 0 {
			break
		}
		for _, name := range names {
			n, _ := repoLabelInUseCount(ctx, client, org, name, labelName)
			total += n
		}
		page++
	}
	return total, nil
}

// ---- Repo label handlers ----

func (h handlers) CreateRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	name, _ := req.GetArguments()["name"].(string)
	colorRaw, _ := req.GetArguments()["color"].(string)
	description, _ := req.GetArguments()["description"].(string)

	color, err := normalizeColor(colorRaw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.CreateLabelOption{
		Name:        name,
		Color:       color,
		Description: description,
	}
	label, err := client.CreateRepoLabel(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create repo label: %w", err))
	}
	return to.TextResult(label)
}

// labelEdit builds the edit options shared by the repo and org edit tools from
// the supplied arguments; the error is the caller's validation failure.
func labelEdit(args map[string]any, tool string) (forgejo_sdk.EditLabelOption, error) {
	nameRaw, nameSet := args["name"].(string)
	colorRaw, colorSet := args["color"].(string)
	descRaw, descSet := args["description"].(string)

	if !nameSet && !colorSet && !descSet {
		return forgejo_sdk.EditLabelOption{}, fmt.Errorf("%s: at least one of name, color, description must be provided", tool)
	}

	opt := forgejo_sdk.EditLabelOption{}
	if nameSet && nameRaw != "" {
		opt.Name = ptr.To(nameRaw)
	}
	if colorSet && colorRaw != "" {
		color, err := normalizeColor(colorRaw)
		if err != nil {
			return forgejo_sdk.EditLabelOption{}, err
		}
		opt.Color = ptr.To(color)
	}
	if descSet {
		opt.Description = ptr.To(descRaw)
	}
	return opt, nil
}

func (h handlers) EditRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	opt, err := labelEdit(req.GetArguments(), "edit_repo_label")
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, err := client.EditRepoLabel(ctx, owner, repo, int64(idF), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit repo label: %w", err))
	}
	return to.TextResult(label)
}

func (h handlers) DeleteRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])
	deleteMode, _ := req.GetArguments()["delete_mode"].(string)
	id := int64(idF)

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	if deleteMode != "force" {
		label, err := client.GetRepoLabel(ctx, owner, repo, id)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get repo label: %w", err))
		}
		count, err := repoLabelInUseCount(ctx, client, owner, repo, label.Name)
		if err != nil {
			return to.ErrorResult(err)
		}
		if count > 0 {
			return to.ErrorResult(fmt.Errorf("label %q is used by %d issue(s)/PR(s); set delete_mode=force to delete anyway", label.Name, count))
		}
	}

	if err := client.DeleteRepoLabel(ctx, owner, repo, id); err != nil {
		if forge.IsNotFound(err) {
			return to.ErrorResult(fmt.Errorf("label %d not found", id))
		}
		return to.ErrorResult(fmt.Errorf("delete repo label: %w", err))
	}
	return to.TextResult(map[string]any{"deleted": true, "id": id})
}

func (h handlers) GetRepoLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoLabelFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, err := client.GetRepoLabel(ctx, owner, repo, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repo label: %w", err))
	}
	return to.TextResult(label)
}

// ---- Org label handlers ----

func (h handlers) CreateOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	name, _ := req.GetArguments()["name"].(string)
	colorRaw, _ := req.GetArguments()["color"].(string)
	description, _ := req.GetArguments()["description"].(string)

	color, err := normalizeColor(colorRaw)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, err := client.CreateOrgLabel(ctx, org, forgejo_sdk.CreateLabelOption{Name: name, Color: color, Description: description})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create org label: %w", err))
	}
	return to.TextResult(label)
}

func (h handlers) EditOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	opt, err := labelEdit(req.GetArguments(), "edit_org_label")
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, err := client.EditOrgLabel(ctx, org, int64(idF), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit org label: %w", err))
	}
	return to.TextResult(label)
}

func (h handlers) DeleteOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])
	deleteMode, _ := req.GetArguments()["delete_mode"].(string)
	id := int64(idF)

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	if deleteMode != "force" {
		label, err := client.GetOrgLabel(ctx, org, id)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get org label: %w", err))
		}

		count, countErr := orgLabelInUseCount(ctx, client, org, label.Name)
		if countErr != nil {
			return to.ErrorResult(fmt.Errorf("label %q in-use count failed (token may lack org-repo access); use delete_mode=force to override: %w", label.Name, countErr))
		}
		if count > 0 {
			return to.ErrorResult(fmt.Errorf("label %q is used by %d issue(s)/PR(s) in visible repos (count excludes inaccessible repos); set delete_mode=force to delete anyway", label.Name, count))
		}
	}

	if err := client.DeleteOrgLabel(ctx, org, id); err != nil {
		return to.ErrorResult(fmt.Errorf("delete org label: %w", err))
	}
	return to.TextResult(map[string]any{"deleted": true, "id": id})
}

func (h handlers) GetOrgLabelFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetOrgLabelFn")
	org, _ := req.GetArguments()["org"].(string)
	idF, _ := to.Float64(req.GetArguments()["id"])

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	label, err := client.GetOrgLabel(ctx, org, int64(idF))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get org label: %w", err))
	}
	return to.TextResult(label)
}
