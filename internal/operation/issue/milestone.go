// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateMilestoneToolName = "create_milestone"
	EditMilestoneToolName   = "edit_milestone"
	GetMilestoneToolName    = "get_milestone"
)

var (
	CreateMilestoneTool = mcp.NewTool(
		CreateMilestoneToolName,
		annotate.Write(),
		mcp.WithDescription("Create an open repository milestone. Returns the milestone as the server stored it: id, title, description, state, due_date, open_issues, closed_issues."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description("Milestone title")),
		mcp.WithString("description", mcp.Description("Milestone description")),
		mcp.WithString("due_date", mcp.Description("Due date as an RFC3339 timestamp, e.g. 2026-12-01T00:00:00Z"), lean.Keep("RFC3339 time, e.g. 2026-12-01T00:00:00Z")),
	)

	EditMilestoneTool = mcp.NewTool(
		EditMilestoneToolName,
		annotate.Write(),
		mcp.WithDescription("Edit a repository milestone. Only the fields given change. Set state to closed to close a milestone, or open to reopen it. Returns the milestone as the server stored it. A due date cannot be cleared."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Milestone ID (a positive integer)")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("state", mcp.Description("open or closed"), lean.Keep("open or closed")),
		mcp.WithString("due_date", mcp.Description("New due date as an RFC3339 timestamp"), lean.Keep("RFC3339 time")),
	)

	GetMilestoneTool = mcp.NewTool(
		GetMilestoneToolName,
		annotate.Read(),
		mcp.WithDescription("Get one repository milestone by ID. Returns the milestone as the server stored it. A missing ID is an error, not an empty result."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Milestone ID (a positive integer)")),
	)
)

func (h handlers) registerMilestoneTools(s *server.MCPServer) {
	s.AddTool(CreateMilestoneTool, h.CreateMilestoneFn)
	s.AddTool(EditMilestoneTool, h.EditMilestoneFn)
	s.AddTool(GetMilestoneTool, h.GetMilestoneFn)
}

type createMilestoneArgs struct {
	Owner       string `json:"owner" required:"true"`
	Repo        string `json:"repo" required:"true"`
	Title       string `json:"title" required:"true"`
	Description string `json:"description"`
	DueDate     string `json:"due_date"`
}

type editMilestoneArgs struct {
	Owner       string     `json:"owner" required:"true"`
	Repo        string     `json:"repo" required:"true"`
	ID          decode.Arg `json:"id" required:"true" schema:"number"`
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	State       *string    `json:"state"`
	DueDate     *string    `json:"due_date"`
}

type getMilestoneArgs struct {
	Owner string     `json:"owner" required:"true"`
	Repo  string     `json:"repo" required:"true"`
	ID    decode.Arg `json:"id" required:"true" schema:"number"`
}

// milestoneOut is the milestone a write or get returns: the stored values,
// with the due date as RFC3339 or null.
type milestoneOut struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	State        string  `json:"state"`
	DueDate      *string `json:"due_date"`
	OpenIssues   int     `json:"open_issues"`
	ClosedIssues int     `json:"closed_issues"`
}

func milestoneOf(m *model.Milestone) milestoneOut {
	out := milestoneOut{
		ID: m.ID, Title: m.Title, Description: m.Description, State: m.State,
		OpenIssues: m.OpenIssues, ClosedIssues: m.ClosedIssues,
	}
	if m.Deadline != nil {
		due := m.Deadline.UTC().Format(time.RFC3339)
		out.DueDate = &due
	}
	return out
}

// milestoneID reads the id argument: a whole number from 1 up. Zero, negative,
// fractional and non-numeric values are rejected, never truncated.
func milestoneID(a decode.Arg) (int64, error) {
	id, err := a.Int("id", 0, 1, math.MaxInt)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("id must be an integer between 1 and %d", math.MaxInt)
	}
	return int64(id), nil
}

// dueDate reads an optional RFC3339 due date. An empty string is no date.
func dueDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, errors.New("due_date must be an RFC3339 timestamp, e.g. 2026-12-01T00:00:00Z")
	}
	return &t, nil
}

func (h handlers) CreateMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a createMilestoneArgs) (*mcp.CallToolResult, error) {
		due, err := dueDate(&a.DueDate)
		if err != nil {
			return mcpresult.Fail(err)
		}
		m, err := s.CreateMilestone(ctx, a.Owner, a.Repo, apppissue.MilestoneNew{Title: a.Title, Description: a.Description, Due: due})
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(milestoneOf(m), nil)
	})(ctx, req)
}

func (h handlers) EditMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a editMilestoneArgs) (*mcp.CallToolResult, error) {
		id, err := milestoneID(a.ID)
		if err != nil {
			return mcpresult.Fail(err)
		}
		due, err := dueDate(a.DueDate)
		if err != nil {
			return mcpresult.Fail(err)
		}
		edit := apppissue.MilestoneEdit{Tool: EditMilestoneToolName, Title: a.Title, Description: a.Description, State: a.State, Due: due}
		m, err := s.EditMilestone(ctx, a.Owner, a.Repo, id, edit)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(milestoneOf(m), nil)
	})(ctx, req)
}

func (h handlers) GetMilestoneFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a getMilestoneArgs) (*mcp.CallToolResult, error) {
		id, err := milestoneID(a.ID)
		if err != nil {
			return mcpresult.Fail(err)
		}
		m, err := s.GetMilestone(ctx, a.Owner, a.Repo, id)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(milestoneOf(m))
	})(ctx, req)
}
