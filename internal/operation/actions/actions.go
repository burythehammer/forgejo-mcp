// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"fmt"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/actionsadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DispatchWorkflowToolName = "dispatch_workflow"
)

var DispatchWorkflowTool = mcp.NewTool(
	DispatchWorkflowToolName,
	annotate.Write(),
	mcp.WithDescription("Trigger a workflow run"),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("workflow", mcp.Required(), mcp.Description(params.Workflow)),
	mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
	mcp.WithString("inputs", mcp.Description(params.Inputs)),
)

// handlers hold the factory rather than a port so each call is bound to the
// credential of its own request.
type handlers struct{ forge forge.Factory }

// service is the Binder for every Actions tool: it looks up the request's
// credential and binds the Actions use cases to it. The request pipeline calls
// it only after the arguments are valid.
func (h handlers) service(ctx context.Context) (*app.Service, error) {
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

// Validate checks owner, repo, workflow and ref, then the inputs JSON, in the
// order the tool always has. A bad request is refused before the credential
// lookup.
func (a dispatchArgs) Validate() error {
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if err := required("workflow", a.Workflow); err != nil {
		return err
	}
	if err := required("ref", a.Ref); err != nil {
		return err
	}
	_, err := a.inputs()
	return err
}

// inputs parses the optional inputs JSON. An absent inputs argument is nil.
func (a dispatchArgs) inputs() (map[string]string, error) {
	if a.Inputs == "" {
		return nil, nil
	}
	var inputs map[string]string
	if err := json.Unmarshal([]byte(a.Inputs), &inputs); err != nil {
		return nil, fmt.Errorf(`invalid inputs JSON: %w (expected format: {"key": "value"})`, err)
	}
	return inputs, nil
}

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(DispatchWorkflowTool, pipeline.Handle(h.service, dispatchWorkflow))
	s.AddTool(ListWorkflowRunsTool, pipeline.Handle(h.service, listWorkflowRuns))
	s.AddTool(GetWorkflowRunTool, pipeline.Handle(h.service, getWorkflowRun))
	s.AddTool(ListActionRunJobsTool, pipeline.Handle(h.service, listActionRunJobs))
	s.AddTool(GetActionJobLogsTool, pipeline.Handle(h.service, getActionJobLogs))
	s.AddTool(CancelWorkflowRunTool, pipeline.Handle(h.service, cancelWorkflowRun))
	s.AddTool(DeleteWorkflowRunTool, pipeline.Handle(h.service, deleteWorkflowRun))
	s.AddTool(ListActionRunArtifactsTool, pipeline.Handle(h.service, listActionRunArtifacts))
	s.AddTool(GetActionArtifactTool, pipeline.Handle(h.service, getActionArtifact))
	s.AddTool(ListRepoActionSecretsTool, pipeline.Handle(h.service, listRepoActionSecrets))
	s.AddTool(CreateOrUpdateRepoActionSecretTool, pipeline.Handle(h.service, createOrUpdateRepoActionSecret))
	s.AddTool(DeleteRepoActionSecretTool, pipeline.Handle(h.service, deleteRepoActionSecret))
	s.AddTool(ListOrgActionSecretsTool, pipeline.Handle(h.service, listOrgActionSecrets))
	s.AddTool(CreateOrUpdateOrgActionSecretTool, pipeline.Handle(h.service, createOrUpdateOrgActionSecret))
	s.AddTool(DeleteOrgActionSecretTool, pipeline.Handle(h.service, deleteOrgActionSecret))
}

func dispatchWorkflow(ctx context.Context, svc *app.Service, a dispatchArgs) (*mcp.CallToolResult, error) {
	inputs, err := a.inputs()
	if err != nil {
		return mcpresult.Fail(err)
	}
	url, err := svc.Dispatch(ctx, a.Owner, a.Repo, a.Workflow, app.DispatchInput{Ref: a.Ref, Inputs: inputs})
	if err != nil {
		return mcpresult.Fail(err)
	}

	result := fmt.Sprintf("Workflow dispatched successfully!\n  Workflow: %s\n  Ref: %s\n  URL: %s",
		a.Workflow, a.Ref, url)

	return to.TextResult(result)
}
