// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	apppissue "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListIssueDependenciesToolName = "list_issue_dependencies"
	ListIssueDependentsToolName   = "list_issue_dependents"
	AddIssueDependencyToolName    = "add_issue_dependency"
	RemoveIssueDependencyToolName = "remove_issue_dependency"
)

var (
	ListIssueDependenciesTool = mcp.NewTool(
		ListIssueDependenciesToolName,
		mcp.WithDescription("List issues that the given issue depends on. Pagination uses page (1-based) and limit (page size); the response echoes page and limit so callers can fetch the next page. Returns an empty list if the issue has no dependencies. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListIssueDependentsTool = mcp.NewTool(
		ListIssueDependentsToolName,
		mcp.WithDescription("List issues that depend on the given issue. Pagination uses page (1-based) and limit (page size); the response echoes page and limit so callers can fetch the next page. Returns an empty list if no issue depends on it. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	AddIssueDependencyTool = mcp.NewTool(
		AddIssueDependencyToolName,
		mcp.WithDescription("Make one issue depend on another issue. The issue identified by index will depend on depends_on_index. The dependency may live in a different repository (cross-repo): pass depends_on_owner/depends_on_repo, which default to owner/repo. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("depends_on_index", mcp.Required(), mcp.Description("Issue index that the given issue should depend on")),
		mcp.WithString("depends_on_owner", mcp.Description("Owner of the repository the dependency issue lives in (defaults to owner)")),
		mcp.WithString("depends_on_repo", mcp.Description("Repository the dependency issue lives in (defaults to repo)")),
	)

	RemoveIssueDependencyTool = mcp.NewTool(
		RemoveIssueDependencyToolName,
		mcp.WithDescription("Remove a dependency from the given issue. The dependency on dependency_index is removed from the issue identified by index. For a cross-repo dependency, pass dependency_owner/dependency_repo, which default to owner/repo. This tool fails if the repository has disabled issue dependencies."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithNumber("dependency_index", mcp.Required(), mcp.Description("Issue index to remove as a dependency")),
		mcp.WithString("dependency_owner", mcp.Description("Owner of the repository the dependency issue lives in (defaults to owner)")),
		mcp.WithString("dependency_repo", mcp.Description("Repository the dependency issue lives in (defaults to repo)")),
	)
)

func (h handlers) registerDependencyTools(s *server.MCPServer) {
	s.AddTool(ListIssueDependenciesTool, h.ListIssueDependenciesFn)
	s.AddTool(ListIssueDependentsTool, h.ListIssueDependentsFn)
	s.AddTool(AddIssueDependencyTool, h.AddIssueDependencyFn)
	s.AddTool(RemoveIssueDependencyTool, h.RemoveIssueDependencyFn)
}

type listDependenciesArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// The numbers and the cross-repository names below are kept raw (any), not
// typed: the handlers have always refused a malformed one with an error naming
// it, which a typed field would silently zero under decode.Lax. The schema tags
// tell mcptest.ArgsMatchSchema what the tool's schema says each one is.
type addDependencyArgs struct {
	Owner          string `json:"owner" required:"true"`
	Repo           string `json:"repo" required:"true"`
	Index          any    `json:"index" required:"true" schema:"number"`
	DependsOnIndex any    `json:"depends_on_index" required:"true" schema:"number"`
	DependsOnOwner any    `json:"depends_on_owner" schema:"string"`
	DependsOnRepo  any    `json:"depends_on_repo" schema:"string"`
}

type removeDependencyArgs struct {
	Owner           string `json:"owner" required:"true"`
	Repo            string `json:"repo" required:"true"`
	Index           any    `json:"index" required:"true" schema:"number"`
	DependencyIndex any    `json:"dependency_index" required:"true" schema:"number"`
	DependencyOwner any    `json:"dependency_owner" schema:"string"`
	DependencyRepo  any    `json:"dependency_repo" schema:"string"`
}

func (h handlers) ListIssueDependenciesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListIssueDependenciesFn", func(ctx context.Context, s *apppissue.Service, a listDependenciesArgs) (*mcp.CallToolResult, error) {
		return render(s.Dependencies(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}),
			apppissue.Paging{Page: intOr(a.Page, 1), Limit: intOr(a.Limit, 20)}))
	})
}

func (h handlers) ListIssueDependentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListIssueDependentsFn", func(ctx context.Context, s *apppissue.Service, a listDependenciesArgs) (*mcp.CallToolResult, error) {
		return render(s.Dependents(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}),
			apppissue.Paging{Page: intOr(a.Page, 1), Limit: intOr(a.Limit, 20)}))
	})
}

func (h handlers) AddIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "AddIssueDependencyFn", func(ctx context.Context, s *apppissue.Service, a addDependencyArgs) (*mcp.CallToolResult, error) {
		index, err := numberArg(a.Index, "index")
		if err != nil {
			return fail(err)
		}
		dependsOn, err := numberArg(a.DependsOnIndex, "depends_on_index")
		if err != nil {
			return fail(err)
		}
		depOwner, depRepo, err := crossRepoArgs(a.DependsOnOwner, "depends_on_owner", a.DependsOnRepo, "depends_on_repo", a.Owner, a.Repo)
		if err != nil {
			return fail(err)
		}
		if err := s.AddDependency(ctx,
			apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: int64(index)},
			apppissue.Target{Owner: depOwner, Repo: depRepo, Index: int64(dependsOn)},
		); err != nil {
			return fail(err)
		}
		return to.TextResult(fmt.Sprintf("Issue #%d now depends on %s/%s#%d", int64(index), depOwner, depRepo, int64(dependsOn)))
	})
}

func (h handlers) RemoveIssueDependencyFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "RemoveIssueDependencyFn", func(ctx context.Context, s *apppissue.Service, a removeDependencyArgs) (*mcp.CallToolResult, error) {
		index, err := numberArg(a.Index, "index")
		if err != nil {
			return fail(err)
		}
		dependencyIndex, err := numberArg(a.DependencyIndex, "dependency_index")
		if err != nil {
			return fail(err)
		}
		depOwner, depRepo, err := crossRepoArgs(a.DependencyOwner, "dependency_owner", a.DependencyRepo, "dependency_repo", a.Owner, a.Repo)
		if err != nil {
			return fail(err)
		}
		if err := s.RemoveDependency(ctx,
			apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: int64(index)},
			apppissue.Target{Owner: depOwner, Repo: depRepo, Index: int64(dependencyIndex)},
		); err != nil {
			return fail(err)
		}
		return to.TextResult(fmt.Sprintf("Removed dependency on %s/%s#%d from issue #%d", depOwner, depRepo, int64(dependencyIndex), int64(index)))
	})
}

// numberArg reads a required number argument. A missing or malformed one is an
// error that names it.
func numberArg(raw any, key string) (float64, error) {
	v, err := to.Float64(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

// crossRepoArgs resolves the optional owner/repo pair naming the repository a
// dependency issue lives in, defaulting to the target issue's own repository.
//
// Absent and malformed are kept apart, the way pkg/to separates Float64 from
// Float64Ok: an omitted, nil or empty argument means "the target's own repo",
// but a value of the wrong type is an error. Collapsing the two would write a
// SAME-repo dependency for a caller that asked for a different one and report
// success, which is worse than refusing.
func crossRepoArgs(ownerRaw any, ownerKey string, repoRaw any, repoKey string, defaultOwner, defaultRepo string) (string, string, error) {
	depOwner, err := optionalString(ownerRaw, ownerKey, defaultOwner)
	if err != nil {
		return "", "", err
	}
	depRepo, err := optionalString(repoRaw, repoKey, defaultRepo)
	if err != nil {
		return "", "", err
	}
	return depOwner, depRepo, nil
}

// optionalString returns raw as a string, fallback when it is absent, nil or
// empty, and an error naming the argument when it is present but not a string.
func optionalString(raw any, key, fallback string) (string, error) {
	if raw == nil {
		return fallback, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected a string, got %T", key, raw)
	}
	if s == "" {
		return fallback, nil
	}
	return s, nil
}
