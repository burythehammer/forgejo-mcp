// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"fmt"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/actionsadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DispatchWorkflowToolName = "dispatch_workflow"
)

var DispatchWorkflowTool = mcp.NewTool(
	DispatchWorkflowToolName,
	mcp.WithDescription("Trigger a workflow run"),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("workflow", mcp.Required(), mcp.Description(params.Workflow)),
	mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
	mcp.WithString("inputs", mcp.Description(params.Inputs)),
)

// handlers serve the Actions tools. They hold the factory rather than a port
// so each call is bound to the credential of its own request. Each handler is
// the MCP adapter of one use case in internal/app/actions: check the arguments, run the
// use case, render the result.
type handlers struct{ forge forge.Factory }

// useCases returns the Actions use cases over the request's Forgejo port. It
// is called after the arguments are checked, as the handlers always did.
func useCases(ctx context.Context, h handlers) (*app.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return app.New(actionsadapter.New(port)), nil
}

// dispatchArgs are the arguments of dispatch_workflow.
type dispatchArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	Workflow string `json:"workflow" required:"true"`
	Ref      string `json:"ref" required:"true"`
	Inputs   string `json:"inputs"`
}

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(DispatchWorkflowTool, h.DispatchWorkflowFn)
	s.AddTool(ListWorkflowRunsTool, h.ListWorkflowRunsFn)
	s.AddTool(GetWorkflowRunTool, h.GetWorkflowRunFn)
	s.AddTool(ListActionRunJobsTool, h.ListActionRunJobsFn)
	s.AddTool(GetActionJobLogsTool, h.GetActionJobLogsFn)
	s.AddTool(CancelWorkflowRunTool, h.CancelWorkflowRunFn)
	s.AddTool(DeleteWorkflowRunTool, h.DeleteWorkflowRunFn)
	s.AddTool(ListActionRunArtifactsTool, h.ListActionRunArtifactsFn)
	s.AddTool(GetActionArtifactTool, h.GetActionArtifactFn)
	s.AddTool(ListRepoActionSecretsTool, h.ListRepoActionSecretsFn)
	s.AddTool(CreateOrUpdateRepoActionSecretTool, h.CreateOrUpdateRepoActionSecretFn)
	s.AddTool(DeleteRepoActionSecretTool, h.DeleteRepoActionSecretFn)
	s.AddTool(ListOrgActionSecretsTool, h.ListOrgActionSecretsFn)
	s.AddTool(CreateOrUpdateOrgActionSecretTool, h.CreateOrUpdateOrgActionSecretFn)
	s.AddTool(DeleteOrgActionSecretTool, h.DeleteOrgActionSecretFn)
}

func (h handlers) DispatchWorkflowFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DispatchWorkflowFn")

	var a dispatchArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("workflow", a.Workflow); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("ref", a.Ref); err != nil {
		return to.ErrorResult(err)
	}

	// Parse optional inputs JSON
	var inputs map[string]string
	if a.Inputs != "" {
		if err := json.Unmarshal([]byte(a.Inputs), &inputs); err != nil {
			return to.ErrorResult(fmt.Errorf(`invalid inputs JSON: %w (expected format: {"key": "value"})`, err))
		}
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	url, err := svc.Dispatch(ctx, a.Owner, a.Repo, a.Workflow, app.DispatchInput{Ref: a.Ref, Inputs: inputs})
	if err != nil {
		return to.ErrorResult(err)
	}

	result := fmt.Sprintf("Workflow dispatched successfully!\n  Workflow: %s\n  Ref: %s\n  URL: %s",
		a.Workflow, a.Ref, url)

	return to.TextResult(result)
}
