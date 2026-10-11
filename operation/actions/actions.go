// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

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

// handlers serve the Actions tools from the Forgejo port.
type handlers struct{ forge forge.Factory }

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

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	workflow, ok := req.GetArguments()["workflow"].(string)
	if !ok || workflow == "" {
		return to.ErrorResult(errors.New("workflow is required"))
	}
	ref, ok := req.GetArguments()["ref"].(string)
	if !ok || ref == "" {
		return to.ErrorResult(errors.New("ref is required"))
	}

	// Parse optional inputs JSON
	var inputs map[string]string
	if inputsJSON, ok := req.GetArguments()["inputs"].(string); ok && inputsJSON != "" {
		if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
			return to.ErrorResult(fmt.Errorf(`invalid inputs JSON: %w (expected format: {"key": "value"})`, err))
		}
	}

	opt := forgejo_sdk.DispatchWorkflowOption{
		Ref:    ref,
		Inputs: inputs,
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := port.DispatchWorkflow(ctx, owner, repo, workflow, opt); err != nil {
		return to.ErrorResult(fmt.Errorf("failed to dispatch workflow: %w", err))
	}

	result := fmt.Sprintf("Workflow dispatched successfully!\n  Workflow: %s\n  Ref: %s\n  URL: %s",
		workflow, ref, port.ActionsURL(owner, repo))

	return to.TextResult(result)
}
