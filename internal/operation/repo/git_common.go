// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repogitadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// gitService returns the branch, commit, contents and file use cases bound to
// this request's credential.
func (h handlers) gitService(ctx context.Context) (*apprepo.GitService, error) {
	f, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apprepo.NewGitService(repogitadapter.New(f)), nil
}

// need is the value of a required argument. It reports the error the tools have
// always given when the argument is missing or not a string.
func need(name string, v *string) (string, error) {
	if v == nil {
		return "", fmt.Errorf("%s is required", name)
	}
	return *v, nil
}

// gitText renders a use case's result, or its error, as the tools' JSON text.
func gitText(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(v)
}
