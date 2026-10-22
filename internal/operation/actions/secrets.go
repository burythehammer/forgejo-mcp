// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"math"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoActionSecretsToolName          = "list_repo_action_secrets"
	CreateOrUpdateRepoActionSecretToolName = "create_or_update_repo_action_secret"
	DeleteRepoActionSecretToolName         = "delete_repo_action_secret"
	ListOrgActionSecretsToolName           = "list_org_action_secrets"
	CreateOrUpdateOrgActionSecretToolName  = "create_or_update_org_action_secret"
	DeleteOrgActionSecretToolName          = "delete_org_action_secret"

	defaultActionSecretsLimit = 30
	maxActionSecretsLimit     = 50
)

// actionSecretMutationResult is the output of the create and delete tools. It
// carries no value: Forgejo never echoes a secret back.
type actionSecretMutationResult struct {
	Status     string `json:"status"`
	Owner      string `json:"owner,omitempty"`
	Repo       string `json:"repo,omitempty"`
	Org        string `json:"org,omitempty"`
	SecretName string `json:"secret_name"`
}

// The argument structs of the secret tools. A value is never logged or echoed.
type (
	repoSecretsListArgs struct {
		Owner string `json:"owner" required:"true"`
		Repo  string `json:"repo" required:"true"`
		Page  any    `json:"page" schema:"number"`
		Limit any    `json:"limit" schema:"number"`
	}
	repoSecretSetArgs struct {
		Owner      string `json:"owner" required:"true"`
		Repo       string `json:"repo" required:"true"`
		SecretName string `json:"secret_name" required:"true"`
		Data       string `json:"data" required:"true"`
	}
	repoSecretNameArgs struct {
		Owner      string `json:"owner" required:"true"`
		Repo       string `json:"repo" required:"true"`
		SecretName string `json:"secret_name" required:"true"`
	}
	orgSecretsListArgs struct {
		Org   string `json:"org" required:"true"`
		Page  any    `json:"page" schema:"number"`
		Limit any    `json:"limit" schema:"number"`
	}
	orgSecretSetArgs struct {
		Org        string `json:"org" required:"true"`
		SecretName string `json:"secret_name" required:"true"`
		Data       string `json:"data" required:"true"`
	}
	orgSecretNameArgs struct {
		Org        string `json:"org" required:"true"`
		SecretName string `json:"secret_name" required:"true"`
	}
)

var (
	ListRepoActionSecretsTool = mcp.NewTool(
		ListRepoActionSecretsToolName,
		mcp.WithDescription("List action secrets for a repository (names only, values are never exposed by the Forgejo API). Returns a response envelope {owner, repo, secrets, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateRepoActionSecretTool = mcp.NewTool(
		CreateOrUpdateRepoActionSecretToolName,
		mcp.WithDescription("Create or update an action secret for a repository. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteRepoActionSecretTool = mcp.NewTool(
		DeleteRepoActionSecretToolName,
		mcp.WithDescription("Delete an action secret from a repository."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)

	ListOrgActionSecretsTool = mcp.NewTool(
		ListOrgActionSecretsToolName,
		mcp.WithDescription("List action secrets for an organization (names only, values are never exposed by the Forgejo API). Returns a response envelope {org, secrets, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateOrgActionSecretTool = mcp.NewTool(
		CreateOrUpdateOrgActionSecretToolName,
		mcp.WithDescription("Create or update an action secret for an organization. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteOrgActionSecretTool = mcp.NewTool(
		DeleteOrgActionSecretToolName,
		mcp.WithDescription("Delete an action secret from an organization."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)
)

func (h handlers) ListRepoActionSecretsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoActionSecretsFn")

	var a repoSecretsListArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntArg(a.Page, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntArg(a.Limit, "limit", 0, 1, maxActionSecretsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := svc.ListRepoSecrets(ctx, a.Owner, a.Repo, app.Paging{Page: page, Limit: limit})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(repoSecretPage(a.Owner, a.Repo, out))
}

func (h handlers) CreateOrUpdateRepoActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrUpdateRepoActionSecretFn")

	var a repoSecretSetArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("data", a.Data); err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.SetRepoSecret(ctx, a.Owner, a.Repo, a.SecretName, a.Data); err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Owner:      a.Owner,
		Repo:       a.Repo,
		SecretName: a.SecretName,
	})
}

func (h handlers) DeleteRepoActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteRepoActionSecretFn")

	var a repoSecretNameArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteRepoSecret(ctx, a.Owner, a.Repo, a.SecretName); err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Owner:      a.Owner,
		Repo:       a.Repo,
		SecretName: a.SecretName,
	})
}

func (h handlers) ListOrgActionSecretsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgActionSecretsFn")

	var a orgSecretsListArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("org", a.Org); err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntArg(a.Page, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntArg(a.Limit, "limit", 0, 1, maxActionSecretsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := svc.ListOrgSecrets(ctx, a.Org, app.Paging{Page: page, Limit: limit})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(orgSecretPage(a.Org, out))
}

func (h handlers) CreateOrUpdateOrgActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrUpdateOrgActionSecretFn")

	var a orgSecretSetArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("org", a.Org); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("data", a.Data); err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.SetOrgSecret(ctx, a.Org, a.SecretName, a.Data); err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Org:        a.Org,
		SecretName: a.SecretName,
	})
}

func (h handlers) DeleteOrgActionSecretFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgActionSecretFn")

	var a orgSecretNameArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("org", a.Org); err != nil {
		return to.ErrorResult(err)
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteOrgSecret(ctx, a.Org, a.SecretName); err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Org:        a.Org,
		SecretName: a.SecretName,
	})
}
