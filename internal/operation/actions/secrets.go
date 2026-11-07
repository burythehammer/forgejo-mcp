// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"math"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
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
		annotate.Read(),
		mcp.WithDescription("List action secrets for a repository (names only, values are never exposed by the Forgejo API). Returns a response envelope {owner, repo, secrets, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateRepoActionSecretTool = mcp.NewTool(
		CreateOrUpdateRepoActionSecretToolName,
		annotate.Write(),
		mcp.WithDescription("Create or update an action secret for a repository. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteRepoActionSecretTool = mcp.NewTool(
		DeleteRepoActionSecretToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete an action secret from a repository."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)

	ListOrgActionSecretsTool = mcp.NewTool(
		ListOrgActionSecretsToolName,
		annotate.Read(),
		mcp.WithDescription("List action secrets for an organization (names only, values are never exposed by the Forgejo API). Returns a response envelope {org, secrets, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionSecretsLimit), mcp.Min(1), mcp.Max(maxActionSecretsLimit)),
	)

	CreateOrUpdateOrgActionSecretTool = mcp.NewTool(
		CreateOrUpdateOrgActionSecretToolName,
		annotate.Write(),
		mcp.WithDescription("Create or update an action secret for an organization. Overwrites any existing secret with the same name; the value is never echoed back."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
		mcp.WithString("data", mcp.Required(), mcp.Description(params.SecretData)),
	)

	DeleteOrgActionSecretTool = mcp.NewTool(
		DeleteOrgActionSecretToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete an action secret from an organization."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("secret_name", mcp.Required(), mcp.Description(params.SecretName)),
	)
)

// secretPaging reads the page and limit of a secret list. Repository and
// organization lists take the same bounds.
func secretPaging(page, limit any) (app.Paging, error) {
	p, err := numarg.IntDefault("page", page, 1, numarg.Bounds{Min: int64(1), Max: int64(math.MaxInt32)})
	if err != nil {
		return app.Paging{}, err
	}
	l, err := numarg.IntDefault("limit", limit, 0, numarg.Bounds{Min: int64(1), Max: int64(maxActionSecretsLimit)})
	if err != nil {
		return app.Paging{}, err
	}
	return app.Paging{Page: p, Limit: l}, nil
}

// Validate checks owner and repo, then the paging arguments, before the
// credential lookup.
func (a repoSecretsListArgs) Validate() error {
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := secretPaging(a.Page, a.Limit)
	return err
}

func listRepoActionSecrets(ctx context.Context, svc *app.Service, a repoSecretsListArgs) (*mcp.CallToolResult, error) {
	paging, err := secretPaging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	out, err := svc.ListRepoSecrets(ctx, a.Owner, a.Repo, paging)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(repoSecretPage(a.Owner, a.Repo, out))
}

// Validate checks owner, repo, secret_name and data, in that order, before the
// credential lookup.
func (a repoSecretSetArgs) Validate() error {
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return err
	}
	return required("data", a.Data)
}

func createOrUpdateRepoActionSecret(ctx context.Context, svc *app.Service, a repoSecretSetArgs) (*mcp.CallToolResult, error) {
	if err := svc.SetRepoSecret(ctx, a.Owner, a.Repo, a.SecretName, a.Data); err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Owner:      a.Owner,
		Repo:       a.Repo,
		SecretName: a.SecretName,
	})
}

// Validate checks owner, repo and secret_name, in that order, before the
// credential lookup.
func (a repoSecretNameArgs) Validate() error {
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	return required("secret_name", a.SecretName)
}

func deleteRepoActionSecret(ctx context.Context, svc *app.Service, a repoSecretNameArgs) (*mcp.CallToolResult, error) {
	if err := svc.DeleteRepoSecret(ctx, a.Owner, a.Repo, a.SecretName); err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Owner:      a.Owner,
		Repo:       a.Repo,
		SecretName: a.SecretName,
	})
}

// Validate checks org, then the paging arguments, before the credential lookup.
func (a orgSecretsListArgs) Validate() error {
	if err := required("org", a.Org); err != nil {
		return err
	}
	_, err := secretPaging(a.Page, a.Limit)
	return err
}

func listOrgActionSecrets(ctx context.Context, svc *app.Service, a orgSecretsListArgs) (*mcp.CallToolResult, error) {
	paging, err := secretPaging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	out, err := svc.ListOrgSecrets(ctx, a.Org, paging)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(orgSecretPage(a.Org, out))
}

// Validate checks org, secret_name and data, in that order, before the
// credential lookup.
func (a orgSecretSetArgs) Validate() error {
	if err := required("org", a.Org); err != nil {
		return err
	}
	if err := required("secret_name", a.SecretName); err != nil {
		return err
	}
	return required("data", a.Data)
}

func createOrUpdateOrgActionSecret(ctx context.Context, svc *app.Service, a orgSecretSetArgs) (*mcp.CallToolResult, error) {
	if err := svc.SetOrgSecret(ctx, a.Org, a.SecretName, a.Data); err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "created_or_updated",
		Org:        a.Org,
		SecretName: a.SecretName,
	})
}

// Validate checks org and secret_name, in that order, before the credential
// lookup.
func (a orgSecretNameArgs) Validate() error {
	if err := required("org", a.Org); err != nil {
		return err
	}
	return required("secret_name", a.SecretName)
}

func deleteOrgActionSecret(ctx context.Context, svc *app.Service, a orgSecretNameArgs) (*mcp.CallToolResult, error) {
	if err := svc.DeleteOrgSecret(ctx, a.Org, a.SecretName); err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(actionSecretMutationResult{
		Status:     "deleted",
		Org:        a.Org,
		SecretName: a.SecretName,
	})
}
